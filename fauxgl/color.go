package fauxgl

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

var (
	// Discard empty color
	Discard = Color{}
	// Transparent transparent color
	Transparent = Color{}
	// Black black color
	Black = Color{0, 0, 0, 1}
	// White white color
	White = Color{1, 1, 1, 1}
)

// Color color struct
type Color struct {
	R, G, B, A float64
}

// Gray returns gray of given value
func Gray(x float64) Color {
	return Color{x, x, x, 1}
}

// MakeColor converts color from color module to fauxgl
func MakeColor(c color.Color) Color {
	r, g, b, a := c.RGBA()
	const d = 0xffff
	return Color{float64(r) / d, float64(g) / d, float64(b) / d, float64(a) / d}
}

// HexColor converts hex string to color
func HexColor(x string) Color {
	x = strings.Trim(x, "#")
	var r, g, b, a int
	a = 255
	switch len(x) {
	case 3:
		fmt.Sscanf(x, "%1x%1x%1x", &r, &g, &b)
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
	case 4:
		fmt.Sscanf(x, "%1x%1x%1x%1x", &r, &g, &b, &a)
		r = (r << 4) | r
		g = (g << 4) | g
		b = (b << 4) | b
		a = (a << 4) | a
	case 6:
		fmt.Sscanf(x, "%02x%02x%02x", &r, &g, &b)
	case 8:
		fmt.Sscanf(x, "%02x%02x%02x%02x", &r, &g, &b, &a)
	}
	const d = 0xff
	return Color{float64(r) / d, float64(g) / d, float64(b) / d, float64(a) / d}
}

// NRGBA returns nrgba color from fauxgl color
func (a Color) NRGBA() color.NRGBA {
	const d = 0xff
	r := Clamp(a.R, 0, 1)
	g := Clamp(a.G, 0, 1)
	b := Clamp(a.B, 0, 1)
	alpha := Clamp(a.A, 0, 1)
	return color.NRGBA{uint8(r * d), uint8(g * d), uint8(b * d), uint8(alpha * d)}
}

// Opaque makes a color opaque
func (a Color) Opaque() Color {
	return Color{a.R, a.G, a.B, 1}
}

// Alpha sets the alpha of a color
func (a Color) Alpha(alpha float64) Color {
	return Color{a.R, a.G, a.B, alpha}
}

// Lerp lerps two colors
func (a Color) Lerp(b Color, t float64) Color {
	return a.Add(b.Sub(a).MulScalar(t))
}

// Add adds two colors
func (a Color) Add(b Color) Color {
	return Color{a.R + b.R, a.G + b.G, a.B + b.B, a.A + b.A}
}

// Sub subtracts two colors
func (a Color) Sub(b Color) Color {
	return Color{a.R - b.R, a.G - b.G, a.B - b.B, a.A - b.A}
}

// Mul multiplies two colors
func (a Color) Mul(b Color) Color {
	return Color{a.R * b.R, a.G * b.G, a.B * b.B, a.A * b.A}
}

// Div divides two colors
func (a Color) Div(b Color) Color {
	return Color{a.R / b.R, a.G / b.G, a.B / b.B, a.A / b.A}
}

// AddScalar adds based on scalar
func (a Color) AddScalar(b float64) Color {
	return Color{a.R + b, a.G + b, a.B + b, a.A + b}
}

// SubScalar subtracts based on scalar
func (a Color) SubScalar(b float64) Color {
	return Color{a.R - b, a.G - b, a.B - b, a.A - b}
}

// MulScalar multiplies based on scalar
func (a Color) MulScalar(b float64) Color {
	return Color{a.R * b, a.G * b, a.B * b, a.A * b}
}

// DivScalar divides based on scalar
func (a Color) DivScalar(b float64) Color {
	return Color{a.R / b, a.G / b, a.B / b, a.A / b}
}

// Pow applies exponent to each value
func (a Color) Pow(b float64) Color {
	return Color{math.Pow(a.R, b), math.Pow(a.G, b), math.Pow(a.B, b), math.Pow(a.A, b)}
}

// Min minimums each color value
func (a Color) Min(b Color) Color {
	return Color{math.Min(a.R, b.R), math.Min(a.G, b.G), math.Min(a.B, b.B), math.Min(a.A, b.A)}
}

// Max maxes each color value
func (a Color) Max(b Color) Color {
	return Color{math.Max(a.R, b.R), math.Max(a.G, b.G), math.Max(a.B, b.B), math.Max(a.A, b.A)}
}
