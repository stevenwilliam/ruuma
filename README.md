# ruuma

> TODO(domain): one-line description of what ruuma is.

Greenfield product built in the ruuma house style: **Go (gin + gorm) +
PostgreSQL 18** backend, **React + TypeScript** frontend, hexagonal
architecture, pure and exhaustively-tested domain layer. Full product and
engineering spec lives in
[`docs/`](docs/) — [`docs/02-business-rules.md`](docs/02-business-rules.md) is
normative. Build/working conventions are in [`CLAUDE.md`](CLAUDE.md).

**Start here:** if this is the first working session, open
[`initial-start-prompt.md`](initial-start-prompt.md) — it's the prompt that
kicks off the discussion to define the product before any code is written.

## Stack

Go (latest) · gin · gorm · PostgreSQL 18 · S3/MinIO · JWT · Prometheus ·
React 18 + Vite + Tailwind. gorm is the ORM; money paths use raw SQL with
integer math. The domain layer is pure.

## Quick start (local)

```bash
cp .env.example .env
docker compose up -d          # postgres, minio (+ mailpit) — TODO once compose exists
go run ./cmd/api migrate      # apply migrations + seed
go run ./cmd/api serve        # http://localhost:8080/health
go test ./...
```

> Nothing under `cmd/`, `internal/`, or `db/` exists yet — the repo currently
> holds docs and conventions only. Code lands after the product is defined.

See [`docs/11-local-dev-setup.md`](docs/11-local-dev-setup.md) and
[`docs/RUN-WHEN-BACK.md`](docs/RUN-WHEN-BACK.md).
