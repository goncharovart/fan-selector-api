# fan-selector-api

Go microservice for HVAC fan duty-point matching. Given a target airflow (Q, m³/h)
and static pressure (P, Pa), returns ranked fan models whose performance curves
intersect that operating point, with computed efficiency at the intersection.

Built spec-first with Claude Code following the BMad-Method workflow.

## Why this exists

When you select a centrifugal fan for an HVAC system you don't pick by nameplate —
you pick by where the duct system's resistance curve crosses the fan's P–Q curve.
That intersection is the *operating point*. A fan that nominally handles your flow
can still be wrong if the operating point sits in the stall region or far from peak
efficiency.

Performance curves are usually published as polynomial coefficients (cubic or
quartic) for pressure and power as a function of flow. This service indexes those
polynomials in Postgres, evaluates them under load, and returns matches in
sub-100ms.

## API

```
GET /v1/match?q=3000&p=300&tolerance=0.1
```

Response:

```json
{
  "operating_point": { "q_m3h": 3000, "p_pa": 300 },
  "tolerance": 0.1,
  "matches": [
    {
      "fan_id": "vr-80-75-4",
      "label": "ВР 80-75 №4",
      "rpm": 1450,
      "q_at_intersection_m3h": 2987.4,
      "p_at_intersection_pa": 303.2,
      "power_kw": 0.74,
      "efficiency": 0.62,
      "distance": 0.011
    }
  ],
  "count": 1,
  "elapsed_ms": 12
}
```

Health:

```
GET /healthz   → 200 ok
GET /readyz    → 200 ok | 503 if Postgres or Redis are unreachable
```

## Stack

- **Go 1.23** — `net/http` + `chi` router + `slog` structured logging
- **PostgreSQL 16** — fan / variant / coefficient storage; jsonb for raw polynomials
- **Redis 7** — per-query result cache keyed by `(q, p, tolerance)` SHA-256
- **OpenTelemetry** — traces shipped to Google Cloud Trace
- **Testcontainers** — integration tests against ephemeral Postgres

Runs on **Google Cloud Run** behind Cloud SQL (Postgres) and Memorystore (Redis).
Build and deploy via Cloud Build → Artifact Registry → Cloud Run.

## Repo layout

```
.
├── cmd/server/             # main.go entrypoint
├── internal/
│   ├── api/                # HTTP handlers, middleware
│   ├── matching/           # polynomial evaluation + duty-point selector
│   ├── storage/            # Postgres + Redis adapters
│   ├── config/             # env-driven config
│   └── observability/      # slog + OTel setup
├── migrations/             # goose-compatible SQL migrations
├── specs/                  # SDD artifacts (PRD, architecture, stories)
├── deploy/                 # Cloud Run service yaml, deploy scripts
└── .github/workflows/      # CI pipeline
```

## Spec-driven workflow

This project follows BMad-Method spec-driven development:

1. `specs/PRD.md` — what we're building and why
2. `specs/architecture.md` — system design, data model, request flow
3. `specs/stories/*.md` — implementation tasks in order, each closing one acceptance criterion

Code is written **after** the relevant story is locked. Each PR pairs a story
update with the implementation diff. AI assistance (Claude Code) is used at every
stage of the spec→code→review loop.

## Local development

```bash
# Postgres + Redis via Docker Compose
docker compose up -d

# Apply migrations
go run ./cmd/migrate up

# Seed sample polynomials
go run ./cmd/seed

# Run server
go run ./cmd/server

# Test
go test ./...
go test -tags=integration ./...
```

## Deployment

See `deploy/README.md`. TL;DR — one-command deploy:

```bash
gcloud builds submit --config=cloudbuild.yaml
```

## License

MIT
