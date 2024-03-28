package fauxgl

import (
	"net/http"
)

// Object struct for objects
// objects can be passed to the renderer to be rendererd
type Object struct {
	Mesh    *Mesh
	Texture Texture
	Color   Color
}

// NewEmptyObject returns an empty object
func NewEmptyObject() *Object {
	return &Object{}
}

// NewObject returns an object with generated mesh
func NewObject(triangles []*Triangle, lines []*Line) *Object {
	return &Object{NewMesh(triangles, lines), nil, Discard}
}

// NewObjectFromMesh returns an object from mesh
func NewObjectFromMesh(mesh *Mesh) *Object {
	return &Object{mesh, nil, Discard}
}

// NewObjectFromFile returns an object from file path
func NewObjectFromFile(path string) *Object {
	o := &Object{}
	o.AddMeshFromFile(path)
	o.SetColor(HexColor("777"))
	return o
}

// NewTriangleObject returns an object with generated mesh
func NewTriangleObject(triangles []*Triangle) *Object {
	return &Object{NewTriangleMesh(triangles), nil, Discard}
}

// NewLineObject returns an object with generated mesh
func NewLineObject(lines []*Line) *Object {
	return &Object{NewLineMesh(lines), nil, Discard}
}

// AddMeshFromFile add mesh to obj
func (o *Object) AddMeshFromFile(path string) {
	o.Mesh, _ = LoadOBJ(path)
}

// SetColor set the color of the mesh
func (o *Object) SetColor(c Color) {
	for _, t := range o.Mesh.Triangles {
		t.SetColor(c)
	}
}

// LoadObject load object from files
func LoadObject(path string) (mesh *Mesh) {
	mesh, err := LoadOBJ(path)
	if err != nil {
		panic(err)
	}
	
	return mesh
}

func LoadObjectFromURL(url string) (*Mesh) {
	file, err := http.Get(url)
    if err != nil {
        panic(err)
    }
	var obj, err2 = LoadOBJFromReader(file.Body)
	if err != nil {
		panic(err2)
	}
	return obj
}