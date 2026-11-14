// Package spine wyznacza kolory grzbietu pudełka (tło, akcent, tusz) z okładki.
package spine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RGB to kolor 8-bitowy.
type RGB struct{ R, G, B uint8 }

// Hex zwraca kolor jako #RRGGBB (wielkie litery).
func (c RGB) Hex() string { return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B) }

// ParseHex czyta #RRGGBB.
func ParseHex(s string) (RGB, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return RGB{}, fmt.Errorf("zły kolor %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return RGB{}, fmt.Errorf("zły kolor %q", s)
	}
	return RGB{uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}

func channel(v uint8) float64 {
	c := float64(v) / 255
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// Luminance to względna luminancja według WCAG 2.
func (c RGB) Luminance() float64 {
	return 0.2126*channel(c.R) + 0.7152*channel(c.G) + 0.0722*channel(c.B)
}

// Lightness to jasność L* (CIE Lab, 0–100), bliższa temu, jak widzi ją oko.
func (c RGB) Lightness() float64 {
	y := c.Luminance()
	if y <= 216.0/24389 {
		return y * 24389 / 27
	}
	return 116*math.Cbrt(y) - 16
}

// hueSat zwraca odcień (0–360) i nasycenie (0–1) w modelu HSL.
func (c RGB) hueSat() (float64, float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := max(r, g, b), min(r, g, b)
	d := mx - mn
	if d == 0 {
		return 0, 0
	}
	l := (mx + mn) / 2
	s := d / (1 - math.Abs(2*l-1))
	var h float64
	switch mx {
	case r:
		h = math.Mod((g-b)/d, 6)
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	h *= 60
	if h < 0 {
		h += 360
	}
	return h, s
}

// Contrast to współczynnik kontrastu WCAG między dwoma kolorami (1–21).
func Contrast(a, b RGB) float64 {
	la, lb := a.Luminance(), b.Luminance()
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// mix miesza c z kolorem t w proporcji f (0 = c, 1 = t).
func mix(c, t RGB, f float64) RGB {
	m := func(a, b uint8) uint8 { return uint8(math.Round(float64(a)*(1-f) + float64(b)*f)) }
	return RGB{m(c.R, t.R), m(c.G, t.G), m(c.B, t.B)}
}

var (
	white = RGB{0xFF, 0xFF, 0xFF}
	black = RGB{0, 0, 0}
	// InkLight i InkDark to jedyne kolory tekstu na grzbiecie.
	InkLight = white
	InkDark  = RGB{0x14, 0x12, 0x1A}
)

// MinContrast to wymagany kontrast tekstu na grzbiecie (WCAG AA).
const MinContrast = 4.5

// pickInk wybiera tusz o większym kontraście z tłem.
func pickInk(bg RGB) RGB {
	if Contrast(bg, InkLight) >= Contrast(bg, InkDark) {
		return InkLight
	}
	return InkDark
}

// ensureReadable zwraca tło i tusz z kontrastem co najmniej 4,5:1. Kolory
// o średniej jasności nie osiągają go z żadnym z dwóch tuszy (maksimum ok. 4,3:1),
// więc wtedy przyciemniamy albo rozjaśniamy tło w stronę przeciwną do tuszu.
func ensureReadable(bg RGB) (RGB, RGB) {
	ink := pickInk(bg)
	toward := black
	if ink == InkDark {
		toward = white
	}
	out := bg
	for f := 0.05; Contrast(out, ink) < MinContrast && f <= 1; f += 0.05 {
		out = mix(bg, toward, f)
	}
	return out, ink
}
