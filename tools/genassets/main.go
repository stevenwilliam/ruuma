// Command genassets renders the brand image assets that are derived from the
// logo rather than drawn by hand, so they can be regenerated instead of being
// binary files nobody can reproduce.
//
// Two jobs:
//
//   - Square app icons. The wordmark is 507x262. A browser handed a 1.94:1
//     favicon squashes it into a square tab slot, which reads as a stretched
//     logo — the bug this fixes. Padding it onto a square canvas keeps the
//     aspect ratio the design system insists on (docs/10 §1).
//
//   - Menu photo placeholders, one per SKU. They are deliberately textless:
//     ruuma ships ID and EN from message catalogues (CLAUDE.md §10), so a dish
//     name baked into a JPEG would be untranslatable. The card supplies colour
//     and the UI draws the name over it.
//
// Usage: go run ./tools/genassets
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

const (
	root    = "/home/dev/projects/ruuma"
	logoSrc = root + "/web/public/brand/ruuma-logo-emerald.png"
	brandNS = root + "/web/public/brand"
	menuDir = root + "/web/public/dish"

	cardW, cardH = 1200, 900 // 4:3 — the proportion the menu grid reserves
)

func main() {
	// Off by default: this tool is normally run to regenerate an icon, and the
	// placeholder cards share a directory with real photography (D31).
	force := flag.Bool("force", false,
		"overwrite existing dish images, including real photography")
	flag.Parse()

	if err := run(*force); err != nil {
		log.Fatalf("genassets: %v", err)
	}
}

func run(force bool) error {
	if err := lightLogo(); err != nil {
		return err
	}
	if err := icons(); err != nil {
		return err
	}
	if err := shareCard(); err != nil {
		return err
	}
	return cards(force)
}

// shareCard renders the 1200x630 image used by og:image and twitter:image.
//
// It exists because ruuma's customers share links in WhatsApp, and a link with
// no share image gets a blank card — the single highest-value SEO asset for
// this business, and nothing to do with Google. 1200x630 is the size every
// platform crops from; handing them the square favicon instead means the
// wordmark gets cut off left and right.
//
// Emerald fill with the reversed-out wordmark, matching the site header, so a
// pasted link is recognisably ruuma before anyone reads the title.
func shareCard() error {
	const w, h = 1200, 630

	f, err := os.Open(filepath.Join(brandNS, "ruuma-logo-white.png"))
	if err != nil {
		return fmt.Errorf("open white logo (run lightLogo first): %w", err)
	}
	defer f.Close()

	logo, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decode white logo: %w", err)
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
	// --primary. Flat, not a gradient: a share card is rendered at thumbnail
	// size in a chat list, where a gradient turns to mud.
	xdraw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.NRGBA{0x27, 0x70, 0x66, 0xFF}},
		image.Point{}, xdraw.Src)

	// Half the width, centred. Same clear-space discipline as the icons.
	lb := logo.Bounds()
	target := w / 2
	scale := float64(target) / float64(lb.Dx())
	lh := int(math.Round(float64(lb.Dy()) * scale))
	dst := image.Rect(0, 0, target, lh).Add(image.Pt((w-target)/2, (h-lh)/2))
	xdraw.CatmullRom.Scale(canvas, dst, logo, lb, xdraw.Over, nil)

	path := filepath.Join(brandNS, "ruuma-share-1200x630.png")
	if err := writePNG(path, canvas); err != nil {
		return err
	}
	fmt.Printf("  ruuma-share-1200x630.png (%dx%d)\n", w, h)
	return nil
}

