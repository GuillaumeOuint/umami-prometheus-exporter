# Umami Prometheus Exporter

A small Go exporter that fetches data from an Umami Analytics instance and exposes Prometheus metrics.

The exporter logs in to Umami using credentials and refreshes data periodically (default 1m) so Prometheus scrapes use cached metrics. Metrics are exposed on /metrics and health on /healthz.

Project layout

- [`cmd/exporter/main.go`](cmd/exporter/main.go:1) - entrypoint
- [`internal/config/config.go`](internal/config/config.go:1) - environment configuration loader
- [`internal/umami/client.go`](internal/umami/client.go:1) - Umami API client (login + endpoints)
- [`internal/metrics/metrics.go`](internal/metrics/metrics.go:1) - Prometheus collectors and registration
- [`internal/updater/updater.go`](internal/updater/updater.go:1) - periodic fetcher that updates metrics
- [`internal/server/server.go`](internal/server/server.go:1) - HTTP server wiring (/metrics, /healthz)

Prerequisites

- Go 1.25+ (for local build)
- An Umami Analytics instance reachable from this exporter
- A Prometheus server to scrape this exporter

Build & run

1. Build locally

  go build -o umami-exporter ./cmd/exporter

2. Run

  UMAMI_URL=https://umami.example.com UMAMI_USERNAME=you UMAMI_PASSWORD=pass ./umami-exporter

3. Alternatively use go run:

  UMAMI_URL=https://umami.example.com UMAMI_USERNAME=you UMAMI_PASSWORD=pass go run ./cmd/exporter

Docker

- Build:

  docker build -t umami-exporter:latest .

- Run (use `.env` or pass envs directly):

  docker run --rm --env-file .env -p 9465:9465 umami-exporter:latest

Configuration

Copy [`.env.example`](.env.example:1) to `.env` and set values, or export the following environment variables:

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

Prometheus scrape example

scrape_configs:
  - job_name: 'umami'
    static_configs:
      - targets: ['umami-exporter:9465']
    metrics_path: /metrics

Health endpoint

- GET /healthz returns JSON:
  { "last_fetch": <unix>, "success": true|false }

Implementation notes

- The exporter logs in to Umami using the provided credentials and caches the token in memory.
- The updater fetches websites and stats on a configurable interval (default 1m) and updates Prometheus metrics.
- Metrics are kept in memory and exposed via /metrics; the exporter avoids querying Umami on every scrape.

Security

- Keep credentials secure (use Docker secrets, Kubernetes secrets, or environment injection).
- Do not commit real credentials.

Development

- Code entrypoint: [`cmd/exporter/main.go`](cmd/exporter/main.go)
- Config loader: [`internal/config/config.go`](internal/config/config.go)
- Umami client: [`internal/umami/client.go`](internal/umami/client.go)
- Metrics registration: [`internal/metrics/metrics.go`](internal/metrics/metrics.go)
- Updater logic: [`internal/updater/updater.go`](internal/updater/updater.go)
- HTTP server: [`internal/server/server.go`](internal/server/server.go)

Contributing

- Feel free to open PRs or issues.

License

- MIT