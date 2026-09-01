// Command bootstrap-admin inserts the initial platform_admin account after a production reset.
// Credentials are read from BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD at runtime only.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	appauth "github.com/avf/avf-vending-api/internal/app/auth"
	"github.com/avf/avf-vending-api/internal/config"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	_ = godotenv.Load()
	force := flag.Bool("force", false, "insert even when other auth accounts already exist")
	flag.Parse()

	username := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_USERNAME"))
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" || password == "" {
		fmt.Fprintln(os.Stderr, "bootstrap-admin: BOOTSTRAP_ADMIN_USERNAME and BOOTSTRAP_ADMIN_PASSWORD must be set")
		os.Exit(2)
	}

	normalized, err := appauth.NormalizeUsernameForBootstrap(username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: invalid username: %v\n", err)
		os.Exit(2)
	}

	// Bootstrap-only: deliberately bypasses validatePassword so a short initial password
	// (e.g. operator-supplied at reset time) can be rotated immediately via change-password,
	// which enforces the global min-length policy on all interactive paths.
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: hash failed: %v\n", err)
		os.Exit(1)
	}
	password = ""

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: config: %v\n", err)
		os.Exit(2)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.Postgres.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	if !*force {
		var n int
		if err := conn.QueryRow(ctx, `SELECT count(*)::int FROM platform_auth_accounts`).Scan(&n); err != nil {
			fmt.Fprintf(os.Stderr, "bootstrap-admin: account count: %v\n", err)
			os.Exit(1)
		}
		if n > 0 {
			fmt.Fprintln(os.Stderr, "bootstrap-admin: refusing to run: platform_auth_accounts is not empty (pass -force to override)")
			os.Exit(2)
		}
	}

	var accountID string
	var exists bool
	if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_auth_accounts WHERE lower(username) = lower($1))`, normalized).Scan(&exists); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: lookup: %v\n", err)
		os.Exit(1)
	}
	if exists {
		fmt.Fprintf(os.Stdout, "bootstrap-admin: account %q already exists (no-op)\n", normalized)
		os.Exit(0)
	}

	err = conn.QueryRow(ctx, `
INSERT INTO platform_auth_accounts (username, email, password_hash, roles, status)
VALUES ($1, NULL, $2, $3, 'active')
RETURNING id::text`,
		normalized, string(hash), []string{plauth.RolePlatformAdmin},
	).Scan(&accountID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: insert: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "bootstrap-admin: created account id=%s username=%s roles=[platform_admin]\n", accountID, normalized)
}
