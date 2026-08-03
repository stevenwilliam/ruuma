// Command dishphotos fetches real dish photography from Wikimedia Commons and
// renders it to the 4:3 cards the menu grid expects (D31, docs/10 §1.1).
//
// Why Commons: the placeholder cards from tools/genassets are honest but they
// are not food, and a menu sells with photographs. Commons is the one large
// source whose licensing survives contact with a business that takes real
// money — every file here is public domain or a CC licence that permits
// commercial use, and the attribution each one requires is written to
// web/src/credits.json and rendered at /credits.
//
// This does not replace real photography of ruuma's own kitchen. It replaces
// coloured shapes with the right dish, which is the difference between a menu
// a customer can read and one they cannot.
//
// Usage:
//
//	go run ./tools/dishphotos          # fetch everything
//	go run ./tools/dishphotos IDN-001  # re-fetch one SKU
package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	root      = "/home/dev/projects/ruuma"
	outDir    = root + "/web/public/dish"
	creditsTo = root + "/web/src/credits.json"

	cardW, cardH = 1200, 900

	// Wikimedia blocks generic clients. Their policy asks for a descriptive
	// agent with a contact address.
	userAgent = "ruuma-dishphotos/1.0 (https://github.com/stevenwilliam/ruuma; itdept.sfg@gmail.com)"
	apiBase   = "https://commons.wikimedia.org/w/api.php"
)

// picks names a preferred Commons file per SKU, and searchFor gives the query
// used when that title does not resolve. Preferring an exact title keeps the
// tool reproducible — a search result can change under you — while the
// fallback stops a renamed or deleted file from leaving a hole in the menu.
//
// Whatever lands here still has to be looked at. A search for "iced tea"
// happily returns diagrams and bottle labels, and only a human can tell that
// from a glass of tea.
var picks = map[string]string{
	"IDN-001": "Nasi Goreng Kampung.jpg",
	"IDN-002": "Ayam bakar.jpg",
	"IDN-003": "Beef rendang.jpg",
	"IDN-004": "Gado-gado Indonesian salad.JPG",
	"IDN-005": "Sate Ayam Ponorogo.jpg",
	"IDN-006": "Soto Betawi Bening.jpg",
	"CHN-001": "Beef chow fun.jpg",
	"CHN-002": "Kung Pao Chicken (28407052).jpeg",
	"CHN-003": "Claypot tofu.jpg",
	"CHN-004": "Dim Sum Breakfast.jpg",
	"CHN-005": "Peking Duck 3.jpg",
	"CHN-006": "Red braised pork belly.jpg",
	"WST-001": "Grilled chicken breast, Santo Domingo, La Palma.jpg",
	"WST-002": "Fish and chips blackpool.jpg",
	"WST-003": "Espaguetis a la carbonara.jpg",
	"WST-004": "Caesar salad (1).jpg",
	"WST-005": "Cheeseburger (17237580619).jpg",
	"DRK-001": "Es teh manis.jpg",
	"DRK-002": "Orange juice 1.jpg",
	"DRK-003": "Es Kopi Susu Gula Aren.jpg",
	"DRK-004": "Glass of Water - Flickr - Greg Riegler Photography.jpg",
}

// searchFor is the fallback query when a preferred title does not resolve.
// Indonesian dish names are used untranslated where Commons files are indexed
// that way — "sate ayam" finds satay, "chicken satay" finds rather less.
var searchFor = map[string]string{
	"IDN-001": "nasi goreng",
	"IDN-002": "ayam bakar grilled chicken",
	"IDN-003": "rendang",
	"IDN-004": "gado-gado",
	"IDN-005": "sate ayam satay",
	"IDN-006": "soto betawi",
	"CHN-001": "beef chow fun kwetiau",
	"CHN-002": "kung pao chicken",
	"CHN-003": "tofu claypot",
	"CHN-004": "dim sum steamer",
	"CHN-005": "peking duck",
	"CHN-006": "braised pork belly soy",
	"WST-001": "grilled chicken steak plate",
	"WST-002": "fish and chips",
	"WST-003": "spaghetti carbonara",
	"WST-004": "caesar salad",
	"WST-005": "hamburger beef burger",
	"DRK-001": "iced tea glass",
	"DRK-002": "orange juice glass",
	"DRK-003": "iced coffee milk glass",
	"DRK-004": "glass of water",
}

