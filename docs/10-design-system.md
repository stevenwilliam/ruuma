# Design System — ruuma

**Version:** 0.1 (scaffold)
**Date:** 31 July 2026
**Direction:** _TODO(domain)_. Mobile-first (design at 360px). WCAG AA minimum.

Only relevant if ruuma has a UI. React 18 + Tailwind + design tokens as CSS
variables, light + dark.

---

## 1. Palette

> TODO(domain): tokens as CSS variables. Every text/background pair must meet
> WCAG AA. Define light (default) and dark.

| Token | Light | Dark | Use |
|-------|-------|------|-----|
| `--bg` | | | page canvas |
| `--surface` | | | cards, sheets |
| `--primary` | | | primary actions |
| `--text` | | | body text |

## 2. Typography

> TODO: type scale, families, weights.

## 3. Components

> TODO: buttons, inputs, cards, tables, toasts — states and a11y.

### 3.1 Lists & tables — search box is mandatory

Every list/table view **must** include a search box that filters its data
(BR-1.5.1). The search input sits at the top of the list, is debounced, and
searches the columns relevant to that entity. This is a non-negotiable pattern,
not a per-screen decision.

## 4. Accessibility

- AA contrast, visible focus rings, keyboard-navigable, respects
  `prefers-reduced-motion` and `prefers-color-scheme`.
