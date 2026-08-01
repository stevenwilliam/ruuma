# Deployment — ruuma

**Version:** 1.0
**Date:** 2 August 2026

Production topology and policy. The copy-paste procedure for an empty machine is
`14-production-deployment-handbook.md`; this document says *what* and *why*.

---

## 1. Topology (D21)

Single Ubuntu node, all absolute paths:

| Component | Where | Notes |
|---|---|---|
| nginx | `/etc/nginx/sites-available/ruuma` | TLS termination, reverse proxy, static SPA |
| API + worker | `/opt/ruuma/bin/ruuma`, systemd units `ruuma-api`, `ruuma-worker` | listens on `127.0.0.1:8080` only |
| PostgreSQL 18 | native, `/var/lib/postgresql/18/main` | local socket; no public port |
| MinIO | systemd, `/var/lib/minio`, `127.0.0.1:9000` | private buckets, presigned URLs |
| SPA build | `/opt/ruuma/web` | served by nginx |
| Config | `/etc/ruuma/ruuma.env` (mode 600, owner `ruuma`) | secrets only here |
| Backups | `/var/backups/ruuma` | nightly, off-node copy |

Domains: `ruuma.id` (customer) and `admin.ruuma.id` (admin SPA); both hit the
same API, which serves `/api/v1`. The admin router group is separate in code
(`12-security.md` A01).

## 2. Build & release

- Multi-stage `Dockerfile` → static Go binary (`CGO_ENABLED=0`), or `make build`
  on the host. The SPA builds to `web/dist` and is copied to `/opt/ruuma/web`.
- Release order: build → **`ruuma migrate`** → restart `ruuma-api` → restart
  `ruuma-worker`. The new binary never serves against an unmigrated schema.
- Migrations are forward-only in production; `.down.sql` exists and is CI-tested
  but is a development and rollback-of-last-resort tool.
- Version and git SHA are compiled in and exposed at `/health`.

## 3. Configuration

Everything through `/etc/ruuma/ruuma.env`, documented in `.env.example`. No
secret is ever in the image, the repo, or a log line. Business configuration does
**not** live here — it lives in `sys_parameters` (BR-1.4.1), so the group can
retune without a deploy.

Required keys: `APP_ENV`, `APP_PORT`, `APP_BASE_URL`, `DATABASE_URL`,
`JWT_SIGNING_KEY`, `JWT_PREVIOUS_KEY` (rotation), `MINIO_ENDPOINT`,
`MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`, `SMTP_*`, `WAHA_URL`,
`WAHA_SESSION`, `WAHA_API_KEY`, `GOOGLE_OAUTH_*`, `INSTAGRAM_OAUTH_*`,
`CORS_ALLOWED_ORIGINS`, `TRUSTED_PROXIES`.

**Secret rotation:** JWT keys rotate by setting `JWT_PREVIOUS_KEY` to the old key
for one refresh-token lifetime, then removing it. Database and MinIO credentials
rotate by creating the new credential, deploying, then revoking the old one.

## 4. TLS

certbot (nginx plugin) for `ruuma.id`, `www.ruuma.id`, `admin.ruuma.id`.
TLS 1.2+ only, HSTS with `includeSubDomains; preload` once the domain is
confirmed stable. Auto-renewal via the packaged systemd timer, monitored.

> **CDN caveat carried from SCHOOLCATERING:** if a CDN ever fronts object
> storage, it must be **DNS-only (unproxied)** — a proxied CDN breaks presigned
> uploads.

## 5. Least privilege

- Postgres: the app connects as `ruuma_app`, which has `SELECT/INSERT/UPDATE/
  DELETE` on business tables, **`INSERT`-only** on `order_events`,
  `payment_events` and `audit_log` (BR-2.10.2), and no `CREATE`. Migrations run
  as the owner role, not the app role.
- MinIO: the app credential can read/write only the ruuma bucket; the bucket
  policy is private and objects are reachable only by presigned URL.
- systemd units run as the unprivileged `ruuma` user with `NoNewPrivileges`,
  `ProtectSystem=strict`, `PrivateTmp`, and a read-only filesystem except the
  runtime directory.
- `/metrics` binds to `127.0.0.1` and is never proxied publicly.

## 6. Backups & recovery

- **Nightly** `pg_dump -Fc` to `/var/backups/ruuma/db/`, 30 daily + 12 monthly
  retained, copied off-node.
- **MinIO** bucket mirrored nightly to the same off-node target — payment proofs
  are financial evidence.
- `/etc/ruuma/ruuma.env` backed up separately, encrypted.
- **Restore drill quarterly**, timed against the 30-minute recovery target:
  restore the dump into a scratch database, run `ruuma migrate` (no-op), point a
  staging binary at it, verify order counts and one payment proof fetch.

## 7. Rollback

1. Keep the previous binary at `/opt/ruuma/bin/ruuma.previous` and the previous
   SPA build at `/opt/ruuma/web.previous`.
2. Roll back the binary and SPA first; a forward-compatible migration usually
   makes this sufficient.
3. Only if the migration is the problem: apply its `.down.sql` — reviewed and
   CI-tested — then redeploy the previous binary.
4. Every rollback is recorded in the release log with the reason.

## 8. Notification provider in production

Phase 1 sends customer WhatsApp through **WAHA**, which is an unofficial
WhatsApp-Web gateway; Steven has accepted that risk (D11). Operational
consequences to plan for: the session can drop and needs re-linking by QR, and
the number can be rate-limited or banned. Mitigations: session state is
persisted on a volume, failures retry with backoff and land in the
`notifications` table, and `notify.provider` can be switched to `meta_cloud` (or
`log`) from `sys_parameters` without a deploy.
