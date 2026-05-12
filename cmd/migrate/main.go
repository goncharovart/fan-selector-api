// Command migrate applies SQL migrations embedded into the binary.
// Designed for distroless images: no shell, no external tooling. The binary
// reads its migrations via go:embed at compile time, so deployment never has
// to ship loose .sql files.
//
// Usage:
//   migrate up        — apply every migration newer than the recorded head
//   migrate version   — print the current head
//
// Migrations are applied in lexicographic order. A `schema_migrations` table
// records which files have run. Inspired by goose, but trimmed to one .go
// file because the project only needs forward migrations on every deploy.
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/goncharovart/fan-selector-api/migrations"
)

var migrationsFS = migrations.FS

func main() {
	if len(os.Args) < 2 {
		log.Fatal("migrate: usage — migrate up | migrate version")
	}
	cmd := os.Args[1]

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("migrate: DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		log.Fatalf("migrate: connect: %v", err)
	}
	defer conn.Close(ctx)

	if err := ensureMigrationsTable(ctx, conn); err != nil {
		log.Fatalf("migrate: ensure table: %v", err)
	}

	switch cmd {
	case "up":
		if err := up(ctx, conn); err != nil {
			log.Fatalf("migrate: up: %v", err)
		}
	case "version":
		ver, err := head(ctx, conn)
		if err != nil {
			log.Fatalf("migrate: version: %v", err)
		}
		if ver == "" {
			fmt.Println("no migrations applied")
		} else {
			fmt.Println(ver)
		}
	default:
		log.Fatalf("migrate: unknown command %q", cmd)
	}
}

func ensureMigrationsTable(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)
	`)
	return err
}

func head(ctx context.Context, conn *pgx.Conn) (string, error) {
	var ver string
	err := conn.QueryRow(ctx, `
		SELECT version FROM schema_migrations
		ORDER BY version DESC
		LIMIT 1
	`).Scan(&ver)
	if err != nil && err.Error() != "no rows in result set" {
		return "", err
	}
	return ver, nil
}

func up(ctx context.Context, conn *pgx.Conn) error {
	all, err := listMigrations()
	if err != nil {
		return err
	}

	applied, err := alreadyApplied(ctx, conn)
	if err != nil {
		return err
	}

	for _, m := range all {
		if applied[m.version] {
			continue
		}
		if err := applyOne(ctx, conn, m); err != nil {
			return fmt.Errorf("apply %s: %w", m.version, err)
		}
		fmt.Printf("applied %s\n", m.version)
	}
	return nil
}

type migration struct {
	version string // e.g. "0001_init"
	path    string // e.g. "migrations/0001_init.up.sql"
	body    string
}

// listMigrations returns up-migration files in lexicographic order. Down
// migrations are intentionally ignored — the binary only moves forward.
func listMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, e.Name())
		if err != nil {
			return nil, err
		}
		// Strip goose pragmas; we don't use goose's runner here.
		clean := stripGoosePragmas(string(body))
		out = append(out, migration{
			version: strings.TrimSuffix(e.Name(), ".up.sql"),
			path:    e.Name(),
			body:    clean,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func stripGoosePragmas(s string) string {
	// goose markers are line comments; trim them so the SQL parses on its own.
	lines := strings.Split(s, "\n")
	kept := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "-- +goose") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func alreadyApplied(ctx context.Context, conn *pgx.Conn) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func applyOne(ctx context.Context, conn *pgx.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, m.body); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, m.version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
