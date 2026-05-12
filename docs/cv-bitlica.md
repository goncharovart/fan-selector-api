# Artur Goncharov

**Backend / Full-stack engineer · HVAC engineer**

Telegram: [@gonartur](https://t.me/gonartur) · Phone: +7 (995) 376-31-73 · Email: wentpromregionug@gmail.com

Flagship: [wentmarket.ru](https://wentmarket.ru) · Side: [github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api)

---

## Summary

Backend-leaning full-stack engineer with **6 years in HVAC engineering software**
and ~2 years shipping production web stacks. I work spec-first with AI coding
agents (Claude Code, Cursor) end-to-end — spec → architecture → implementation
→ deploy. My niche is domains where the algorithm matters as much as the
plumbing: fan-curve matching, duct loss, life-cycle cost, acoustic selection.

Comfortable in TypeScript/Node primary stack; currently ramping on Go via a
real side project to broaden the backend toolbelt.

---

## Selected experience

### Wentmarket — Founder / Lead engineer · 2024–present
B2B platform for HVAC engineers, in production at [wentmarket.ru](https://wentmarket.ru).
Built solo with AI-assisted workflow (Claude Code primary). Real users,
real orders, real payments.

- **Fan-selection engine.** Indexed 17 000+ polynomial performance curves
  in PostgreSQL with GiST range filtering; duty-point matching solves
  `P_fan(Q) = P_target` per request and returns ranked results in <100 ms p95.
- **Order pipeline.** Server actions + BullMQ workers + Redis L2 cache.
  Yookassa payment integration with deterministic SHA-256 idempotency keys
  and 54-FZ fiscal receipts. Webhook validation via API call rather than
  HMAC to defeat replay.
- **CDEK delivery integration.** 871-line service layer: OAuth token refresh
  on 401, exponential backoff (3 retries, max 8 s), 30 s per-request timeout,
  full order/intake/tracking/waybill flow.
- **Soft-delete extension** via Prisma `$extends` across 8 models with
  consistent query filtering.
- **Headless Bitrix24 loader** — loads the vendor widget, hides its UI via
  injected CSS + MutationObserver, exposes `window.openB24Chat()` for our
  own triggers. Avoids two competing chat buttons.
- **1C ERP sync** through CommerceML XML; products/prices/stock pipeline.
- **Meilisearch** with graceful fallback to Prisma `LIKE` when the indexer
  is unreachable — single observable degradation rather than a crash.
- **Stack:** Next.js 16, React 19, TypeScript, Prisma 7, PostgreSQL,
  Redis (BullMQ + cache), Meilisearch, Sentry, NextAuth v5.
- **Infra:** self-hosted VPS, systemd + nginx, custom deploy pipeline
  (`tar | ssh | build-on-server`).

### Goal-Energo — HVAC project engineer · 2018–2024
Six years sizing ventilation, smoke-extract, and AHU systems for industrial
and commercial buildings. Built internal sizing spreadsheets/scripts that
later became the seed for Wentmarket's engineering engines.

- Polynomial curve fitting from measured fan data → coefficient catalog
  used today in Wentmarket.
- Authored project sizing methodology used across the team.
- Customer-facing: gathered duty-point requirements, produced
  specifications and bid packages.

---

## Side project — `fan-selector-api`

[github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api)

A standalone Go microservice that extracts Wentmarket's fan-matching engine
into a clean, testable service. Built spec-first (BMad-style) on Cloud Run.
**Purpose:** demonstrate Go + GCP + SDD workflow on a domain I know cold.

- **Go 1.23**, chi router, pgx/v5, go-redis/v9, OpenTelemetry, slog.
- **Postgres 16** with GiST range index on the fan duty-point envelope;
  prefilter narrows from full catalog to candidates in a single indexed scan.
- **Bisection root finder** for `P_fan(Q) = P_target` with input
  defenses (NaN/Inf, sign-change check, boundary hits, monotone-decreasing
  curve assumption documented).
- **Redis cache** with deterministic SHA-256 keys, TTL jitter (±10%) to
  prevent stampedes, graceful degradation to a NopCache when Redis is down.
- **Distroless multi-stage Docker image**, non-root user.
- **Cloud Build → Artifact Registry → Cloud Run** one-command deploy
  via `cloudbuild.yaml`; secrets via Secret Manager; Postgres via Cloud SQL
  Unix socket.
- **GitHub Actions CI:** `go vet`, `go test -race`, golangci-lint, image build.
- **SDD artifacts:** PRD, architecture, per-story acceptance criteria
  ([specs/](https://github.com/goncharovart/fan-selector-api/tree/main/specs))
  written before implementation. Each story closes with a self-contained PR.

Live endpoint: `https://fan-selector-...run.app/v1/match?q=3000&p=300`

---

## Skills

**Backend.** TypeScript/Node, Go (ramping), PostgreSQL, Redis, BullMQ, REST,
OpenAPI, OpenTelemetry, structured logging, idempotency, observability.

**Cloud / DevOps.** GCP Cloud Run, Cloud SQL, Cloud Build, Secret Manager,
Artifact Registry; Docker (multi-stage, distroless); GitHub Actions CI/CD;
Linux/systemd; nginx; custom Bash deploy pipelines.

**AI-assisted workflow.** Spec-driven development (BMad-Method, custom spec
templates), Claude Code as primary IDE companion, subagent parallelization for
independent tasks, automated test loops, prompt-based hooks for guardrails.

**Frontend.** Next.js, React, TypeScript, Tailwind, Framer Motion, Prisma.

**Domain.** HVAC system sizing, fan / duct / acoustic / life-cycle-cost
calculations, project specification, 1C ERP integration, CDEK logistics,
Yookassa / 54-FZ fiscal compliance.

---

## Education

**Don State Technical University** — engineering degree, Heating, Ventilation
and Air-Conditioning. 2013–2018.

---

## Languages

Russian — native · English — B2 (technical reading/writing fluent; speaking
improves under load).

---

## Notes for the recruiter

- **Go experience is recent.** Strong in TypeScript backend, currently
  building a real Go service to close the gap (see `fan-selector-api`).
  Honest about the ramp; not honest about pretending it's not a ramp.
- **AI-assisted shipping is my baseline, not an experiment.** Wentmarket
  shipped to production with Claude Code at every step — PRDs, architecture,
  per-feature stories, code, tests, deploy. I can talk through any commit.
- **Open to relocation** to Poland / Georgia / Belarus / other.
- Available immediately.
