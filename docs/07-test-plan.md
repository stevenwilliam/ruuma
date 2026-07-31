# Test Plan — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

---

## 1. Strategy

- **Domain layer:** exhaustive unit tests. It is pure, so tests are fast and need
  no mocks. Every BR-x.y rule has at least one test that references its ID in the
  test name.
- **Adapters:** integration tests against real dependencies (Postgres, MinIO) via
  Docker. No mocking the database.
- **API:** end-to-end handler tests over the router for the critical flows.
- **Money paths:** property/edge tests for rounding, over/underflow, and the
  integer invariants (BR-1.1.x).

## 2. Critical scenarios

> TODO(domain): the flows that must never break. Each becomes an e2e test.

## 3. QA checklist (pre-release)

- [ ] `go test ./...` green
- [ ] `go vet ./...` clean
- [ ] Migrations up **and** down apply cleanly on a fresh DB
- [ ] All BR-x.y rules have a referencing test
- [ ] `RUN-WHEN-BACK.md` steps pass against the Docker stack
- [ ] Docs in sync with behaviour
