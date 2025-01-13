package main

import (
	"aeno" // if there is no aeno, use fauxgl
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
        "io"
        "reflect"
)

const (
	scale      = 1
	fovy       = 22.5
	near       = 0.001
	far        = 1000
	amb        = "606060" // d4d4d4
	lightcolor = "dadada" // 696969
	Dimentions = 512      // april fools (15)
)

var (
	eye           = aeno.V(0.82, 0.85, 2)
	center        = aeno.V(0, 0, 0)
	up            = aeno.V(0, 1.3, 0)
	light         = aeno.V(12, 16, 25).Normalize()
	cdnDirectory  = "/var/www/html/public/cdn" // set this to your storage root
	serverAddress = ":4316"                    // do not put links like (renderer.example.com) until after pentesting
	AccessKey     = "key"
)

type RenderEvent struct {
	RenderType string `json:"RenderType"`
	Hash       string `json:"Hash"`
	RenderJson UserConfig `json:"RenderJson"`
}

type UserConfig struct {
	Items struct {
		Face   string   `json:"face"`
		Hats   []string `json:"hats"`
		Addon  string   `json:"addon"`
		Tool   string   `json:"tool"`
		Head   string   `json:"head"`
		Pants  string   `json:"pants"`
		Shirt  string   `json:"shirt"`
		Tshirt string   `json:"tshirt"`
	} `json:"items"`
	Colors struct {
		HeadColor     string `json:"head_color"`
		TorsoColor    string `json:"torso_color"`
		LeftLegColor  string `json:"leftLeg_color"`
		RightLegColor string `json:"rightLeg_color"`
		LeftArmColor  string `json:"leftArm_color"`
		RightArmColor string `json:"rightArm_color"`
	} `json:"colors"`
}

