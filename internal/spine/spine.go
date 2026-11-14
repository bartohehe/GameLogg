package spine

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	_ "image/jpeg" // dekodery rejestrują się w image.Decode
	_ "image/png"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

// Colors to kolory grzbietu jako #RRGGBB.
type Colors struct {
	Color  string // dominujący kolor okładki (tło grzbietu)
	Accent string // drugi kolor, wyraźnie inny jasnością albo odcieniem
	Ink    string // #FFFFFF albo #14121A, kontrast z Color co najmniej 4,5:1
}

const (
	// MaxCoverBytes to limit pobieranej okładki.
	MaxCoverBytes = 2 << 20
	// CoverTimeout to limit czasu pobierania okładki.
	CoverTimeout = 5 * time.Second

	sampleGrid = 64 // okładkę próbkujemy siatką najwyżej 64×64 punkty
	clusters   = 5
	iterations = 10
	minShare   = 0.05 // akcent musi zajmować co najmniej 5% okładki
)

// ErrTooLarge: okładka przekracza MaxCoverBytes.
var ErrTooLarge = errors.New("okładka jest większa niż 2 MB")

// Extractor pobiera okładki i liczy z nich kolory.
type Extractor struct {
	http     *http.Client
	maxBytes int64
}

// NewExtractor tworzy ekstraktor z limitem 5 s i 2 MB na okładkę.
func NewExtractor() *Extractor {
	return &Extractor{http: &http.Client{Timeout: CoverTimeout}, maxBytes: MaxCoverBytes}
}

// FromCover zawsze zwraca użyteczne kolory. Gdy okładki nie da się pobrać
// albo zdekodować, kolory pochodzą z hasha tytułu, a błąd mówi dlaczego
// (do logów).
func (e *Extractor) FromCover(ctx context.Context, coverURL, title string) (Colors, error) {
	if coverURL == "" {
		return Fallback(title), nil
	}
	img, err := e.fetch(ctx, coverURL)
	if err != nil {
		return Fallback(title), err
	}
	c, ok := FromImage(img)
	if !ok {
		return Fallback(title), errors.New("okładka nie ma nieprzezroczystych pikseli")
	}
	return c, nil
}

func (e *Extractor) fetch(ctx context.Context, url string) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, CoverTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pobieranie okładki: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pobieranie okładki: status %d", resp.StatusCode)
	}
	if resp.ContentLength > e.maxBytes {
		return nil, ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, e.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("pobieranie okładki: %w", err)
	}
	if int64(len(data)) > e.maxBytes {
		return nil, ErrTooLarge
	}
	img, _, err := image.Decode(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("dekodowanie okładki: %w", err)
	}
	return img, nil
}

type cluster struct {
	center RGB
	count  int
}

// FromImage liczy kolory z obrazu metodą k-średnich na pomniejszonej siatce
// punktów. Zwraca false dla obrazu bez nieprzezroczystych pikseli.
func FromImage(img image.Image) (Colors, bool) {
	px := sample(img)
	if len(px) == 0 {
		return Colors{}, false
	}
	cs := kmeans(px)
	dominant := cs[0].center

	accent, found := RGB{}, false
	for _, c := range cs[1:] {
		if float64(c.count)/float64(len(px)) < minShare {
			continue
		}
		if distinct(dominant, c.center) {
			accent, found = c.center, true
			break // klastry są posortowane malejąco, więc to największy wyraźnie inny
		}
	}
	if !found {
		accent = derivedAccent(dominant)
	}

	bg, ink := ensureReadable(dominant)
	return Colors{Color: bg.Hex(), Accent: accent.Hex(), Ink: ink.Hex()}, true
}

func sample(img image.Image) []RGB {
	b := img.Bounds()
	stepX := max(1, b.Dx()/sampleGrid)
	stepY := max(1, b.Dy()/sampleGrid)
	px := make([]RGB, 0, sampleGrid*sampleGrid)
	for y := b.Min.Y + stepY/2; y < b.Max.Y; y += stepY {
		for x := b.Min.X + stepX/2; x < b.Max.X; x += stepX {
			r, g, bl, a := img.At(x, y).RGBA()
			if a < 0x8000 {
				continue
			}
			// RGBA zwraca wartości przemnożone przez alfę; przywracamy pełny kolor.
			px = append(px, RGB{uint8(r * 0xFF / a), uint8(g * 0xFF / a), uint8(bl * 0xFF / a)})
		}
	}
	return px
}

