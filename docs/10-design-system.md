# Design System — ruuma

**Version:** 0.2 (brand locked)
**Date:** 2 August 2026
**Direction:** warm, calm, appetite-forward emerald. Mobile-first (design at
360px). WCAG AA minimum. **No PWA** (D22) — responsive web only.

React 18 + Tailwind + design tokens as CSS variables, light + dark.

---

## 1. Brand

The logo is the source of the palette (D20).

- **Asset:** `web/public/brand/ruuma-logo-emerald.png` (507×262, RGBA, transparent)
- **Sampled brand colour:** `#277066` — a deep emerald/teal green. This is
  `--primary`; nothing else competes with it.
- Wordmark is "RUUMA" in a high-contrast serif with a roof mark over the final
  A, and "eatery" set below in a light italic serif. Keep clear space of at
  least the cap-height of "R" on every side. Never recolour the logo — on dark
  surfaces use the same asset on `--surface`, not an inverted variant.

## 2. Palette

Tokens are CSS variables on `:root`, overridden under `[data-theme="dark"]` and
`@media (prefers-color-scheme: dark)`. Every foreground/background pair below was
contrast-checked; the ratio is stated and all are ≥ 4.5:1 (WCAG AA for body text).

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| `--bg` | `#F7F9F8` | `#0D1512` | page canvas |
| `--surface` | `#FFFFFF` | `#14201C` | cards, sheets, menus |
| `--border` | `#DCE5E1` | `#24352F` | hairlines, input borders |
| `--text` | `#101915` | `#E8F0EC` | body text |
| `--text-muted` | `#5A6B65` | `#9FB3AB` | secondary text, help |
| `--primary` | `#277066` | `#4FA695` | primary actions, links, brand |
| `--primary-hover` | `#1F5B53` | `#6FBCAC` | hover/active on primary |
| `--primary-fg` | `#FFFFFF` | `#0D1512` | text on a primary fill |
| `--primary-subtle` | `#E6F2EF` | `#17332C` | selected slot, badges, tints |
| `--warning` | `#B45309` | `#F0B357` | "almost full" slots, ageing payments |
| `--danger` | `#B3261E` | `#FF9A8A` | destructive actions, errors |

Verified ratios (foreground on background):

| Pair | Ratio |
|---|---|
| `#277066` on `#FFFFFF` / `#F7F9F8` | 5.84 / 5.52 |
| `#FFFFFF` on `#277066` (primary button) | 5.84 |
| `#101915` on `#FFFFFF` | 17.92 |
| `#5A6B65` on `#FFFFFF` / `#F7F9F8` | 5.64 / 5.33 |
| `#B45309` on `#FFFFFF` | 5.02 |
| `#B3261E` on `#FFFFFF` | 6.54 |
| `#4FA695` on `#0D1512` / `#14201C` | 6.37 / 5.76 |
| `#0D1512` on `#4FA695` (dark primary button) | 6.37 |
| `#E8F0EC` / `#9FB3AB` on `#0D1512` | 15.98 / 8.39 |
| `#F0B357` / `#FF9A8A` on `#14201C` | 9.01 / 8.17 |

Colour is never the only signal: a full slot carries a reason string, a warning
state carries an icon and text, and dietary/allergen tags are labelled, not
colour-coded alone.

## 3. Typography

Self-hosted (no third-party font CDN — see `12-security.md`, A08).

| Role | Family | Sizes |
|---|---|---|
| Display / headings | **Plus Jakarta Sans** 600/700 | 32 / 24 / 20 / 18 |
| Body / UI | **Inter** 400/500 | 16 base, 14 secondary, 12 meta |
| Numeric (prices, capacity, totals) | Inter, `font-variant-numeric: tabular-nums` | inherits |

Body never goes below 14px. Prices always render with thousands separators in
`id-ID` format (`Rp 125.000`) and use tabular numerals so columns align.

## 4. Components

Buttons (primary / secondary / ghost / destructive), inputs, select, date
picker, **slot picker**, quantity stepper, cards, tables, tabs, toasts, modals,
empty states, skeletons. Every interactive element has hover, focus-visible,
active, disabled and loading states.

- **Focus ring:** `2px solid var(--primary)` with a `2px` offset, always
  visible — never removed for aesthetics.
- **Touch targets:** minimum 44×44px.
- **Disabled slots** show the *reason* (past, closed, full, after cutoff), not
  just a greyed box (BR-2.x scheduling rules).

### 4.1 Lists & tables — search box is mandatory

Every list/table view **must** include a search box that filters its data
(BR-1.5.1). The search input sits at the top of the list, is debounced (300ms),
and searches the columns relevant to that entity. This is a non-negotiable
pattern, not a per-screen decision.

### 4.2 Menu item card

Photo (WebP, responsive `srcset` from MinIO, lazy), name, short description,
price, dietary/spice/halal tags, and an availability state (available · low
stock with count · sold out today · unavailable at this store). Sold-out items
stay visible but are not addable — hiding them makes the menu feel broken.

## 5. Accessibility

- AA contrast (table above), visible focus rings, full keyboard operation of the
  date and slot pickers (arrow keys move, Enter selects, Escape closes).
- Real `<label>` elements; errors announced via `aria-live="polite"` and tied to
  the field with `aria-describedby`.
- Respects `prefers-reduced-motion` and `prefers-color-scheme`.
- Language toggle sets `<html lang>` to `id` or `en`; all copy comes from message
  catalogues, never inline strings.
