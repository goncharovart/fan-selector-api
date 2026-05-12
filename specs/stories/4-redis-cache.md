# Story 4 — Redis cache

## Why
Identical queries from sizing tools (the same engineer typing duct dimensions)
arrive in bursts. We don't want to repeat the polynomial work.

## Acceptance criteria
1. `internal/storage/cache.go` exposes:
   - `NewRedis(ctx, dsn) (*Cache, error)` — if dsn is empty, returns a `NopCache`
     that always misses
   - `Get(ctx, key) ([]byte, bool, error)` — bool indicates hit
   - `Set(ctx, key, value, ttl)` — fire-and-forget on errors (log only)
   - `Ping(ctx) error`
2. Cache key in `internal/matching/selector.go`:
   `sha256("v1|" + canonical(MatchRequest))`, hex-encoded.
3. TTL = `CACHE_TTL` env (default 5m). Jitter ±10% to avoid stampedes.
4. On Redis outage: log a single warning, fall through to uncached evaluation.
   `/readyz` reports 503 only while the outage is ongoing.
5. Tests cover: hit path, miss path, marshalling round-trip, NopCache behavior.

## Out of scope
Distributed singleflight — for v1 we accept occasional duplicate computation.
