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
| `dish/<SKU>.jpg` | dish photography, 1200×900 — see below |

```bash
/usr/local/go/bin/go run ./tools/genassets
```

**Placing the wordmark: give it `self-start` inside a column flex container.**
`h-* w-auto` alone does not protect the aspect ratio there. A flex item is
stretched along the cross axis by default, and in a `flex-col` the cross axis
is the *width* — so `align-items: stretch` silently overrides `w-auto` and
pulls the logo out to the full container width against a pinned height. This
distorted both the customer footer and the admin sign-in page. Row containers
using `items-center` are unaffected, which is why the two headers never showed
it.

**The icons must stay square.** The wordmark is 1.94:1, and a browser handed a
non-square favicon squashes it into its square slot — which is why the logo
looked stretched in the tab. The generator pads it onto a square canvas at 12%
clear space instead of cropping or restretching it.

### 1.2 Dish photography

Real photographs, fetched from Wikimedia Commons by `tools/dishphotos` and
centre-cropped to 4:3 (D31):

```bash
/usr/local/go/bin/go run ./tools/dishphotos          # all SKUs
/usr/local/go/bin/go run ./tools/dishphotos IDN-001  # just one
```

**Only commercially-usable licences are accepted.** The tool refuses NonCommercial
and NoDerivatives outright — a menu on a site taking money is a commercial use.
Every image's photographer, licence and source URL are written to
`web/src/credits.json` and published at **`/credits`**, linked from the footer
of every page. CC BY and CC BY-SA require that; it is an obligation, not a
courtesy, and removing the link breaks the licence.

**Every fetched image must be looked at before it ships.** File titles are not
trustworthy. A search for "grilled chicken steak" returned American
chicken-fried steak smothered in gravy; "braised pork belly" returned a bao bun
photographed on top of a rival restaurant's menu and QR code. Neither is
detectable from metadata.

`tools/genassets` still produces the textless placeholder cards, which remain
the fallback when a fetch fails — a coloured plate is better than a broken
image. They are textless because ruuma ships ID and EN from message catalogues
(CLAUDE.md §10), so a dish name baked into a JPEG cannot be translated.

**Commons photography is a stopgap.** It shows the right dish but not *ruuma's*
dish, and a customer comparing the photo to what they collect will notice.
Replace it with the kitchen's own photography before launch; the admin
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
| `--text-muted` | `#4E5D58` | `#9FB3AB` | secondary text, help |
| `--primary` | `#277066` | `#4FA695` | primary actions, links, brand |
| `--primary-hover` | `#1F5B53` | `#6FBCAC` | hover/active on primary |
| `--primary-fg` | `#FFFFFF` | `#0D1512` | text on a primary fill |
| `--primary-ink` | `#1F5B53` | `#6FBCAC` | emerald as **text** — links, ghost buttons, badges |
| `--primary-subtle` | `#E6F2EF` | `#17332C` | selected slot, badges, tints |
| `--warning` | `#B45309` | `#F0B357` | "almost full" slots, ageing payments |
| `--danger` | `#B3261E` | `#FF9A8A` | destructive actions, errors |

Verified ratios (foreground on background):

| Pair | Ratio |
|---|---|
| `#277066` on `#FFFFFF` / `#F7F9F8` | 5.84 / 5.52 |
| `#FFFFFF` on `#277066` (primary button) | 5.84 |
| `#101915` on `#FFFFFF` | 17.92 |
| `#4E5D58` on `#FFFFFF` / `#F7F9F8` | 6.93 / 6.55 |
| `#1F5B53` on `#FFFFFF` / `#F7F9F8` | 7.84 / 7.41 |
| `#B45309` on `#FFFFFF` | 5.02 |
| `#B3261E` on `#FFFFFF` | 6.54 |
| `#4FA695` on `#0D1512` / `#14201C` | 6.37 / 5.76 |
| `#0D1512` on `#4FA695` (dark primary button) | 6.37 |
| `#E8F0EC` / `#9FB3AB` on `#0D1512` | 15.98 / 8.39 |
| `#F0B357` / `#FF9A8A` on `#14201C` | 9.01 / 8.17 |