// lightLogo derives the reversed-out wordmark used on the emerald header and
// footer. The brand logo is #277066; on a #277066 background it is invisible.
//
// This is not "recolouring the logo" in the sense docs/10 §1 forbids — the
// letterforms are untouched and only the ink changes, which is exactly the
// light asset that section assumes exists. Alpha is preserved so the edges
// stay anti-aliased instead of turning into a hard cutout.
func lightLogo() error {
	f, err := os.Open(logoSrc)
	if err != nil {
		return fmt.Errorf("open logo: %w", err)
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decode logo: %w", err)
	}

	b := src.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			// --primary-fg: white text on a primary fill (docs/10 §2).
			// #nosec G115 -- RGBA() returns alpha in 0..0xFFFF, so a>>8 is 0..0xFF.
			out.SetNRGBA(x, y, color.NRGBA{0xFF, 0xFF, 0xFF, uint8(a >> 8)})
		}
	}

	path := filepath.Join(brandNS, "ruuma-logo-white.png")
	if err := writePNG(path, out); err != nil {
		return err
	}
	fmt.Printf("  ruuma-logo-white.png (%dx%d)\n", b.Dx(), b.Dy())
	return nil
}

// ── square icons ─────────────────────────────────────────────────────────────

// icons pads the wordmark onto a square canvas at the sizes browsers and iOS
// actually ask for. The logo is never cropped and never re-proportioned; it is
// scaled to fit the width and centred, which is what "never recolour or distort
// the logo" means in practice (docs/10 §1).
func icons() error {
	f, err := os.Open(logoSrc)
	if err != nil {
		return fmt.Errorf("open logo: %w", err)
	}
	defer f.Close()

	logo, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("decode logo: %w", err)
	}

	// Transparent for the tab icon; iOS composites its own background behind
	// apple-touch-icon and a transparent one comes out muddy, so that gets the
	// off-white page background instead (docs/10 §2, --bg).
	specs := []struct {
		name string
		size int
		bg   color.NRGBA
	}{
		{"ruuma-icon-512.png", 512, color.NRGBA{0, 0, 0, 0}},
		{"ruuma-icon-192.png", 192, color.NRGBA{0, 0, 0, 0}},
		{"ruuma-apple-touch-icon.png", 180, color.NRGBA{0xF7, 0xF9, 0xF8, 0xFF}},
	}

	for _, s := range specs {
		canvas := image.NewNRGBA(image.Rect(0, 0, s.size, s.size))
		xdraw.Draw(canvas, canvas.Bounds(), &image.Uniform{s.bg}, image.Point{}, xdraw.Src)

		// 12% padding each side: enough clear space for the mark to breathe at
		// 16px without shrinking the letterforms into mush.
		inner := int(float64(s.size) * 0.76)
		lb := logo.Bounds()
		scale := float64(inner) / float64(lb.Dx())
		w := inner
		h := int(math.Round(float64(lb.Dy()) * scale))

		dst := image.Rect(0, 0, w, h).Add(image.Pt((s.size-w)/2, (s.size-h)/2))
		xdraw.CatmullRom.Scale(canvas, dst, logo, lb, xdraw.Over, nil)

		if err := writePNG(filepath.Join(brandNS, s.name), canvas); err != nil {
			return err
		}
		fmt.Printf("  %s (%dx%d)\n", s.name, s.size, s.size)
	}
	return nil
}

// ── menu placeholder cards ───────────────────────────────────────────────────

// palette is keyed by SKU prefix so a cuisine reads as a family at a glance in
// the menu grid, while each dish still gets its own shade.
var palette = map[string][2]color.NRGBA{
	// from -> to, a soft vertical gradient
	"IDN": {{0xC2, 0x6B, 0x3A, 0xFF}, {0x8A, 0x3F, 0x1E, 0xFF}}, // warm terracotta
	"CHN": {{0xB3, 0x3E, 0x3E, 0xFF}, {0x73, 0x21, 0x28, 0xFF}}, // deep rust red
	"WST": {{0x7D, 0x8B, 0x4F, 0xFF}, {0x4A, 0x58, 0x2C, 0xFF}}, // olive
	"DRK": {{0x27, 0x70, 0x66, 0xFF}, {0x14, 0x40, 0x3A, 0xFF}}, // brand emerald
}

