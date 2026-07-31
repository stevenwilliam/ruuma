# Local Dev Setup — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

---

## 1. Prerequisites

- Go 1.22+
- Docker + Docker Compose
- Node 20+ (only if working on `web/`)

## 2. First run

```bash
cp .env.example .env          # once compose/.env.example exist
docker compose up -d          # postgres, minio (+ mailpit)
go run ./cmd/api migrate      # apply migrations + seed
go run ./cmd/api serve        # http://localhost:8080/health
go test ./...
```

## 3. Everyday commands

```bash
go run ./cmd/api serve        # run API
go test ./...                 # unit + integration
go vet ./...                  # static checks
cd web && npm run dev         # SPA dev server (if UI)
```

> The `cmd/`, `internal/`, `db/`, and `web/` trees do not exist yet — this repo
> currently holds docs and conventions only. This file becomes real once the
> service is scaffolded.

## 4. Editor

Use `vi` in runbooks and shell instructions.

## 5. Notes carried from SCHOOLCATERING

- On this Windows host, compose Postgres could not bind port `55432` — publish on
  a different host port and point tests at it via a `TEST_PG_DSN` env var.
