package aeno

import (
	"bytes"
	"image/png"
	"log"
	"sync"

	"github.com/nfnt/resize"
)

// Scene struct to store all data for a scene
type Scene struct {
	Context         *Context
	Objects         []*Object
	Shader          *PhongShader
	eye, center, up Vector
	fovy, aspect    float64
}

// NewScene returns a new scene
func NewScene(eye Vector, center Vector, up Vector, fovy float64, size int, scale int, light Vector, ambient string, diffuse string, near, far float64) *Scene {
	aspect := float64(size) / float64(size)
	matrix := LookAt(eye, center, up).Perspective(fovy, aspect, near, far)
	shader := NewPhongShader(matrix, light, eye, HexColor(ambient), HexColor(diffuse))
	context := NewContext(size*scale, size*scale, 5, shader)
	return &Scene{context, nil, shader, eye, center, up, fovy, aspect}
}

// AddObject adds an object to the scene
func (s *Scene) AddObject(o *Object) {
	s.Objects = append(s.Objects, o)
}

// AddObjects is a convenience method to add multiple objects
func (s *Scene) AddObjects(objects []*Object) {
	for _, o := range objects {
		s.AddObject(o)
	}
}

// FitObjectsToScene fits the objects into a 0.5 unit bounding box
func (s *Scene) FitObjectsToScene(eye, center, up Vector, fovy, aspect, near, far float64) (matrix Matrix) {
	matrix = LookAt(eye, center, up).Perspective(fovy, aspect, near, far)
	shader := NewPhongShader(matrix, Vector{}, eye, HexColor("000000"), HexColor("000000"))

	allMesh := NewEmptyMesh()
	var boxes []Box
	for _, o := range s.Objects {
		if o.Mesh == nil {
			continue
		}
		allMesh.Add(o.Mesh)
		bb := o.Mesh.BoundingBox()
		boxes = append(boxes, bb)
	}
	box := BoxForBoxes(boxes)
	b := NewCubeForBox(box)
	b.BiUnitCube()
	allMesh.FitInside(b.BoundingBox(), V(0.5, 0.5, 0.5))

	indexed := 0
	var addedFOV float64
	for _, o := range s.Objects {
		if o.Mesh == nil {
			continue
		}
		num := len(o.Mesh.Triangles)
		tris := allMesh.Triangles[indexed : num+indexed]
		allInside := false
		for !allInside && len(tris) > 0 {
			for _, t := range tris {
				v1 := shader.Vertex(t.V1)
				v2 := shader.Vertex(t.V2)
				v3 := shader.Vertex(t.V3)

				if v1.Outside() || v2.Outside() || v3.Outside() {
					addedFOV += 5
					matrix = LookAt(eye, center, up).Perspective(fovy+addedFOV, aspect, near, far)
					shader.Matrix = matrix
					allInside = false
				} else {
					allInside = true
				}
			}
		}

		o.Mesh = NewTriangleMesh(tris)
		indexed += num
	}

	return
}

// Draw draws the scene
func (s *Scene) Draw(fit bool, path string, objects []*Object) {
	s.AddObjects(objects)
	if fit {
		s.Shader.Matrix = s.FitObjectsToScene(s.eye, s.center, s.up, s.fovy, s.aspect, 1, 999)
	}
	var wg sync.WaitGroup
	wg.Add(len(s.Objects))
	for _, o := range s.Objects {
		if o.Mesh == nil {
			wg.Done()
			log.Printf("Object attempted to render with nil mesh")
			continue
		}
		go s.Context.DrawObject(o, &wg)
	}
	wg.Wait()
	image := s.Context.Image()
	image = resize.Resize(512, 512, image, resize.Bilinear)

	buf := new(bytes.Buffer)
	png.Encode(buf, image)
	SavePNG(path, image)
	return
}

func GenerateScene(fit bool, path string, objects []*Object, eye Vector, center Vector, up Vector, fovy float64, size int, scale int, light Vector, ambient string, diffuse string, near, far float64) {
	scene := NewScene(eye, center, up, fovy, size, scale, light, ambient, diffuse, near, far)
	scene.Draw(fit, path, objects)
}
