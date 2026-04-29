// Command outbox-replay is a minimal operator CLI for listing and manually nudging transactional outbox rows.
// Prefer admin HTTP APIs when available; use this from bastion/CI with DATABASE_URL when HTTP is unavailable.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	appoutbox "github.com/avf/avf-vending-api/internal/app/outbox"
	"github.com/avf/avf-vending-api/internal/config"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	if len(os.Args) < 2 {
		usage()
	}
	cfg, err := config.Load()
	if err != nil {
		fatalf("config: %v", err)
	}
	if cfg.Postgres.URL == "" {
		fatalf("DATABASE_URL / Postgres URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := platformdb.NewPool(ctx, &cfg.Postgres)
	if err != nil {
		fatalf("postgres: %v", err)
	}
	defer pool.Close()

	adm := appoutbox.NewAdminService(pool)
	if adm == nil {
		fatalf("outbox admin service not available")
	}

	switch os.Args[1] {
	case "list":
		listCmd(ctx, adm, os.Args[2:])
	case "requeue":
		requeueCmd(ctx, adm, os.Args[2:])
	case "replay-dlq":
		replayDLQCmd(ctx, adm, os.Args[2:])
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage:
  %s list   [-after RFC3339] [-before RFC3339] [-status string] [-limit N]
  %s requeue -id N [-note reason] [-yes]
  %s replay-dlq -id N -confirm-poison-replay

Dead-letter (poison) replay requires -confirm-poison-replay. Requeue does not mutate financial rows.
`, os.Args[0], os.Args[0], os.Args[0])
	os.Exit(2)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func listCmd(ctx context.Context, adm *appoutbox.AdminService, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	after := fs.String("after", "", "created_at lower bound (RFC3339); default 24h ago")
	before := fs.String("before", "", "created_at upper bound exclusive (RFC3339); default now")
	status := fs.String("status", "", "optional: pending, failed, publishing")
	limit := fs.Int("limit", 100, "max rows (capped at 500)")
	_ = fs.Parse(args)

	now := time.Now().UTC()
	createdBefore := now
	if *before != "" {
		var err error
		createdBefore, err = time.Parse(time.RFC3339, *before)
		if err != nil {
			fatalf("before: %v", err)
		}
	}
	createdAfter := createdBefore.Add(-24 * time.Hour)
	if *after != "" {
		var err error
		createdAfter, err = time.Parse(time.RFC3339, *after)
		if err != nil {
			fatalf("after: %v", err)
		}
	}

	rows, err := adm.ListPendingWindow(ctx, createdAfter, createdBefore, *status, int32(*limit))
	if err != nil {
		fatalf("list: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rows); err != nil {
		fatalf("encode: %v", err)
	}
}

func requeueCmd(ctx context.Context, adm *appoutbox.AdminService, args []string) {
	fs := flag.NewFlagSet("requeue", flag.ExitOnError)
	id := fs.Int64("id", 0, "outbox_events.id")
	note := fs.String("note", "", "optional note stored in last_publish_error when non-empty")
	yes := fs.Bool("yes", false, "skip interactive confirmation prompt")
	_ = fs.Parse(args)
	if *id <= 0 {
		fatalf("requeue: -id is required")
	}
	if !*yes {
		fmt.Fprintf(os.Stderr, "requeue outbox id=%d: this clears lease/backoff on an unpublished row. Type yes: ", *id)
		var line string
		_, _ = fmt.Scanln(&line)
		if line != "yes" {
			fatalf("aborted")
		}
	}
	n, err := adm.RequeuePendingByID(ctx, *id, *note)
	if err != nil {
		fatalf("requeue: %v", err)
	}
	fmt.Fprintf(os.Stdout, "rows_updated=%d\n", n)
	if n == 0 {
		fmt.Fprintf(os.Stderr, "warning: no row matched (already published, dead-lettered, or unknown id)\n")
	}
}

func replayDLQCmd(ctx context.Context, adm *appoutbox.AdminService, args []string) {
	fs := flag.NewFlagSet("replay-dlq", flag.ExitOnError)
	id := fs.Int64("id", 0, "outbox_events.id")
	confirm := fs.Bool("confirm-poison-replay", false, "required; acknowledges replaying a quarantined message")
	_ = fs.Parse(args)
	if *id <= 0 {
		fatalf("replay-dlq: -id is required")
	}
	n, err := adm.ReplayDeadLetterAfterConfirm(ctx, *id, *confirm)
	if err != nil {
		fatalf("replay-dlq: %v", err)
	}
	fmt.Fprintf(os.Stdout, "rows_updated=%d\n", n)
	if n == 0 {
		fmt.Fprintf(os.Stderr, "warning: no dead-letter row matched id=%s\n", strconv.FormatInt(*id, 10))
	}
}
