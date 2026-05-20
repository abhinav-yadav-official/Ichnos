OPENSEARCH_HOST_PORT ?= 9200

.PHONY: up down build run-crawler run-api push deploy

up:
	OPENSEARCH_HOST_PORT=$(OPENSEARCH_HOST_PORT) docker compose up -d

down:
	OPENSEARCH_HOST_PORT=$(OPENSEARCH_HOST_PORT) docker compose down

build:
	go build ./cmd/crawler
	go build ./cmd/api

run-crawler:
	go run ./cmd/crawler

run-api:
	go run ./cmd/api

push:
	git push origin main

deploy:
	ssh abhiyadav.in "cd ~/Ichnos && git pull && docker compose -f docker-compose.prod.yml up -d --build"
