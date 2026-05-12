# Story 3 — HTTP API

## Why
The whole product is the API. Has to be small, predictable, and easy to call
from any HTTP client.

## Acceptance criteria
1. `internal/api/handler.go` wires:
   - `GET /v1/match` — Selector → Match → JSON
   - `GET /healthz` — always 200 ok
   - `GET /readyz` — 200 if Store.Ping and Cache.Ping both succeed; 503 otherwise
2. Router: `go-chi/chi/v5`. No global middleware except `RealIP`, `RequestID`, recovering panics, and OTel span middleware.
3. Query param validation:
   - `q` and `p` are required, parsed as float, must be > 0
   - `tolerance` optional, default 0.05, must be in `(0, 0.5]`
   - `limit` optional, default 10, must be in `[1, 50]`
4. Response shape exactly as in `README.md`. JSON encoding via `encoding/json`.
5. All error responses are `{"error": "code", "message": "human readable"}` with appropriate HTTP status.
6. Handler tests use `httptest.NewRecorder` and a fake Store + fake Cache. No real DB or Redis.

## Out of scope
Auth — admin endpoints come in a later story.
