# fan-selector-api

[![ci](https://github.com/goncharovart/fan-selector-api/actions/workflows/ci.yml/badge.svg)](https://github.com/goncharovart/fan-selector-api/actions/workflows/ci.yml)
![go](https://img.shields.io/badge/go-1.24-00ADD8?logo=go)
![license](https://img.shields.io/badge/license-MIT-blue)

Go microservice for HVAC fan duty-point matching. Given a target airflow
(Q, m³/h) and static pressure (P, Pa), it returns ranked fan models whose
performance curves intersect that operating point, with computed efficiency
at the intersection.

Built spec-first with Claude Code following the BMad-Method workflow — every
feature lands as a PR that pairs an updated story under `specs/stories/` with
its implementation.

## Why this exists

When you size a ventilation system, the fan you pick has to sit on a viable
operating point of its P–Q curve — the intersection between the duct system's
resistance curve and the fan's published pressure characteristic. A fan that
nominally handles your flow can still be wrong if the operating point sits in
the stall region or far from peak efficiency.

Performance curves are usually published as polynomial coefficients (cubic or
quartic) for pressure `P(Q)` and shaft power `N(Q)`. This service indexes
those polynomials in Postgres, evaluates them under load, and returns matches
in well under 100 ms.

It's a small slice of [wentmarket.ru](https://wentmarket.ru) — a B2B HVAC
platform I'm running in production — extracted into a clean Go service so the
duty-point engine can be reused by other tools.

## Architecture

```mermaid
flowchart LR
    Caller["HTTP caller<br/>(sizing UI, curl, …)"] -- "GET /v1/match?q&p" --> API["chi router<br/>fan-selector-api"]
    API --> Cache[("Redis cache<br/>sha256(req) → JSON")]
    API --> Engine{{"matching engine<br/>polynomial eval + bisection"}}
    Engine --> Storage[("PostgreSQL<br/>GiST range index")]
    API --> OTel["OpenTelemetry<br/>Cloud Trace / OTLP"]

    classDef store fill:#1e293b,stroke:#475569,color:#e2e8f0
    classDef external fill:#0f172a,stroke:#334155,color:#e2e8f0
    class Cache,Storage store
    class OTel,Caller external
```

A duty-point request flows through four stages:

1. **Cache lookup.** Identical `(q, p, tolerance, limit)` queries hit Redis with
   a `sha256("v1|q=…|p=…|tol=…|lim=…")` key and a 5-minute TTL with ±10%
   jitter to prevent stampedes.
2. **Postgres prefilter.** A GiST range index on `(q_min_m3h, q_max_m3h)` lets
   the catalog query reduce candidates to fans whose declared envelope brackets
   the target Q — typically tens, not thousands.
3. **Engine.** For each candidate, bisection solves `P_fan(Q) = P_target`
   inside `[q_min, q_max]`, computes power `N(Q*)`, derives efficiency, and
   drops anything outside the requested tolerance.
4. **Sort + serialize.** Stable sort by efficiency desc, distance asc; encode
   to JSON; write back to Redis on the way out.

## Quick start

```bash
# Boot Postgres + Redis
docker compose up -d

# Apply migrations and seed a small synthetic catalog
DATABASE_URL='postgres://fan:fan@127.0.0.1:5432/fan_selector?sslmode=disable' go run ./cmd/migrate up
DATABASE_URL='postgres://fan:fan@127.0.0.1:5432/fan_selector?sslmode=disable' go run ./cmd/seed

# Run the server
DATABASE_URL='postgres://fan:fan@127.0.0.1:5432/fan_selector?sslmode=disable' \
REDIS_URL='redis://127.0.0.1:6379/0' \
go run ./cmd/server
```

## API by example

### 1 · Happy path — fans that match

```bash
$ curl -s 'http://localhost:8080/v1/match?q=3000&p=300' | jq
{
  "operating_point": { "q_m3h": 3000, "p_pa": 300 },
  "tolerance": 0.05,
  "matches": [
    {
      "fan_id": "demo-300",
      "label": "Demo-300 №4",
      "rpm": 1450,
      "q_at_intersection_m3h": 2998.4,
      "p_at_intersection_pa": 300.1,
      "power_kw": 0.84,
      "efficiency": 0.301,
      "distance": 0.00053
    }
  ],
  "count": 1,
  "elapsed_ms": 11
}
```

### 2 · No-match — duty point off any curve

```bash
$ curl -s 'http://localhost:8080/v1/match?q=50000&p=2500' | jq
{
  "operating_point": { "q_m3h": 50000, "p_pa": 2500 },
  "tolerance": 0.05,
  "matches": [],
  "count": 0,
  "elapsed_ms": 7
}
```

### 3 · Bad input — descriptive 400

```bash
$ curl -s -i 'http://localhost:8080/v1/match?q=abc&p=300' | head -3
HTTP/1.1 400 Bad Request
Content-Type: application/json
{"error":"invalid_request","message":"q must be a number"}
```

### 4 · Cache hit — same query returns in <10 ms

```bash
$ curl -s -D - 'http://localhost:8080/v1/match?q=3000&p=300' -o /dev/null | grep -i cache
X-Cache: HIT
```

## Health

```bash
$ curl -s http://localhost:8080/healthz   # always 200 once the process is alive
{"status":"ok"}

$ curl -s -i http://localhost:8080/readyz # 200 if Postgres + Redis are reachable
HTTP/1.1 200 OK
{"status":"ready"}
```

## Performance

Measured on a 12th-gen Intel i5-12400F, Go 1.25, Windows (`go test -bench`).

| Workload | Throughput | Allocations |
|---|---|---|
| `Eval` (cubic polynomial) | **0.65 ns/op** | 0 B/op · 0 allocs |
| `Solve` (bisection, ≤60 iter) | **53 ns/op** | 0 B/op · 0 allocs |
| `Evaluate` — 10 candidates | **790 ns/op** | 920 B · 2 allocs |
| `Evaluate` — 50 candidates | **3.7 µs/op** | 4.2 KB · 4 allocs |
| `CacheKey` (SHA-256 + sprintf) | **424 ns/op** | 184 B · 6 allocs |

In context: a realistic prefilter returns 10–30 candidates, so the engine
spends ~1–2 µs of CPU per request. The PostgreSQL prefilter + JSON encoding
dominate; the 100 ms p95 budget has ~99 ms of headroom on warm cache miss.

## Observability

Every request creates a tree of spans you can pick apart in Cloud Trace, Tempo,
Jaeger, or any OTLP collector. Set `OTEL_EXPORTER_OTLP_ENDPOINT` to enable:

```
http.request
├── cache.get          (attrs: cache.hit, cache.value_bytes)
├── storage.candidates (attrs: storage.q_target, storage.candidates_found)
├── match.evaluate     (attrs: match.candidates_in, match.candidates_out, match.tolerance)
└── cache.set          (attrs: cache.value_bytes, cache.ttl_ms)
```

Structured logs go to stdout as JSON; `trace_id` is included on every line when
a span is active, so logs and traces correlate in the Cloud Logging UI.

## Repo layout

```
.
├── cmd/
│   ├── server/             main.go entrypoint, SIGTERM graceful shutdown
│   ├── migrate/            distroless-compatible SQL migrator with embedded *.sql
│   └── seed/               idempotent loader for 12 synthetic sample fans
├── internal/
│   ├── api/                chi handlers, request validation, cache wiring
│   ├── matching/           pure-function polynomial engine + bisection solver
│   ├── storage/            pgxpool + Redis adapters (Cache interface with NopCache)
│   ├── config/             env-driven configuration
│   └── observability/      slog JSON + OpenTelemetry tracer setup
├── migrations/             goose-compatible *.sql + embed.FS exporter
├── specs/                  PRD + architecture + per-story acceptance criteria
├── deploy/                 GCP Cloud Run and Fly.io deployment guides
└── .github/workflows/      CI: unit, integration (testcontainers), bench, lint, image
```

## Testing

```bash
# Unit tests — no external dependencies. ~0.04s total.
go test ./...

# Race detector + coverage profile.
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1
# total:  (statements)  45.3%

# Benchmarks.
go test -run=^$ -bench=. -benchmem -benchtime=2s ./internal/matching/

# Integration tests against a real Postgres started via testcontainers-go.
# Requires Docker on the host.
go test -tags=integration ./internal/storage/...
```

Coverage on the hot paths:

| Package | Coverage |
|---|---|
| `internal/matching` | **93.8%** — engine is the critical business logic |
| `internal/api` | **90.0%** — every error path is exercised |
| `internal/storage` | covered by `-tags=integration` against real Postgres |

## Spec-driven workflow

This repo uses BMad-Method-style spec-driven development:

1. **`specs/PRD.md`** — what we're building and why, success metrics, risks.
2. **`specs/architecture.md`** — system design, data model, request flow,
   failure modes, configuration table.
3. **`specs/stories/*.md`** — implementation tasks in order, each closing one
   acceptance criterion. Code is written **after** the relevant story is
   locked.

Each PR pairs a story update with the implementation diff. Claude Code is the
day-to-day driver — but the spec gates the code, not the other way around.

## Deployment

Two paths are committed; pick whichever you have billing for.

- **GCP Cloud Run** — [`deploy/README.md`](deploy/README.md).
  Cloud Build → Artifact Registry → Cloud Run, Cloud SQL Postgres,
  Memorystore Redis, secrets via Secret Manager.
- **Fly.io** — [`deploy/fly.md`](deploy/fly.md).
  `flyctl deploy --remote-only` with Fly Postgres and optional Upstash Redis.

Both options use the same Dockerfile and embedded-migration runner
(`cmd/migrate`), so the binary is portable across managed-container hosts.

## License

MIT
