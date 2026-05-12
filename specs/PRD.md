# PRD — fan-selector-api

> **Status:** Draft v1 · **Owner:** Artur Goncharov · **Last updated:** 2026-05-12

## 1. Problem

HVAC engineers spend the first 5 minutes of every system design picking a fan that
sits at a viable operating point on its P–Q curve. The conventional path is:

1. Open the manufacturer PDF
2. Eyeball the graph for the closest curve that passes through the target (Q, P)
3. Cross-check power, efficiency, and noise from a side table
4. Repeat across 4–5 manufacturers to compare

This is slow, error-prone (curves overlap, scales vary), and impossible to
automate inside a larger sizing workflow (e.g., a duct-design tool that wants to
auto-suggest fans as the user adjusts the system).

## 2. Goals

- **G1.** Given `(Q, P)`, return ranked fan candidates in **p95 < 100 ms**
- **G2.** Compute the **actual** operating point on each fan curve, not just the
  rated point — i.e., solve `P_fan(Q) = P_target` numerically
- **G3.** Report efficiency at the intersection so the caller can rank by it
- **G4.** Expose a clean HTTP/JSON API so the service plugs into any sizing tool
- **G5.** Run as a single stateless container on Cloud Run, scale to zero

## 3. Non-goals

- Acoustic / noise prediction (separate service)
- BIM export, datasheets, drawings
- UI — this is a backend service consumed by other services and frontends
- Curve fitting from raw measurements — the service accepts pre-fitted
  polynomial coefficients only

## 4. Users

- **HVAC sizing tools** — call the API on each user keystroke to update fan suggestions
- **Internal procurement** — bulk-match a list of project duty points against the catalog
- **Engineers via CLI** — `curl` for ad-hoc lookups during early design

## 5. Success metrics

| Metric | Target |
|---|---|
| p95 latency `/v1/match` (cache miss) | < 100 ms |
| p95 latency `/v1/match` (cache hit) | < 10 ms |
| Polynomial evaluation accuracy | within 1.5% of reference Python impl across 500 sample points |
| Service availability | 99.5% / month |
| Cold start (Cloud Run) | < 2 s |

## 6. Functional requirements

### F1. Duty-point match endpoint
- `GET /v1/match?q={float}&p={float}&tolerance={float}&limit={int}`
- Returns fans whose curve passes within `tolerance` (relative) of `(Q, P)`
- Each match includes operating-point `(Q*, P*)`, power, efficiency, distance score
- Sorted by efficiency (descending), then by distance (ascending)

### F2. Catalog ingestion
- `POST /v1/admin/fans` (auth-gated) — accept fan + variant + coefficient bundle
- Polynomial coefficients stored as `jsonb` arrays in Postgres
- Idempotent on `(manufacturer, model, size, rpm)`

### F3. Cache
- Identical `(q, p, tolerance, limit)` queries served from Redis with 5-minute TTL
- Cache key = SHA-256 of canonicalized query string
- Cache misses populate transparently

### F4. Health
- `/healthz` — process alive
- `/readyz` — Postgres reachable AND Redis reachable; HTTP 503 otherwise

### F5. Observability
- Structured JSON logs via `slog`
- OpenTelemetry traces with spans for: request, db lookup, polynomial eval, cache get/set
- Traces exported to Google Cloud Trace

## 7. Non-functional requirements

- **Stateless** — no on-disk state, all data in Postgres
- **12-factor config** — env-driven only, no config files
- **Graceful shutdown** — SIGTERM drains in-flight requests within 10 s
- **Connection pooling** — pgxpool with sane defaults (max 25 conns)

## 8. Out-of-scope (deferred)

- gRPC endpoint — start with HTTP/JSON, add gRPC if a consumer needs it
- Async batch matching — separate `/v1/match/batch` endpoint, future story
- Per-tenant catalogs — single global catalog initially
- AuthN/authZ — accept Bearer token on `/v1/admin/*` only, validate via Cloud IAM

## 9. Risks

- **Polynomial coefficient quality.** Real-world coefficients have edge cases
  (zero-crossings, near-stall behavior). Mitigation: clamp evaluations outside
  manufacturer-declared `(Q_min, Q_max)`.
- **Cold start on Cloud Run.** Mitigation: min-instances=1 once traffic justifies cost.
- **Cache stampede.** Mitigation: jitter on TTL, singleflight pattern around evaluation.