var useDefault UserConfig = UserConfig{
	Items: struct {
		Face  string   `json:"face"`
		Hats  []string `json:"hats"`
		Addon string   `json:"addon"`
		Tool  string   `json:"tool"`
		Head  string   `json:"head"`

		Pants  string `json:"pants"`
		Shirt  string `json:"shirt"`
		Tshirt string `json:"tshirt"`
	}{
		Face:   "none",
		Hats:   []string{"none"},
		Addon:  "none",
		Head:   "none",
		Tool:   "none",
		Pants:  "none",
		Shirt:  "none",
		Tshirt: "none",
	},
	Colors: struct {
		HeadColor     string `json:"head_color"`
		TorsoColor    string `json:"torso_color"`
		LeftLegColor  string `json:"leftLeg_color"`
		RightLegColor string `json:"rightLeg_color"`
		LeftArmColor  string `json:"leftArm_color"`
		RightArmColor string `json:"rightArm_color"`
	}{
		HeadColor:     "eab372",
		TorsoColor:    "85ad00",
		LeftLegColor:  "eab372",
		RightLegColor: "37302c",
		LeftArmColor:  "eab372",
		RightArmColor: "37302c",
	},
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if AccessKey != "" && r.Header.Get("Aeo-Access-Key") != AccessKey {
			fmt.Println("Unauthorized request")
			http.Error(w, "Unauthorized request", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost {
			fmt.Println("Method not allowed")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		renderCommand(w, r)
	})

	// Start the HTTP server
	fmt.Printf("Starting server on %s\n", serverAddress)
	if err := http.ListenAndServe(serverAddress, nil); err != nil {
		fmt.Println("HTTP server error:", err)
	}
}

func renderCommand(w http.ResponseWriter, r *http.Request) {

        // Read the request body
                body, err := io.ReadAll(r.Body)
                if err != nil {
                        http.Error(w, "Error reading request body", http.StatusBadRequest)
                        return
                }
                defer r.Body.Close()

                // Decode the request body into a RenderEvent struct
                var e RenderEvent
                err = json.Unmarshal([]byte(body), &e)
                if err != nil {
                        fmt.Println("Error decoding request:", err)
                        fmt.Println("Request Body:", string(body)) // For debugging
                        http.Error(w, "Invalid request body", http.StatusBadRequest)
                        return
                }

                fmt.Println(e.RenderType) 

	// Extract query parameters with default values
	fmt.Println("Running Function", e.RenderType)
	switch e.RenderType {
	case "user":
		renderUser(e, w, r)
		renderHeadshot(w, r)
	case "item":
		renderItem(w, r)
	case "item_preview":
		renderItemPreview(w, r)
	default:
		fmt.Println("Invalid renderType:", e.RenderType)
		return
	}
}

func renderUser(e RenderEvent, w http.ResponseWriter, r *http.Request) {

	// Delegate user avatar rendering logic here
	fmt.Println("Getting userstring", e.Hash)

	// Get UserJson from the URL query parameters
	userJson:= e.RenderJson

	// Check if UserJson is present
        if reflect.ValueOf(userJson).IsZero() {
		log.Println("Warning: UserJson query parameter is missing, the avatar will not render !")
		http.Error(w, "UserJson query parameter is missing", http.StatusBadRequest)
		return
	}
	updatedUserConfig, err := updateJson(e.RenderJson, 
                userJson.Items.Face, 
                userJson.Colors.HeadColor, 
                userJson.Colors.TorsoColor, 
                userJson.Colors.LeftLegColor, 
                userJson.Colors.RightLegColor, 
                userJson.Colors.LeftArmColor, 
                userJson.Colors.RightArmColor)
        if err != nil {
                // Handle the error 
                fmt.Println("Error updating UserConfig:", err) 
                http.Error(w, "Error updating user configuration", http.StatusInternalServerError)
                return 
        }
        e.RenderJson = updatedUserConfig 


	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	objects := generateObjects(userJson)
	fmt.Println("Exporting to", cdnDirectory, "thumbnails")
	path := filepath.Join(cdnDirectory, "thumbnails", e.Hash+".png")

	aeno.GenerateScene(
		true,
		path,
		objects,
		eye,
		center,
		up,
		fovy,
		Dimentions,
		scale,
		light,
		amb,
		lightcolor,
		near,
		far,
	)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")

}

func renderItemPreview(w http.ResponseWriter, r *http.Request) {
	// Delegate item preview rendering logic here
	var isFace bool
	var isTool bool
	var isShirt bool
	var isTshirt bool
	var isPants bool
	var PathMod bool

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		hash = "default"
	}

	fmt.Println("Getting item_preview hash", hash)

	item := r.URL.Query().Get("item")
	if item == "" {
		item = "none"
	}

	if isFaceParam, err := strconv.ParseBool(r.URL.Query().Get("isFace")); err == nil {
		isFace = isFaceParam
	}
	if PathModParam, err := strconv.ParseBool(r.URL.Query().Get("pathmod")); err == nil {
		PathMod = PathModParam
	}
	if isPantsParam, err := strconv.ParseBool(r.URL.Query().Get("isPants")); err == nil {
		isPants = isPantsParam
	}

	if isTshirtParam, err := strconv.ParseBool(r.URL.Query().Get("isTshirt")); err == nil {
		isTshirt = isTshirtParam
	}

	if isToolParam, err := strconv.ParseBool(r.URL.Query().Get("isTool")); err == nil {
		isTool = isToolParam
	}
	if isShirtParam, err := strconv.ParseBool(r.URL.Query().Get("isShirt")); err == nil {
		isShirt = isShirtParam
	}

	if hash == "default" {
		fmt.Println("Avatar Hash is required")
		return
	}
	if item == "none" {
		fmt.Println("Item String is required")
		return
	}

	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	objects := generatePreview(
		isFace,
		isTool,
		isShirt,
		isTshirt,
		isPants,
		item,
	)
	fmt.Println("Exporting to", cdnDirectory, "thumbnails")
	if PathMod {
		path := filepath.Join(cdnDirectory, "thumbnails", hash+"_preview.png")
		aeno.GenerateScene(
			true,
			path,
			objects,
			eye,
			center,
			up,
			fovy,
			Dimentions,
			scale,
			light,
			amb,
			lightcolor,
			near,
			far,
		)

	} else {
		path := filepath.Join(cdnDirectory, "thumbnails", hash+".png")
		aeno.GenerateScene(
			true,
			path,
			objects,
			eye,
			center,
			up,
			fovy,
			Dimentions,
			scale,
			light,
			amb,
			lightcolor,
			near,
			far,
		)

	}
	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")
}

