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
	ssh abhiyadav.in 'set -eu; for pkg in docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc; do sudo apt-get remove -y $$pkg >/dev/null 2>&1 || true; done; sudo apt-get update; sudo apt-get install -y ca-certificates curl git; sudo install -m 0755 -d /etc/apt/keyrings; sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc; sudo chmod a+r /etc/apt/keyrings/docker.asc; . /etc/os-release; echo "deb [arch=$$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $${VERSION_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null; sudo apt-get update; sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin; sudo systemctl enable --now docker; if [ ! -d ~/Ichnos ]; then git clone https://github.com/abhinav-yadav-official/Ichnos ~/Ichnos; fi'

deploy-env:
	scp .env.prod abhiyadav.in:~/Ichnos/.env.prod
	ssh abhiyadav.in "cp ~/Ichnos/.env.prod ~/Ichnos/.env && chmod 600 ~/Ichnos/.env.prod ~/Ichnos/.env"

deploy-nginx:
	scp docker/nginx/ichnos.locations.conf abhiyadav.in:/tmp/ichnos.locations.conf
	ssh abhiyadav.in "sudo install -m 0644 /tmp/ichnos.locations.conf /etc/nginx/snippets/ichnos.locations.conf && if ! sudo grep -qxF 'include /etc/nginx/snippets/ichnos.locations.conf;' /etc/nginx/snippets/site-locations.conf; then echo 'include /etc/nginx/snippets/ichnos.locations.conf;' | sudo tee -a /etc/nginx/snippets/site-locations.conf >/dev/null; fi && sudo nginx -t && sudo systemctl reload nginx"
