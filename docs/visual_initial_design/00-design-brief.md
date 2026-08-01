# ruuma — Visual Design Brief

**Version:** 0.1
**Date:** 1 August 2026
**Purpose:** the single, self-contained document to hand to a visual-design tool
(Claude design / artifact mode) so it can produce ruuma's initial screens.

This brief **merges** everything design-relevant from `CLAUDE.md` and
`docs/00`–`05`, `10`. Verbatim copies of those sources are in `./sources/` for
reference — but this file is the one to drop in. Where this brief and
`02-business-rules.md` disagree, **business rules win**.

---

## 0. Read this first — what is and isn't decided

ruuma's **domain is not yet defined.** `docs/01-PRD.md`, `02-business-rules.md`
§2, and `03-data-model.md` are structured placeholders; decision **D8 — "what
ruuma actually is"** is still open, as are open questions Q1–Q4 in `docs/00`.

So this brief splits into two halves, and you should treat them differently:

- **§1–§6 are settled.** Stack, conventions, mandatory patterns, accessibility,
  error and data handling. Design against these as hard constraints.
- **§7–§8 are blank.** Product context and the domain screen inventory. They
  need Steven's answers before any domain screen can be designed truthfully.

**What can be designed right now, honestly:** the app shell, navigation frame,
authentication flow, the System Parameters admin module (fully specified — see
§5.2), the design-token system, and the complete component + state library
(§4, §6). That is a real, useful foundation and it does not depend on the domain.

**What cannot:** any domain screen. Inventing entities to fill a mockup would put
fictional nouns into the design that the PRD will later contradict.

---

## 1. Product context

| Field | Value |
|---|---|
| Codename | ruuma |
| Owner | stevenwilliam |
| Status | greenfield — domain undefined |
| Problem statement | **TODO(domain)** — Q1 |
| Personas | **TODO(domain)** — Q4 |
| v1 thin slice | **TODO(domain)** — Q4 |
| Handles money? | **TODO(domain)** — Q2 |
| Currency / timezone / language | **TODO(domain)** — Q2 |
| UI or API-only? | **TODO(domain)** — Q3 |
| Design direction / tone | **TODO(domain)** |

> The entire design system in `10-design-system.md` is prefaced *"only relevant
> if ruuma has a UI"* (Q3). If the answer to Q3 is API-only, this folder is moot.

---

## 2. Platform & technical constraints

Non-negotiable — the design must be buildable in exactly this:

| Constraint | Value |
|---|---|
| Framework | **React 18** (pinned — not 19) |
| Build | Vite 5 |
| Language | TypeScript 5 |
| Styling | **Tailwind 3** (not 4) |
| Tokens | CSS custom properties, consumed by Tailwind |
| Source layout | `web/src/{components,lib,pages}` |
| Breakpoint baseline | **Mobile-first — design at 360px**, then scale up |
| Themes | **Light (default) + dark**, both first-class |
| Accessibility floor | **WCAG AA** |

Design at 360px first. A layout that only works on desktop is non-conformant.

---

## 3. Non-negotiable UI rules

These come from `CLAUDE.md §7` and the normative `02-business-rules.md`. They are
rules, not preferences — a screen violating one is incomplete.

### 3.1 Search box on every list — **BR-1.5.1**

> Every screen that lists or tables data provides a search box that filters that
> data. A list without search is non-conformant.

Pattern: the search input sits **at the top of the list**, is **debounced**, and
searches the columns relevant to that entity. This is a global pattern, not a
per-screen decision. Every list mockup must show it.

### 3.2 Configurable values live in `sys_parameters` — **BR-1.4.1/.2/.3**

Anything that could change without a deploy — company phone, email, address, tax
rate, thresholds, feature toggles — is a row in `sys_parameters`, never
hard-coded. Design implication: **never bake such a value into a mockup as
static text.** Company phone in a footer, a tax rate on an invoice, a support
email — all render from config. Show them as bound values.

Parameters flagged `is_secret` are **masked in the UI** and never logged
(BR-1.4.3) — design a reveal/masked treatment for these.

### 3.3 Cursor pagination — no page numbers

The API is **cursor-based** (`?limit=&cursor=`), not offset-based. This forbids
the classic numbered pager (`1 2 3 … 47`) and any "jump to last page" control —
there is no total-page count to render. Use **"Load more"** or infinite scroll
with an explicit end-of-list state.

### 3.4 Identifiers are UUIDv7

Primary keys are UUIDs (BR-1.2.1) — long, ugly, and **not for display**. Never
surface a raw UUID as a user-facing reference. Human-facing codes, if the domain
needs them, use CSPRNG + Crockford base32 (BR-1.2.2, format TODO). Design a
"copy ID" affordance for support/debug contexts rather than printing UUIDs inline.

### 3.5 Money is integers — **BR-1.1.1/.2/.3**

*If* ruuma handles money (Q2, unconfirmed). Stored as `BIGINT` in whole minor
units; all arithmetic is integer; percentages round half-up. Design implication:
**format at the display edge only**, never show a partially-rounded intermediate,
and never design an input that implies float precision. Currency symbol,
separators, and subunit behaviour are TODO until Q2 is answered.

### 3.6 Time is UTC, displayed in the operating timezone

