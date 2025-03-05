package aeno

import (
	"net/http"
	"github.com/go-gl/mathgl/mgl64"
)

// Object struct for objects
// objects can be passed to the renderer to be rendererd
type Object struct {
	Mesh    *Mesh
	Texture Texture
	Color   Color
	Matrix  mgl64.Mat4
}

// NewEmptyObject returns an empty object
func NewEmptyObject() *Object {
    return &Object{Matrix: mgl64.Ident4()} // Initialize with identity!
}

func NewObject(triangles[]*Triangle, lines[]*Line) *Object {
    return &Object{Mesh: NewMesh(triangles, lines), Matrix: mgl64.Ident4()} // Initialize with identity!
}

func NewObjectFromMesh(mesh *Mesh) *Object {
    return &Object{Mesh: mesh, Matrix: mgl64.Ident4()} // Initialize with identity!
}

func NewObjectFromFile(path string) *Object {
    o:= &Object{Matrix: mgl64.Ident4()} // Initialize with identity!
    o.AddMeshFromFile(path)
    o.SetColor(HexColor("777"))
    return o
}

func NewTriangleObject(triangles[]*Triangle) *Object {
    return &Object{Mesh: NewTriangleMesh(triangles), Matrix: mgl64.Ident4()} // Initialize with identity!
}

func NewLineObject(lines[]*Line) *Object {
    return &Object{Mesh: NewLineMesh(lines), Matrix: mgl64.Ident4()} // Initialize with identity!
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
	if err2 != nil {
		panic(err2)
	}
	return obj
}