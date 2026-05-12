# Architecture — fan-selector-api

## High-level

```
   ┌──────────────┐    HTTP/JSON     ┌──────────────────┐
   │   Caller     │ ───────────────▶ │   Cloud Run      │
   │ (sizing UI,  │ ◀─────────────── │   fan-selector   │
   │  curl, …)    │                  │   (Go binary)    │
   └──────────────┘                  └────────┬─────────┘
                                              │
                          ┌───────────────────┼───────────────────┐
                          │                   │                   │
                          ▼                   ▼                   ▼
                   ┌────────────┐      ┌────────────┐      ┌────────────┐
                   │  Cloud SQL │      │ Memorystore│      │ Cloud Trace│
                   │  Postgres  │      │   Redis    │      │  (OTel)    │
                   └────────────┘      └────────────┘      └────────────┘
```

## Data model

```sql
CREATE TABLE fan_models (
    id              text PRIMARY KEY,            -- "vr-80-75-4"
    manufacturer    text NOT NULL,
    series          text NOT NULL,               -- "ВР 80-75"
    size            text NOT NULL,               -- "№4"
    rpm             integer NOT NULL,
    impeller_d_mm   integer,
    metadata        jsonb NOT NULL DEFAULT '{}'
);

CREATE TABLE fan_curves (
    fan_id          text NOT NULL REFERENCES fan_models(id) ON DELETE CASCADE,
    q_min_m3h       real NOT NULL,
    q_max_m3h       real NOT NULL,
    -- Polynomial coefficients [a0, a1, a2, a3, ...] for:
    p_coeffs        real[] NOT NULL,             -- P(Q) static pressure
    n_coeffs        real[] NOT NULL,             -- N(Q) shaft power, kW
    PRIMARY KEY (fan_id)
);

CREATE INDEX fan_curves_q_range ON fan_curves USING gist (
    numrange(q_min_m3h::numeric, q_max_m3h::numeric, '[]')
);
```

Indexing strategy: a GiST index on `(Q_min, Q_max)` as a range lets the matching
query prefilter to fans whose declared envelope brackets the target `Q` before
we evaluate any polynomial.

## Request flow — `/v1/match`

```
1.  HTTP handler validates q, p, tolerance, limit; binds to MatchRequest.
2.  Cache key = sha256(canonical(MatchRequest)). Redis GET.
       hit  → unmarshal, return.
       miss → continue.
3.  Storage layer prefilter:
       SELECT fan_id, p_coeffs, n_coeffs, q_min, q_max
       FROM fan_curves WHERE q_target BETWEEN q_min AND q_max;
4.  Matching engine, per candidate:
       a. Solve P_fan(Q) = p_target for Q in [q_min, q_max] via bisection.
       b. If no root, candidate is dropped.
       c. Compute power N(Q*) and efficiency η = (Q* · P_target) / (3600 · N · 1000).
       d. Compute distance score |Q_target − Q*| / Q_target.
       e. Drop if distance > tolerance.
5.  Sort by efficiency desc, distance asc. Take first `limit` (default 10).
6.  Redis SET key value EX 300 (with jitter).
7.  Return JSON.
```

## Concurrency

- pgxpool with `MaxConns=25`, `MinConns=2`. Cloud Run instance default = 80
  concurrent requests; pool sized to absorb them with headroom.
- Matching loop is CPU-bound and runs sequentially per request — no fan-out;
  candidate sets are small (typically < 50 after the GiST prefilter).
- Redis client (go-redis/v9) with default pool, `PoolSize=10`.

## Failure modes

| Failure | Behaviour |
|---|---|
| Postgres unreachable | `/readyz` → 503; `/v1/match` → 503 with retry-after |
| Redis unreachable | log warning, evaluate uncached; `/readyz` → 503 |
| Polynomial has no root in range | candidate silently skipped (not an error) |
| Tolerance impossible to satisfy | response `count: 0`, no candidates |
| SIGTERM | 10 s graceful shutdown, refuses new conns, drains in-flight |

## Configuration

All via env vars. Defaults in `internal/config/config.go`.

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen |
| `DATABASE_URL` | — | Postgres DSN |
| `REDIS_URL` | — | Redis DSN (optional; if absent → no cache, log warning) |
| `LOG_LEVEL` | `info` | `debug \| info \| warn \| error` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | — | If set, enable OTel export |
| `CACHE_TTL` | `5m` | Result cache TTL |
| `MAX_CANDIDATES_PER_QUERY` | `100` | Prefilter cap to bound work |

## Deployment topology

- **Artifact Registry** — Docker image `gcr.io/$PROJECT/fan-selector:$SHA`
- **Cloud Run service** — region `europe-west1`, min-instances=0, max=10,
  concurrency=80, CPU=1, memory=512Mi
- **Cloud SQL** — Postgres 16, db-g1-small, private IP via VPC connector
- **Memorystore** — Redis 7, BASIC tier, 1 GB
- **Secret Manager** — `DATABASE_URL`, `REDIS_URL` injected as Cloud Run secrets
- **Cloud Build** — trigger on `main` push; runs `cloudbuild.yaml`