Stored `timestamptz` in UTC (BR-1.3.1); the business-day timezone is TODO(domain).
Design implication: any date that drives business logic (cutoffs, "today") needs
an unambiguous rendering — show the timezone where a boundary matters.

---

## 4. Design tokens — to be defined

`10-design-system.md` defines the *shape*; the values are TODO. Fill both
columns, and every text/background pair must clear **WCAG AA**.

| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg` | | | page canvas |
| `--surface` | | | cards, sheets |
| `--primary` | | | primary actions |
| `--text` | | | body text |

Still needed: semantic tokens (success / warning / danger / info), border and
focus-ring tokens, elevation, radius, spacing scale, and a type scale
(families, weights, sizes). Typography and the component inventory are both
open TODOs in `10`.

---

## 5. Screens that can be designed today

These are fully derivable from the settled docs. **No domain knowledge required.**

### 5.1 Authentication

JWT, short-lived **access + refresh** (`05` §3). Screens: sign in; signed-out /
session-expired; forgot + reset password if in scope. Passwords are hashed
server-side — the UI never implies recoverability. Rate limiting exists
(`platform/ratelimit`), so **429 needs a designed state** (see §6.2).

### 5.2 System Parameters (admin) — fully specified

The one module whose requirements are complete today. Full CRUD behind an admin
permission (BR-1.4.2), with these fields from `03-data-model.md §2.1`:

| Field | Type | UI note |
|---|---|---|
| `key` | text, unique | e.g. `company.phone` — monospace, unique-validated |
| `value` | text | typed on read per `data_type` |
| `data_type` | enum | `string \| int \| bool \| decimal \| json` — input varies by type |
| `description` | text | helper text |
| `is_secret` | bool | **masked in UI when true** (BR-1.4.3) |
| `updated_by` | uuid | attribution — show as user, not raw UUID |
| `created_at` / `updated_at` | timestamptz | rendered in operating tz |

Screens: **list** (with mandatory search box, §3.1), **create**, **read/detail**,
**edit**, **delete confirmation**. The `data_type` field driving different input
controls is the interesting design problem here — a `bool` and a `json` parameter
should not share one text box.

### 5.3 App shell

Navigation frame, header, user menu, permission-gated nav items (the permissions
matrix itself is TODO — `02` §3), theme toggle honouring `prefers-color-scheme`,
and responsive behaviour from 360px up.

---

## 6. States every screen must cover

### 6.1 Content states

Loading (skeleton preferred over spinner), empty (first-run vs. no-search-results
— these are **different** states and need different copy), partial / paginating,
and populated.

### 6.2 Error states — one model, mapped from the API

Every API error shares one envelope (`04` §2):

```json
{ "error": { "code": "STRING_CODE", "message": "human readable", "details": {} } }
```

Driver and internal errors **never** leak to clients — so the UI never renders a
stack trace or SQL text. Each status needs a designed treatment:

| HTTP | Meaning | UI treatment |
|---|---|---|
| 400 | validation / malformed | inline field errors from `details` |
| 401 | missing/invalid auth | re-auth prompt / redirect to sign in |
| 403 | authenticated, not permitted | explain, don't just hide |
| 404 | not found / not visible | neutral — must not confirm existence |
| 409 | conflict / state violation | explain what changed, offer refresh |
| 422 | business-rule violation | surface the rule in human terms |
| 429 | rate limited | show retry timing, disable submit |
| 500 | unexpected | generic + a support/correlation reference |

Note the 403/404 pair: 404 is documented as *"not found **or** not visible to
caller"*. The copy must not distinguish them, or the UI leaks existence.

### 6.3 Accessibility (WCAG AA floor)

AA contrast on every pair, visible focus rings, fully keyboard-navigable,
respects `prefers-reduced-motion` and `prefers-color-scheme`, correct labels and
roles, and errors announced to assistive tech — not colour-only.

---

## 7. Domain screens — blocked

**Nothing here yet.** Populating this section requires answers to §8. Once the
core nouns exist (`initial-start-prompt.md` topic C), each entity typically needs:
list (+ search, §3.1), detail, create/edit, delete confirmation, and any
state-transition actions its business rules define.

---

## 8. Questions blocking the domain screens

Carried from `docs/00` §3, plus the design-specific ones:

| # | Question | Blocks |
|---|---|---|
| Q1 | What is ruuma's domain / problem? | everything |
| Q2 | Money? Currency, timezone, language? | §3.5, §3.6, formatting |
| Q3 | User-facing UI, or API-only for v1? | **this entire folder** |
| Q4 | Personas, and the thin v1 slice? | §7, navigation, roles |
| Q5 | Design direction / tone — what should ruuma *feel* like? | §4 |
| Q6 | Brand colours or existing identity to honour? | §4 |
| Q7 | Primary device — phone, desktop, or both equally? | layout priority |
| Q8 | Roles for the permissions matrix (`02` §3)? | nav gating, §5.3 |

---

## 9. Handing work back

Design decisions are **not** final until they land in the docs. Per `CLAUDE.md`
§5 and §8, when a visual decision is made:

1. Write tokens, typography, and components into `docs/10-design-system.md`.
2. Log the decision in `docs/00-README-and-decisions.md` (next free ID after D7),
   with date and rationale.
3. Update `docs/PROGRESS.md`.
4. Any new mandatory UI pattern becomes a `BR-x.y` rule in `02-business-rules.md`.

A decision that isn't in the docs didn't happen.