var skus = []string{
	"IDN-001", "IDN-002", "IDN-003", "IDN-004", "IDN-005", "IDN-006",
	"CHN-001", "CHN-002", "CHN-003", "CHN-004", "CHN-005", "CHN-006",
	"WST-001", "WST-002", "WST-003", "WST-004", "WST-005",
	"DRK-001", "DRK-002", "DRK-003", "DRK-004",
}

// cards writes the placeholder dish images — but only for SKUs that do not
// already have one.
//
// This skip is not an optimisation, it is a guard. The placeholders live at the
// same paths as the real Wikimedia photography fetched by tools/dishphotos
// (D31), so the original unconditional loop silently replaced 21 reviewed,
// licence-checked photographs with coloured blobs the moment anyone ran this
// tool to regenerate an icon. It did exactly that on 2026-08-12 and was caught
// only because the files happened to be committed.
//
// Pass -force to overwrite deliberately.
func cards(force bool) error {
	if err := os.MkdirAll(menuDir, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", menuDir, err)
	}

	written, kept := 0, 0
	for _, sku := range skus {
		path := filepath.Join(menuDir, sku+".jpg")
		if !force {
			if _, err := os.Stat(path); err == nil {
				kept++
				continue
			}
		}
		if err := writeJPEG(path, card(sku)); err != nil {
			return err
		}
		written++
	}

	fmt.Printf("  %d menu cards written, %d existing kept in %s\n", written, kept, menuDir)
	if kept > 0 {
		fmt.Printf("  (existing images are left alone — pass -force to replace real photography)\n")
	}
	return nil
}

// foods are the fill colours the abstract dish shapes use, per cuisine. Kept
// food-plausible rather than brand-pretty: the point of the card is appetite,
// and a plate of emerald blobs does not read as something to eat.
var foods = map[string][]color.NRGBA{
	"IDN": {{0xF2, 0xE3, 0xC4, 0xFF}, {0xC8, 0x3E, 0x2A, 0xFF}, {0xE8, 0xA9, 0x3C, 0xFF}, {0x6E, 0x8F, 0x3A, 0xFF}},
	"CHN": {{0xE8, 0xC0, 0x6A, 0xFF}, {0xB5, 0x33, 0x2C, 0xFF}, {0x8A, 0x5A, 0x2B, 0xFF}, {0x5E, 0x8A, 0x3E, 0xFF}},
	"WST": {{0xD9, 0xA5, 0x62, 0xFF}, {0xC2, 0x40, 0x33, 0xFF}, {0xF0, 0xD9, 0x7A, 0xFF}, {0x74, 0x9B, 0x45, 0xFF}},
	"DRK": {{0xD8, 0x9B, 0x4A, 0xFF}, {0xE8, 0xC4, 0x7A, 0xFF}, {0xF2, 0xEA, 0xD8, 0xFF}, {0x8F, 0xC2, 0x9E, 0xFF}},
}

// rng is a tiny deterministic generator. The same SKU must always produce the
// same card: regenerating assets should never reshuffle how the menu looks.
type rng struct{ s uint64 }

func newRNG(sku string) *rng {
	var h uint64 = 1469598103934665603
	for _, r := range sku {
		// #nosec G115 -- ranging a string yields valid non-negative code points,
		// and this is a hash: only determinism matters, not the numeric value.
		h ^= uint64(r)
		h *= 1099511628211
	}
	return &rng{s: h}
}

func (r *rng) next() float64 {
	r.s ^= r.s << 13
	r.s ^= r.s >> 7
	r.s ^= r.s << 17
	return float64(r.s%1_000_000) / 1_000_000
}

// span returns a value in [lo, hi).
func (r *rng) span(lo, hi float64) float64 { return lo + (hi-lo)*r.next() }

// blob is one abstract piece of food: a slightly squashed, slightly rotated
// disc. Several overlapping ones read as a served dish at thumbnail size far
// better than any single geometric shape does.
type blob struct {
	cx, cy, rx, ry, rot float64
	// n is the superellipse exponent: 2 (the default) is an ellipse, higher
	// values square the shape off. A glass needs straight sides, and an
	// ellipse standing in for one just looks like an egg.
	n    float64
	fill color.NRGBA
}

