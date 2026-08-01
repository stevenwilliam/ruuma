// Package db embeds the numbered SQL migrations so a single binary carries its
// own schema (CLAUDE.md §2). Migrations are the source of truth; gorm models
// map onto them and AutoMigrate is never used.
package db

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migration is one numbered step with both directions.
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// DataMigrations are the migrations that carry reference data rather than
// schema. A test harness that empties the database has to put these back:
// TRUNCATE ... CASCADE reaches sys_parameters through its updated_by foreign
// key, and a service without its parameters silently falls back to compiled
// defaults with no templates at all.
func DataMigrations() ([]Migration, error) {
	all, err := Migrations()
	if err != nil {
		return nil, err
	}
	var out []Migration
	for _, m := range all {
		if strings.Contains(m.Name, "reference_data") || strings.Contains(m.Name, "parameters") {
			out = append(out, m)
		}
	}
	return out, nil
}

// Migrations returns every migration ordered by version. A step missing its
// .down.sql is an error: forward-only in production still means reversible in
// development and CI (CLAUDE.md §4).
func Migrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("db: read migrations: %w", err)
	}

	byVersion := map[int]*Migration{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		name := e.Name()
		parts := strings.SplitN(strings.TrimSuffix(name, ".sql"), "_", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("db: bad migration filename %q", name)
		}
		version, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("db: bad migration version in %q: %w", name, err)
		}
		body, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, fmt.Errorf("db: read %q: %w", name, err)
		}

		m, ok := byVersion[version]
		if !ok {
			m = &Migration{Version: version}
			byVersion[version] = m
		}
		switch {
		case strings.HasSuffix(parts[1], ".up"):
			m.Name = strings.TrimSuffix(parts[1], ".up")
			m.Up = string(body)
		case strings.HasSuffix(parts[1], ".down"):
			m.Down = string(body)
		default:
			return nil, fmt.Errorf("db: migration %q must end in .up.sql or .down.sql", name)
		}
	}

	out := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.Up == "" {
			return nil, fmt.Errorf("db: migration %04d has no .up.sql", m.Version)
		}
		if m.Down == "" {
			return nil, fmt.Errorf("db: migration %04d (%s) has no .down.sql", m.Version, m.Name)
		}
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}
