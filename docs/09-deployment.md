# Deployment — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

Target: Ubuntu host, Docker (or systemd), TLS, automated backups. Mirrors the
SCHOOLCATERING production runbook shape.

---

## 1. Topology

> TODO: single-node vs. split; reverse proxy (Caddy/nginx) for TLS; Postgres and
> object storage placement.

## 2. Build & release

- Multi-stage `Dockerfile` → small static Go binary.
- Migrations run on deploy (`cmd/api migrate`) before the new binary serves.

## 3. Configuration

- All config via env; documented in `.env.example`. No secrets in the image.

## 4. TLS

> TODO. Note the CDN caveat carried from SCHOOLCATERING: if a CDN fronts object
> storage, it must be **DNS-only (unproxied)** — a proxied CDN breaks presigned
> uploads.

## 5. Backups & recovery

> TODO: Postgres dump cadence + retention, object-storage backup, restore drill.

## 6. Rollback

> TODO: keep previous image; migrations must have working `.down.sql`.
