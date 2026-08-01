// Package migrate applies the embedded numbered migrations.
//
// Forward-only in production (CLAUDE.md §4): Down exists for development, CI
// (which runs up → down → up on a fresh database) and rollback of last resort.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/stevenwilliam/ruuma/db"
)

const schemaTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version     INT PRIMARY KEY,
  name        TEXT NOT NULL,
  applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
)`

// Status reports which versions are applied.
type Status struct {
	Applied []int
	Pending []int
}

// Up applies every pending migration, each in its own transaction so a failure
// leaves the schema at the last good version rather than half-applied.
func Up(ctx context.Context, conn *sql.DB, log *slog.Logger) (int, error) {
	if _, err := conn.ExecContext(ctx, schemaTable); err != nil {
		return 0, fmt.Errorf("migrate: schema table: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return 0, err
	}
	migrations, err := db.Migrations()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := runStep(ctx, conn, m.Up, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`, m.Version, m.Name)
			return err
		}); err != nil {
			return count, fmt.Errorf("migrate: %04d_%s up: %w", m.Version, m.Name, err)
		}
		if log != nil {
			log.Info("migration applied", "version", m.Version, "name", m.Name)
		}
		count++
	}
	return count, nil
}

// Down rolls back the last n applied migrations. n <= 0 rolls back everything.
func Down(ctx context.Context, conn *sql.DB, n int, log *slog.Logger) (int, error) {
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return 0, err
	}
	migrations, err := db.Migrations()
	if err != nil {
		return 0, err
	}

	count := 0
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		if !applied[m.Version] {
			continue
		}
		if n > 0 && count >= n {
			break
		}
		if err := runStep(ctx, conn, m.Down, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version)
			return err
		}); err != nil {
			return count, fmt.Errorf("migrate: %04d_%s down: %w", m.Version, m.Name, err)
		}
		if log != nil {
			log.Info("migration rolled back", "version", m.Version, "name", m.Name)
		}
		count++
	}
	return count, nil
}

// Current reports applied and pending versions.
func Current(ctx context.Context, conn *sql.DB) (Status, error) {
	var st Status
	if _, err := conn.ExecContext(ctx, schemaTable); err != nil {
		return st, fmt.Errorf("migrate: schema table: %w", err)
	}
	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return st, err
	}
	migrations, err := db.Migrations()
	if err != nil {
		return st, err
	}
	for _, m := range migrations {
		if applied[m.Version] {
			st.Applied = append(st.Applied, m.Version)
		} else {
			st.Pending = append(st.Pending, m.Version)
		}
	}
	return st, nil
}

func runStep(ctx context.Context, conn *sql.DB, body string, record func(*sql.Tx) error) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if err := record(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func appliedVersions(ctx context.Context, conn *sql.DB) (map[int]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: read applied: %w", err)
	}
	defer func() { _ = rows.Close() }()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}
