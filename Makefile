SHELL := /bin/bash
COMPOSE := docker compose
API_DIR := apps/api
WEB_DIR := apps/web

.PHONY: help up down logs migrate-up migrate-down sqlc test test-go test-web lint lint-go lint-web seed build clean

help:
	@echo "Targets:"
	@echo "  up            build and start db + api + web"
	@echo "  down          stop the stack (keeps the database volume)"
	@echo "  logs          follow service logs"
	@echo "  migrate-up    apply database migrations"
	@echo "  migrate-down  roll every migration back"
	@echo "  sqlc          regenerate database code from db/queries"
	@echo "  test          run Go and web test suites"
	@echo "  lint          vet Go code and lint the web app"
	@echo "  seed          load the demo organizer, judges and event"
	@echo "  build         build the API and web production bundles"

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down

logs:
	$(COMPOSE) logs -f --tail=100

migrate-up:
	$(COMPOSE) run --rm api /app/migrate

migrate-down:
	$(COMPOSE) run --rm api /app/migrate -down

sqlc:
	cd $(API_DIR) && sqlc generate

test: test-go test-web

test-go:
	cd $(API_DIR) && go test ./...

test-web:
	cd $(WEB_DIR) && npm test -- --run

lint: lint-go lint-web

lint-go:
	cd $(API_DIR) && go vet ./...

lint-web:
	cd $(WEB_DIR) && npm run lint

seed:
	$(COMPOSE) run --rm api /app/seed

build:
	cd $(API_DIR) && go build ./...
	cd $(WEB_DIR) && npm run build

clean:
	$(COMPOSE) down -v