// credit is what the licence obliges us to publish, per image.
type credit struct {
	SKU       string `json:"sku"`
	File      string `json:"file"`
	Author    string `json:"author"`
	Licence   string `json:"licence"`
	LicenceURL string `json:"licence_url"`
	SourceURL string `json:"source_url"`
}

func main() {
	only := os.Args[1:]
	if err := run(only); err != nil {
		log.Fatalf("dishphotos: %v", err)
	}
}

func run(only []string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	existing := loadCredits()
	client := &http.Client{Timeout: 60 * time.Second}

	skus := make([]string, 0, len(picks))
	for sku := range picks {
		if len(only) > 0 && !contains(only, sku) {
			continue
		}
		skus = append(skus, sku)
	}
	sort.Strings(skus)

	for _, sku := range skus {
		c, err := fetch(client, sku, picks[sku])
		if err != nil && strings.Contains(err.Error(), "no such file") {
			// Preferred title is gone; fall back to searching Commons.
			if title, serr := search(client, searchFor[sku]); serr == nil {
				fmt.Printf("  .. %-8s preferred title missing, searched %q -> %s\n",
					sku, searchFor[sku], title)
				c, err = fetch(client, sku, title)
			}
		}
		if err != nil {
			// One bad file must not abandon the other twenty. The placeholder
			// stays in place for that SKU and the failure is reported.
			fmt.Printf("  !! %-8s %v\n", sku, err)
			continue
		}
		existing[sku] = *c
		fmt.Printf("  ok %-8s %s — %s\n", sku, trunc(c.Author, 28), c.Licence)
		// Commons asks clients not to hammer the API.
		time.Sleep(300 * time.Millisecond)
	}

	return saveCredits(existing)
}