Colour is never the only signal: a full slot carries a reason string, a warning
state carries an icon and text, and dietary/allergen tags are labelled, not
colour-coded alone.

### 2.1 Ambient background wash

The page canvas is not flat. Behind everything sit two fixed layers on
`body::before` / `body::after`:

| Token | Light | Dark | What it is |
|---|---|---|---|
| `--wash-emerald` | `rgba(168,222,206,.35)` | `rgba(18,60,52,.45)` | two of the three radial pools |
| `--wash-sand` | `rgba(242,206,158,.30)` | `rgba(58,44,24,.35)` | one warm pool, so a food page does not read clinical |
| `--grain-opacity` | `0.03` | `0.04` | inline SVG turbulence over the whole viewport |

**The pools are picked by luminance direction, and that is the whole trick.**
Dark text only loses contrast when the background gets *darker*, so in light
mode the pools are light and saturated — a soft mint and a soft peach — and run
at 30–35% while the AA table barely moves. Dark mode inverts it: pools deeper
than `--bg`, so light text never loses.

The first version used `--primary` itself at 7%. Emerald is darker than `--bg`,
so it paid contrast on every percent and was still invisible — it shifted the
canvas by 36/765 and shipped looking like a flat page. Colour, not opacity, is
what makes a wash readable. `scripts/contrast.py` now reports that shift and
warns below 60/765.

The grain is not decoration for its own sake: a gradient this large bands
visibly on an 8-bit panel without it, and it supplies the handmade warmth a
restaurant wants and a dashboard does not.

**These alphas are a contrast budget, not a taste setting.** Every ratio in the
table above is measured against `--bg`, and the wash sits between `--bg` and the
text. The worst case is all three pools overlapping plus grain, which composites
to `#C8DCCB` in light and `#223931` in dark. Against those:

| Foreground | On worst-case wash | Verdict |
|---|---|---|
| `--text` | 12.41 (light) / 10.61 (dark) | pass |
| `--text-muted` | 4.80 / 5.57 | pass |
| `--primary-ink` | 5.43 / 5.55 | pass |
| `--primary` **as text** | 4.05 / 4.23 | **fails** — use `--primary-ink` |

Two consequences, both already applied:

- `--text-muted` was darkened from `#5A6B65` to `#4E5D58`. At the old value the
  worst case was 3.69 and the wash broke AA outright.
- `--primary-ink` exists because `--primary` is sampled from the logo (D20) and
  cannot move. The fill stays exactly as it was; emerald *text* uses the ink
  instead. It is not a new colour — it is the existing `--primary-hover` shade,
  named for its second job.

Raising any wash alpha means re-running that budget. The figures come from
`scripts/contrast.py`.

### 2.2 Motion

One tier only: **subtle**. Durations 200–400ms, ease-out
(`cubic-bezier(.25,.46,.45,.94)`), travel capped at 12px so an entrance reads as
a fade rather than a slide.

| Utility | Where | Detail |
|---|---|---|
| `.rise-in` | `<main>`, keyed to the route | 350ms fade + 12px rise; the whole page transition |
| `.stagger` | menu grid, keyed to filter/sort | same animation, 40ms apart, **capped at eight steps** |
| `wash-drift` | `body::before` | 32s, `translate3d` + `scale(1.06)`, alternating |

Rules that are not negotiable:

- **Transform and opacity only.** The wash drifts by transform, never by
  `background-position`, which would repaint a full-viewport gradient every
  frame. This menu is mostly read on mid-range Android.
- **No exit animation on navigation.** An exit tween delays the next page; back
  and forward have to stay instant.
- **The stagger caps at eight.** Past that, cards share the final 280ms delay.
  Compounding it turns a choreographed list into one that looks slow to load.
- **`prefers-reduced-motion` zeroes delays as well as durations**, and stops
  `wash-drift` outright rather than running it imperceptibly. A zero duration
  does not help if a 280ms delay survives it.
- **The wash and grain are hidden in `@media print`** — they are fixed layers
  and would otherwise tile a green cast across the kitchen's production sheet.

