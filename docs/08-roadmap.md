# Roadmap — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

---

## 1. Sequencing principle

Build in the order that lets ruuma **run one real end-to-end case**, then widen.
Front-load the operational spine (the thing that delivers the core value), not
the fun-to-demo surface. Everything before the first real run is cost; everything
after is investment against known demand.

## 2. Milestones

### M0 — Definition (now)
- [x] Repo, docs scaffold, `CLAUDE.md`, start prompt
- [ ] Product-definition discussion → fill 01, 02, 03
- [ ] Decision log + open questions resolved in `00`

### M1 — Foundation
*Goal: the domain model exists and is correct.*
- [ ] Module, CI, `.env.example`, docker-compose, Makefile, Dockerfile
- [ ] Schema, numbered migrations, seed
- [ ] Auth (register, login, refresh) + role/permission matrix
- [ ] `platform/*` copied & adapted from SCHOOLCATERING
- [ ] Pure domain packages + exhaustive tests

### M2 — First vertical slice
> TODO(domain): the single end-to-end workflow that proves ruuma works.

### M3+ — Widen
> TODO(domain).