// coverage returns how much of the pixel this blob covers, smoothed at the
// edge so the shapes are not aliased into staircases.
func (b blob) coverage(x, y float64) float64 {
	n := b.n
	if n == 0 {
		n = 2
	}
	dx, dy := x-b.cx, y-b.cy
	cos, sin := math.Cos(-b.rot), math.Sin(-b.rot)
	u := math.Abs((dx*cos - dy*sin) / b.rx)
	v := math.Abs((dx*sin + dy*cos) / b.ry)
	d := math.Pow(math.Pow(u, n)+math.Pow(v, n), 1/n)
	const aa = 0.02
	return clamp01((1 - d) / aa)
}

// card draws one placeholder dish: a cuisine-tinted backdrop, a plate (or a
// glass, for drinks), and a handful of abstract food shapes. Textless on
// purpose — see the package comment.
func card(sku string) image.Image {
	prefix := sku[:3]
	pair, ok := palette[prefix]
	if !ok {
		pair, prefix = palette["DRK"], "DRK"
	}
	from, to := pair[0], pair[1]
	pantry := foods[prefix]

	r := newRNG(sku)
	shift := (r.next() - 0.5) * 0.10

	img := image.NewNRGBA(image.Rect(0, 0, cardW, cardH))

	cx, cy := float64(cardW)*0.5, float64(cardH)*0.53
	plateR := float64(cardH) * 0.36
	drink := prefix == "DRK"

	shapes := dish(r, cx, cy, plateR, pantry, drink)

	for y := 0; y < cardH; y++ {
		t := float64(y) / float64(cardH-1)
		base := color.NRGBA{
			R: lerp(from.R, to.R, t, shift),
			G: lerp(from.G, to.G, t, shift),
			B: lerp(from.B, to.B, t, shift),
			A: 0xFF,
		}
		fy := float64(y)
		for x := 0; x < cardW; x++ {
			fx := float64(x)
			c := base

			// Vignette: darker corners push the eye to the middle of the card,
			// which is where the food is.
			vx := fx/float64(cardW) - 0.5
			vy := fy/float64(cardH) - 0.5
			c = lighten(c, -0.22*math.Hypot(vx, vy)*math.Hypot(vx, vy)*2)

			// Contact shadow, so the plate sits on the surface instead of
			// floating above it.
			sd := math.Hypot((fx-cx)/(plateR*1.35), (fy-(cy+plateR*0.16))/(plateR*1.18))
			if sd < 1 {
				c = lighten(c, -0.20*(1-sd))
			}

			for _, s := range shapes {
				if cov := s.coverage(fx, fy); cov > 0 {
					c = mix(c, s.fill, cov)
				}
			}

			// Top-left highlight, the light direction every other surface in
			// the design system uses.
			h := 1 - math.Hypot(fx/float64(cardW)-0.28, fy/float64(cardH)-0.20)
			if h > 0 {
				c = lighten(c, 0.10*h*h)
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// dish builds the shape stack back-to-front: vessel first, then what is in it.
func dish(r *rng, cx, cy, plateR float64, pantry []color.NRGBA, drink bool) []blob {
	cream := color.NRGBA{0xFA, 0xF6, 0xEE, 0xFF}
	var out []blob

	if drink {
		// A tumbler: near-straight sides (superellipse), liquid filled to just
		// below the rim, ice floating at the top of the liquid rather than
		// scattered through the whole glass.
		gw, gh := plateR*0.58, plateR*1.24
		glassTop := cy - gh
		liquidTop := glassTop + gh*0.42
		liquidH := (cy + gh - liquidTop) / 2

		out = append(out,
			blob{cx: cx, cy: cy, rx: gw, ry: gh, n: 5, fill: color.NRGBA{0xE4, 0xEC, 0xE9, 0xFF}},
			blob{cx: cx, cy: liquidTop + liquidH, rx: gw * 0.88, ry: liquidH, n: 5, fill: pantry[0]},
			// Rim highlight — the cue that says "glass" more than anything else.
			blob{cx: cx, cy: glassTop + gh*0.03, rx: gw * 0.9, ry: gh * 0.035, n: 4,
				fill: color.NRGBA{0xF8, 0xFB, 0xFA, 0xFF}},
		)
		for i := 0; i < 3; i++ {
			out = append(out, blob{
				cx: cx + r.span(-gw*0.45, gw*0.45),
				cy: liquidTop + r.span(0, liquidH*0.7),
				rx: gw * 0.30, ry: gw * 0.26, rot: r.span(0, math.Pi), n: 3,
				fill: pantry[2],
			})
		}
		return out
	}

	// Plate: rim ring, then the well.
	out = append(out,
		blob{cx: cx, cy: cy, rx: plateR * 1.10, ry: plateR * 1.02, fill: shade(cream, -0.10)},
		blob{cx: cx, cy: cy, rx: plateR * 0.92, ry: plateR * 0.85, fill: cream},
	)

	// A mound in the middle — rice, noodles, whatever the dish is.
	out = append(out, blob{
		cx: cx + r.span(-0.04, 0.04)*plateR, cy: cy - plateR*0.06,
		rx: plateR * r.span(0.44, 0.54), ry: plateR * r.span(0.38, 0.46),
		rot: r.span(-0.3, 0.3), fill: pantry[0],
	})

	// Three or four components arranged around it.
	n := 3 + int(r.span(0, 2))
	start := r.span(0, math.Pi*2)
	for i := 0; i < n; i++ {
		a := start + float64(i)*(math.Pi*2/float64(n)) + r.span(-0.25, 0.25)
		rad := plateR * r.span(0.40, 0.52)
		out = append(out, blob{
			cx:  cx + math.Cos(a)*rad,
			cy:  cy + math.Sin(a)*rad*0.86,
			rx:  plateR * r.span(0.20, 0.30),
			ry:  plateR * r.span(0.17, 0.25),
			rot: r.span(0, math.Pi),
			// Cycle through the pantry, skipping the mound colour.
			fill: pantry[1+i%(len(pantry)-1)],
		})
	}

	// Garnish flecks: small, high-contrast, and the thing that stops the card
	// reading as three flat shapes.
	for i := 0; i < 6; i++ {
		a := r.span(0, math.Pi*2)
		rad := plateR * r.span(0.10, 0.62)
		out = append(out, blob{
			cx: cx + math.Cos(a)*rad, cy: cy + math.Sin(a)*rad*0.86,
			rx: plateR * 0.055, ry: plateR * 0.042, rot: r.span(0, math.Pi),
			fill: pantry[len(pantry)-1],
		})
	}
	return out
}

func mix(dst, src color.NRGBA, a float64) color.NRGBA {
	return color.NRGBA{
		R: clamp(float64(dst.R) + (float64(src.R)-float64(dst.R))*a),
		G: clamp(float64(dst.G) + (float64(src.G)-float64(dst.G))*a),
		B: clamp(float64(dst.B) + (float64(src.B)-float64(dst.B))*a),
		A: 0xFF,
	}
}

func shade(c color.NRGBA, k float64) color.NRGBA { return lighten(c, k) }

func clamp01(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return v
	}
}

func lerp(a, b uint8, t, shift float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	v *= 1 + shift
	return clamp(v)
}

func lighten(c color.NRGBA, k float64) color.NRGBA {
	return color.NRGBA{
		R: clamp(float64(c.R) + (255-float64(c.R))*k),
		G: clamp(float64(c.G) + (255-float64(c.G))*k),
		B: clamp(float64(c.B) + (255-float64(c.B))*k),
		A: c.A,
	}
}

func clamp(v float64) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return uint8(math.Round(v))
	}
}

// ── io ───────────────────────────────────────────────────────────────────────

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path) // #nosec G304 -- fixed paths under the repo
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}

func writeJPEG(path string, img image.Image) error {
	f, err := os.Create(path) // #nosec G304 -- fixed paths under the repo
	if err != nil {
		return err
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 82}); err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return nil
}