### 2.3 Floating contact button

A WhatsApp FAB sits bottom-right of every customer page — **112px**, above the
safe-area inset (`env(safe-area-inset-bottom)`, which is why `index.html` sets
`viewport-fit=cover`). It is `z-40` — the same layer as the sticky header, never
above it, so a future modal still wins.

It is an inline SVG in WhatsApp's own `#25D366`, deliberately exempt from this
palette: a recoloured mark stops reading as WhatsApp. The icon is
`aria-hidden`; the link carries the accessible name from the message catalogue.

Number, on/off and greeting come from `sys_parameters` (BR-1.4.5) — see
`04-api-specification.md` for `GET /public-config`. **The button hides itself
when the number is blank, whatever the enabled flag says.** A contact button
that opens a chat with nobody is worse than no button: the customer believes
they have asked, and waits.

On the item page the sticky add-to-cart bar reserves `pe-32` (128px = the
button plus its inset) up to the `2xl` breakpoint so the FAB never covers the
primary conversion control. Not `xl`: at 1280 the bar ends at 1152 and the
button starts at exactly 1152, so they touch — 1536 is the first width with
real clearance.

At 112px on a 375px phone this is a large object, deliberately. It is the only
element allowed to sit over the page.

### 2.4 Language picker

Two options side by side — flag plus `ID`/`EN` — with the active one marked by
`aria-pressed` and a filled pill.

It replaced a single button labelled with the *current* language, which is
ambiguous in the way every one-button language toggle is: a button label
normally describes what pressing it does, so "EN" reads as "you are in English"
to half of users and "switch to English" to the other half. Showing both
removes the question.

On flags: a flag is a country, not a language, and the usual objection is real.
It survives here because ruuma ships exactly two languages for one market, where
both flags are unambiguous — and because **the flag is never the only signal**.
Every option keeps its visible `ID`/`EN` label, the SVGs are `aria-hidden`, and
the pair carries a group label from the message catalogue. Nothing depends on
recognising the artwork.

Inline SVG, not the flag emoji: emoji flags do not render at all on Windows,
which falls back to a bare letter pair.

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

### 4.0 Writes go through `AsyncButton`, not `Button`

Any control that posts, puts or deletes uses **`AsyncButton`** (`web/src/components/ui.tsx`).
`Button` is for navigation and local state only. `AsyncButton` guarantees two
things a plain button cannot:

1. **One click, one write.** It disables itself for the life of the request and
   sets `aria-busy`. This is not cosmetic: every mutating call site passes a
   fresh `crypto.randomUUID()` as its idempotency key, so two clicks are two
   *distinct* operations to the API rather than one retried — the key that
   exists to make a retry safe does nothing for the double-click it is most
   needed for. Verifying a payment twice would write two rows to
   `payment_events`, and those are immutable (D26).
2. **A confirmation on anything that cannot be walked back**, via the `confirm`
   prop. Native `window.confirm` on purpose — keyboard-accessible and
   focus-correct without a hand-rolled focus trap, and consistent with the
   `window.prompt` already used to capture mismatch and rejection reasons.

An action whose own flow already prompts for a value (set stock, reject a
payment, edit a parameter) does **not** also confirm — the prompt is the
confirmation. Actions that only ever make things *more* available (lift an 86,
activate a store) do not confirm either; only the destructive direction asks.

Which actions confirm today:

| Action | Why it asks |
| --- | --- |
| Verify a payment (matching amount) | Money, recorded permanently (D26) |
| Close a date / blackout | Takes effect immediately, stops every remaining slot (D27) |
| Deactivate a staff member | Access is revoked at once |
| Deactivate a store | Disappears from the customer site at once (D30) |
| 86 a dish | Customers stop being able to order it at once |
| Hand an order over (`PICKED_UP`) | Terminal state, no screen walks it back |

`onRun` is expected to render its own failure through `ErrorNote`; `AsyncButton`
catches and logs as a backstop so a caller that forgets cannot turn a failed
write into an unhandled promise rejection. Behaviour is pinned by
`web/src/__tests__/async-button.test.tsx`.

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

