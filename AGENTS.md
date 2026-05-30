# AGENTS.md

Guidance for Codex and other agents working in this repository.

## Project Purpose

Ichnos is a domain-specific web crawler and search engine written in Go. The
crawler fetches pages from configured seed URLs, pushes parsed page payloads
through Redis, bulk-indexes them into OpenSearch, and serves search results via
a small HTTP API and server-rendered htmx UI.

## Directory Structure

- `cmd/crawler/` - crawler binary. Runs crawl workers, the Redis stream
  indexer consumer, `/metrics`, and pprof on `:6060`.
- `cmd/api/` - search API binary. Serves UI/API routes on `:8080`.
- `internal/config/` - environment-based configuration loader.
- `internal/crawler/` - frontier, bloom dedupe, robots/politeness, fetcher,
  parser, and crawl orchestration.
- `internal/indexer/` - OpenSearch client, index mapping/alias setup, bulk
  indexing, and Redis stream consumer.
- `internal/search/` - query DSL, OpenSearch search service, chi router, and
  HTTP handlers.
- `internal/metrics/` - Prometheus metrics and queue-depth reporting.
- `templates/` - embedded HTML templates for the web UI.
- `docker/` - Dockerfiles, Prometheus/Grafana config, alerts, and nginx
  production snippets.
- `docker-compose.yml` - local Redis/OpenSearch/Prometheus/Grafana stack.
- `docker-compose.prod.yml` - production compose stack for the VPS deployment.

## Build, Run, Test

Prerequisites: Go 1.25+, Docker, and Docker Compose.

```sh
make build          # go build ./cmd/crawler and ./cmd/api
make up             # start local Redis, OpenSearch, Prometheus, Grafana
make down           # stop the local compose stack
make run-crawler    # run crawler process; needs env + Redis + OpenSearch
make run-api        # run API process; needs env + Redis + OpenSearch

go test ./...                                   # all tests
go test ./internal/crawler/                     # one package
go test ./internal/crawler/ -run TestFrontier   # one test regexp
go test ./... -race                             # race detector
```

Use `cp .env.example .env` for local environment values. Tests are plain Go
unit/table tests and use fakes such as `miniredis`; `go test ./...` should not
require running services.

## Key Conventions

- Configuration comes from environment variables only. When adding config, add
  the field in `internal/config/config.go`, update `requiredEnvVars` if it is
  mandatory, and keep `.env.example` in sync.
- Data flow is one-way: crawler -> Redis stream -> indexer consumer ->
  OpenSearch -> search API. The crawler and API do not call each other.
- The crawler binary currently starts both crawl workers and the indexer
  consumer in one process.
- Search reads from the OpenSearch alias defined by the indexer package; keep
  mapping, indexing fields, and query fields aligned.
- HTML templates are embedded via `templates/templates.go`; changing template
  names or funcs may require updates there and in `internal/search/http.go`.
- `BASE_PATH` is optional and normalized to `/name`. When set, routes are
  mounted both at `/` and at the base path for subpath deployments.
- Use standard Go formatting (`gofmt`) and keep tests close to the package they
  cover.

## Gotchas

- Both binaries fail fast when required environment variables are missing or
  invalid.
- The local compose stack runs only dependencies and observability services;
  use `make run-crawler` and `make run-api` to run the Go binaries locally.
- Local OpenSearch maps to `${OPENSEARCH_HOST_PORT:-9200}`; override
  `OPENSEARCH_HOST_PORT` if port 9200 is occupied.
- Production is configured for `/ichnos` behind nginx and uses
  `docker-compose.prod.yml`; do not assume root-path routing when editing prod
  config.
- Redis stream name and consumer group are configurable via `STREAM_NAME` and
  `CONSUMER_GROUP`; tests and code should avoid hard-coding those unless the
  surrounding code already does.
