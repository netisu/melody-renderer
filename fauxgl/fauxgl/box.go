package fauxgl

import "math"

// EmptyBox f
var EmptyBox = Box{}

// Box f
type Box struct {
	Min, Max Vector
}

// BoxForBoxes f
func BoxForBoxes(boxes []Box) Box {
	if len(boxes) == 0 {
		return EmptyBox
	}
	x0 := boxes[0].Min.X
	y0 := boxes[0].Min.Y
	z0 := boxes[0].Min.Z
	x1 := boxes[0].Max.X
	y1 := boxes[0].Max.Y
	z1 := boxes[0].Max.Z
	for _, box := range boxes {
		x0 = math.Min(x0, box.Min.X)
		y0 = math.Min(y0, box.Min.Y)
		z0 = math.Min(z0, box.Min.Z)
		x1 = math.Max(x1, box.Max.X)
		y1 = math.Max(y1, box.Max.Y)
		z1 = math.Max(z1, box.Max.Z)
	}
	return Box{Vector{x0, y0, z0}, Vector{x1, y1, z1}}
}

// Volume f
func (a Box) Volume() float64 {
	s := a.Size()
	return s.X * s.Y * s.Z
}

// Anchor f
func (a Box) Anchor(anchor Vector) Vector {
	return a.Min.Add(a.Size().Mul(anchor))
}

// Center f
func (a Box) Center() Vector {
	return a.Anchor(Vector{0.5, 0.5, 0.5})
}

// Size f
func (a Box) Size() Vector {
	return a.Max.Sub(a.Min)
}

// Extend f
func (a Box) Extend(b Box) Box {
	if a == EmptyBox {
		return b
	}
	return Box{a.Min.Min(b.Min), a.Max.Max(b.Max)}
}

// Offset f
func (a Box) Offset(x float64) Box {
	return Box{a.Min.SubScalar(x), a.Max.AddScalar(x)}
}

// Translate f
func (a Box) Translate(v Vector) Box {
	return Box{a.Min.Add(v), a.Max.Add(v)}
}

// Contains f
func (a Box) Contains(b Vector) bool {
	return a.Min.X <= b.X && a.Max.X >= b.X &&
		a.Min.Y <= b.Y && a.Max.Y >= b.Y &&
		a.Min.Z <= b.Z && a.Max.Z >= b.Z
}

// ContainsBox f
func (a Box) ContainsBox(b Box) bool {
	return a.Min.X <= b.Min.X && a.Max.X >= b.Max.X &&
		a.Min.Y <= b.Min.Y && a.Max.Y >= b.Max.Y &&
		a.Min.Z <= b.Min.Z && a.Max.Z >= b.Max.Z
}

// Intersects f
func (a Box) Intersects(b Box) bool {
	return !(a.Min.X > b.Max.X || a.Max.X < b.Min.X || a.Min.Y > b.Max.Y ||
		a.Max.Y < b.Min.Y || a.Min.Z > b.Max.Z || a.Max.Z < b.Min.Z)
}

// Intersection f
func (a Box) Intersection(b Box) Box {
	if !a.Intersects(b) {
		return EmptyBox
	}
	min := a.Min.Max(b.Min)
	max := a.Max.Min(b.Max)
	min, max = min.Min(max), min.Max(max)
	return Box{min, max}
}

// Transform f
func (a Box) Transform(m Matrix) Box {
	return m.MulBox(a)
}