// fetch pulls one file's metadata and a scaled rendering, then writes the card.
func fetch(client *http.Client, sku, file string) (*credit, error) {
	q := url.Values{
		"action": {"query"},
		"titles": {"File:" + file},
		"prop":   {"imageinfo"},
		"iiprop": {"url|extmetadata"},
		// Ask Commons to scale server-side: the originals run to 20 MB and we
		// only ever show 1200px.
		"iiurlwidth": {"1600"},
		"format":     {"json"},
	}

	var out struct {
		Query struct {
			Pages map[string]struct {
				Missing   *string `json:"missing"`
				ImageInfo []struct {
					ThumbURL     string `json:"thumburl"`
					URL          string `json:"url"`
					DescriptionURL string `json:"descriptionurl"`
					// Value is not always a string — Commons returns bare
					// numbers for some fields, which makes a string-typed
					// field fail the whole document.
					ExtMetadata map[string]struct {
						Value any `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := getJSON(client, apiBase+"?"+q.Encode(), &out); err != nil {
		return nil, err
	}

	for _, p := range out.Query.Pages {
		if p.Missing != nil {
			return nil, fmt.Errorf("no such file on Commons: %s", file)
		}
		if len(p.ImageInfo) == 0 {
			return nil, fmt.Errorf("no imageinfo for %s", file)
		}
		ii := p.ImageInfo[0]

		meta := func(k string) string { return stripHTML(toStr(ii.ExtMetadata[k].Value)) }
		licence := meta("LicenseShortName")
		if !permitsCommercial(licence) {
			return nil, fmt.Errorf("licence %q is not commercially usable", licence)
		}

		src := ii.ThumbURL
		if src == "" {
			src = ii.URL
		}
		img, err := download(client, src)
		if err != nil {
			return nil, err
		}
		if err := writeCard(filepath.Join(outDir, sku+".jpg"), img); err != nil {
			return nil, err
		}

		return &credit{
			SKU: sku, File: file,
			Author:     firstNonEmpty(meta("Artist"), meta("Credit"), "Wikimedia Commons"),
			Licence:    licence,
			LicenceURL: toStr(ii.ExtMetadata["LicenseUrl"].Value),
			SourceURL:  ii.DescriptionURL,
		}, nil
	}
	return nil, fmt.Errorf("empty response for %s", file)
}

// permitsCommercial keeps NonCommercial and NoDerivatives licences out. A menu
// on a site taking money is a commercial use, and getting this wrong is the
// kind of mistake that surfaces as a takedown notice.
func permitsCommercial(l string) bool {
	s := strings.ToLower(l)
	if strings.Contains(s, "nc") || strings.Contains(s, "noncommercial") ||
		strings.Contains(s, "nd") && !strings.Contains(s, "and") {
		return false
	}
	for _, ok := range []string{"cc0", "public domain", "cc by", "cc-by", "attribution"} {
		if strings.Contains(s, ok) {
			return true
		}
	}
	return false
}

func download(client *http.Client, src string) (image.Image, error) {
	req, _ := http.NewRequest(http.MethodGet, src, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", src, resp.Status)
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, 40<<20))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return img, nil
}

// writeCard centre-crops to 4:3 and scales to the card size. Cropping rather
// than letterboxing: a band of grey around a photo looks like a broken image,
// and food photographs are almost always centre-weighted.
func writeCard(path string, src image.Image) error {
	b := src.Bounds()
	want := float64(cardW) / float64(cardH)
	got := float64(b.Dx()) / float64(b.Dy())

	crop := b
	if got > want {
		w := int(float64(b.Dy()) * want)
		off := (b.Dx() - w) / 2
		crop = image.Rect(b.Min.X+off, b.Min.Y, b.Min.X+off+w, b.Max.Y)
	} else if got < want {
		h := int(float64(b.Dx()) / want)
		// Bias slightly above centre: on a tall food photo the plate sits
		// high and the lower third is usually table.
		off := (b.Dy() - h) * 2 / 5
		crop = image.Rect(b.Min.X, b.Min.Y+off, b.Max.X, b.Min.Y+off+h)
	}

	dst := image.NewRGBA(image.Rect(0, 0, cardW, cardH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, xdraw.Src, nil)

	f, err := os.Create(path) // #nosec G304 -- fixed path under the repo
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, dst, &jpeg.Options{Quality: 84})
}

// search returns the title of the first image file matching a query. Namespace
// 6 is File:. Commons rejects a filetype: prefix here and silently returns
// nothing, so results are filtered after the fact instead.
func search(client *http.Client, query string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("no search term configured")
	}
	q := url.Values{
		"action":      {"query"},
		"list":        {"search"},
		"srsearch":    {query},
		"srnamespace": {"6"},
		"srlimit":     {"1"},
		"format":      {"json"},
	}
	var out struct {
		Query struct {
			Search []struct {
				Title string `json:"title"`
			} `json:"search"`
		} `json:"query"`
	}
	if err := getJSON(client, apiBase+"?"+q.Encode(), &out); err != nil {
		return "", err
	}
	if len(out.Query.Search) == 0 {
		return "", fmt.Errorf("no results for %q", query)
	}
	return strings.TrimPrefix(out.Query.Search[0].Title, "File:"), nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// toStr flattens the loosely-typed extmetadata values into text.
func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strings.TrimSuffix(fmt.Sprintf("%.0f", t), ".0")
	default:
		return fmt.Sprint(t)
	}
}

func getJSON(client *http.Client, u string, v any) error {
	req, _ := http.NewRequest(http.MethodGet, u, nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#039;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.Join(strings.Fields(s), " ")
}

func loadCredits() map[string]credit {
	m := map[string]credit{}
	raw, err := os.ReadFile(creditsTo)
	if err != nil {
		return m
	}
	var list []credit
	if json.Unmarshal(raw, &list) == nil {
		for _, c := range list {
			m[c.SKU] = c
		}
	}
	return m
}

func saveCredits(m map[string]credit) error {
	list := make([]credit, 0, len(m))
	for _, c := range m {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].SKU < list[j].SKU })

	raw, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(creditsTo, append(raw, '\n'), 0o644)
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if strings.EqualFold(h, needle) {
			return true
		}
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
