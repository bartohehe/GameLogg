package spine

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func mustHex(t *testing.T, s string) RGB {
	t.Helper()
	c, err := ParseHex(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// twoTone: górne `share` wysokości w kolorze a, reszta w b.
func twoTone(a, b RGB, share float64) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 120, 160))
	cut := int(160 * share)
	for y := range 160 {
		c := b
		if y < cut {
			c = a
		}
		for x := range 120 {
			img.Set(x, y, color.RGBA{c.R, c.G, c.B, 255})
		}
	}
	return img
}

func near(a, b RGB, tol int) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

func TestContrastAndInk(t *testing.T) {
	tests := []struct {
		bg   string
		ink  string
		want float64 // minimalny oczekiwany kontrast
	}{
		{"#FFFFFF", "#14121A", 17},
		{"#000000", "#FFFFFF", 21},
		{"#2E2A22", "#FFFFFF", 12},
		{"#F2E64A", "#14121A", 12},
		{"#7FD8BE", "#14121A", 9},
	}
	for _, tt := range tests {
		bg := mustHex(t, tt.bg)
		if got := pickInk(bg).Hex(); got != tt.ink {
			t.Errorf("pickInk(%s) = %s, oczekiwano %s", tt.bg, got, tt.ink)
		}
		if c := Contrast(bg, mustHex(t, tt.ink)); c < tt.want {
			t.Errorf("Contrast(%s, %s) = %.2f", tt.bg, tt.ink, c)
		}
	}
	if c := Contrast(white, black); c < 20.99 || c > 21.01 {
		t.Errorf("biały/czarny = %.3f", c)
	}
}

func TestFromImageTwoTones(t *testing.T) {
	tests := []struct {
		name       string
		a, b       string
		share      float64
		wantColor  string
		wantAccent string
		wantInk    string
	}{
		{"ciemne tło, złoty akcent", "#2E2A22", "#D7B55B", 0.7, "#2E2A22", "#D7B55B", "#FFFFFF"},
		{"jasne tło, granatowy akcent", "#DDE6F2", "#2F6BD8", 0.75, "#DDE6F2", "#2F6BD8", "#14121A"},
		{"akcent w mniejszości", "#B8232A", "#141414", 0.6, "#B8232A", "#141414", "#FFFFFF"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ok := FromImage(twoTone(mustHex(t, tt.a), mustHex(t, tt.b), tt.share))
			if !ok {
				t.Fatal("brak kolorów")
			}
			if !near(mustHex(t, c.Color), mustHex(t, tt.wantColor), 2) {
				t.Errorf("Color = %s, oczekiwano ~%s", c.Color, tt.wantColor)
			}
			if !near(mustHex(t, c.Accent), mustHex(t, tt.wantAccent), 2) {
				t.Errorf("Accent = %s, oczekiwano ~%s", c.Accent, tt.wantAccent)
			}
			if c.Ink != tt.wantInk {
				t.Errorf("Ink = %s, oczekiwano %s", c.Ink, tt.wantInk)
			}
		})
	}
}

func TestFromImageSolidDerivesAccent(t *testing.T) {
	for _, hex := range []string{"#20305A", "#E9ECEF", "#7A7A7A"} {
		c, _ := FromImage(twoTone(mustHex(t, hex), mustHex(t, hex), 1))
		bg, acc := mustHex(t, c.Color), mustHex(t, c.Accent)
		if d := bg.Lightness() - acc.Lightness(); d > -15 && d < 15 {
			t.Errorf("%s: akcent %s za mało różni się od tła %s", hex, c.Accent, c.Color)
		}
	}
}

func TestFromImageIgnoresTransparent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10))
	if _, ok := FromImage(img); ok {
		t.Error("przezroczysty obraz nie powinien dać kolorów")
	}
}

// Każde tło po korekcie ma kontrast co najmniej 4,5:1 z wybranym tuszem.
func TestInkContrastAlwaysAtLeast45(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 3000 {
		bg := RGB{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256))}
		out, ink := ensureReadable(bg)
		if c := Contrast(out, ink); c < MinContrast {
			t.Fatalf("%s → %s z tuszem %s: kontrast %.2f", bg.Hex(), out.Hex(), ink.Hex(), c)
		}
		if ink != InkLight && ink != InkDark {
			t.Fatalf("nieznany tusz %s", ink.Hex())
		}
	}
}

