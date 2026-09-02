SHELL := /bin/sh

GO_TOOLCHAIN_IMAGE := golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395
NODE_TOOLCHAIN_IMAGE := node:24.20.0-alpine3.23@sha256:0388af2af070cd4736a1567cfed02469ba117848845b4165d87a333edb53d2ca
SQLC_VERSION := v1.31.1
OAPI_CODEGEN_VERSION := v2.8.0

.PHONY: doctor install generate generate-database generate-openapi generate-openapi-go generate-openapi-typescript generate-check fmt lint test test-integration test-e2e test-all compose-up compose-down migrate seed dev

doctor:
	@printf 'Node: '; node --version || true
	@printf 'pnpm: '; pnpm --version || true
	@printf 'Go: '; go version || printf 'unavailable; Docker image fallback remains available\n'
	@printf 'Docker: '; docker --version || true
	@printf 'Docker Compose: '; docker compose version || true

install:
	pnpm install --frozen-lockfile
	docker compose build api

generate: generate-database generate-openapi

generate-database:
	docker run --rm --mount type=bind,source="$(CURDIR)",target=/workspace -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

generate-openapi: generate-openapi-go generate-openapi-typescript

generate-openapi-go:
	docker run --rm --mount type=bind,source="$(CURDIR)",target=/workspace -w /workspace/api $(GO_TOOLCHAIN_IMAGE) go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config openapi.codegen.yaml openapi.yaml

generate-openapi-typescript:
	docker run --rm --mount type=bind,source="$(CURDIR)",target=/workspace -w /workspace $(NODE_TOOLCHAIN_IMAGE) sh -c 'corepack enable && pnpm --filter @ause-discovery/openapi-generator exec openapi-ts -i /workspace/api/openapi.yaml -o /workspace/frontend/src/api/generated'

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
	docker compose build api migrate
	docker compose up --wait postgres
	docker compose run --rm migrate
	docker compose run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync

dev:
	docker compose up postgres meilisearch -d
	pnpm --filter @ause-discovery/frontend dev
