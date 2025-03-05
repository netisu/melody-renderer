package aeno

import (
	"math"
)

var clipPlanes = []clipPlane{
	{VectorW{1, 0, 0, 1}, VectorW{-1, 0, 0, 1}},
	{VectorW{-1, 0, 0, 1}, VectorW{1, 0, 0, 1}},
	{VectorW{0, 1, 0, 1}, VectorW{0, -1, 0, 1}},
	{VectorW{0, -1, 0, 1}, VectorW{0, 1, 0, 1}},
	{VectorW{0, 0, 1, 1}, VectorW{0, 0, -1, 1}},
	{VectorW{0, 0, -1, 1}, VectorW{0, 0, 1, 1}},
}

type clipPlane struct {
	P, N VectorW
}

// Point represents a 2D point with x and y coordinates.
type Point struct {
	X, Y float64
}

//  Calculate the intersection point of two lines.
func Intersect(l1, l2 Line) (Point, bool) {
	x1, y1 := l1.V1.Position.X, l1.V1.Position.Y
	x2, y2 := l1.V2.Position.X, l1.V2.Position.Y
	x3, y3 := l2.V1.Position.X, l2.V1.Position.Y
	x4, y4 := l2.V2.Position.X, l2.V2.Position.Y

	denominator := (x1-x2)*(y3-y4) - (y1-y2)*(x3-x4)
	if math.Abs(denominator) < 1e-9 { // Check for parallel lines
		return Point{}, false
	}

	t := ((x1-x3)*(y3-y4) - (y1-y3)*(x3-x4)) / denominator
	u := -((x1-x2)*(y1-y3) - (y1-y2)*(x1-x3)) / denominator

	if 0 <= t && t <= 1 && 0 <= u && u <= 1 {
		x := x1 + t*(x2-x1)
		y := y1 + t*(y2-y1)
		return Point{x, y}, true
	}
	return Point{}, false
}

// Edge represents a line segment with indices of its vertices.
type Edge struct {
	Start, End int
}

// Intersection represents an intersection point between two edges.
type Intersection struct {
	Point          Point
	Subject        Edge
	Clip           Edge
	Classification string
}

// ClassifyIntersection determines whether the intersection is an Entry or Exit point.
// This implementation is simplified and may require adjustments based on specific needs.
func ClassifyIntersection(subject, clip Line, intersection Point) string {
	// Calculate the cross product of the subject line's direction and the vector from
	// the subject line's start point to the intersection point.
	subjectDir := Vector{subject.V2.Position.X - subject.V1.Position.X, subject.V2.Position.Y - subject.V1.Position.Y, 0}
	subjectToIntersection := Vector{intersection.X - subject.V1.Position.X, intersection.Y - subject.V1.Position.Y, 0}
	subjectCross := subjectDir.X*subjectToIntersection.Y - subjectDir.Y*subjectToIntersection.X

	// Calculate the cross product of the clip line's direction and the vector from
	// the clip line's start point to the intersection point.
	clipDir := Vector{clip.V2.Position.X - clip.V1.Position.X, clip.V2.Position.Y - clip.V1.Position.Y, 0}
	clipToIntersection := Vector{intersection.X - clip.V1.Position.X, intersection.Y - clip.V1.Position.Y, 0}
	clipCross := clipDir.X*clipToIntersection.Y - clipDir.Y*clipToIntersection.X

	if subjectCross > 0 && clipCross > 0 {
		return "Entry"
	} else if subjectCross < 0 && clipCross < 0 {
		return "Exit"
	}

	// Handle special cases (e.g., collinear edges)
	return "Unknown"
}

