# Umami Prometheus Exporter

A small, lightweight Go exporter that fetches metrics from an Umami Analytics instance and exposes them to Prometheus.

## Overview

The exporter authenticates to Umami using provided credentials and refreshes data on a configurable interval (default: 1m). Scrapes are served on /metrics and a health endpoint is available at /healthz. The exporter caches results between refreshes so Prometheus scrapes hit the local cache instead of querying the Umami API on every scrape.

Table of Contents

- [Project layout](#project-layout)
- [Prerequisites](#prerequisites)
- [Build & run](#build--run)
- [Docker](#docker)
- [Kubernetes deployment (optional)](#kubernetes-deployment-optional)
- [Configuration](#configuration)
- [Exposed metrics](#exposed-metrics)
- [Health endpoint](#health-endpoint)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Project layout

- [`cmd/exporter/main.go`](cmd/exporter/main.go) - entrypoint
- [`internal/config/config.go`](internal/config/config.go) - environment configuration loader
- [`internal/umami/client.go`](internal/umami/client.go) - Umami API client (login + endpoints)
- [`internal/metrics/metrics.go`](internal/metrics/metrics.go) - Prometheus collectors and registration
- [`internal/updater/updater.go`](internal/updater/updater.go) - periodic fetcher that updates metrics
- [`internal/server/server.go`](internal/server/server.go) - HTTP server wiring (/metrics, /healthz)
- [`deploy/`](deploy/) - Kubernetes manifests (deployment, service, secret, servicemonitor)

## Prerequisites

- Go 1.25+ (for local build)
- An Umami Analytics instance reachable from this exporter
- A Prometheus server to scrape this exporter (or Prometheus Operator / kube-prometheus-stack)

## Build & run

1. Build locally

   go build -o umami-exporter ./cmd/exporter

2. Run

   UMAMI_URL=https://umami.example.com UMAMI_USERNAME=you UMAMI_PASSWORD=pass ./umami-exporter

3. Alternatively use go run:

   UMAMI_URL=https://umami.example.com UMAMI_USERNAME=you UMAMI_PASSWORD=pass go run ./cmd/exporter

## Docker

- Build:

   docker build -t umami-exporter:latest .

- Run (use `.env` or pass envs directly):

   docker run --rm --env-file .env -p 9465:9465 umami-exporter:latest

### Docker Hub image

The official Docker image is available on Docker Hub at `guillaumeouint2/umami-exporter`. The current published tag is `1.0.1`.

You can pull and run the prebuilt image:

   docker pull guillaumeouint2/umami-exporter:1.0.1
   docker run --rm --env-file .env -p 9465:9465 guillaumeouint2/umami-exporter:1.0.1

## Kubernetes deployment (optional)

This repository contains Kubernetes manifests under the `deploy/` directory to run the exporter inside a cluster and expose it to Prometheus via ServiceMonitor (Prometheus Operator).

### Manifests (deploy/)

- [`deploy/deployment.yml`](deploy/deployment.yml) — Deployment + Service manifest (container port 9465)
- [`deploy/service.yml`](deploy/service.yml) — Service targeted by ServiceMonitor
- [`deploy/secret.yml`](deploy/secret.yml) — Secret template with environment variables
- [`deploy/servicemonitor.yml`](deploy/servicemonitor.yml) — ServiceMonitor for Prometheus Operator

#### Apply the manifests (example)

1. Create the secret (recommended way to avoid embedding credentials):

```bash
kubectl create secret generic umami-exporter-secret \
  --from-literal=UMAMI_URL='https://umami.example.com' \
  --from-literal=UMAMI_USERNAME='user' \
  --from-literal=UMAMI_PASSWORD='pass'
```

If you want to create the secret in a specific namespace, add --namespace <namespace> (or -n <namespace>) to the command above.

2. Apply deployment and service:

```bash
kubectl apply -f deploy/deployment.yml
kubectl apply -f deploy/service.yml
```

To apply to a specific namespace, add --namespace <namespace> (or -n <namespace>) to each command.

3. If you use Prometheus Operator, apply the ServiceMonitor:

```bash
kubectl apply -f deploy/servicemonitor.yml
```

Optionally add --namespace <namespace> if your ServiceMonitor is namespaced.

### Notes

- The `deploy/deployment.yml` uses image `umami-exporter:latest` by default. Replace this with your registry image (for example `myregistry/umami-exporter:1.0.1`) and push before applying:

```bash
docker build -t myregistry/umami-exporter:1.0.1 .
docker push myregistry/umami-exporter:1.0.1
```

- The ServiceMonitor selects services by labels. The example uses `app: umami-exporter` and a label `release: prometheus` on the ServiceMonitor; change the labels if your Prometheus Operator has a different selector.

- Ensure the Prometheus Operator and the ServiceMonitor CRD are installed in your cluster (for kube-prometheus-stack / prometheus-operator).

- Use SealedSecrets/ExternalSecrets or another secrets-management solution for production instead of committing credentials in YAML.

- Verify deployment:

```bash
kubectl get pods -l app=umami-exporter
kubectl get svc umami-exporter
kubectl get servicemonitor -l app=umami-exporter --all-namespaces
```

Configuration

Copy [`.env.example`](.env.example) to `.env` and set values, or export the following environment variables:

- UMAMI_URL (required) — base URL of your Umami instance, include scheme (https://...)
- UMAMI_USERNAME (required)
- UMAMI_PASSWORD (required)
- EXPORTER_PORT (default 9465)
- UMAMI_REFRESH_INTERVAL (default 1m) — Go duration string
- UMAMI_CONCURRENCY (default 5) — parallel requests to Umami
- UMAMI_METRIC_LIMIT (default 100) — per-type result limit
- UMAMI_METRIC_TYPES (csv) — types to fetch: url,referrer,browser,os,device,country,event
- UMAMI_HTTP_TIMEOUT (default 15s)

Exposed metrics

- umami_fetch_success (gauge): 1 if last refresh succeeded, 0 otherwise
- umami_last_fetch_timestamp_seconds (gauge): unix timestamp of last successful fetch
- umami_website_pageviews{website_id,name,domain}
- umami_website_visitors{website_id,name,domain}
- umami_website_visits{website_id,name,domain}
- umami_website_bounces{website_id,name,domain}
- umami_website_totaltime_seconds{website_id,name,domain}
- umami_website_active_visitors{website_id,name,domain}
- umami_metric_value{website_id,name,domain,type,value} — generic metric for types such as url/referrer/browser/etc.

## Prometheus scrape example (static scrape)

scrape_configs:
  - job_name: 'umami'
    static_configs:
      - targets: ['umami-exporter:9465']
    metrics_path: /metrics

If using Prometheus Operator / ServiceMonitor, the provided ServiceMonitor will configure scraping automatically when applied.

## Health endpoint

- GET /healthz returns JSON:
  { "last_fetch": <unix>, "success": true|false }

## Implementation notes

- The exporter logs in to Umami using the provided credentials and caches the token in memory.
- The updater fetches websites and stats on a configurable interval (default 1m) and updates Prometheus metrics.
- Metrics are kept in memory and exposed via /metrics; the exporter avoids querying Umami on every scrape.

## Security

- Keep credentials secure (use Docker secrets, Kubernetes secrets, or environment injection).
- Do not commit real credentials.

## Development

- Code entrypoint: [`cmd/exporter/main.go`](cmd/exporter/main.go)
- Config loader: [`internal/config/config.go`](internal/config/config.go)
- Umami client: [`internal/umami/client.go`](internal/umami/client.go)
- Metrics registration: [`internal/metrics/metrics.go`](internal/metrics/metrics.go)
- Updater logic: [`internal/updater/updater.go`](internal/updater/updater.go)
- HTTP server: [`internal/server/server.go`](internal/server/server.go)

## Contributing

- Feel free to open PRs or issues.

## License

- MIT