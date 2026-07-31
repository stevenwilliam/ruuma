# Business Rules — ruuma

**Status:** Normative. Where this conflicts with any other document in the set,
this document wins.
**Version:** 0.1 (scaffold)
**Date:** 31 July 2026

Rules are identified `BR-x.y`. Engineering references these IDs in code comments
and test names.

---

## 1. Foundations

### 1.1 Money

> Applies only if ruuma handles money — confirm in D2 (see `00`).

- **BR-1.1.1** All monetary values are stored as `BIGINT` in the whole minor
  unit of the chosen currency. _TODO(domain): currency + whether a subunit exists._
- **BR-1.1.2** All arithmetic on money uses integers. Floating point is
  prohibited in any code path touching money.
- **BR-1.1.3** Percentage values round to the nearest whole unit, half-up:
  `round(amount * bps / 10000) = floor((amount * bps + 5000) / 10000)`.

### 1.2 Identifiers

- **BR-1.2.1** All primary keys are UUIDv7 — time-ordered for index locality,
  not sequential in a way that leaks volume.
- **BR-1.2.2** Human-facing identifiers (if any) use a CSPRNG + Crockford
  base32, are unique, non-guessable, and encode no user identity.
  _TODO(domain): format._

### 1.3 Time

- **BR-1.3.1** All timestamps are stored in UTC. The operating timezone for
  business-day logic is _TODO(domain)_.

---

## 2. Domain rules

> TODO(domain): the heart of ruuma. Each rule: an ID, a single unambiguous
> sentence, and (where useful) the failure mode it prevents. Group by entity or
> workflow. Examples of the *kind* of rule this section holds: state machines
> (what transitions are legal), gates (what must be true before an action),
> invariants (what must never happen), cutoffs and capacity, permissions.

_None defined yet._

---

## 3. Permissions matrix

> TODO(domain): roles × actions. What each role may and may not do.