// Weiler-Atherton polygon clipping algorithm.
func WeilerAtherton(subject, clip []VectorW) [][]VectorW {

	subjectPoints := make([]Point, len(subject))
        clipPoints := make([]Point, len(clip))
        for i, v := range subject {
                subjectPoints[i] = Point{v.X, v.Y} 
        }
        for i, v := range clip {
                clipPoints[i] = Point{v.X, v.Y} 
        }
	// Find all intersections
	intersections := []Intersection{}
	for i := 0; i < len(subjectPoints); i++ {
		subjectEdge := Edge{i, (i + 1) % len(subjectPoints)}
		subjectLine := Line{
            Vertex{Position: Vector{subjectPoints[i].X, subjectPoints[i].Y, 0}}, 
			Vertex{Position: Vector{subjectPoints[(i+1)%len(subjectPoints)].X, subjectPoints[(i+1)%len(subjectPoints)].Y, 0}},		}
		for j := 0; j < len(clipPoints); j++ {
			clipEdge := Edge{j, (j + 1) % len(clipPoints)}
			clipLine := Line{
				Vertex{Position: Vector{clipPoints[j].X, clipPoints[j].Y, 0}}, 
				Vertex{Position: Vector{clipPoints[(j+1)%len(clipPoints)].X, clipPoints[(j+1)%len(clipPoints)].Y, 0}},
			}
			intersection, ok := Intersect(subjectLine, clipLine)
			if ok {
				intersections = append(intersections, Intersection{
					Point:   intersection,
					Subject: subjectEdge,
					Clip:    clipEdge,
				})
			}
		}
	}

	// Classify intersections (simplified) (wip)
	for i := range intersections {
		intersections[i].Classification = ClassifyIntersection(
			Line{
				Vertex{Position: Vector{subjectPoints[intersections[i].Subject.Start].X, subjectPoints[intersections[i].Subject.Start].Y, 0}},
				Vertex{Position: Vector{subjectPoints[intersections[i].Subject.End].X, subjectPoints[intersections[i].Subject.End].Y, 0}},
			},
			Line{
				Vertex{Position: Vector{clipPoints[intersections[i].Clip.Start].X, clipPoints[intersections[i].Clip.Start].Y, 0}},
				Vertex{Position: Vector{clipPoints[intersections[i].Clip.End].X, clipPoints[intersections[i].Clip.End].Y, 0}},
			},
			intersections[i].Point,
		)
	}

	// Extract resulting polygons (simplified) (wip)
	resultingPolygons := [][]VectorW{}
	// (Implement polygon extraction logic later)
	return resultingPolygons
}

func (p clipPlane) pointInFront(v VectorW) bool {
	return v.Sub(p.P).Dot(p.N) > 0
}

func (p clipPlane) intersectSegment(v0, v1 VectorW) VectorW {
	u := v1.Sub(v0)
	w := v0.Sub(p.P)
	d := p.N.Dot(u)
	n := -p.N.Dot(w)
	return v0.Add(u.MulScalar(n / d))
}

func sutherlandHodgman(points []VectorW, planes []clipPlane) []VectorW {
	output := points
	for _, plane := range planes {
		input := output
		output = nil
		if len(input) == 0 {
			return nil
		}
		s := input[len(input)-1]
		for _, e := range input {
			if plane.pointInFront(e) {
				if !plane.pointInFront(s) {
					x := plane.intersectSegment(s, e)
					output = append(output, x)
				}
				output = append(output, e)
			} else if plane.pointInFront(s) {
				x := plane.intersectSegment(s, e)
				output = append(output, x)
			}
			s = e
		}
	}
	return output
}

// ClipTriangle f
func ClipTriangle(t *Triangle) []*Triangle {
	w1 := t.V1.Output
	w2 := t.V2.Output
	w3 := t.V3.Output
	p1 := w1.Vector()
	p2 := w2.Vector()
	p3 := w3.Vector()
	points := []VectorW{w1, w2, w3}
	newPoints := sutherlandHodgman(points, clipPlanes)
	var result []*Triangle
	for i := 2; i < len(newPoints); i++ {
		b1 := Barycentric(p1, p2, p3, newPoints[0].Vector())
		b2 := Barycentric(p1, p2, p3, newPoints[i-1].Vector())
		b3 := Barycentric(p1, p2, p3, newPoints[i].Vector())
		v1 := InterpolateVertexes(t.V1, t.V2, t.V3, b1)
		v2 := InterpolateVertexes(t.V1, t.V2, t.V3, b2)
		v3 := InterpolateVertexes(t.V1, t.V2, t.V3, b3)
		result = append(result, NewTriangle(v1, v2, v3))
	}
	return result
}

// ClipLine f
func ClipLine(l *Line) *Line {
	// TODO: interpolate vertex attributes when clipped
	w1 := l.V1.Output
	w2 := l.V2.Output
	for _, plane := range clipPlanes {
		f1 := plane.pointInFront(w1)
		f2 := plane.pointInFront(w2)
		if f1 && f2 {
			continue
		} else if f1 {
			w2 = plane.intersectSegment(w1, w2)
		} else if f2 {
			w1 = plane.intersectSegment(w2, w1)
		} else {
			return nil
		}
	}
	v1 := l.V1
	v2 := l.V2
	v1.Output = w1
	v2.Output = w2
	return NewLine(v1, v2)
}
func ClipVectorLine(l *Line) *Line {
	subject := []VectorW{{l.V1.Position.X, l.V1.Position.Y, 0, 0}, {l.V2.Position.X, l.V2.Position.Y, 0, 0}}
	clip := []VectorW{
		{0, 0, 0, 1}, {1, 0, 0, 1}, {1, 1, 0, 1}, {0, 1, 0, 1},
	}

	result := WeilerAtherton(subject, clip)

	if len(result) == 0 {
		return nil
	}

	v1 := Vertex{Position: Vector{result[0][0].X, result[0][0].Y, 0}}
	v2 := Vertex{Position: Vector{result[0][1].X, result[0][1].Y, 0}}

	return NewLine(v1, v2)
}
