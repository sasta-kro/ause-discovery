SHELL := /bin/sh

GO_TOOLCHAIN_IMAGE := golang:1.27.0-alpine3.23@sha256:3747dcba41c8b0db3211fda4db61638b980e17ac5bb3c94460a975a9cfe19395
NODE_TOOLCHAIN_IMAGE := node:24.20.0-alpine3.23@sha256:0388af2af070cd4736a1567cfed02469ba117848845b4165d87a333edb53d2ca
SQLC_VERSION := v1.31.1
OAPI_CODEGEN_VERSION := v2.8.0
GOVULNCHECK_VERSION := v1.7.0
GO_MODULE_CACHE_VOLUME := ause-discovery-go-mod-cache
GO_TEST_NETWORK ?= ause-discovery_default
GO_TEST_DATABASE_URL ?= postgres://ause:ause@postgres:5432/postgres?sslmode=disable
GO_RUN := docker run --rm -v $(GO_MODULE_CACHE_VOLUME):/go/pkg/mod --mount type=bind,source="$(CURDIR)",target=/workspace

.PHONY: doctor install generate generate-database generate-openapi generate-openapi-go generate-openapi-typescript generate-check fmt lint test test-integration test-e2e test-all compose-up compose-down migrate seed dev check-action-pins check-action-pins-test check-frontend check-go-format go-mod-verify go-vet go-test go-test-integration check-integration check-source check-compose check-images vuln-report

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

# Requires a prior make install so api/generator has node_modules. The pinned
# container invokes the pure-JavaScript generator shim directly; containerized
# pnpm would otherwise try to purge a host-platform node_modules tree.
generate-openapi-typescript:
	docker run --rm --mount type=bind,source="$(CURDIR)",target=/workspace -w /workspace $(NODE_TOOLCHAIN_IMAGE) /workspace/api/generator/node_modules/.bin/openapi-ts -i /workspace/api/openapi.yaml -o /workspace/frontend/src/api/generated

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
	docker compose up -d --wait postgres meilisearch
	$(MAKE) go-test-integration

test-e2e:
	pnpm --filter @ause-discovery/frontend exec playwright test

test-all: check-source check-integration generate-check check-compose check-images vuln-report test-e2e

compose-up:
	docker compose up --build -d

compose-down:
	docker compose down

migrate:
	docker compose run --rm migrate

seed:
	docker compose build api
	docker compose up --wait postgres
	docker compose run --rm migrate
	docker compose run --rm --no-deps --entrypoint /usr/local/bin/ausectl api catalog sync

dev:
	docker compose up postgres meilisearch -d
	pnpm --filter @ause-discovery/frontend dev

check-action-pins:
	scripts/check-action-pins.sh

check-action-pins-test:
	scripts/check-action-pins.test.sh

check-frontend:
	pnpm --filter @ause-discovery/frontend lint
	pnpm --filter @ause-discovery/frontend test
	pnpm --filter @ause-discovery/frontend build
	VITE_PUBLIC_BASE_PATH=/ pnpm --filter @ause-discovery/frontend build

go-mod-verify:
	$(GO_RUN) -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go mod verify

check-go-format:
	@files="$$(git ls-files 'backend/**/*.go')"; \
	unformatted="$$($(GO_RUN) -w /workspace $(GO_TOOLCHAIN_IMAGE) gofmt -l $$files)"; \
	if [ -n "$$unformatted" ]; then printf 'gofmt required in:\n%s\n' "$$unformatted"; exit 1; fi

go-vet:
	$(GO_RUN) -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go vet ./...

go-test:
	$(GO_RUN) -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go test ./... -count=1

# Integration tests create isolated databases on the referenced PostgreSQL
# instance, so the URL must point at a server the test container can reach.
go-test-integration:
	$(GO_RUN) --network $(GO_TEST_NETWORK) -e AUSE_TEST_DATABASE_URL='$(GO_TEST_DATABASE_URL)' -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go test ./... -count=1

check-integration:
	@docker compose exec -T postgres pg_isready -U ause -d postgres >/dev/null 2>&1 || { printf 'PostgreSQL is not reachable through docker compose; start it with: docker compose up -d postgres\n'; exit 1; }
	$(MAKE) go-test-integration

check-source: check-action-pins check-frontend check-go-format go-mod-verify go-vet go-test

check-compose:
	docker compose config --quiet

check-images:
	docker build -f backend/Dockerfile -t ause-discovery-api:check .
	docker build -f frontend/Dockerfile --build-arg VITE_PUBLIC_BASE_PATH=/ause-discovery/ -t ause-discovery-web:check .
	docker build -f frontend/Dockerfile --build-arg VITE_PUBLIC_BASE_PATH=/ -t ause-discovery-web-root:check .

# govulncheck gates on reachable vulnerabilities in shipped Go code. The pnpm
# lockfile audit covers both shipped dependencies and development tooling.
vuln-report:
	$(GO_RUN) -w /workspace/backend $(GO_TOOLCHAIN_IMAGE) go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...
	pnpm audit --audit-level high
