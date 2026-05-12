//go:build integration

package storage_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/goncharovart/fan-selector-api/internal/matching"
	"github.com/goncharovart/fan-selector-api/internal/storage"
	"github.com/goncharovart/fan-selector-api/migrations"
)

// startPostgres boots a throwaway Postgres 16 container, returns its DSN and
// a cleanup function. The container is destroyed in t.Cleanup so even a
// panic doesn't leak it.
func startPostgres(ctx context.Context, t *testing.T, db string) string {
	t.Helper()
	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(db),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}

// TestCandidates_RangeFilter spins up a real Postgres, applies the schema
// via embedded migrations, inserts a handful of fans, then asserts the
// storage layer only returns the fans whose declared envelope brackets the
// target flow.
//
// Run with `go test -tags=integration ./internal/storage/...` (Docker required).
func TestCandidates_RangeFilter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	dsn := startPostgres(ctx, t, "fan_selector_test")
	if err := applyMigrations(ctx, dsn); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := seedFans(ctx, dsn); err != nil {
		t.Fatalf("seed fans: %v", err)
	}

	store, err := storage.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(store.Close)

	// fan-a: envelope 1000–3000 — brackets Q=2000 ✓
	// fan-b: envelope 4000–6000 — does not bracket Q=2000 ✗
	// fan-c: envelope 1500–2500 — brackets Q=2000 ✓
	got, err := store.Candidates(ctx, 2000, 100)
	if err != nil {
		t.Fatalf("Candidates Q=2000: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates bracketing Q=2000, got %d (%v)", len(got), idsOf(got))
	}
	wantIDs := map[string]bool{"fan-a": true, "fan-c": true}
	for _, c := range got {
		if !wantIDs[c.FanID] {
			t.Errorf("unexpected candidate %q in Q=2000 result", c.FanID)
		}
	}

	// fan-b only at Q=5000
	got, err = store.Candidates(ctx, 5000, 100)
	if err != nil {
		t.Fatalf("Candidates Q=5000: %v", err)
	}
	if len(got) != 1 || got[0].FanID != "fan-b" {
		t.Errorf("expected only fan-b at Q=5000, got %v", idsOf(got))
	}

	// Outside any envelope ⇒ zero results, not an error.
	got, err = store.Candidates(ctx, 99999, 100)
	if err != nil {
		t.Fatalf("Candidates Q=99999: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected zero candidates beyond all envelopes, got %v", idsOf(got))
	}
}

// TestPing_StoreVerifiesConnection asserts Ping behaves correctly while the
// DB is up and starts failing once the container is gone.
func TestPing_StoreVerifiesConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("ping_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("dsn: %v", err)
	}

	store, err := storage.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.Ping(ctx); err != nil {
		t.Errorf("Ping while up: %v", err)
	}

	// Kill the container — pings must start failing.
	if err := pg.Terminate(ctx); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if err := store.Ping(ctx); err == nil {
		t.Error("expected Ping to fail after container terminated")
	}
	store.Close()
}

// --- helpers ---

func applyMigrations(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		body, err := migrations.FS.ReadFile(e.Name())
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, stripGoose(string(body))); err != nil {
			return err
		}
	}
	return nil
}

func seedFans(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	fans := []struct {
		id, series, size string
		rpm              int
		qmin, qmax       float32
	}{
		{"fan-a", "TestSeries", "A", 1450, 1000, 3000},
		{"fan-b", "TestSeries", "B", 1450, 4000, 6000},
		{"fan-c", "TestSeries", "C", 1450, 1500, 2500},
	}
	pCoeffs := []float32{400, -0.05}
	nCoeffs := []float32{0.5, 0.0001}
	for _, f := range fans {
		if _, err := conn.Exec(ctx, `
			INSERT INTO fan_models (id, manufacturer, series, size, rpm)
			VALUES ($1, 'Test', $2, $3, $4)
		`, f.id, f.series, f.size, f.rpm); err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `
			INSERT INTO fan_curves (fan_id, q_min_m3h, q_max_m3h, p_coeffs, n_coeffs)
			VALUES ($1, $2, $3, $4, $5)
		`, f.id, f.qmin, f.qmax, pCoeffs, nCoeffs); err != nil {
			return err
		}
	}
	return nil
}

// stripGoose removes `-- +goose` directives so the SQL parses on its own
// outside the goose runner.
func stripGoose(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "-- +goose") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.Join(kept, "\n")
}

func idsOf(cs []matching.FanCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.FanID
	}
	return out
}
