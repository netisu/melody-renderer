package main

import (
	. "fauxgl"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

var (
	eye     = V(-0.75, 0.85, -2)
	center    = V(0, 0.06, 0)
	up      = V(0, 1, 0)
	Dimentions  = 512
	CameraScale = 1 // set to 4 or 5 for production, 2 or 3 for testing and 1 for obj formating
	light    = V(0, 6, -4).Normalize()
	fovy     = 22.5 
	near     = 1.0 
	far     = 1000.0
	color    = "#828282" // #828282 blender lighting
	Amb     = "#d4d4d4" // #d4d4d4 blender ambiance
	cdnDirectory = "./cdn" // set this to your storage root
)

func renderCommand(w http.ResponseWriter, r *http.Request) {

	// Extract query parameters from the HTTP request with default values
	renderType := r.URL.Query().Get("renderType")

	switch renderType {
	case "user":
		renderUser(w, r)
	case "item_preview":
		renderItemPreview(w, r)
	case "item":
		renderItem(w, r)
	case "headshot":
		renderHeadshot(w, r)
	default:
		fmt.Println("Invalid renderType:", renderType)
		return
	}
}

func renderUser(w http.ResponseWriter, r *http.Request) {
	// Delegate user avatar rendering logic here
	fmt.Println("Rendering User Avatar...")
	// ... (call generateObjects and GenerateScene with user specific logic)
}

func renderItemPreview(w http.ResponseWriter, r *http.Request) {
	// Delegate item preview rendering logic here
	fmt.Println("Rendering Item Preview...")
	// ... (call generateObjects and GenerateScene with item preview specific logic)
}

func renderItem(w http.ResponseWriter, r *http.Request) {
	// Delegate item rendering logic here
	fmt.Println("Rendering Item...")
	// ... (call generateObjects and GenerateScene with item specific logic)
}

func renderHeadshot(w http.ResponseWriter, r *http.Request) {
	// Delegate headshot rendering logic here (combine user and headshot logic)
	fmt.Println("Rendering Headshot...")
	// ... (call generateObjects and GenerateScene for user and headshot together)
}

func main() {

	// HTTP endpoint for rendering avatars
	http.HandleFunc("/", renderCommand)

	// Set up and start the HTTP server
	serverAddress := ":8001" // Set the port for the HTTP server
	fmt.Printf("Starting server on %s\n", serverAddress)

	if err := http.ListenAndServe(serverAddress, nil); err != nil {
		fmt.Println("HTTP server error:", err)
	}
}

func GenerateScene(shadows bool, path string, objects []*Object, eye, center, up Vec3, fovy float64, width, height, light, amb, background string, near, far float64) {
	// ... (your existing Scene Generation logic)
}

func AddFace(facePath string) Texture {
	// ... (your existing face loading logic)
}

// ... (other helper functions like RenderHats, ToolClause, generateObjects)
