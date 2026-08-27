# ruuma — design reference

The working numbers. `docs/10-design-system.md` carries the reasoning and the
brand-guideline reading; this is the sheet to have open while building, so
nothing here is a judgement call — every ratio below was measured with
`scripts/contrast.py` and can be re-measured.

**This file is per project.** The `impeccable` skill beside it is portable; a
brand is not. A new project gets its own `design.md` in the same place.

---

## 1. Type

| Role | Family | Notes |
| --- | --- | --- |
| Display | **Playfair Display** (variable, 400–900) | headings, and the echo of the serif wordmark |
| Body / UI | **Inter** 400/500 | everything a customer reads |
| Numerals | `font-variant-numeric: tabular-nums` | prices, capacity, totals — columns must align |

Both are **self-hosted** in `web/public/fonts/`. Never a font CDN
(`docs/12-security.md`, A08) — it hands a third party every visitor's IP and the
page they are on.

- **One variable file, not three weights.** Google Fonts serves byte-identical
  files for 500/600/700 of a variable family. `@font-face` declares
  `font-weight: 400 900` once, over `/fonts/playfair-display-var.woff2` (38 KB,
  SIL OFL 1.1, licence beside it).
- **Preload with `crossorigin`** from `index.html` — required even same-origin,
  or the browser fetches the font twice.
- `.font-display` carries `letter-spacing: -0.015em`. A serif at heading sizes
  has more built-in spacing than a screen wants.
- **Body never goes below 14px.** Prices render `id-ID` (`Rp 125.000`).

### Scale

| Role | Sizes |
| --- | --- |
| Display / headings | 36 / 30 / 20 / 18 |
| Body / UI | 16 base · 14 secondary · 12 meta |
| `.eyebrow` | 11px, `0.18em` tracking, uppercase, `--text-muted` |

---

## 2. Palette

Two themes. Tokens are CSS variables on `:root`, overridden under
`[data-theme="dark"]` and `@media (prefers-color-scheme: dark)`.

| Token | Light | Dark | Role |
| --- | --- | --- | --- |
| `--bg` | `#F7F9F8` | `#0D1512` | page canvas |
| `--surface` | `#FFFFFF` | `#14201C` | cards, sheets, menus — **opaque**, for inputs and the sticky bar |
| `--surface-card` | `rgba(255,255,255,.72)` | `rgba(20,32,28,.72)` | translucent cards over the backdrop |
| `--border` | `#DCE5E1` | `#24352F` | hairlines — **1.29 on white: decoration, never a boundary that carries meaning** |
| `--text` | `#101915` | `#E8F0EC` | body text |
| `--text-muted` | `#4E5D58` | `#9FB3AB` | secondary text, help |
| `--primary` | `#277066` | `#4FA695` | **fills only** — brand, primary button |
| `--primary-hover` | `#1F5B53` | `#6FBCAC` | hover/active on a primary fill |
| `--primary-fg` | `#FFFFFF` | `#0D1512` | ink on a primary fill |
| `--primary-ink` | `#1F5B53` | `#6FBCAC` | **emerald as text** — links, ghost buttons, badges |
| `--primary-subtle` | `#E6F2EF` | `#17332C` | selected slot, badges, tints |
| `--warning` | `#B45309` | `#F0B357` | "almost full" slots, ageing payments |
| `--danger` | `#B3261E` | `#FF9A8A` | destructive actions, errors |
| `--scrim` | `rgba(247,249,248,.82)` | `rgba(13,21,18,.86)` | overlay between the photo and the page |
| WhatsApp | `#25D366` | `#25D366` | **exempt from this palette** — a recoloured mark stops reading as WhatsApp |

---

## 3. Measured contrast — check here before choosing a colour

**Light theme**, on `#FFFFFF` cards and the `#F7F9F8` canvas:

| Ink | on white | on canvas | |
| --- | ---: | ---: | --- |
| `#101915` text | 17.92 | 16.95 | AAA |
| `#1F5B53` primary-ink | 7.84 | 7.41 | AAA — this is the link colour |
| `#4E5D58` muted | 6.93 | 6.55 | AA |
| `#B3261E` danger | 6.54 | — | AA |
| `#277066` primary | 5.84 | 5.52 | AA — passes, but see the rule below |
| `#B45309` warning | 5.02 | — | AA |
| `#DCE5E1` border | **1.29** | — | ✗ hairline only |

On the `#E6F2EF` subtle tint: `#1F5B53` is **6.83**, `#277066` is **5.09**.
White on the `#277066` primary fill is **5.84**.

