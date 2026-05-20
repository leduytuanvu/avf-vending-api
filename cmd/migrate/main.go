// Command migrate runs goose migrations from /app/migrations (embedded in the production image).
// Usage: migrate validate | status | version | up
// Requires DATABASE_URL for status, version, and up.
package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const defaultMigrationsDir = "/app/migrations"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: migrate <validate|status|version|up>")
		return 2
	}

	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = defaultMigrationsDir
	}

	switch os.Args[1] {
	case "validate":
		return validateMigrations(dir)
	case "status", "version", "up":
		return runDBCommand(os.Args[1], dir)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		return 2
	}
}

func validateMigrations(dir string) int {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: migrations directory missing or not a directory: %s\n", dir)
		return 1
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: list migrations: %v\n", err)
		return 1
	}
	if len(matches) == 0 {
		fmt.Fprintf(os.Stderr, "error: no .sql migration files in %s\n", dir)
		return 1
	}

	for _, path := range matches {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: unreadable migration %s: %v\n", path, err)
			return 1
		}
		_ = f.Close()
	}

	fmt.Printf("OK: %d migration file(s) in %s\n", len(matches), dir)
	return 0
}

func runDBCommand(command, dir string) int {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "error: DATABASE_URL is required")
		return 2
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		fmt.Fprintf(os.Stderr, "error: goose dialect: %v\n", err)
		return 1
	}

	switch command {
	case "status":
		if err := goose.Status(db, dir); err != nil {
			fmt.Fprintf(os.Stderr, "error: migration status: %v\n", err)
			return 1
		}
	case "version":
		version, err := goose.GetDBVersion(db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: migration version: %v\n", err)
			return 1
		}
		fmt.Println(version)
	case "up":
		if err := goose.Up(db, dir); err != nil {
			fmt.Fprintf(os.Stderr, "error: migration up: %v\n", err)
			return 1
		}
	}
	return 0
}
