# Deployment — fan-selector-api on Fly.io

Alternative to Cloud Run. Same Dockerfile, same binary, runs on Fly's
managed Firecracker VMs instead of Google's managed containers.

## Prerequisites

1. Fly.io account (`flyctl auth signup`)
2. `flyctl` CLI installed

## One-time setup

```bash
# 1. Create the app stub. Skip image build — we'll deploy explicitly.
flyctl apps create fan-selector-api --org personal

# 2. Provision Postgres. Fly Postgres is managed Postgres on top of Fly Machines.
#    pick 'Development - single node' for the free tier; ~$1.94/mo for the
#    shared-cpu-1x VM if you exceed free allowances.
flyctl postgres create \
    --name fan-selector-db \
    --region fra \
    --vm-size shared-cpu-1x \
    --volume-size 1 \
    --initial-cluster-size 1

# 3. Attach Postgres to the app — sets DATABASE_URL automatically.
flyctl postgres attach fan-selector-db --app fan-selector-api

# 4. Redis is optional. Upstash Redis on Fly's marketplace is free up to
#    10K commands/day. Skip if you don't need caching for the demo.
flyctl redis create \
    --name fan-selector-cache \
    --org personal \
    --region fra \
    --no-replicas \
    --plan free

# 5. Wire REDIS_URL into the app (Upstash gives the dsn after create).
flyctl secrets set REDIS_URL="redis://default:PASSWORD@fly-fan-selector-cache.upstash.io:6379" \
    --app fan-selector-api

# 6. Deploy. Fly builds the image remotely; you don't need Docker locally.
flyctl deploy --app fan-selector-api --remote-only
```

## Apply migrations

Fly Postgres doesn't run goose automatically. Easiest is a one-shot job
from a machine that already has Postgres access:

```bash
# Open a shell with DATABASE_URL preset inside the app's network
flyctl ssh console --app fan-selector-api -C "sh"

# Inside the container — but we built distroless without /bin/sh, so use
# `fly proxy` from your laptop instead:
flyctl proxy 5432 --app fan-selector-db

# In another terminal, with goose installed locally:
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir=./migrations postgres "host=127.0.0.1 port=5432 user=postgres dbname=fan_selector_api sslmode=disable password=$FLY_PG_PASSWORD" up
```

## Smoke-test

```bash
flyctl status --app fan-selector-api
URL=https://fan-selector-api.fly.dev
curl -s "$URL/healthz"
curl -s "$URL/v1/match?q=3000&p=300" | jq
```

## Useful

```bash
flyctl logs --app fan-selector-api          # tail logs
flyctl ssh console --app fan-selector-api   # interactive shell (only works if image has sh)
flyctl machine list --app fan-selector-api  # see running VMs
flyctl scale count 1 --app fan-selector-api # ensure 1 running machine
```

## Rollback

```bash
flyctl releases --app fan-selector-api
flyctl deploy --image registry.fly.io/fan-selector-api:deployment-PREVIOUS --app fan-selector-api
```