**Dark theme**, on `#0D1512` canvas and `#14201C` surface:

| Ink | on canvas | on surface | |
| --- | ---: | ---: | --- |
| `#E8F0EC` text | 15.98 | 14.45 | AAA |
| `#F0B357` warning | — | 9.01 | AAA |
| `#9FB3AB` muted | 8.39 | 7.59 | AAA |
| `#6FBCAC` primary-ink | 8.35 | 7.55 | AAA |
| `#FF9A8A` danger | — | 8.17 | AAA |
| `#4FA695` primary | 6.37 | 5.76 | AA |

`#0D1512` on the `#4FA695` dark primary fill is **6.37**.

**Over the scrimmed photographic backdrop** — the worst case, composited over a
pure-black and a pure-white pixel and taking the worse of the two, so the
guarantee holds for *any* photograph dropped in later:

| | light | dark |
| --- | ---: | ---: |
| scrimmed span | `#C4C6C5`–`#F1F3F2` | `#121917`–`#353C39` |
| `--text` | 10.45 | 9.77 |
| `--text-muted` | 5.14 | 5.13 |
| `--primary-ink` | 4.57 | 5.11 |
| `--text` on a translucent card | 15.58 | 13.13 |
| `--primary` **as a fill, not text** | 3.41 | 3.90 |

That last row is why `--primary-ink` exists. Do not thin the scrim without
re-running the script.

---

## 4. Rules that are not taste

- **`--primary` fills; `--primary-ink` writes.** Emerald as text over the
  scrimmed backdrop measures 3.41 light / 3.90 dark — below AA. Every emerald
  string uses `--primary-ink`.
- **`--border` at 1.29 is a hairline, not a boundary.** Anything that must be
  *found* — a focus ring, an input edge, a control outline — needs ≥ 3:1 and
  uses `--primary`.
- **Focus ring:** `2px solid var(--primary)` at `2px` offset, always visible,
  never removed without a replacement that measures.
- **Colour is never the only signal.** A full slot carries a reason string, a
  warning carries an icon and text, allergen tags are labelled.
- **AA is the floor, and contrast is calculated.** State the ratio next to the
  token. `python3 scripts/contrast.py` prints the standing set — **it takes no
  arguments and silently ignores any you pass**, so for an ad-hoc pair use:
  `python3 -c "import sys;sys.path.insert(0,'scripts');from contrast import ratio,rgb;print(round(ratio(rgb('277066'),rgb('ffffff')),2))"`
- **44px minimum touch target**; motion is one subtle tier, travel capped at
  12px, and `prefers-reduced-motion` zeroes delays as well as durations.
- Mobile-first, designed at 360px. **No PWA** (D22) — responsive web only.

---

## 5. Brand assets

- **Wordmark**: "RUUMA" in a high-contrast serif with a roof mark over the final
  A, "eatery" below in a light italic serif. Clear space of at least the
  cap-height of "R" on every side. Never redrawn, restretched, or recoloured to
  a third colour.
- **On an emerald fill use `ruuma-logo-white.png`** — the emerald wordmark on a
  `#277066` header is invisible.
- **Give the wordmark `self-start` in a `flex-col`.** `h-* w-auto` does not
  protect the aspect ratio there: a flex item stretches along the cross axis,
  which in a column *is the width*, so `align-items: stretch` overrides `w-auto`
  and pulls the logo to full container width against a pinned height. This
  distorted the customer footer and the admin sign-in page. Row containers with
  `items-center` are unaffected — which is why the two headers never showed it.
- **Icons must stay square.** The wordmark is 1.94:1 and a browser squashes a
  non-square favicon into its square slot. `tools/genassets` pads onto a square
  canvas at 12% clear space rather than cropping or restretching.
- Everything derived is **generated, not committed as unreproducible binary**:
  `/usr/local/go/bin/go run ./tools/genassets`.
- **Photography is licence-checked and looked at.** `tools/dishphotos` refuses
  NonCommercial and NoDerivatives — a menu on a site taking money is commercial
  use. Photographer, licence and source URL go to `web/src/credits.json` and are
  published at `/credits`, linked from every footer; CC BY and CC BY-SA require
  that, and removing the link breaks the licence.
- **Every fetched image is looked at before it ships.** Titles are not
  trustworthy: "grilled chicken steak" returned American chicken-fried steak,
  "braised pork belly" returned a bao bun on a rival's menu and QR code, and a
  tumpeng candidate was three identifiable faces — a personality-rights problem
  CC BY-SA does not license.
