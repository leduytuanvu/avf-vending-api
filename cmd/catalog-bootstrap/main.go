// Command catalog-bootstrap imports production catalog from reviewed manifest JSON.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/avf/avf-vending-api/internal/catalogbootstrap"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	var (
		manifestPath      = flag.String("manifest", "", "path to catalog-import-final.json or enriched manifest")
		evidenceDir       = flag.String("evidence-dir", ".catalog-bootstrap-evidence", "evidence output directory")
		environment       = flag.String("environment", "", "expected APP_ENV (e.g. production)")
		validateOnly      = flag.Bool("validate-only", false, "validate manifest only")
		dryRun            = flag.Bool("dry-run", false, "simulate import without writes")
		uploadImages      = flag.Bool("upload-images", false, "upload source images to Cloudinary")
		importTaxonomy    = flag.Bool("import-taxonomy", false, "import categories, brands, tags")
		importProducts    = flag.Bool("import-products", false, "import products with media")
		importPrices      = flag.Bool("import-prices", false, "import price book items")
		verifyOnly        = flag.Bool("verify-only", false, "run database verification only")
		preflightOnly     = flag.Bool("preflight-only", false, "record DB and Cloudinary fingerprint")
		resume            = flag.Bool("resume", false, "resume from checkpoint")
		confirmProduction = flag.String("confirm-production", "", "required for production writes: CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION")
	)
	flag.Parse()

	if *manifestPath == "" {
		fmt.Fprintln(os.Stderr, "catalog-bootstrap: --manifest is required")
		os.Exit(2)
	}

	manifest, err := catalogbootstrap.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: manifest: %v\n", err)
		os.Exit(1)
	}
	issues := catalogbootstrap.ValidateManifest(manifest)
	if len(issues) > 0 {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: validation failed:\n- %s\n", strings.Join(issues, "\n- "))
		os.Exit(1)
	}
	if *validateOnly {
		fmt.Printf("manifest valid: %d products\n", len(manifest.Products))
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: config: %v\n", err)
		os.Exit(2)
	}
	if *environment != "" && string(cfg.AppEnv) != strings.TrimSpace(*environment) {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: APP_ENV=%s does not match --environment=%s\n", cfg.AppEnv, *environment)
		os.Exit(2)
	}

	evidence, err := catalogbootstrap.NewEvidenceWriter(*evidenceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: evidence: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.Postgres.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: db connect: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if *preflightOnly {
		if err := catalogbootstrap.PreflightProduction(ctx, cfg, pool, evidence); err != nil {
			fmt.Fprintf(os.Stderr, "catalog-bootstrap: preflight: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("preflight recorded")
		return
	}

	lookup := catalogbootstrap.NewLookup(pool)
	if *verifyOnly {
		metrics, err := catalogbootstrap.VerifyDatabase(ctx, lookup, len(manifest.Products))
		if err != nil {
			fmt.Fprintf(os.Stderr, "catalog-bootstrap: verify: %v\n", err)
			os.Exit(1)
		}
		_ = evidence.WriteJSON("15-database-final-audit.json", metrics)
		fmt.Printf("verify complete: %+v\n", metrics)
		return
	}

	writes := *uploadImages || *importTaxonomy || *importProducts || *importPrices
	if writes && !*dryRun {
		if cfg.AppEnv == config.AppEnvProduction {
			if *confirmProduction != "CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION" {
				fmt.Fprintln(os.Stderr, "catalog-bootstrap: production import requires --confirm-production=CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION")
				os.Exit(2)
			}
		}
	}

	runner, err := catalogbootstrap.NewRunner(ctx, cfg, pool, manifest, *evidenceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: runner: %v\n", err)
		os.Exit(1)
	}

	// Default full import when no phase flags set.
	if !*dryRun && !*uploadImages && !*importTaxonomy && !*importProducts && !*importPrices {
		*uploadImages = true
		*importTaxonomy = true
		*importProducts = true
		*importPrices = true
	}
	if *dryRun {
		*importTaxonomy = true
	}

	err = runner.Run(ctx, catalogbootstrap.Options{
		DryRun:            *dryRun,
		UploadImages:      *uploadImages,
		ImportTaxonomy:    *importTaxonomy,
		ImportProducts:    *importProducts,
		ImportPrices:      *importPrices,
		Resume:            *resume,
		ConfirmProduction: *confirmProduction == "CONFIRM-CATALOG-BOOTSTRAP-PRODUCTION",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "catalog-bootstrap: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("catalog-bootstrap complete")
}