func renderItem(w http.ResponseWriter, r *http.Request) {
	// Delegate item rendering logic here
	item := r.URL.Query().Get("item")
	if item == "" {
		item = "none"
	}

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		hash = "default"
	}

	if hash == "default" {
		fmt.Println("itemstring is required")
		return
	}

	fmt.Println("Getting itemstring", hash)

	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	objects := RenderHats(item)
	fmt.Println("Exporting to", cdnDirectory, "thumbnails")
	path := filepath.Join(cdnDirectory, "thumbnails", hash+".png")

	aeno.GenerateScene(
		true,
		path,
		objects,
		aeno.V(1, 2, 3),
		center,
		up,
		fovy,
		Dimentions,
		scale,
		light,
		amb,
		lightcolor,
		near,
		far,
	)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")
	// ... (call generateObjects and GenerateScene with item specific logic)
}

func renderHeadshot(w http.ResponseWriter, r *http.Request) {
	// Delegate headshot rendering logic here
	fmt.Println("Rendering Headshot...")
	var (
		headshot_eye    = aeno.V(4, 7, 17)
		headshot_center = aeno.V(-0.5, 6.8, 0)
		headshot_up     = aeno.V(0, 4, 0)
	)

	hash := r.URL.Query().Get("hash")
	if hash == "" {
		hash = "default"
	}

	// Delegate user avatar rendering logic here
	fmt.Println("Getting userstring_head", hash)

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
	addon := r.URL.Query().Get("addon")

	if addon == "" {

		addon = "none"

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
		face = "none"
	}

	shirt := r.URL.Query().Get("shirt")
	if shirt == "" {
		shirt = "none"
	}

	tshirt := r.URL.Query().Get("tshirt")
	if tshirt == "" {
		tshirt = "none"
	}

	pants := r.URL.Query().Get("pants")
	if pants == "" {
		pants = "none"
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
	objects := generateHeadshot(
		torso_color, leftLeg_color, rightLeg_color, rightArm_color, head_color,
		faceTexture,
		shirt, pants, tshirt,
		hat1, hat2, hat3, hat4, hat5, hat6, addon,
		leftArm_color,
	)
	fmt.Println("Exporting to", cdnDirectory, "thumbnails")
	path := filepath.Join(cdnDirectory, "thumbnails", hash+"_headshot.png")

	aeno.GenerateScene(
		false,
		path,
		objects,
		headshot_eye,
		headshot_center,
		headshot_up,
		fovy,
		Dimentions,
		scale,
		light,
		amb,
		lightcolor,
		near,
		far,
	)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")
}

func RenderHats(hats ...string) []*aeno.Object {
	var objects []*aeno.Object

	for _, hat := range hats {
		if hat != "none" {
			obj := &aeno.Object{
				Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/uploads/"+hat+".obj")),
				Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+hat+".png")),
			}
			objects = append(objects, obj)
		}
	}

	return objects
}

