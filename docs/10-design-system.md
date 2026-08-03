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
  least the cap-height of "R" on every side. The letterforms are never
  redrawn, restretched or recoloured to a third colour.
- **On an emerald fill, use `ruuma-logo-white.png`.** The emerald wordmark on a
  `#277066` header is invisible. That file is the same artwork with the ink set
  to `--primary-fg` and the alpha preserved, so edges stay anti-aliased.

### 1.1 Derived assets — `tools/genassets`

Everything below is generated from the logo, so it can be regenerated rather
than being binary files nobody can reproduce (D31):

| File | Use |
|---|---|
| `brand/ruuma-logo-white.png` | header and footer on the emerald fill |
| `brand/ruuma-icon-512.png`, `-192.png` | favicon |
| `brand/ruuma-apple-touch-icon.png` | iOS home screen, opaque `--bg` behind it |
| `dish/<SKU>.jpg` | placeholder dish photography, 1200×900 |

```bash
/usr/local/go/bin/go run ./tools/genassets
```

**The icons must stay square.** The wordmark is 1.94:1, and a browser handed a
non-square favicon squashes it into its square slot — which is why the logo
looked stretched in the tab. The generator pads it onto a square canvas at 12%
clear space instead of cropping or restretching it.

**Dish cards are textless.** ruuma ships ID and EN from message catalogues
(CLAUDE.md §10), so a dish name baked into a JPEG cannot be translated; the
card supplies colour and the UI draws the name over it. Each card is
deterministic per SKU — regenerating never reshuffles how the menu looks — and
tinted by cuisine so a category reads as a family in the grid. These are
**placeholders and are replaced by real photography before launch**; the admin
photo-upload path to MinIO is unchanged.

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

**The photo leads.** A 4:3 image fills the full card width above the text — a
customer decides with their eyes, so the picture gets the space and the grid
runs **two across on desktop, not three**. Below it: name, short description,
price, dietary/spice/halal tags, and an availability state (available · low
stock with count · sold out today · unavailable at this store).

Sold-out items stay visible but are not addable — hiding them makes the menu
feel broken. Sold out is shown as a wash **over the photo** with the image
desaturated, not as a tag in the badge row where it was easy to miss.

Images are lazy-loaded and carry intrinsic `width`/`height` so the grid does
not shift as they arrive. `alt` is empty: the name sits next to the image as
real text, so describing it again is noise to a screen reader.

Source is `web/public/dish/<SKU>.jpg` (§1.1). The MinIO-backed upload path
exists in the admin but is **not yet wired to the customer menu** — the API
returns `photo_key`, and no route serves an object by key. Real photography
needs that route before it can appear here.

### 4.3 App chrome — header and footer

Both are a full-bleed `--primary` fill with `--primary-fg` content, bracketing
the page in brand colour. The header is sticky.

Everything inside them has to be re-toned for the dark fill; the light-surface
values are unreadable there:

| Element | On the emerald fill |
|---|---|
| Wordmark | `ruuma-logo-white.png`, never the emerald asset |
| Nav link, resting | `--primary-fg` at 90% |
| Nav link, hover | white at 15% |
| Nav link, active | solid white pill, `--primary` text |
| Cart count badge | white pill with `--primary` text — **inverts to `--primary` on white when the cart link is active**, or it disappears into the active pill |
| Language toggle | shared with the admin header, which is a light surface, so it takes an explicit `tone` prop rather than one hard-coded colour |

Contrast: `#FFFFFF` on `#277066` is 5.84:1 (§2), so body-sized nav text passes
AA comfortably.

## 5. Accessibility

- AA contrast (table above), visible focus rings, full keyboard operation of the
  date and slot pickers (arrow keys move, Enter selects, Escape closes).
- Real `<label>` elements; errors announced via `aria-live="polite"` and tied to
  the field with `aria-describedby`.
- Respects `prefers-reduced-motion` and `prefers-color-scheme`.
- Language toggle sets `<html lang>` to `id` or `en`; all copy comes from message
  catalogues, never inline strings.
