package aeno

import (
	"bytes"
	"image"
	"math"
	"net/http"
)

// Texture interface for texture
type Texture interface {
	Sample(u, v float64) Color
	BilinearSample(u, v float64) Color
}

// LoadTexture returns texture from filepath
func LoadTex(path string) (Texture, error) {
	im, err := LoadImage(path)
	if err != nil {
		return nil, err
	}
	return NewImageTexture(im), nil
}

// TexFromBytes returns fauxgl texture created with given bytes
func TexFromBytes(data []byte) (tex Texture) {
	img, _, _ := image.Decode(bytes.NewReader(data))

	tex = NewImageTexture(img)

	return
}

// ImageTexture struct to hold image
type ImageTexture struct {
	Width  int
	Height int
	Image  image.Image
}

// NewImageTexture image.Image to texture
func NewImageTexture(im image.Image) Texture {
	size := im.Bounds().Max
	return &ImageTexture{size.X, size.Y, im}
}

// Sample get the color of a texture at coordinates
func (t *ImageTexture) Sample(u, v float64) Color {
	v = 1 - v
	u -= math.Floor(u)
	v -= math.Floor(v)
	x := int(u * float64(t.Width))
	y := int(v * float64(t.Height))
	return MakeColor(t.Image.At(x, y))
}

func LoadTexture(path string) (Texture) {
	tex, error := LoadTex(path)
	if error != nil {
		panic(error)
	}
	return tex
}

func LoadTextureFromURL(url string) (Texture) {
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	im, _, err := image.Decode(resp.Body)
	return NewImageTexture(im)
}

func (t *ImageTexture) BilinearSample(u, v float64) Color {
	v = 1 - v
	u -= math.Floor(u)
	v -= math.Floor(v)
	x := u * float64(t.Width-1)
	y := v * float64(t.Height-1)
	x0 := int(x)
	y0 := int(y)
	x1 := x0 + 1
	y1 := y0 + 1
	x -= float64(x0)
	y -= float64(y0)
	c00 := MakeColor(t.Image.At(x0, y0))
	c01 := MakeColor(t.Image.At(x0, y1))
	c10 := MakeColor(t.Image.At(x1, y0))
	c11 := MakeColor(t.Image.At(x1, y1))
	c := Color{}
	c = c.Add(c00.MulScalar((1 - x) * (1 - y)))
	c = c.Add(c10.MulScalar(x * (1 - y)))
	c = c.Add(c01.MulScalar((1 - x) * y))
	c = c.Add(c11.MulScalar(x * y))
	return c
}