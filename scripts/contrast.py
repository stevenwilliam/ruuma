#!/usr/bin/env python3
"""Contrast budget for the ambient background wash (docs/10 §2.1).

The palette table in docs/10 §2 states every ratio against a flat --bg. The
wash sits between --bg and the text, so those numbers are not the ones that
ship. This recomputes them against the worst case — all three radial pools
overlapping plus grain — and fails loudly if anything drops under WCAG AA.

Run it after touching --wash-*, --grain-opacity, --text-muted or --primary-ink:

    python3 /home/dev/projects/ruuma/scripts/contrast.py

Exit status is 0 when every pair passes, 1 otherwise, so it can gate a build.
"""

import sys

AA_BODY = 4.5


def channel(c: float) -> float:
    c /= 255
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def luminance(rgb):
    r, g, b = (channel(x) for x in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(fg, bg) -> float:
    a, b = luminance(fg), luminance(bg)
    hi, lo = max(a, b), min(a, b)
    return (hi + 0.05) / (lo + 0.05)


def over(fg, alpha, bg):
    """Composite fg at `alpha` onto bg."""
    return tuple(alpha * f + (1 - alpha) * b for f, b in zip(fg, bg))


def rgb(hex_colour: str):
    h = hex_colour.lstrip("#")
    return tuple(int(h[i : i + 2], 16) for i in (0, 2, 4))


def to_hex(c) -> str:
    return "#%02X%02X%02X" % tuple(int(round(x)) for x in c)


def worst_case(scrim, scrim_a, grain, grain_a):
    """The page backdrop is a photograph, so the worst case is not a fixed
    colour — it is whichever extreme pixel the photo happens to put behind the
    text. Both are checked, and the guarantee is the worse of the two, so it
    holds for any image dropped in later (ruuma's own photography, eventually).
    """
    worst, worst_lum = None, None
    for pixel in ((0, 0, 0), (255, 255, 255)):
        c = over(scrim, scrim_a, pixel)
        c = over(grain, grain_a, c)
        # Keep the composite furthest from mid-grey in the unhelpful direction:
        # simply test both and let the caller take the minimum ratio.
        if worst is None:
            worst, worst_lum = [c], None
        else:
            worst.append(c)
    return worst


THEMES = {
    "light": {
        "bg": rgb("f7f9f8"),
        "wash": (rgb("f7f9f8"), 0.82, rgb("000000"), 0.03),
        "foregrounds": {
            "--text": rgb("101915"),
            "--text-muted": rgb("414d49"),
            "--primary-ink": rgb("1f5b53"),
        },
        # Kept in the report as the reason --primary-ink exists at all.
        "informational": {"--primary (fill, not text)": rgb("277066")},
        "base": rgb("f7f9f8"),
        "card": (rgb("ffffff"), 0.72),
    },
    "dark": {
        "bg": rgb("0d1512"),
        "wash": (rgb("0d1512"), 0.86, rgb("ffffff"), 0.03),
        "foregrounds": {
            "--text": rgb("e8f0ec"),
            "--text-muted": rgb("9fb3ab"),
            "--primary-ink": rgb("6fbcac"),
        },
        "informational": {"--primary (fill, not text)": rgb("4fa695")},
        "base": rgb("0d1512"),
        "card": (rgb("14201c"), 0.72),
    },
}


def main() -> int:
    failures = 0

    for name, theme in THEMES.items():
        composites = worst_case(*theme["wash"])
        # How much of the photograph survives the scrim. A backdrop nobody can
        # see is a backdrop that is not doing its job: three earlier versions
        # of this shipped at 36/765 and read as a flat page.
        visible = sum(abs(a - b) for a, b in zip(composites[0], composites[1]))
        print(f"\n{name.upper()} — scrimmed backdrop spans "
              f"{to_hex(composites[0])}..{to_hex(composites[1])} "
              f"(photo range {visible:.0f}/765)")
        if visible < 60:
            print("  NOTE  the scrim is so thick the photograph is barely visible")

        for token, colour in theme["foregrounds"].items():
            r = min(ratio(colour, c) for c in composites)
            ok = r >= AA_BODY
            failures += not ok
            print(f"  {'PASS' if ok else 'FAIL'}  {token:<16} {r:5.2f}")

        # Cards are translucent, so their text sits on surface-over-wash, not
        # on a flat --surface. Checked separately because that is where most
        # body copy actually lives.
        card_fill, card_alpha = theme["card"]
        cards = [over(card_fill, card_alpha, c) for c in composites]
        print(f"  card {to_hex(cards[0])}..{to_hex(cards[1])} (translucent over the backdrop)")
        for token, colour in theme["foregrounds"].items():
            r = min(ratio(colour, c) for c in cards)
            ok = r >= AA_BODY
            failures += not ok
            print(f"  {'PASS' if ok else 'FAIL'}  {token:<16} {r:5.2f}  on card")

        for token, colour in theme["informational"].items():
            r = min(ratio(colour, c) for c in composites)
            note = "" if r >= AA_BODY else "  <- why --primary-ink exists"
            print(f"  ....  {token:<16} {r:5.2f}{note}")

    print()
    if failures:
        print(f"{failures} pair(s) below AA ({AA_BODY}:1). Lower the wash alphas "
              f"or darken the foreground — see docs/10 §2.1.")
        return 1
    print(f"All body-text pairs clear AA ({AA_BODY}:1) against the wash.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
