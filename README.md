<div align="center">

# Ichnos

**Domain-specific web crawler and search engine in Go.**

[![Release](https://img.shields.io/github/v/release/abhinav-yadav-official/Ichnos?style=for-the-badge)](https://github.com/abhinav-yadav-official/Ichnos/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue?style=for-the-badge)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go&logoColor=white)](go.mod)
[![OpenSearch](https://img.shields.io/badge/Index-OpenSearch-005EB8?style=for-the-badge&logo=opensearch&logoColor=white)]()

</div>

## Overview

Ichnos crawls a target domain, indexes pages into OpenSearch, and serves full-text search through a lightweight web UI. It is built as a complete pipeline — fetcher to frontier to indexer to query API to UI — with Grafana dashboards for observability.

## Features

- **Crawler** — fetcher with a URL frontier, deduplication, and politeness controls (see [Concepts](#concepts)).
- **Indexer** — streams crawled pages and bulk-ingests them into an OpenSearch index.
- **Search API** — query builder over `chi` HTTP routes with Prometheus metrics.
- **Web UI** — server-rendered result cards via `htmx` + `html/template`.
- **Observability** — Grafana dashboards and alerts.

## Architecture

```
crawler ──> redis frontier ──> indexer ──> OpenSearch ──> search API ──> htmx UI
                                                   └──> Grafana / Prometheus
```

## Installation

Prereqs: Go 1.25+, Docker.

```sh
git clone https://github.com/abhinav-yadav-official/Ichnos.git
cd Ichnos
cp .env.example .env
make            # build / run targets — see Makefile
docker compose up
```

Production stack: `docker-compose.prod.yml` (crawler + api + redis + opensearch) with `.env.prod`, served behind an Nginx subpath (`BASE_PATH=/ichnos`).

## Concepts

- **URL frontier** — the prioritised queue of URLs still to crawl. Ichnos keeps it in Redis so crawl state survives restarts and can be shared across workers.
- **Deduplication filter** — a compact set membership structure rejects URLs already seen without storing every URL in full, keeping memory bounded on large crawls.
- **Politeness** — per-host rate limiting and crawl-delay so the crawler never hammers a single server.

## License

[MIT](LICENSE) © 2026 Abhinav Yadav
