# API Specification — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

REST over HTTP, `chi` router. JSON in/out. JWT auth. All responses share one
error model.

---

## 1. Conventions

- **Base path:** `/api/v1`
- **Auth:** `Authorization: Bearer <jwt>`. _TODO(domain): token lifetimes, refresh._
- **IDs:** UUIDv7 in path/params.
- **Idempotency:** mutating money/order-shaped endpoints accept an
  `Idempotency-Key` header and replay safely. _TODO(domain): which endpoints._
- **Pagination:** cursor-based; `?limit=&cursor=`.

## 2. Error model

Every error is:

```json
{ "error": { "code": "STRING_CODE", "message": "human readable", "details": {} } }
```

Codes are stable, mapped from `platform/apierror`. Driver/internal errors never
leak to clients.

| HTTP | When |
|------|------|
| 400 | validation / malformed |
| 401 | missing/invalid auth |
| 403 | authenticated but not permitted |
| 404 | not found / not visible to caller |
| 409 | conflict / state violation |
| 422 | semantically invalid (business rule) |
| 429 | rate limited |
| 500 | unexpected |

## 3. Endpoints

- `GET /health` — liveness.
- `GET /metrics` — Prometheus.

> TODO(domain): the resource endpoints, grouped by entity, each with method,
> path, auth/role, request, response, and the BR-x.y rules it enforces.
