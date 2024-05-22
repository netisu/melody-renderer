package aeno

import (
    "math/rand"
)

//AeoShader is a simplified shader combining Diffuse and Glossy reflection.
type AeoShader struct {
    DiffuseColor  Color
    GlossyColor   Color
    DiffuseFactor float64
    GlossyFactor  float64
}

// NewBlenderStyleShader creates a new BlenderStyleShader.
func NewBlenderStyleShader(diffuseColor, glossyColor Color, diffuseFactor, glossyFactor float64) *AeoShader {
    return &AeoShader{
        DiffuseColor:  diffuseColor,
        GlossyColor:   glossyColor,
        DiffuseFactor: diffuseFactor,
        GlossyFactor:  glossyFactor,
    }
}

// Shade returns the color of a point on the object.
func (s *AeoShader) Shade(hit Hit) Color {
    // Calculate Diffuse component
    diffuse := s.DiffuseColor.MulScalar(s.DiffuseFactor)

    // Calculate Glossy component (simulate rough reflection)
    glossy := s.GlossyColor.MulScalar(s.GlossyFactor)

    // Combine Diffuse and Glossy components
    result := diffuse.Add(glossy)

    // Randomly jitter the color to simulate some variation
    randJitter := Color{
        R: rand.Float64() * 0.1,
        G: rand.Float64() * 0.1,
        B: rand.Float64() * 0.1,
    }
    result = result.Add(randJitter)

    return result
}
