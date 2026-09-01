SHELL := /bin/sh

.PHONY: doctor install generate generate-check fmt lint test test-integration test-e2e test-all compose-up compose-down migrate seed dev

doctor:
	@printf 'Node: '; node --version || true
	@printf 'pnpm: '; pnpm --version || true
	@printf 'Go: '; go version || printf 'unavailable; Docker image fallback remains available\n'
	@printf 'Docker: '; docker --version || true
	@printf 'Docker Compose: '; docker compose version || true

install:
	pnpm install --frozen-lockfile
	docker compose build api

generate:
	pnpm --filter @ause-discovery/frontend exec openapi-ts -i ../api/openapi.yaml -o src/api/generated

generate-check: generate
	@git diff --exit-code -- backend/generated frontend/src/api/generated

fmt:
	pnpm --filter @ause-discovery/frontend exec prettier --write "src/**/*.{ts,tsx,css}" "*.json" "../api/*.yaml" "../compose.yaml"

lint:
	pnpm --filter @ause-discovery/frontend lint

test:
	pnpm --filter @ause-discovery/frontend test
	docker build -f backend/Dockerfile -t ause-discovery-api:test .

test-integration:
	docker compose up --build --wait postgres meilisearch

test-e2e:
	pnpm --filter @ause-discovery/frontend exec playwright test

test-all: lint test

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

migrate:
	docker compose run --rm migrate

seed:
	@printf 'Catalog synchronization is not available before the data increment.\n'

dev:
	docker compose up postgres meilisearch -d
	pnpm --filter @ause-discovery/frontend dev