func ToolClause(tool, leftArmColor, rightArmColorParam, shirt string) []*aeno.Object {
	armObjects := []*aeno.Object{}
	if tool != "none" {
		if shirt != "none" {
			// Load tool left arm object
			armObjects = append(armObjects, &aeno.Object{
				Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/toolarm.obj")),
				Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+shirt+".png")),
				Color:   aeno.HexColor(leftArmColor),
			})
		} else {
			armObjects = append(armObjects, &aeno.Object{
				Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/toolarm.obj")),
				Color: aeno.HexColor(leftArmColor),
			})
		}

		// Load tool object based on tool name
		toolObj := &aeno.Object{
			Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+tool+".png")),
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/uploads/"+tool+".obj")),
		}

		// Append tool objects based on if theres a tool
		armObjects = append(armObjects, toolObj)
	} else {
		if shirt != "none" {
			// Append regular left arm if theres no tool
			armObjects = append(armObjects, &aeno.Object{
				Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/leftarm.obj")),
				Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+shirt+".png")),
				Color:   aeno.HexColor(leftArmColor),
			})
		} else {
			// Append regular left arm if theres no tool
			armObjects = append(armObjects, &aeno.Object{
				Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/leftarm.obj")),
				Color: aeno.HexColor(leftArmColor),
			})
		}
	}

	return armObjects
}

func generateObjects(userConfig UserConfig) []*aeno.Object {
	// Extract relevant data from the UserConfig struct
	torsoColor := userConfig.Colors.TorsoColor
	leftLegColor := userConfig.Colors.LeftLegColor
	rightLegColor := userConfig.Colors.RightLegColor
	rightArmColor := userConfig.Colors.RightArmColor

	leftArmColor := userConfig.Colors.LeftArmColor
	headColor := userConfig.Colors.HeadColor

	faceTexture := AddFace(userConfig.Items.Face)

	shirt := userConfig.Items.Shirt
	pants := userConfig.Items.Pants
	tshirt := userConfig.Items.Tshirt
	hat1 := userConfig.Items.Hats[0]
	hat2 := userConfig.Items.Hats[1]
	hat3 := userConfig.Items.Hats[2]
	hat4 := userConfig.Items.Hats[3]
	hat5 := userConfig.Items.Hats[4]
	addon := userConfig.Items.Addon
	tool := userConfig.Items.Tool

	objects := Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, tool, rightArmColor, pants, shirt, tshirt)

	// Render and append the face object if a face texture is available
	if faceTexture != nil {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/head.obj")),
			Texture: faceTexture,
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, faceObject)
	}

	// Render and append the hat objects
	hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, addon)
	objects = append(objects, hatObjects...)

	return objects
}
func generateHeadshot(
	torsoColor, leftLegColor, rightLegColor, rightArmColor, headColor string,
	faceTexture aeno.Texture,
	shirt, pants, tshirt,
	hat1, hat2, hat3, hat4, hat5, hat6, addon string,
	leftArmColor string,
) []*aeno.Object {
	objects := Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, "none", rightArmColor, pants, shirt, tshirt)

	// Render and append the face object if a face texture is available
	if faceTexture != nil {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/head.obj")),
			Texture: faceTexture,
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, faceObject)
	}

	// Render and append the hat objects
	hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, hat6, addon)
	objects = append(objects, hatObjects...)

	return objects
}

func Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, tool, rightArmColorParam, pants, shirt, tshirt string) []*aeno.Object {
	objects := []*aeno.Object{}

	// Load torso object
	objects = append(objects, &aeno.Object{
		Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/torso.obj")),
		Color: aeno.HexColor(torsoColor),
	})

	// Load right arm object
	// Render and append the arm objects
	objects = append(objects, &aeno.Object{
		Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/rightarm.obj")),
		Color: aeno.HexColor(rightArmColorParam),
	})

	// Load leg objects (always loaded)
	objects = append(objects,
		&aeno.Object{
			Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/leftleg.obj")),
			Color: aeno.HexColor(leftLegColor),
		},
		&aeno.Object{
			Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/rightleg.obj")),
			Color: aeno.HexColor(rightLegColor),
		},
	)

	// Load shirt texture if provided
	if shirt != "none" {
		shirtTexture := aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+shirt+".png"))
		for _, obj := range objects[0:2] { // Skip torso and right arm
			obj.Texture = shirtTexture
		}
	}

	// Load pants texture if provided (similar to shirt shii)
	if pants != "none" {
		pantsTexture := aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+pants+".png"))
		for _, obj := range objects[2:] { // Skip torso and right arm
			obj.Texture = pantsTexture
		}
	}
	if tshirt != "none" {
		TshirtLoader := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/tshirt.obj")),
			Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+tshirt+".png")),
		}
		objects = append(objects, TshirtLoader)
	}

	// Handle tool logic

	armObjects := ToolClause(tool, leftArmColor, rightArmColorParam, shirt)
	objects = append(objects, armObjects...)

	return objects
}