// kmeans grupuje piksele i zwraca klastry posortowane malejąco po liczności.
// Środki startowe to kwantyle jasności, więc wynik jest deterministyczny.
func kmeans(px []RGB) []cluster {
	sorted := slices.Clone(px)
	slices.SortFunc(sorted, func(a, b RGB) int {
		la, lb := a.Luminance(), b.Luminance()
		switch {
		case la < lb:
			return -1
		case la > lb:
			return 1
		}
		return 0
	})
	k := min(clusters, len(px))
	centers := make([][3]float64, k)
	for i := range k {
		c := sorted[(2*i+1)*len(sorted)/(2*k)]
		centers[i] = [3]float64{float64(c.R), float64(c.G), float64(c.B)}
	}

	assign := make([]int, len(px))
	for range iterations {
		sums := make([][4]float64, k)
		for i, p := range px {
			best, bestD := 0, math.Inf(1)
			for j, c := range centers {
				dr, dg, db := float64(p.R)-c[0], float64(p.G)-c[1], float64(p.B)-c[2]
				if d := dr*dr + dg*dg + db*db; d < bestD {
					best, bestD = j, d
				}
			}
			assign[i] = best
			sums[best][0] += float64(p.R)
			sums[best][1] += float64(p.G)
			sums[best][2] += float64(p.B)
			sums[best][3]++
		}
		for j := range centers {
			if n := sums[j][3]; n > 0 {
				centers[j] = [3]float64{sums[j][0] / n, sums[j][1] / n, sums[j][2] / n}
			}
		}
	}

	counts := make([]int, k)
	for _, a := range assign {
		counts[a]++
	}
	out := make([]cluster, 0, k)
	for j, c := range centers {
		if counts[j] == 0 {
			continue
		}
		out = append(out, cluster{
			center: RGB{uint8(math.Round(c[0])), uint8(math.Round(c[1])), uint8(math.Round(c[2]))},
			count:  counts[j],
		})
	}
	slices.SortStableFunc(out, func(a, b cluster) int { return b.count - a.count })
	return out
}

// distinct mówi, czy kolory wyraźnie różnią się jasnością albo odcieniem.
func distinct(a, b RGB) bool {
	if math.Abs(a.Lightness()-b.Lightness()) >= 20 {
		return true
	}
	ha, sa := a.hueSat()
	hb, sb := b.hueSat()
	if sa < 0.2 || sb < 0.2 {
		return false // odcień szarości nic nie znaczy
	}
	d := math.Abs(ha - hb)
	return math.Min(d, 360-d) >= 45
}

// derivedAccent buduje akcent z koloru dominującego, gdy okładka jest
// jednolita: jasne kolory przyciemniamy, ciemne rozjaśniamy.
func derivedAccent(c RGB) RGB {
	if c.Lightness() > 55 {
		return mix(c, black, 0.45)
	}
	return mix(c, white, 0.45)
}

// palette to 12 stonowanych barw dla grzbietów bez okładki.
var palette = []RGB{
	{0x3E, 0x4A, 0x6B}, {0x6B, 0x3E, 0x4A}, {0x3E, 0x6B, 0x5A}, {0x6B, 0x5A, 0x3E},
	{0x4A, 0x3E, 0x6B}, {0x2F, 0x5D, 0x6B}, {0x6B, 0x4A, 0x2F}, {0x4F, 0x6B, 0x3E},
	{0x5A, 0x3E, 0x5E}, {0x3E, 0x55, 0x6B}, {0x6B, 0x3E, 0x3E}, {0x3A, 0x4A, 0x3E},
}

// Fallback wyznacza kolory deterministycznie z tytułu.
func Fallback(title string) Colors {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(title))))
	base := palette[h.Sum32()%uint32(len(palette))]
	bg, ink := ensureReadable(base)
	return Colors{Color: bg.Hex(), Accent: mix(base, white, 0.45).Hex(), Ink: ink.Hex()}
}