Source is `web/public/dish/<SKU>.jpg` (§1.2). The MinIO-backed upload path
exists in the admin but is **not yet wired to the customer menu** — the API
returns `photo_key`, and no route serves an object by key. Real photography
needs that route before it can appear here.

### 4.3 App chrome — header and footer

Both are a full-bleed `--primary` fill with `--primary-fg` content, bracketing
the page in brand colour. The header is sticky.

**The footer is a sticky footer in the layout sense, not `position: fixed`.**
The shell is a column flex container of `min-h-dvh`, `<main>` takes the slack
with `flex-1`, and the footer lands at the bottom of the viewport on a short
page (an empty cart, sign-in) and below the content on a long one.

It is deliberately not pinned to the viewport: this footer carries the
wordmark, the cuisine line, the copyright and the photo-credits link, and
holding that much over the screen would eat the bottom of every phone — the
space the menu grid and the checkout CTA need most. The floating contact button
(§2.3) is the only thing that gets to sit over the page.

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
- **Decorative emoji carry `aria-hidden="true"`.** The chilli scale on a menu
  card is a visual quantity, not content — the badge next to it already says
  "spicy", and without the attribute a screen reader reads "hot pepper" once per
  level after the word that just said it. Emoji are never the only carrier of
  meaning.
- Images that a neighbouring heading already names take `alt=""` rather than a
  duplicate description — the dish photo on a menu card is the case in point,
  since the `<h2>` beside it is the dish name. This is deliberate, not a
  missing alt.

### 5.1 Known gap — admin copy is not in the catalogues

The customer app reads every string from `web/src/i18n`. The admin app does
not: page titles, table headings, button labels and the confirmation messages
added in §4.0 are inline English. That contradicts CLAUDE.md §10 and the last
bullet above, and it spans roughly ten files, so it is recorded here as a debt
rather than quietly half-fixed. Admin is staff-facing and English-only in
practice today; this must be closed before the admin guide (docs/16) claims
bilingual support.

## 6. SEO

The baseline in `99-steven-preference.md` §13, as it lands here.

| Surface | Where | Note |
|---|---|---|
| `<title>` + description per route | `web/src/lib/seo.ts`, `useSeo()` | brand suffix appended in one place so it cannot drift |
| `noindex` on private routes | `useNoIndex()` on cart, checkout, orders, order, auth | removed on unmount, or it leaks onto the next route |
| Open Graph + Twitter card | **static in `web/index.html`** | see below |
| `robots.txt`, `sitemap.xml` | `web/public/` | robots disallows the whole transactional surface |
| JSON-LD `Restaurant` | static in `web/index.html` | hand-maintained; update when the outlet changes |
| Share image | `brand/ruuma-share-1200x630.png` | generated by `tools/genassets` |

**The OG tags are static, and that is the point.** ruuma is a client-rendered
SPA, and link-preview bots — WhatsApp, Instagram, Facebook, Slack — do not
execute JavaScript. Anything React sets at runtime is invisible to them. Since
ruuma's customers share links in WhatsApp (D28 makes it the notification
channel too), a blank preview card is the most expensive SEO defect available,
and it has nothing to do with Google.

The cost of that choice: **every URL previews as the site, not as the specific
dish.** Per-page previews need prerendering or SSR. That is a real project, not
a tweak, and it is recorded in `08-roadmap.md` rather than half-built.

`useSeo` runs in the browser, so it reaches Google — which does render JS — and
fixes tabs and history immediately. It does nothing for preview bots. The two
mechanisms are not redundant; they cover different readers.

**JSON-LD is hand-maintained.** `index.html` is a static file served by nginx
and cannot read `sys_parameters`, so the address, cuisines and menu URL are
literals matching the single outlet from D30. They must be updated by hand when
the outlet details change or a second store opens.

Verify the way a bot sees it, not the way a browser does:

```bash
curl -s https://ruuma.id/ | grep -oE '<title>[^<]*|og:[a-z:]* content="[^"]*'
```
