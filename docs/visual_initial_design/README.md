# visual_initial_design

Drop-in package for ruuma's **initial visual design** work.

## What to hand to the design tool

**`00-design-brief.md`** — that one file. It is self-contained: every
design-relevant constraint from `CLAUDE.md` and `docs/00`–`05`, `10` merged into
a single brief, with the open questions marked. You should not need to attach
anything else.

## Contents

| File | What it is |
|---|---|
| `00-design-brief.md` | **The deliverable.** Merged, self-contained design brief. |
| `sources/` | Verbatim copies of the source docs, for reference only. |

### `sources/`

Copied at commit `d2fe4b7`, unmodified. These are **snapshots** — the originals
in `docs/` remain canonical. If a source changes, re-copy and re-merge; don't
edit these.

| Source | Why it's design-relevant |
|---|---|
| `CLAUDE.md` | §3 frontend stack, §7 product & UI conventions |
| `00-README-and-decisions.md` | decision log D1–D7, open questions Q1–Q4 |
| `01-PRD.md` | personas + scope (currently all `TODO(domain)`) |
| `02-business-rules.md` | **normative** — BR-1.4.x config, BR-1.5.1 search |
| `03-data-model.md` | `sys_parameters` schema → the one fully specified module |
| `04-api-specification.md` | error model, cursor pagination, auth |
| `05-architecture-and-nfr.md` | frontend stack, security posture |
| `10-design-system.md` | tokens, a11y, mandatory list-search pattern |

## Before you start — the honest state

ruuma's **domain is not defined yet.** D8 ("what ruuma actually is") is open, and
so are Q1–Q4. The PRD, business-rules §2, and the data model are structured
placeholders.

That means:

- **Designable now:** app shell + navigation, authentication, the System
  Parameters admin module (fully specified), design tokens, and the full
  component + state library. A genuinely useful foundation.
- **Blocked:** every domain screen. There are no entities to design against yet.

Q3 in particular — *"is there a user-facing UI, or API-only for v1?"* — is
unanswered, and `10-design-system.md` opens by saying it is "only relevant if
ruuma has a UI." Worth settling that before investing much here.

## Handing results back

Per `CLAUDE.md` §5/§8, design decisions land in the canonical docs — tokens and
components into `docs/10-design-system.md`, the decision into `docs/00`'s log,
status into `docs/PROGRESS.md`, and any new mandatory pattern as a `BR-x.y` rule
in `docs/02-business-rules.md`. See `00-design-brief.md` §9.