func generatePreview(
	isFace bool,
	isTool bool,
	isShirt bool,
	isTshirt bool,
	isPants bool,
	item string,
) []*aeno.Object {
	objects := []*aeno.Object{
		{
			Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/rightarm.obj")),
			Color: aeno.HexColor("d3d3d3"),
		},
		{
			Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/torso.obj")),
			Color: aeno.HexColor("5579C6"),
		},
	}
	if !isTool {
		leftArm := &aeno.Object{
			Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/leftarm.obj")),
			Color: aeno.HexColor("d3d3d3"),
		}
		objects = append(objects, leftArm)
	}
	LeftLeg := &aeno.Object{
		Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/leftleg.obj")),
		Color: aeno.HexColor("d3d3d3"),
	}
	RightLeg := &aeno.Object{
		Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/rightleg.obj")),
		Color: aeno.HexColor("d3d3d3"),
	}

	objects = append(objects, LeftLeg, RightLeg)

	if isTool {
		// Render and append the arm objects
		armObject := ToolClause(item, "d3d3d3", "d3d3d3", "none")
		objects = append(objects, armObject...)
	}

	// Render and append the face object if a face texture is available
	if isFace {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/head.obj")),
			Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+item+".png")),
			Color:   aeno.HexColor("d3d3d3"),
		}
		objects = append(objects, faceObject)
	} else {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/head.obj")),
			Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/assets/default.png")),
			Color:   aeno.HexColor("d3d3d3"),
		}
		objects = append(objects, faceObject)
	}

	if isTshirt {
		TshirtLoader := &aeno.Object{
			Mesh:    aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/tshirt.obj")),
			Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+item+".png")),
			Color:   aeno.HexColor("5579C6"),
		}
		objects = append(objects, TshirtLoader)
	}

	if !isTool && !isTshirt && isShirt {
		shirtTexture := aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+item+".png"))
		for _, obj := range objects[0:3] { // Skip torso and right arm
			obj.Texture = shirtTexture
		}
	}

	if !isTool && !isTshirt && !isShirt && isPants {
		pantsTexture := aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+item+".png"))
		for _, obj := range objects[3:5] { // Skip torso and right arm
			obj.Texture = pantsTexture
		}
	}

	if !isTool && !isFace && !isTshirt && !isShirt && !isPants {
		hatObject := RenderHats(item)
		objects = append(objects, hatObject...)
	}

	return objects
}

func AddFace(facePath string) aeno.Texture {
	var face aeno.Texture

	if facePath != "none" {
		face = aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+facePath+".png"))
	} else {
		face = aeno.LoadTexture(filepath.Join(cdnDirectory, "/assets/default.png"))
	}

	return face
}

func updateJson(userConfig UserConfig, face string, headColor string, torsoColor string, 
        leftLegColor string, rightLegColor string, leftArmColor string, rightArmColor string) (UserConfig, error) {

        userConfig.Items.Face = face
        userConfig.Colors.HeadColor = headColor
        userConfig.Colors.TorsoColor = torsoColor
        userConfig.Colors.LeftLegColor = leftLegColor
        userConfig.Colors.RightLegColor = rightLegColor
        userConfig.Colors.LeftArmColor = leftArmColor
        userConfig.Colors.RightArmColor = rightArmColor

        return userConfig, nil
}
