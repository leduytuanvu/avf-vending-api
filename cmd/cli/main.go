package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/version"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	validate := flag.Bool("validate-config", false, "load and validate configuration from the environment, then exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	classifyLayoutDimensions := flag.Bool("classify-layout-dimensions", false, "run legacy layout dimension classification migration audit")
	flag.Parse()

	if *showVersion {
		if version.Commit != "" {
			fmt.Printf("%s %s (%s)\n", version.Name, version.Version, version.Commit)
		} else {
			fmt.Printf("%s %s\n", version.Name, version.Version)
		}
		os.Exit(0)
	}

	if *validate {
		if _, err := config.Load(); err != nil {
			fmt.Fprintf(os.Stderr, "config invalid: %v\n", err)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stdout, "config ok")
		os.Exit(0)
	}

	if *classifyLayoutDimensions {
		if err := runClassifyLayoutDimensions(); err != nil {
			fmt.Fprintf(os.Stderr, "classify-layout-dimensions failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "classify-layout-dimensions ok")
		os.Exit(0)
	}

	fmt.Fprintln(os.Stderr, "usage: cli -validate-config | -version | -classify-layout-dimensions")
	os.Exit(1)
}

func runClassifyLayoutDimensions() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(context.Background(), cfg.Postgres.URL)
	if err != nil {
		return err
	}
	defer pool.Close()
	svc := &layoutassignment.Service{Pool: pool}
	return svc.RunLayoutDimensionMigrationClassify(context.Background())
}
