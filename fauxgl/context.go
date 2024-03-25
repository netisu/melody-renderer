package fauxgl

import (
	"image"
	"math"
	"runtime"
	"sync"

	"github.com/disintegration/imaging"
)

// Face f
type Face int

const (
	_ Face = iota
	// FaceCW f
	FaceCW
	// FaceCCW f
	FaceCCW
)

// Cull f
type Cull int

const (
	_ Cull = iota
	// CullNone f
	CullNone
	// CullFront f
	CullFront
	// CullBack f
	CullBack
)

// RasterizeInfo f
type RasterizeInfo struct {
	TotalPixels   uint64
	UpdatedPixels uint64
}

// Add f
func (info RasterizeInfo) Add(other RasterizeInfo) RasterizeInfo {
	return RasterizeInfo{
		info.TotalPixels + other.TotalPixels,
		info.UpdatedPixels + other.UpdatedPixels,
	}
}

// Context f
type Context struct {
	Width        int
	Height       int
	SuperSample  int
	Shader       Shader
	ColorBuffer  *image.NRGBA
	DepthBuffer  []float64
	ClearColor   Color
	ReadDepth    bool
	WriteDepth   bool
	WriteColor   bool
	AlphaBlend   bool
	Wireframe    bool
	FrontFace    Face
	Cull         Cull
	LineWidth    float64
	DepthBias    float64
	MinX         int
	MaxX         int
	MinY         int
	MaxY         int
	screenMatrix Matrix
	locks        []sync.Mutex
}

// NewContext f
func NewContext(width, height, superSample int, shader Shader) *Context {
	width = width * superSample
	height = height * superSample
	dc := &Context{}
	dc.Width = width
	dc.Height = height
	dc.SuperSample = superSample
	dc.Shader = shader
	dc.ColorBuffer = image.NewNRGBA(image.Rect(0, 0, width, height))
	dc.DepthBuffer = make([]float64, width*height)
	dc.ClearColor = Transparent
	dc.ReadDepth = true
	dc.WriteDepth = true
	dc.WriteColor = true
	dc.AlphaBlend = true
	dc.Wireframe = false
	dc.FrontFace = FaceCCW
	dc.Cull = CullBack
	dc.LineWidth = 2
	dc.DepthBias = 0
	dc.MinX = width
	dc.MinY = height
	dc.screenMatrix = Screen(width, height)
	dc.locks = make([]sync.Mutex, 256)
	dc.ClearDepthBuffer()
	return dc
}

// Image f
func (dc *Context) Image() image.Image {
	img := dc.ColorBuffer
	imgBorder := int(3 * dc.SuperSample)
	img = imaging.Crop(img, image.Rect(dc.MinX-imgBorder, dc.MaxY+imgBorder, dc.MaxX+imgBorder, dc.MinY-imgBorder))
	img = imaging.Fit(img, dc.Width/dc.SuperSample, dc.Height/dc.SuperSample, imaging.Linear)
	m := image.NewRGBA(image.Rect(0, 0, dc.Width/dc.SuperSample, dc.Height/dc.SuperSample))
	return imaging.OverlayCenter(m, img, 1)
}

// ClearColorBufferWith f
func (dc *Context) ClearColorBufferWith(color Color) {
	c := color.NRGBA()
	for y := 0; y < dc.Height; y++ {
		i := dc.ColorBuffer.PixOffset(0, y)
		for x := 0; x < dc.Width; x++ {
			dc.ColorBuffer.Pix[i+0] = c.R
			dc.ColorBuffer.Pix[i+1] = c.G
			dc.ColorBuffer.Pix[i+2] = c.B
			dc.ColorBuffer.Pix[i+3] = c.A
			i += 4
		}
	}
}

// ClearColorBuffer f
func (dc *Context) ClearColorBuffer() {
	dc.ClearColorBufferWith(dc.ClearColor)
}

// ClearDepthBufferWith f
func (dc *Context) ClearDepthBufferWith(value float64) {
	for i := range dc.DepthBuffer {
		dc.DepthBuffer[i] = value
	}
}

// ClearDepthBuffer f
func (dc *Context) ClearDepthBuffer() {
	dc.ClearDepthBufferWith(math.MaxFloat64)
}

func edge(a, b, c Vector) float64 {
	return (b.X-c.X)*(a.Y-c.Y) - (b.Y-c.Y)*(a.X-c.X)
}

func mathMin(x, y int) int {
	if x > y {
		return y
	}
	return x
}

