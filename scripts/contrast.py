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


def worst_case(bg, emerald, emerald_a, sand, sand_a, grain, grain_a):
    """Three pools overlap — emerald, sand, emerald — then grain on top."""
    c = bg
    for fill, alpha in ((emerald, emerald_a), (sand, sand_a), (emerald, emerald_a)):
        c = over(fill, alpha, c)
    return over(grain, grain_a, c)


THEMES = {
    "light": {
        "bg": rgb("f7f9f8"),
        "wash": (rgb("277066"), 0.07, rgb("b4783c"), 0.05, rgb("000000"), 0.025),
        "foregrounds": {
            "--text": rgb("101915"),
            "--text-muted": rgb("4e5d58"),
            "--primary-ink": rgb("1f5b53"),
        },
        # Kept in the report as the reason --primary-ink exists at all.
        "informational": {"--primary (fill, not text)": rgb("277066")},
    },
    "dark": {
        "bg": rgb("0d1512"),
        "wash": (rgb("4fa695"), 0.10, rgb("c8965a"), 0.05, rgb("ffffff"), 0.035),
        "foregrounds": {
            "--text": rgb("e8f0ec"),
            "--text-muted": rgb("9fb3ab"),
            "--primary-ink": rgb("6fbcac"),
        },
        "informational": {"--primary (fill, not text)": rgb("4fa695")},
    },
}


def main() -> int:
    failures = 0

    for name, theme in THEMES.items():
        composite = worst_case(theme["bg"], *theme["wash"])
        print(f"\n{name.upper()} — worst-case background {to_hex(composite)}")

        for token, colour in theme["foregrounds"].items():
            r = ratio(colour, composite)
            ok = r >= AA_BODY
            failures += not ok
            print(f"  {'PASS' if ok else 'FAIL'}  {token:<16} {r:5.2f}")

        for token, colour in theme["informational"].items():
            r = ratio(colour, composite)
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
