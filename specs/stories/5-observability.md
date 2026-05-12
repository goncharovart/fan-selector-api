# Story 5 — Observability

## Why
Cloud Run gives us logs and basic metrics for free. To debug bad matches and
slow queries we need structured logging and traces tying together the HTTP
request, DB query, and matching work.

## Acceptance criteria
1. `internal/observability/log.go` configures `slog.JSONHandler` with `LOG_LEVEL`
   env. Default `info`. Every log line includes `trace_id` when a span is active.
2. `internal/observability/otel.go` configures OpenTelemetry tracer if
   `OTEL_EXPORTER_OTLP_ENDPOINT` is set; otherwise returns a no-op tracer.
3. Spans:
   - `http.request` — created by chi-otel middleware
   - `match.evaluate` — wraps the candidate loop
   - `storage.candidates` — wraps the DB prefilter
   - `cache.get` / `cache.set`
4. `main.go` flushes traces and the slog handler on SIGTERM before exiting.
5. README has a 5-line snippet showing how to view traces in Cloud Trace.

## Out of scope
Prom metrics — Cloud Run gives request count and latency by default. Add Prom
later if we self-host.
