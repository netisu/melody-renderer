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
	ver = "a.1"
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
	fmt.Println("Aeno", ver, "© Aeo Zatoichi Bax")
	
	hash := r.URL.Query().Get("hash")
        if hash == "" {
                hash = "default"
        }

        head_color := r.URL.Query().Get("head_color")
        if head_color == "" {
                head_color = "d4d4d4"
        }

        torso_color := r.URL.Query().Get("torso_color")
        if torso_color == "" {
                torso_color = "d4d4d4"
        }

        leftLeg_color := r.URL.Query().Get("leftLeg_color")
        if leftLeg_color == "" {
                leftLeg_color = "d4d4d4"
        }

        rightLeg_color := r.URL.Query().Get("rightLeg_color")
        if rightLeg_color == "" {
                rightLeg_color = "d4d4d4"
        }

        leftArm_color := r.URL.Query().Get("leftArm_color")
        if leftArm_color == "" {
                leftArm_color = "d4d4d4"
        }

        rightArm_color := r.URL.Query().Get("rightArm_color")
        if rightArm_color == "" {
                rightArm_color = "d4d4d4"
        }

        hat1 := r.URL.Query().Get("hat_1")
        if hat1 == "" {
                hat1 = "none"
        }

        hat2 := r.URL.Query().Get("hat_2")
        if hat2 == "" {
                hat2 = "none"
        }

        hat3 := r.URL.Query().Get("hat_3")
        if hat3 == "" {
                hat3 = "none"
        }

        hat4 := r.URL.Query().Get("hat_4")
        if hat4 == "" {
                hat4 = "none"
        }

        hat5 := r.URL.Query().Get("hat_5")
        if hat5 == "" {
                hat5 = "none"
        }

        hat6 := r.URL.Query().Get("hat_6")
        if hat6 == "" {
                hat6 = "none"
        }

        face := r.URL.Query().Get("face")
        if face == "" {
                face = "default"
        }

        tool := r.URL.Query().Get("tool")
        if tool == "" {
                tool = "none"
        }

        if hash == "default" {
                fmt.Println("Avatar Hash is required")
                return
        }
	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
        fmt.Println("Drawing Objects...")
        // Get the face texture
        faceTexture := AddFace(face)
        // Generate the list of objects using the function
         objects := generateObjects(
                torso_color, leftLeg_color, rightLeg_color, rightArm_color, head_color,
                faceTexture,
                hat1, hat2, hat3, hat4, hat5, hat6,
                tool, leftArm_color, rightArm_color,
         )
	 fmt.Println("Exporting to", cdnDirectory, "thumbnails")
         path := filepath.Join(cdnDirectory, "thumbnails", hash+".png")
         GenerateScene(true, path, objects, eye, center, up, fovy, Dimentions, CameraScale, light, Amb, color, near, far)
         fmt.Println("Completed in", time.Since(start))
        

        // Set the response content type to image/png
        w.Header().Set("Content-Type", "image/png")

}

func renderItemPreview(w http.ResponseWriter, r *http.Request) {
	// Delegate item preview rendering logic here
	fmt.Println("Rendering Item Preview...")
	// ... (call generateObjects and GenerateScene with item preview specific logic)

        item := r.URL.Query().Get("item")
        if item == "" {
                item = "none"
        }
	
	isHandheld := r.URL.Query().Get("istool")
        if isHandheld == "" {
                isHandheld = false
        }

        if hash == "default" {
                fmt.Println("Avatar Hash is required")
                return
        }
	
	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
        fmt.Println("Drawing Objects...")
        // Get the face texture
        faceTexture := AddFace("default")
        // Generate the list of objects using the function
         objects := generateObjects(
                item,
                isHandheld,
         )
	 fmt.Println("Exporting to", cdnDirectory, "thumbnails")
         path := filepath.Join(cdnDirectory, "thumbnails", hash+".png")
         GenerateScene(true, path, objects, eye, center, up, fovy, Dimentions, CameraScale, light, Amb, color, near, far)
         fmt.Println("Completed in", time.Since(start))
        

        // Set the response content type to image/png
        w.Header().Set("Content-Type", "image/png")
}

