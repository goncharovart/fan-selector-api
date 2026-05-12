# Story 2 — Postgres storage

## Why
Fan catalog must persist between deployments and be queryable by Q-range
prefilter so we don't evaluate the polynomial of every fan in the catalog.

## Acceptance criteria
1. Migration `0001_init.up.sql` creates `fan_models`, `fan_curves`, and the GiST
   range index on `(q_min_m3h, q_max_m3h)`.
2. `internal/storage/postgres.go` exposes:
   - `New(ctx, dsn) (*Store, error)` — pgxpool bootstrap
   - `Candidates(ctx, qTarget, maxN) ([]FanCandidate, error)` — prefilter query
   - `Ping(ctx) error` — used by `/readyz`
3. `FanCandidate` includes `FanID`, `Label`, `Rpm`, `QMin`, `QMax`, `PCoeffs`, `NCoeffs`.
4. `pgxpool.Config` reads from env: `DATABASE_URL`, `MaxConns=25`, `MinConns=2`.
5. Integration test (build tag `integration`) spins up Postgres via testcontainers,
   runs migrations, inserts 5 fans, asserts prefilter returns only the matching set.

## Out of scope
Admin write endpoints (story 6). Seed data is loaded via `cmd/seed`, not the API.
