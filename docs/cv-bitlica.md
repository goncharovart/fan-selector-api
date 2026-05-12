# Артур Гончаров

**Backend / Full-stack инженер · HVAC-инженер**

Telegram: [@gonartur](https://t.me/gonartur) · Телефон: +7 (995) 376-31-73 · Email: goncharov.artur.02@gmail.com

Главный проект: [wentmarket.ru](https://wentmarket.ru) · Pet: [github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api) · [github.com/goncharovart/fan-curve-fitter](https://github.com/goncharovart/fan-curve-fitter)

---

## Кратко

Backend-leaning full-stack разработчик с **6 годами в HVAC-инжиниринге** и ~2 годами production-веба. Работаю spec-first с AI-агентами (Claude Code, Cursor) на всех стадиях — от PRD до деплоя. Ниша — домены, где алгоритм значит столько же, сколько и инфраструктура: подбор вентилятора по рабочей точке, расчёт потерь давления в воздуховодах, акустика, life-cycle cost.

Primary stack — TypeScript/Node; сейчас наращиваю Go через реальный side-проект, чтобы расширить backend-инструментарий.

---

## Опыт

### Wentmarket — основатель / lead engineer · 2024 — наст. время
B2B-платформа для HVAC-инженеров. В продакшне на [wentmarket.ru](https://wentmarket.ru). Собрана соло, AI-assisted (Claude Code). Реальные пользователи, заказы, платежи.

- **Движок подбора вентиляторов.** 17 000+ полиномиальных кривых в PostgreSQL с GiST range-фильтрацией; на каждый запрос решается `P_fan(Q) = P_target`, отдаются ранжированные результаты за p95 < 100 ms.
- **Pipeline заказов.** Server actions + BullMQ воркеры + Redis L2 кеш. Интеграция Yookassa с детерминированными SHA-256 idempotency-ключами и 54-ФЗ чеками. Валидация вебхуков через API-call (а не HMAC), чтобы защититься от replay.
- **Интеграция CDEK.** Service-слой на 871 строк: OAuth refresh на 401, exponential backoff (3 попытки, max 8 c), 30 c timeout, полный flow заказ/курьер/отслеживание/накладная.
- **Soft-delete** через Prisma `$extends` на 8 моделях с согласованной фильтрацией всех запросов.
- **Headless Bitrix24** — грузит вендорский виджет, скрывает его UI через инжектируемый CSS + MutationObserver, экспонирует `window.openB24Chat()` для собственных триггеров. Без двух конкурирующих чат-кнопок.
- **Sync с 1С** через CommerceML XML; pipeline товаров/цен/остатков.
- **Meilisearch** с graceful fallback на Prisma `LIKE` при недоступности индексатора — одна видимая деградация вместо краша.
- **Стек:** Next.js 16, React 19, TypeScript, Prisma 7, PostgreSQL, Redis (BullMQ + cache), Meilisearch, Sentry, NextAuth v5.
- **Инфра:** self-hosted VPS, systemd + nginx, кастомный deploy pipeline (`tar | ssh | build-on-server`).

### Goal-Energo — HVAC project engineer · 2018 — 2024
Шесть лет проектирования систем вентиляции, дымоудаления, приточных установок для промышленных и коммерческих зданий. Внутренние Excel/скрипты для подбора, выросшие в инженерные движки Wentmarket.

- Полиномиальный фит кривых вентиляторов из лабораторных замеров → каталог коэффициентов, который сегодня лежит в основе Wentmarket.
- Методология подбора, принятая командой.
- Customer-facing: сбор требований по рабочей точке, формирование спецификаций и тендерных пакетов.

---

## Side-проекты — связанные через единый pipeline

### `fan-selector-api` — Go-микросервис
[github.com/goncharovart/fan-selector-api](https://github.com/goncharovart/fan-selector-api)

Standalone Go-сервис, в который вынесен движок подбора из Wentmarket. Spec-first (BMad-style).
**Цель:** показать Go + cloud-deploy + SDD-workflow на домене, который я знаю наизусть.

- **Go 1.25**, chi router, pgx/v5, go-redis/v9, OpenTelemetry, slog.
- **PostgreSQL 16** с GiST range-индексом на envelope; prefilter сжимает каталог до кандидатов за один indexed scan.
- **Bisection root finder** для `P_fan(Q) = P_target` с защитой от NaN/Inf, проверкой смены знака, обработкой границ. Бенчмарки: Horner eval 0.65 нс, bisection 53 нс, полный Evaluate на 50 кандидатах 3.7 мкс — у бюджета p95 100 мс остаётся 99 мс запаса.
- **Redis cache** с детерминированными SHA-256 ключами, ±10% jitter на TTL против стампида, graceful деградация на NopCache при отказе Redis.
- **Distroless multi-stage Docker**, non-root пользователь.
- **OpenTelemetry** с ручными spans вокруг `cache.get`/`cache.set`, `storage.candidates`, `match.evaluate`; атрибуты cache hit, candidate counts, value bytes, TTL.
- **Integration-тесты** через testcontainers-go: каждый CI-прогон поднимает реальный Postgres 16, применяет embedded-миграции, проверяет GiST prefilter.
- **Два готовых deploy-пути в репо** — `cloudbuild.yaml` для GCP Cloud Run (Artifact Registry + Cloud SQL + Secret Manager) и `fly.toml` для Fly.io.
- **GitHub Actions CI:** `go vet`, `go test -race`, golangci-lint v2, integration job, benchmark job, docker build.
- **SDD-артефакты:** PRD, architecture, per-story acceptance criteria ([specs/](https://github.com/goncharovart/fan-selector-api/tree/main/specs)) пишутся **до** кода. Каждая story закрывается отдельным self-contained PR.

Локально: `docker compose up -d && go run ./cmd/server`.

### `fan-curve-fitter` — Python CLI
[github.com/goncharovart/fan-curve-fitter](https://github.com/goncharovart/fan-curve-fitter)

Питоновский CLI, который читает CSV-замеры вентилятора, фитит полиномы через `numpy.polyfit` и отдаёт JSON в формате, который `fan-selector-api` ест без преобразования. **End-to-end pipeline:** лабораторные данные → Python → JSON → Go-сервис.

- Python 3.11 / 3.12 / 3.13 (matrix-CI), Typer + Rich UI, pydantic v2, numpy.
- 24 теста, 90% coverage, ruff + mypy strict зелёные.
- Horner-эвалюация полинома **синхронизирована** с Go-реализацией — output и input численно идентичны.

---

## Навыки

**Backend.** TypeScript/Node, Go (наращиваю), PostgreSQL, Redis, BullMQ, REST, OpenAPI, OpenTelemetry, structured logging, идемпотентность, наблюдаемость.

**Cloud / DevOps.** Docker (multi-stage, distroless); GitHub Actions CI/CD; Linux/systemd; nginx; кастомные bash deploy-пайплайны (`tar | ssh | build-on-server`). Deploy-конфиги под GCP Cloud Run (Cloud Build / Cloud SQL / Secret Manager) и Fly.io — на уровне ready-to-apply, продакшн-операции пока не вёл.

**AI-assisted workflow.** Spec-driven development (BMad-Method, кастомные spec-шаблоны), Claude Code как primary IDE-компаньон, параллелизация задач через subagents, автоматические test-loops, prompt-based hooks как guardrails.

**Frontend.** Next.js, React, TypeScript, Tailwind, Framer Motion, Prisma.

**Python.** Typer/Click, numpy, pydantic v2, pytest, ruff, mypy strict.

**Domain.** Подбор HVAC-систем, расчёт вентилятор / воздуховод / шумоглушитель / LCC, формирование спецификаций, интеграция 1С, логистика CDEK, фискалка Yookassa / 54-ФЗ.

---

## Образование

**ДГТУ (Донской государственный технический университет)** — инженер, специальность «Теплогазоснабжение и вентиляция». 2013 — 2018.

---

## Языки

Русский — родной · Английский — B2 (техническое чтение/письмо свободно; устный — улучшается под нагрузкой).

---

## Заметки для рекрутера

- **Go-опыт — недавний.** Сильно владею TypeScript backend, прямо сейчас довожу до production реальный Go-сервис (см. `fan-selector-api`). Честно про ramp; не делаю вид, что ramp'а нет.
- **AI-assisted shipping — baseline, не эксперимент.** Wentmarket отгружен в продакшн с Claude Code на каждом шаге: PRD, architecture, per-feature stories, код, тесты, деплой. Могу детально обсудить любой коммит.
- **Открыт к релокации** в Польшу / Грузию / Беларусь / другое.
- Готов начать сразу.