func renderItem(w http.ResponseWriter, r *http.Request) {
	// Delegate item rendering logic here
	fmt.Println("Rendering Item...")
	
        item := r.URL.Query().Get("hat1")
        if item == "" {
                item = "none"
        }

        if hash == "default" {
                fmt.Println("filename is required")
                return
        }
	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
        fmt.Println("Drawing Objects...")
	
        // Generate the list of objects using the function
         faceTexture := AddFace("default")
        // Generate the list of objects using the function
         objects := generateObjects(
                torso_color, leftLeg_color, rightLeg_color, rightArm_color, head_color,
                faceTexture,
                hat1, hat2, hat3, hat4, hat5, hat6,
                tool, leftArm_color, rightArm_color,
         )
        

        // Set the response content type to image/png
        w.Header().Set("Content-Type", "image/png")
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


func RenderHats(hats ...string) []*Object {
        var objects []*Object

        for _, hat := range hats {
                if hat != "none" {
                        obj := &Object{
                                Mesh:    LoadObject(filepath.Join(cdnDirectory, "/uploads/"+hat+".obj")),
                                Texture: LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+hat+".png")),
                        }
                        objects = append(objects, obj)
                }
        }

        return objects
}

func ToolClause(tool, leftArm_color, rightArm_color string) []*Object {
        var armObjects []*Object

        if tool != "none" {
                // Create objects for the arms with the tool
                leftArm := &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/toolarm.obj")),
                        Color: HexColor(leftArm_color),
                }
                toolObj := &Object{
                        Texture: LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+tool+".png")),
                        Mesh:    LoadObject(filepath.Join(cdnDirectory, "/uploads/"+tool+".obj")),
                }

                armObjects = append(armObjects, leftArm, toolObj)
        } else {
                // Create objects for the arms without the tool
                leftArm := &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/leftarm.obj")),
                        Color: HexColor(leftArm_color),
                }

                armObjects = append(armObjects, leftArm)
        }

        return armObjects
}

func generateObjects(
        torsoColor default "232323", leftLegColor, rightLegColor, rightArmColorParam, headColor string,
        faceTexture Texture,
        hat1, hat2, hat3, hat4, hat5, hat6 string,
        tool, leftArmColor, rightArmColor string,
) []*Object {
        objects := []*Object{
                &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/torso.obj")),
                        Color: HexColor(torsoColor),
                },
                &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/leftleg.obj")),
                        Color: HexColor(leftLegColor),
                },
                &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/rightleg.obj")),
                        Color: HexColor(rightLegColor),
                },
                &Object{
                        Mesh:  LoadObject(filepath.Join(cdnDirectory, "/assets/rightarm.obj")),
                        Color: HexColor(rightArmColorParam),
                },
        }

        // Render and append the face object if a face texture is available
        if faceTexture != nil {
                faceObject := &Object{
                        Mesh:    LoadObject(filepath.Join(cdnDirectory, "/assets/head.obj")),
                        Texture: faceTexture,
                        Color:   HexColor(headColor),
                }
                objects = append(objects, faceObject)
        }

        // Render and append the hat objects
        hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, hat6)
        objects = append(objects, hatObjects...)

        // Render and append the arm objects
        armObjects := ToolClause(tool, leftArmColor, rightArmColor)
        objects = append(objects, armObjects...)

        return objects
}

func AddFace(facePath string) Texture {
        var face Texture

        if facePath != "none" {
                face = LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+facePath+".png"))
        } else {
                face = LoadTextureFromURL("https://cdn.discordapp.com/attachments/883044424903442432/1145691010345730188/face.png")
        }

        return face
}
