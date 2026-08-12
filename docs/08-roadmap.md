# Roadmap — ruuma

**Version:** 1.0
**Date:** 2 August 2026

---

## Phase 1 — v1, pickup only (this build)

The whole thin slice in `01-PRD.md` §3.1: multi-store master data with per-mode,
per-weekday, per-date schedules and same-day blackouts; store-resolved menu with
options, 86s and daily stock; two-axis slot capacity that cannot oversell;
customer accounts across four sign-in methods; manual bank transfer with a *kode
unik*; finance verify/reject/refund and reconciliation; kitchen production board
and tickets; admin CRUD for everything configurable; WhatsApp notifications for
four events; audit log; ASVS L2 hardening with the tests that prove it.

**Sequence:** docs → foundation (platform, migrations, seed) → domain →
services → HTTP → frontend → harden → handbooks. Delivered in one push
(`CLAUDE.md` §9).

## Phase 2 — delivery (D16)

Delivery is already modelled: `store_fulfilment_modes`, `delivery_zones`,
`addresses`, the `delivery` slot type and the `OUT_FOR_DELIVERY`/`DELIVERED`
states all exist and are tested. Phase 2 is therefore a switch plus a UI:

1. Flip `fulfilment.delivery_enabled`.
2. Populate zones and fees per store (named areas; fee, minimum order, free
   threshold).
3. Customer checkout: address selection, zone resolution, fee line, minimum-order
   rule.
4. Ops: dispatch view, mark out-for-delivery and delivered.
5. Per-mode hours already work — a store may stop delivery earlier than pickup.

No migration is expected beyond seeding zones.

## Phase 3 — QRIS and auto-cancel (D25)

1. Implement the `qris` payment provider behind the existing port.
2. Enable the webhook route (HMAC + timestamp + replay protection, already built
   and tested).
3. Raise `orders.auto_cancel_minutes` above zero; the worker starts releasing
   unpaid capacity, and the phase-1 manual controls become a fallback.
4. Finance queue shrinks to exceptions only.

## Phase 4 — growth

Loyalty and stamp cards · scheduled recurring orders · marketplace channel sync ·
driver assignment and tracking · dine-in QR ordering · multi-language menu
authoring beyond ID/EN · inventory linked to daily stock · customer-facing
"nearest store with capacity" suggestions.

### Per-page share previews — prerendering or SSR

The SEO baseline shipped in D39 covers the site with **one** correct preview:
static Open Graph tags in `index.html`, because link-preview bots do not run
JavaScript. Every ruuma URL therefore previews as "Ruuma Eatery", not as the
dish being shared.

Making a shared dish link show *that dish* — its photo, name and price — needs
the HTML for `/menu/:id` to exist before JavaScript runs. That means
prerendering the menu routes at build time, or server-side rendering. Either is
a real project: the menu is per-store and availability changes hourly, so a
prerender has to be regenerated on catalogue changes, and SSR pulls the React
app onto the Go server or a Node process beside it.

Worth doing when WhatsApp sharing of individual dishes becomes a real acquisition
channel. Until then the single site-wide card is the honest trade.

## Explicitly not planned

Table reservations, subscriptions, inter-store stock transfer, and a native
mobile app. A PWA is out by decision (D22) and would need that decision reversed.

## Sequencing rationale

Store scope, slot capacity and the money path are the three things that are
expensive to retrofit and cheap to get right at the start — they are all in
phase 1. Delivery and QRIS are deliberately deferred **because they are additive
to a correct core**, not because they are hard: zones need only data, and QRIS
needs only an adapter. Anything that would reshape the order flow is in phase 1;
anything that plugs into it is later.
