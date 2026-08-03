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
make uat             # publish that build on port 80 for LAN testers (§7)
```

## 4. Services and ports

| Service | URL | Credentials |
|---|---|---|
| API | `http://127.0.0.1:8080` | — |
| SPA (dev) | `http://127.0.0.1:5173`, or `http://192.168.88.101:5173` via `make web-dev-lan` (§6) | — |
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

## 6. Seeing the SPA from your own machine

`claudedev` is headless — no desktop, no browser. `make web-dev` binds Vite to
`127.0.0.1`, which nothing outside the box can reach, so the dev server has to
be published deliberately.

```bash
cd /home/dev/projects/ruuma
make web-dev-lan     # binds 0.0.0.0:5173 instead of 127.0.0.1:5173
```

That alone is not enough: ufw defaults to deny-incoming and 5173 is not in the
allow list.

**Check the source address first.** Developer laptops are not on the server's
`192.168.88.0/24` — they reach the box through a router that NATs them, and the
server sees `172.16.0.1`. A rule written against the wrong subnet silently fails
to match. Confirm what your own connection looks like:

```bash
ss -tnH state established '( sport = :22 )'
# 192.168.88.101:22 <- 172.16.0.1:50779   <- the address to allow
```

Then open the port to **that address only** — never `Anywhere`, because the Vite
dev server has no authentication and serves project source over `/@fs`:

```bash
sudo ufw allow from 172.16.0.1 to any port 5173 proto tcp comment 'ruuma vite dev'
```

Avoid `172.16.0.0/12`: it swallows the Docker bridges (`172.17.0.0/16`,
`172.18.0.0/16`) and would expose the dev server to every container.

Then browse `http://192.168.88.101:5173`. The `/api` proxy in
`/home/dev/projects/ruuma/web/vite.config.ts` still points at `127.0.0.1:8080`,
which resolves on the server side, so the API needs no extra exposure.

`WEB_DEV_HOST` overrides the bind address if `0.0.0.0` is too wide — for example
`make web-dev-lan WEB_DEV_HOST=192.168.88.101` keeps it off the Docker bridges.

Close the port again when you are done:

```bash
sudo ufw delete allow from 172.16.0.1 to any port 5173 proto tcp
```

### Preferred: an SSH tunnel

This changes nothing on the server, needs no sudo, and works even when the path
between laptop and server permits nothing but port 22 — which is the common
case, and the reason the ufw route often fails for reasons ufw cannot fix:

```bash
ssh -L 5173:127.0.0.1:5173 dev@192.168.88.101   # then browse http://localhost:5173
```

Pair it with plain `make web-dev` — the tunnel terminates on the server's
loopback, so the dev server never has to bind a public interface at all.

If `http://192.168.88.101/` (port 80, already allowed in ufw) also fails from
your laptop, the block is upstream of the server and only the tunnel will work.

## 7. UAT on the local network

For user-acceptance testing, do **not** expose the Vite dev server. Serve the
production build through nginx instead:

```bash
cd /home/dev/projects/ruuma
make uat            # web-build, then sudo scripts/uat-deploy.sh
```

Testers then open `http://192.168.88.101/` for the customer site and
`http://192.168.88.101/admin` to sign in as staff. **No firewall change is
needed** — ufw already allows port 80 from anywhere, which is the main reason
this route beats opening 5173.

Why the production build rather than the dev server:

| | Vite dev server | nginx + `web-build` |
|---|---|---|
| Firewall | needs a new ufw rule for 5173 | port 80 already open |
| Serves source | yes, over `/@fs` | no, minified bundle only |
| What testers exercise | unminified dev build with HMR | the artefact that ships |
| Bugs found | can differ from production | representative |

`scripts/uat-deploy.sh` publishes `web/dist` to `/opt/ruuma/web` — the same path
the production handbook uses — installs
`deploy/nginx/ruuma-uat.conf` as the `ruuma` site, and stands the stock
`default` site down so the bare IP reaches ruuma without a DNS entry. It keeps
the previous build at `/opt/ruuma/web.previous` and the previous nginx config at
`/etc/nginx/sites-available/ruuma.pre-uat`; the rollback commands are printed
when it finishes.

To publish a new build during UAT, run `make uat` again.

Sign-in works over plain HTTP because the SPA holds its JWT in `localStorage`
and sends it as an `Authorization: Bearer` header
(`/home/dev/projects/ruuma/web/src/lib/api.ts`) — there is no `Secure` cookie to
be dropped. That also means **LAN traffic is readable**: use seeded demo data
for UAT, never real customer records.

Staff accounts come from `make seed`, which prints the password once. If it has
scrolled away, re-seed with `SEED_PASSWORD` set in the environment or mint an
account with `/usr/local/go/bin/go run ./cmd/api create-owner`.

## 8. Editor

Use `vi` in runbooks and shell instructions.

## 9. Notes carried from SCHOOLCATERING

- If a compose port cannot bind, publish on a different host port and point tests
  at it via an env var rather than editing hard-coded values.
- Never point the dev environment at production object storage; presigned URLs
  from a proxied CDN break uploads.
