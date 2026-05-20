OPENSEARCH_HOST_PORT ?= 9200

.PHONY: up down build run-crawler run-api push vps-setup deploy-env deploy-nginx deploy

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
	ssh abhiyadav.in "cd ~/Ichnos && git pull && sudo docker compose -f docker-compose.prod.yml up -d --build"

vps-setup:
	ssh abhiyadav.in "sudo apt update && sudo apt install -y docker.io docker-compose-plugin git && sudo systemctl enable --now docker && if [ ! -d ~/Ichnos ]; then git clone https://github.com/abhinav-yadav-official/Ichnos ~/Ichnos; fi"

deploy-env:
	scp .env.prod abhiyadav.in:~/Ichnos/.env.prod
	ssh abhiyadav.in "cp ~/Ichnos/.env.prod ~/Ichnos/.env && chmod 600 ~/Ichnos/.env.prod ~/Ichnos/.env"

deploy-nginx:
	scp docker/nginx/ichnos.locations.conf abhiyadav.in:/tmp/ichnos.locations.conf
	ssh abhiyadav.in "sudo install -m 0644 /tmp/ichnos.locations.conf /etc/nginx/snippets/ichnos.locations.conf && if ! sudo grep -qxF 'include /etc/nginx/snippets/ichnos.locations.conf;' /etc/nginx/snippets/site-locations.conf; then echo 'include /etc/nginx/snippets/ichnos.locations.conf;' | sudo tee -a /etc/nginx/snippets/site-locations.conf >/dev/null; fi && sudo nginx -t && sudo systemctl reload nginx"
