// Command api is ruuma's single binary: serve, migrate, seed, worker.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/stevenwilliam/ruuma/internal/platform/config"
	"github.com/stevenwilliam/ruuma/internal/platform/logging"
	"github.com/stevenwilliam/ruuma/internal/platform/migrate"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ruuma: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `ruuma — multi-outlet restaurant ordering

usage: ruuma <command> [flags]

commands:
  serve      run the HTTP API
  worker     run background jobs (slot materialisation, notifications)
  migrate    apply migrations   (--down N | --down all | --status)
  seed       load demo data (never run against production)
  version    print version and commit
`)
}

func run() error {
	if len(os.Args) < 2 {
		usage()
		return fmt.Errorf("no command given")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := os.Args[1]
	if cmd == "version" {
		fmt.Printf("ruuma %s (%s)\n", version, commit)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logging.New(cfg.App.LogLevel, cfg.App.IsProduction())

	switch cmd {
	case "migrate":
		return runMigrate(ctx, cfg, log)
	case "seed":
		return runSeed(ctx, cfg, log)
	case "serve":
		return runServe(ctx, cfg, log)
	case "worker":
		return runWorker(ctx, cfg, log)
	default:
		usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func runMigrate(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	down := fs.String("down", "", "roll back N migrations, or 'all'")
	status := fs.Bool("status", false, "print applied and pending versions")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	conn, err := sql.Open("pgx", cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = conn.Close() }()

	switch {
	case *status:
		st, err := migrate.Current(ctx, conn)
		if err != nil {
			return err
		}
		fmt.Printf("applied: %v\npending: %v\n", st.Applied, st.Pending)
		return nil

	case *down != "":
		n := 0 // 0 = all
		if *down != "all" {
			parsed, err := strconv.Atoi(*down)
			if err != nil {
				return fmt.Errorf("--down expects a number or 'all'")
			}
			n = parsed
		}
		count, err := migrate.Down(ctx, conn, n, log)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back %d migration(s)\n", count)
		return nil

	default:
		count, err := migrate.Up(ctx, conn, log)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s)\n", count)
		return nil
	}
}
