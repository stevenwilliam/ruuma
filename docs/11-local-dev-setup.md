# Local Dev Setup — ruuma

**Version:** 1.0
**Date:** 2 August 2026

Development happens on the shared `claudedev` server, not a laptop (D6, D10).
All paths are absolute.

---

## 1. What is already on the server

| Tool | State | Notes |
|---|---|---|
| Go 1.26.5 | `/usr/local/go/bin/go` | add to `PATH` via `/etc/profile.d/` |
| PostgreSQL 18 | native, `127.0.0.1:5432` | database `ruuma` already created |
| Node 20 + npm 10 | `/usr/bin/node` | installed 2026-08-02 |
| Docker + compose v5 | `docker compose` | used for MinIO + mailpit only |
| WAHA | container `waha`, `127.0.0.1:3000` | **shared** across projects — do not run a second one |
| nginx | site `ruuma`, proxies `127.0.0.1:8080` | `http://ruuma.claudedev.local` |
| Project config | `/etc/ruuma/ruuma.env` | `APP_PORT`, `DATABASE_URL` |

**Deliberately not in the compose file:** PostgreSQL (native, shared) and WAHA
(shared container). Running a second Postgres would fork the data and a second
WAHA would fight over the WhatsApp session (D10).

## 2. First run

```bash
cd /home/dev/projects/ruuma
cp /home/dev/projects/ruuma/.env.example /home/dev/projects/ruuma/.env
vi /home/dev/projects/ruuma/.env          # fill secrets; never commit this file
docker compose -f /home/dev/projects/ruuma/docker-compose.yml up -d   # minio + mailpit
/usr/local/go/bin/go run ./cmd/api migrate
/usr/local/go/bin/go run ./cmd/api seed
/usr/local/go/bin/go run ./cmd/api serve   # http://127.0.0.1:8080/health
```

Frontend:

```bash
cd /home/dev/projects/ruuma/web
npm install
npm run dev          # Vite dev server, proxies /api to 127.0.0.1:8080
```

## 3. Everyday commands

```bash
make run             # API
make worker          # background worker
make migrate         # apply migrations
make migrate-down    # roll back one step (dev only)
make seed            # demo data: 3 stores, menu, staff for every role
make test            # unit tests
make test-integration  # drops + recreates ruuma_test, runs tagged tests
make test-e2e        # end-to-end journey
make test-security   # authz, IDOR, rate-limit, injection, JWT
make check           # vet + staticcheck + gosec + govulncheck + npm audit
make web-build       # production SPA build
```

## 4. Services and ports

| Service | URL | Credentials |
|---|---|---|
| API | `http://127.0.0.1:8080` | — |
| SPA (dev) | `http://127.0.0.1:5173` | — |
| Postgres | `127.0.0.1:5432/ruuma` | `/etc/ruuma/ruuma.env` |
| Test DB | `127.0.0.1:5432/ruuma_test` | recreated by `make test-integration` |
| MinIO | `http://127.0.0.1:9002` (console `9003`) | `.env` |
| mailpit | `http://127.0.0.1:8025` | — |
| WAHA | `http://127.0.0.1:3000` | `/etc/claudedev/whatsapp.env` |

Ports for MinIO are shifted off the defaults so they cannot collide with another
project on the shared server.

## 5. Notifications in dev

Set `notify.provider` to `log` in `sys_parameters` to keep WhatsApp sends in the
`notifications` table without touching the shared WAHA session. Set it to `waha`
only when deliberately testing a real send, and only to your own number.

## 6. Editor

Use `vi` in runbooks and shell instructions.

## 7. Notes carried from SCHOOLCATERING

- If a compose port cannot bind, publish on a different host port and point tests
  at it via an env var rather than editing hard-coded values.
- Never point the dev environment at production object storage; presigned URLs
  from a proxied CDN break uploads.
