# Deployment — fan-selector-api on Google Cloud Run

Single-region, single-environment setup. ~$0–10/month at idle on Cloud Run's
free tier; scales to $50–80/month under sustained traffic.

## Prerequisites

1. A GCP project with billing enabled. Note the project ID.
2. `gcloud` CLI installed and `gcloud auth login` completed.
3. Required APIs enabled:

```bash
gcloud services enable \
    run.googleapis.com \
    cloudbuild.googleapis.com \
    artifactregistry.googleapis.com \
    sqladmin.googleapis.com \
    redis.googleapis.com \
    secretmanager.googleapis.com \
    cloudtrace.googleapis.com
```

## One-time infra setup

```bash
PROJECT=$(gcloud config get-value project)
REGION=europe-west1

# Artifact Registry repo for the container image
gcloud artifacts repositories create containers \
    --repository-format=docker \
    --location=$REGION

# Cloud SQL — Postgres 16, smallest tier
gcloud sql instances create fan-selector-db \
    --database-version=POSTGRES_16 \
    --tier=db-g1-small \
    --region=$REGION \
    --root-password="$(openssl rand -hex 16)"

gcloud sql databases create fan_selector --instance=fan-selector-db
gcloud sql users create fan_selector_app --instance=fan-selector-db \
    --password="$(openssl rand -hex 16)"

# Memorystore Redis (BASIC tier, 1GB) — optional, the app degrades gracefully without it
gcloud redis instances create fan-selector-cache \
    --size=1 --region=$REGION --tier=BASIC

# Secrets
gcloud secrets create DATABASE_URL --replication-policy=automatic
gcloud secrets create REDIS_URL --replication-policy=automatic

# Populate them — replace placeholders with the actual host/password
echo -n "postgres://fan_selector_app:PASSWORD@/fan_selector?host=/cloudsql/$PROJECT:$REGION:fan-selector-db&sslmode=disable" \
    | gcloud secrets versions add DATABASE_URL --data-file=-
echo -n "redis://REDIS_IP:6379/0" \
    | gcloud secrets versions add REDIS_URL --data-file=-

# Grant Cloud Run service account access to secrets
PROJECT_NUMBER=$(gcloud projects describe $PROJECT --format='value(projectNumber)')
SA="$PROJECT_NUMBER-compute@developer.gserviceaccount.com"
gcloud secrets add-iam-policy-binding DATABASE_URL \
    --member="serviceAccount:$SA" --role=roles/secretmanager.secretAccessor
gcloud secrets add-iam-policy-binding REDIS_URL \
    --member="serviceAccount:$SA" --role=roles/secretmanager.secretAccessor
```

## Apply migrations

The app does not auto-migrate. Run goose against Cloud SQL via the proxy:

```bash
# In one terminal:
cloud-sql-proxy $PROJECT:$REGION:fan-selector-db &

# In another:
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir=./migrations postgres "host=127.0.0.1 port=5432 user=fan_selector_app dbname=fan_selector sslmode=disable" up
```

## Deploy

From the repo root:

```bash
gcloud builds submit \
    --config=cloudbuild.yaml \
    --substitutions=_DB_INSTANCE=$PROJECT:$REGION:fan-selector-db
```

The build runs `go test`, builds the distroless image, pushes it to Artifact
Registry, and deploys to Cloud Run. The first deploy takes ~4 minutes; updates
take ~90 seconds.

## Smoke-test

```bash
URL=$(gcloud run services describe fan-selector-api --region=$REGION --format='value(status.url)')
curl -s "$URL/healthz"
curl -s "$URL/v1/match?q=3000&p=300" | jq
```

## Observability

- Logs: Cloud Logging, filter by `resource.labels.service_name="fan-selector-api"`.
- Traces: Cloud Trace, look for spans named `http.request`, `match.evaluate`.
- Set `OTEL_EXPORTER_OTLP_ENDPOINT` to a collector address if you want to
  ship traces somewhere other than Cloud Trace.

## Rollback

```bash
gcloud run services update-traffic fan-selector-api \
    --region=$REGION \
    --to-revisions=PREVIOUS_REVISION=100
```
