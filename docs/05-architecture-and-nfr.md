# Technical Architecture & Non-Functional Requirements — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

---

## 1. Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.22+ | Transactional API + background workers |
| HTTP | `chi` v5 (+ cors) | Standard-library-shaped, no framework lock-in |
| Database | PostgreSQL 16 | |
| DB access | `pgx/v5` | Hand-written SQL. No ORM on money paths. |
| Migrations | numbered SQL, embedded | Forward-only in production |
| Auth | `golang-jwt/jwt/v5` | + `golang.org/x/crypto` for hashing |
| Object storage | S3-compatible / MinIO (`minio-go/v7`) | Images, proofs, PDFs |
| Observability | Prometheus (`client_golang`) + structured logs | `/metrics`, request logging |
| IDs | `google/uuid` (v7) | |
| Frontend | React 18 + Vite + TypeScript + Tailwind | Pin React 18 |

## 2. Layering

Hexagonal: `adapter → app → domain`, `platform` shared. `domain` is pure. See
`../CLAUDE.md §2` for the canonical layout — it governs.

## 3. Security

- Passwords hashed with bcrypt/argon2 (`x/crypto`). Never store plaintext.
- JWT signed, short-lived access + refresh. Rotate secrets via config.
- Rate limiting via `platform/ratelimit`.
- Input validated at the adapter edge; domain assumes valid input.
- Least-privilege DB user in production.

## 4. Performance / reliability targets

> TODO(domain): concurrency, p95 latency, expected load. Baseline: handle the
> real v1 load with headroom; graceful degradation over failure.

## 5. Observability

- `/health` liveness, `/metrics` Prometheus.
- Structured request logs with a request ID propagated through context.

## 6. Deployment

See `09-deployment.md`.
