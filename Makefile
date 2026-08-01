SHELL := /bin/bash
GO     := /usr/local/go/bin/go
BIN    := ./bin/ruuma
PKG    := ./...
ROOT   := /home/dev/projects/ruuma

export PATH := /usr/local/go/bin:/home/dev/go/bin:$(PATH)

.PHONY: help
help: ## show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── build & run ───────────────────────────────────────────────────────────────
.PHONY: build
build: ## build the API binary
	$(GO) build -trimpath -ldflags "-s -w -X main.version=$$(git describe --tags --always --dirty) -X main.commit=$$(git rev-parse --short HEAD)" -o $(BIN) ./cmd/api

.PHONY: run
run: ## run the API
	$(GO) run ./cmd/api serve

.PHONY: worker
worker: ## run the background worker
	$(GO) run ./cmd/api worker

.PHONY: migrate
migrate: ## apply migrations
	$(GO) run ./cmd/api migrate

.PHONY: migrate-down
migrate-down: ## roll back one migration (dev only)
	$(GO) run ./cmd/api migrate --down 1

.PHONY: seed
seed: ## load demo data (3 stores, menu, staff for every role)
	$(GO) run ./cmd/api seed

# ── tests ─────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## unit tests (pure, no I/O)
	$(GO) test -race -count=1 $(PKG)

.PHONY: test-integration
test-integration: reset-testdb ## integration tests against ruuma_test
	$(GO) test -race -count=1 -tags=integration ./internal/adapter/... ./test/integration/...

.PHONY: test-e2e
test-e2e: reset-testdb ## end-to-end journey
	$(GO) test -count=1 -tags=e2e ./test/e2e/...

.PHONY: test-security
test-security: reset-testdb ## authz, IDOR, rate-limit, injection, JWT, uploads
	$(GO) test -count=1 -tags=security ./test/security/...

.PHONY: test-all
test-all: test test-integration test-security test-e2e ## everything

.PHONY: reset-testdb
reset-testdb: ## drop + recreate ruuma_test
	sudo -n -u postgres psql -q -c "DROP DATABASE IF EXISTS ruuma_test;" -c "CREATE DATABASE ruuma_test OWNER ruuma;"

.PHONY: test-br-coverage
test-br-coverage: ## every BR-x.y in docs/02 must be referenced by a test
	@bash $(ROOT)/scripts/br-coverage.sh

.PHONY: cover
cover: ## coverage report
	$(GO) test -count=1 -coverprofile=coverage.out $(PKG) && $(GO) tool cover -func=coverage.out | tail -1

# ── quality & security ────────────────────────────────────────────────────────
.PHONY: check
check: vet staticcheck gosec govulncheck no-shell-out web-audit ## full quality + security gate

.PHONY: vet
vet:
	$(GO) vet $(PKG)

.PHONY: staticcheck
staticcheck:
	@command -v staticcheck >/dev/null || $(GO) install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck $(PKG)

.PHONY: gosec
gosec:
	@command -v gosec >/dev/null || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -quiet -exclude-dir=web $(PKG)

.PHONY: govulncheck
govulncheck:
	@command -v govulncheck >/dev/null || $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck $(PKG)

.PHONY: no-shell-out
no-shell-out: ## A03 — the service must never shell out
	@! grep -rn --include=*.go -E '"os/exec"' ./cmd ./internal || (echo "os/exec is banned (docs/12 A03)"; exit 1)

.PHONY: fmt
fmt:
	$(GO) fmt $(PKG)

.PHONY: tidy
tidy:
	$(GO) mod tidy

# ── frontend ──────────────────────────────────────────────────────────────────
.PHONY: web-install
web-install:
	cd $(ROOT)/web && npm install

.PHONY: web-dev
web-dev:
	cd $(ROOT)/web && npm run dev

.PHONY: web-build
web-build:
	cd $(ROOT)/web && npm run build

.PHONY: web-test
web-test:
	cd $(ROOT)/web && npm run test -- --run

.PHONY: web-audit
web-audit: ## dependency gate: fails on any unaccepted high/critical advisory
	cd $(ROOT)/web && npm run audit

# ── dev environment ───────────────────────────────────────────────────────────
.PHONY: up
up: ## start MinIO + mailpit (Postgres and WAHA are native/shared — D10)
	docker compose -f $(ROOT)/docker-compose.yml up -d

.PHONY: down
down:
	docker compose -f $(ROOT)/docker-compose.yml down
