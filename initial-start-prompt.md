# ruuma — initial start prompt

Paste the block below into a fresh Claude Code session opened in the `ruuma`
repo. It sets the ground rules and drives the first discussion that turns ruuma
from an empty scaffold into a defined product. Do not skip the discussion —
`CLAUDE.md` forbids code before the product is defined in the docs.

---

```
You are starting work on ruuma, a greenfield project.

Before anything else:
1. Read CLAUDE.md in full — it is the build contract. Follow it exactly,
   including: hexagonal architecture (domain/app/adapter/platform), Go + gin +
   gorm + PostgreSQL 18 (raw SQL on money paths), money-as-integers, UUIDv7 ids,
   numbered migrations, search box on every list, configurable values in
   sys_parameters with CRUD, docs-are-normative, editor is vi, full absolute
   paths in OS guides, and auto-commit + push after every completed change.
2. Skim docs/00 through docs/11 and docs/PROGRESS.md so you know the doc set and
   its current (placeholder) state.

Then STOP and do NOT write any code. First we define the product together.
Interview me to fill the TODO(domain) gaps, one topic at a time, waiting for my
answer before moving on. Cover, in this order:

  A. Problem — what pain does ruuma remove, and for whom? Who are the personas?
  B. Scope — the thin v1 slice: the smallest thing that is genuinely useful end
     to end. What is explicitly OUT of v1?
  C. Core nouns — the 5–10 central entities and how they relate.
  D. Money & identity — does ruuma handle money? What currency, timezone,
     language? What are the human-facing identifiers?
  E. Business rules — the non-negotiable logic, the failure modes, what must
     never happen. These become BR-x.y rules.
  F. Interfaces — API surface and whether there is a UI.

As decisions land, WRITE THEM INTO THE DOCS in my house style (mirroring the
existing headers, tone, and the BR-x.y convention in 02-business-rules.md),
keeping every affected doc in sync, and update docs/00's decision log and
docs/PROGRESS.md. Commit and push after each doc lands.

Only once 01 (PRD), 02 (business rules), and 03 (data model) are solid do we
talk about scaffolding the Go service. Ask me before you start writing code.

Ground rule for this repo: the SCHOOLCATERING project (a sibling folder) is the
reference for style and for the reusable internal/platform/* packages. Prefer
adapting those proven shapes over inventing new ones.
```

---

## Notes for the human

- If you already know the domain, you can paste answers to A–F straight into the
  prompt to skip the back-and-forth.
- The reusable `internal/platform/*` packages (config, logging, metrics,
  apierror, id, security, ratelimit, database) can be copied from
  `D:\go\SCHOOLCATERING` and renamed — that's the fastest way to keep the
  backend style identical.