func mathMax(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func (dc *Context) rasterize(v0, v1, v2 Vertex, s0, s1, s2 Vector, fromObject *Object) {
	// integer bounding box
	min := s0.Min(s1.Min(s2)).Floor()
	max := s0.Max(s1.Max(s2)).Ceil()
	dc.MinX = mathMin(dc.MinX, int(min.X))
	dc.MaxX = mathMax(dc.MaxX, int(max.X))
	dc.MinY = mathMin(dc.MinY, int(min.Y))
	dc.MaxY = mathMax(dc.MaxY, int(max.Y))
	x0 := int(min.X)
	x1 := int(max.X)
	y0 := int(min.Y)
	y1 := int(max.Y)

	// forward differencing variables
	p := Vector{float64(x0) + 0.5, float64(y0) + 0.5, 0}
	w00 := edge(s1, s2, p)
	w01 := edge(s2, s0, p)
	w02 := edge(s0, s1, p)
	a01 := s1.Y - s0.Y
	b01 := s0.X - s1.X
	a12 := s2.Y - s1.Y
	b12 := s1.X - s2.X
	a20 := s0.Y - s2.Y
	b20 := s2.X - s0.X

	// reciprocals
	ra := 1 / edge(s0, s1, s2)
	r0 := 1 / v0.Output.W
	r1 := 1 / v1.Output.W
	r2 := 1 / v2.Output.W
	ra12 := 1 / a12
	ra20 := 1 / a20
	ra01 := 1 / a01

	// iterate over all pixels in bounding box
	for y := y0; y <= y1; y++ {
		var d float64
		d0 := -w00 * ra12
		d1 := -w01 * ra20
		d2 := -w02 * ra01
		if w00 < 0 && d0 > d {
			d = d0
		}
		if w01 < 0 && d1 > d {
			d = d1
		}
		if w02 < 0 && d2 > d {
			d = d2
		}
		d = float64(int(d))
		if d < 0 {
			// occurs in pathological cases
			d = 0
		}
		w0 := w00 + a12*d
		w1 := w01 + a20*d
		w2 := w02 + a01*d
		wasInside := false
		for x := x0 + int(d); x <= x1; x++ {
			b0 := w0 * ra
			b1 := w1 * ra
			b2 := w2 * ra
			w0 += a12
			w1 += a20
			w2 += a01
			// check if inside triangle
			if b0 < 0 || b1 < 0 || b2 < 0 {
				if wasInside {
					break
				}
				continue
			}
			wasInside = true
			// check depth buffer for early abort
			i := y*dc.Width + x
			if i < 0 || i >= len(dc.DepthBuffer) {
				// TODO: clipping roundoff error; fix
				// TODO: could also be from fat lines going off screen
				continue
			}
			z := b0*s0.Z + b1*s1.Z + b2*s2.Z
			bz := z + dc.DepthBias
			if dc.ReadDepth && bz > dc.DepthBuffer[i] { // safe w/out lock?
				continue
			}
			// perspective-correct interpolation of vertex data
			b := VectorW{b0 * r0, b1 * r1, b2 * r2, 0}
			b.W = 1 / (b.X + b.Y + b.Z)
			v := InterpolateVertexes(v0, v1, v2, b)
			// invoke fragment shader
			color := dc.Shader.Fragment(v, fromObject)
			if color.A == 0 {
				continue
			}
			// update buffers atomically
			lock := &dc.locks[(x+y)&255]
			lock.Lock()
			// check depth buffer again
			if bz <= dc.DepthBuffer[i] || !dc.ReadDepth {
				if dc.WriteDepth {
					// update depth buffer
					dc.DepthBuffer[i] = z
				}
				if dc.WriteColor {
					// update color buffer
					if dc.AlphaBlend && color.A < 1 {
						sr, sg, sb, sa := color.NRGBA().RGBA()
						a := (0xffff - sa) * 0x101
						j := dc.ColorBuffer.PixOffset(x, y)
						dr := &dc.ColorBuffer.Pix[j+0]
						dg := &dc.ColorBuffer.Pix[j+1]
						db := &dc.ColorBuffer.Pix[j+2]
						da := &dc.ColorBuffer.Pix[j+3]
						*dr = uint8((uint32(*dr)*a/0xffff + sr) >> 8)
						*dg = uint8((uint32(*dg)*a/0xffff + sg) >> 8)
						*db = uint8((uint32(*db)*a/0xffff + sb) >> 8)
						*da = uint8((uint32(*da)*a/0xffff + sa) >> 8)
					} else {
						dc.ColorBuffer.SetNRGBA(x, y, color.NRGBA())
					}
				}
			}
			lock.Unlock()
		}
		w00 += b12
		w01 += b20
		w02 += b01
	}

}

func (dc *Context) line(v0, v1 Vertex, s0, s1 Vector, fromObject *Object) {
	n := s1.Sub(s0).Perpendicular().MulScalar(dc.LineWidth / 2)
	s0 = s0.Add(s0.Sub(s1).Normalize().MulScalar(dc.LineWidth / 2))
	s1 = s1.Add(s1.Sub(s0).Normalize().MulScalar(dc.LineWidth / 2))
	s00 := s0.Add(n)
	s01 := s0.Sub(n)
	s10 := s1.Add(n)
	s11 := s1.Sub(n)
	dc.rasterize(v1, v0, v0, s11, s01, s00, fromObject)
	dc.rasterize(v1, v1, v0, s10, s11, s00, fromObject)
}

func (dc *Context) wireframe(v0, v1, v2 Vertex, s0, s1, s2 Vector, fromObject *Object) {
	dc.line(v0, v1, s0, s1, fromObject)
	dc.line(v1, v2, s1, s2, fromObject)
	dc.line(v2, v0, s2, s0, fromObject)
}

func (dc *Context) drawClippedLine(v0, v1 Vertex, fromObject *Object) {
	// normalized device coordinates
	ndc0 := v0.Output.DivScalar(v0.Output.W).Vector()
	ndc1 := v1.Output.DivScalar(v1.Output.W).Vector()

	// screen coordinates
	s0 := dc.screenMatrix.MulPosition(ndc0)
	s1 := dc.screenMatrix.MulPosition(ndc1)

	// rasterize
	dc.line(v0, v1, s0, s1, fromObject)
}

func (dc *Context) drawClippedTriangle(v0, v1, v2 Vertex, fromObject *Object) {
	// normalized device coordinates
	ndc0 := v0.Output.DivScalar(v0.Output.W).Vector()
	ndc1 := v1.Output.DivScalar(v1.Output.W).Vector()
	ndc2 := v2.Output.DivScalar(v2.Output.W).Vector()

	// back face culling
	a := (ndc1.X-ndc0.X)*(ndc2.Y-ndc0.Y) - (ndc2.X-ndc0.X)*(ndc1.Y-ndc0.Y)
	if a < 0 {
		v0, v2 = v2, v0
		ndc0, ndc2 = ndc2, ndc0
	}
	if dc.Cull == CullFront {
		a = -a
	}
	if dc.FrontFace == FaceCW {
		a = -a
	}
	if dc.Cull != CullNone && a <= 0 {
		return
	}

	// screen coordinates
	s0 := dc.screenMatrix.MulPosition(ndc0)
	s1 := dc.screenMatrix.MulPosition(ndc1)
	s2 := dc.screenMatrix.MulPosition(ndc2)

	// rasterize
	if dc.Wireframe {
		dc.wireframe(v0, v1, v2, s0, s1, s2, fromObject)
	}

	dc.rasterize(v0, v1, v2, s0, s1, s2, fromObject)
}

// DrawLine f
func (dc *Context) DrawLine(t *Line, fromObject *Object) {
	// invoke vertex shader
	v1 := dc.Shader.Vertex(t.V1)
	v2 := dc.Shader.Vertex(t.V2)

	if v1.Outside() || v2.Outside() {
		// clip to viewing volume
		line := ClipLine(NewLine(v1, v2))
		if line != nil {
			dc.drawClippedLine(line.V1, line.V2, fromObject)
		}
	}

	// no need to clip
	dc.drawClippedLine(v1, v2, fromObject)
}

// DrawTriangle f
func (dc *Context) DrawTriangle(t *Triangle, fromObject *Object) {
	// invoke vertex shader
	v1 := dc.Shader.Vertex(t.V1)
	v2 := dc.Shader.Vertex(t.V2)
	v3 := dc.Shader.Vertex(t.V3)

	if v1.Outside() || v2.Outside() || v3.Outside() {
		// clip to viewing volume
		triangles := ClipTriangle(NewTriangle(v1, v2, v3))
		for _, t := range triangles {
			dc.drawClippedTriangle(t.V1, t.V2, t.V3, fromObject)
		}
	}

	// no need to clip
	dc.drawClippedTriangle(v1, v2, v3, fromObject)
}

// DrawLines f
func (dc *Context) DrawLines(o *Object) {
	var wg sync.WaitGroup
	wn := runtime.NumCPU()
	wg.Add(wn)
	for wi := 0; wi < wn; wi++ {
		go func(wi int) {
			for i, l := range o.Mesh.Lines {
				if i%wn == wi {
					dc.DrawLine(l, o)
				}
			}
			wg.Done()
		}(wi)
	}
	wg.Wait()
}

// DrawTriangles f
func (dc *Context) DrawTriangles(o *Object) {
	var wg sync.WaitGroup
	wn := runtime.NumCPU()
	wg.Add(wn)
	for wi := 0; wi < wn; wi++ {
		go func(wi int) {
			for i, t := range o.Mesh.Triangles {
				if i%wn == wi {
					dc.DrawTriangle(t, o)
				}
			}
			wg.Done()
		}(wi)
	}

	wg.Wait()
}

// DrawObject draws the given object
func (dc *Context) DrawObject(o *Object, wg *sync.WaitGroup) {
	dc.DrawTriangles(o)
	dc.DrawLines(o)
	wg.Done()
}
