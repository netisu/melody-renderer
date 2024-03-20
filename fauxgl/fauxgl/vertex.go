package fauxgl

// Vertex f
type Vertex struct {
	Position Vector
	Normal   Vector
	Texture  Vector
	Color    Color
	Output   VectorW
}

// Outside f
func (a Vertex) Outside() bool {
	return a.Output.Outside()
}

// InterpolateVertexes f
func InterpolateVertexes(v1, v2, v3 Vertex, b VectorW) Vertex {
	v := Vertex{}
	v.Position = InterpolateVectors(v1.Position, v2.Position, v3.Position, b)
	v.Normal = InterpolateVectors(v1.Normal, v2.Normal, v3.Normal, b).Normalize()
	v.Texture = InterpolateVectors(v1.Texture, v2.Texture, v3.Texture, b)
	v.Color = InterpolateColors(v1.Color, v2.Color, v3.Color, b)
	v.Output = InterpolateVectorWs(v1.Output, v2.Output, v3.Output, b)
	return v
}

// InterpolateFloats f
func InterpolateFloats(v1, v2, v3 float64, b VectorW) float64 {
	var n float64
	n += v1 * b.X
	n += v2 * b.Y
	n += v3 * b.Z
	return n * b.W
}

// InterpolateColors f
func InterpolateColors(v1, v2, v3 Color, b VectorW) Color {
	n := Color{}
	n = n.Add(v1.MulScalar(b.X))
	n = n.Add(v2.MulScalar(b.Y))
	n = n.Add(v3.MulScalar(b.Z))
	return n.MulScalar(b.W)
}

// InterpolateVectors f
func InterpolateVectors(v1, v2, v3 Vector, b VectorW) Vector {
	n := Vector{}
	n = n.Add(v1.MulScalar(b.X))
	n = n.Add(v2.MulScalar(b.Y))
	n = n.Add(v3.MulScalar(b.Z))
	return n.MulScalar(b.W)
}

// InterpolateVectorWs f
func InterpolateVectorWs(v1, v2, v3, b VectorW) VectorW {
	n := VectorW{}
	n = n.Add(v1.MulScalar(b.X))
	n = n.Add(v2.MulScalar(b.Y))
	n = n.Add(v3.MulScalar(b.Z))
	return n.MulScalar(b.W)
}

// Barycentric f
func Barycentric(p1, p2, p3, p Vector) VectorW {
	v0 := p2.Sub(p1)
	v1 := p3.Sub(p1)
	v2 := p.Sub(p1)
	d00 := v0.Dot(v0)
	d01 := v0.Dot(v1)
	d11 := v1.Dot(v1)
	d20 := v2.Dot(v0)
	d21 := v2.Dot(v1)
	d := d00*d11 - d01*d01
	v := (d11*d20 - d01*d21) / d
	w := (d00*d21 - d01*d20) / d
	u := 1 - v - w
	return VectorW{u, v, w, 1}
}