func TestFallback(t *testing.T) {
	a1, a2 := Fallback("Elden Ring"), Fallback("  elden ring ")
	if a1 != a2 {
		t.Errorf("ten sam tytuł, różne kolory: %+v %+v", a1, a2)
	}
	seen := map[string]bool{}
	for _, title := range []string{"Elden Ring", "Celeste", "Balatro", "Hades II", "Outer Wilds", "Factorio", "Starfield", "Astro Bot"} {
		c := Fallback(title)
		seen[c.Color] = true
		if Contrast(mustHex(t, c.Color), mustHex(t, c.Ink)) < MinContrast {
			t.Errorf("%s: za mały kontrast %+v", title, c)
		}
	}
	if len(seen) < 4 {
		t.Errorf("paleta zastępcza prawie się nie zmienia: %d kolorów na 8 tytułów", len(seen))
	}
	// Kolor z palety (po ewentualnej korekcie kontrastu nadal ciemny i stonowany).
	if !slices.ContainsFunc(palette, func(p RGB) bool { return near(p, mustHex(t, a1.Color), 40) }) {
		t.Errorf("kolor %s nie pochodzi z palety", a1.Color)
	}
}

// Najmniejszy możliwy WebP 1×1 (bezstratny). Go nie ma kodera WebP.
const tinyWebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func encode(t *testing.T, img image.Image, format string) []byte {
	t.Helper()
	var buf bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	case "webp":
		var b []byte
		b, err = base64.StdEncoding.DecodeString(tinyWebP)
		buf.Write(b)
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFromCover(t *testing.T) {
	img := twoTone(mustHex(t, "#2E2A22"), mustHex(t, "#D7B55B"), 0.7)
	files := map[string][]byte{
		"/cover.png":  encode(t, img, "png"),
		"/cover.jpg":  encode(t, img, "jpeg"),
		"/cover.webp": encode(t, nil, "webp"),
		"/junk.jpg":   []byte("to nie jest obraz"),
		"/huge.jpg":   bytes.Repeat([]byte{0xFF}, MaxCoverBytes+10),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow.jpg" {
			select {
			case <-r.Context().Done():
			case <-time.After(2 * time.Second):
			}
			return
		}
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/huge.jpg" {
			// Bez Content-Length: limit musi zadziałać przy czytaniu.
			w.Header().Set("Transfer-Encoding", "chunked")
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()

	e := NewExtractor()
	ctx := context.Background()
	fallback := Fallback("Tytuł")

	for _, path := range []string{"/cover.png", "/cover.jpg"} {
		c, err := e.FromCover(ctx, srv.URL+path, "Tytuł")
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		// JPEG jest stratny, więc tolerancja jest większa.
		if !near(mustHex(t, c.Color), mustHex(t, "#2E2A22"), 6) || !near(mustHex(t, c.Accent), mustHex(t, "#D7B55B"), 6) {
			t.Errorf("%s: %+v", path, c)
		}
	}

	// Testowy WebP ma jeden przezroczysty piksel: sprawdzamy, że dekoder działa.
	if img, err := e.fetch(ctx, srv.URL+"/cover.webp"); err != nil || img.Bounds().Dx() != 1 {
		t.Errorf("webp: %v", err)
	}

	tests := []struct {
		path    string
		wantErr func(error) bool
	}{
		{"/missing.jpg", func(err error) bool { return err != nil && strings.Contains(err.Error(), "404") }},
		{"/junk.jpg", func(err error) bool { return err != nil && strings.Contains(err.Error(), "dekodowanie") }},
		{"/huge.jpg", func(err error) bool { return errors.Is(err, ErrTooLarge) }},
	}
	for _, tt := range tests {
		c, err := e.FromCover(ctx, srv.URL+tt.path, "Tytuł")
		if !tt.wantErr(err) {
			t.Errorf("%s: err = %v", tt.path, err)
		}
		if c != fallback {
			t.Errorf("%s: oczekiwano kolorów zastępczych, jest %+v", tt.path, c)
		}
	}

	if c, err := e.FromCover(ctx, "", "Tytuł"); err != nil || c != fallback {
		t.Errorf("bez okładki: %+v, %v", c, err)
	}

	short := &Extractor{http: &http.Client{Timeout: 100 * time.Millisecond}, maxBytes: MaxCoverBytes}
	start := time.Now()
	if c, err := short.FromCover(ctx, srv.URL+"/slow.jpg", "Tytuł"); err == nil || c != fallback {
		t.Errorf("timeout: %+v, %v", c, err)
	}
	if time.Since(start) > time.Second {
		t.Error("timeout nie przerwał pobierania")
	}
}
