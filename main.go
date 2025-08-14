package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
	"github.com/netisu/aeno"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"reflect"
	"regexp"
	"time"
)

const (
	scale      = 1
	fovy       = 22.5
	near       = 1.0
	far        = 1000
	amb        = "b0b0b0" // d4d4d4
	lightcolor = "808080" // 696969
	Dimentions = 512      // april fools (15)
)

var (
	eye     = aeno.V(-0.75, 0.85, 2)
	center  = aeno.V(0, 0, 0)
	up      = aeno.V(0, 1.5, 0)
	light   = aeno.V(-1, 3, 1).Normalize()
	rootDir = "/var/www/renderer"
)

type ItemData struct {
	Item      string     `json:"item"`
	EditStyle *EditStyle `json:"edit_style"`
}
type BodyParts struct {
	Head     string `json:"head"`
	Torso    string `json:"torso"`
	LeftArm  string `json:"left_arm"`
	RightArm string `json:"right_arm"`
	LeftLeg  string `json:"left_leg"`
	RightLeg string `json:"right_leg"`
}

type EditStyle struct {
	Hash      string `json:"hash"`
	IsModel   bool   `json:"is_model"`
	IsTexture bool   `json:"is_texture"`
}

type RenderEvent struct {
	RenderType string     `json:"RenderType"`
	Hash       string     `json:"Hash"`
	RenderJson UserConfig `json:"RenderJson"` // Use interface{} for flexibility
}

type ItemEvent struct {
	RenderType string     `json:"RenderType"`
	Hash       string     `json:"Hash"`
	RenderJson ItemConfig `json:"RenderJson"` // Use interface{} for flexibility
}

type HatsCollection map[string]ItemData

// hatKeyPattern is a regular expression to match keys like "hat_1", "hat_123", etc.
var hatKeyPattern = regexp.MustCompile(`^hat_\d+$`)

type UserConfig struct {
	BodyParts BodyParts `json:"body_parts"`
	Items     struct {
		Face   ItemData       `json:"face"`
		Hats   HatsCollection `json:"hats"`
		Addon  ItemData       `json:"addon"`
		Tool   ItemData       `json:"tool"`
		Head   ItemData       `json:"head"`
		Pants  ItemData       `json:"pants"`
		Shirt  ItemData       `json:"shirt"`
		Tshirt ItemData       `json:"tshirt"`
	} `json:"items"`
	Colors map[string]string `json:"colors"`
}

type ItemConfig struct {
	ItemType string   `json:"ItemType"`
	Item     ItemData `json:"item"`
	PathMod  bool     `json:"PathMod"`
}

var useDefault UserConfig = UserConfig{
	BodyParts: BodyParts{
		Head:     "cranium",
		Torso:    "chesticle",
		LeftArm:  "arm_left",
		RightArm: "arm_right",
		LeftLeg:  "leg_left",
		RightLeg: "leg_right",
	},
	Items: struct {
		Face   ItemData       `json:"face"`
		Hats   HatsCollection `json:"hats"`
		Addon  ItemData       `json:"addon"`
		Tool   ItemData       `json:"tool"`
		Head   ItemData       `json:"head"`
		Pants  ItemData       `json:"pants"`
		Shirt  ItemData       `json:"shirt"`
		Tshirt ItemData       `json:"tshirt"`
	}{
		Face: ItemData{Item: "none"},
		Hats: HatsCollection{
			"hat_1": {Item: "none"},
			"hat_2": {Item: "none"},
			"hat_3": {Item: "none"},
			"hat_4": {Item: "none"},
			"hat_5": {Item: "none"},
			"hat_6": {Item: "none"},
		},
		Addon:  ItemData{Item: "none"},
		Head:   ItemData{Item: "none"},
		Tool:   ItemData{Item: "none"},
		Pants:  ItemData{Item: "none"},
		Shirt:  ItemData{Item: "none"},
		Tshirt: ItemData{Item: "none"},
	},
	Colors: map[string]string{
		"Head":     "d3d3d3",
		"Torso":    "a08bd0",
		"LeftLeg":  "232323",
		"RightLeg": "232323",
		"LeftArm":  "d3d3d3",
		"RightArm": "d3d3d3",
	},
}

func env(key string) string {

	err := godotenv.Load(path.Join(rootDir, ".env"))
	if err != nil {
		log.Fatalf("Note: .env file not found or could not be loaded.")
	}

	return os.Getenv(key)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if env("POST_KEY") != "" && r.Header.Get("Aeo-Access-Key") != env("POST_KEY") {
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
	fmt.Printf("Starting server on %s\n", env("SERVER_ADDRESS"))
	if err := http.ListenAndServe(env("SERVER_ADDRESS"), nil); err != nil {
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

	var i ItemEvent
	err = json.Unmarshal([]byte(body), &i)
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
		renderUser(e, w)
		renderHeadshot(e, w)
	case "item":
		renderItem(i, w)
	case "item_preview":
		renderItemPreview(i, w)
	default:
		fmt.Println("Invalid renderType:", e.RenderType)
		return
	}
}

func renderUser(e RenderEvent, w http.ResponseWriter) {
	// Delegate user avatar rendering logic here
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(env("S3_ACCESS_KEY"), env("S3_SECRET_KEY"), ""),
		Endpoint:         aws.String(env("S3_ENDPOINT")),
		Region:           aws.String(env("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)

	// Create an uploader with the session and default options
	uploader := s3.New(newSession)

	fmt.Println("Getting userstring", e.Hash)

	// Get UserJson from the URL query parameters
	userJson := e.RenderJson

	// Check if UserJson is present
	if reflect.ValueOf(userJson).IsZero() {
		log.Println("Warning: UserJson query parameter is missing, the avatar will not render !")
		http.Error(w, "UserJson query parameter is missing", http.StatusBadRequest)
		return
	}

	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	objects := generateObjects(userJson)
	fmt.Println("Exporting to", env("TEMP_DIR"), "thumbnails")
	outputFile := path.Join("thumbnails", e.Hash+".png")
	outputPath := path.Join(env("TEMP_DIR"), e.Hash+".png") // Renamed 'path' to 'outputPath' to avoid shadowing

	aspect := float64(Dimentions) / float64(Dimentions)
	matrix := aeno.LookAt(eye, center, up).Perspective(fovy, aspect, near, far)

	// 2. Create and fully configure your Phong shader
	myShader := aeno.NewPhongShader(matrix, light, eye, aeno.HexColor(amb), aeno.HexColor(lightcolor))
	myShader.EnableOutline = true
	myShader.OutlineColor = aeno.HexColor("000000")
	myShader.OutlineFactor = 0.08 // A smaller number makes the line thicker

	// 3. Call the NEW function, passing your custom shader
	aeno.GenerateSceneWithShader(
		true,
		myShader,
		outputPath,
		objects,
		eye,
		center,
		up,
		fovy,
		Dimentions,
		scale,
	)

	fmt.Println("Uploading to the", env("S3_BUCKET"), "s3 bucket")

	f, err := os.Open(outputPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", outputPath, err) // Log the error
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	f.Read(buffer)

	object := s3.PutObjectInput{
		Bucket:             aws.String(env("S3_BUCKET")),
		Key:                aws.String(outputFile),
		Body:               bytes.NewReader(buffer),
		ContentLength:      aws.Int64(size),
		ContentType:        aws.String("image/png"),
		ContentDisposition: aws.String("attachment"),
		ACL:                aws.String("public-read"),
	}

	fmt.Println("File uploaded")
	fmt.Printf("%v\n", object)
	_, err = uploader.PutObject(&object)
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = os.Remove(outputPath)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")

}

func renderItemPreview(i ItemEvent, w http.ResponseWriter) {
	var outputFile string
	var fullPath string

	// Delegate user avatar rendering logic here
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(env("S3_ACCESS_KEY"), env("S3_SECRET_KEY"), ""),
		Endpoint:         aws.String(env("S3_ENDPOINT")),
		Region:           aws.String(env("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)

	// Create an uploader with the session and default options
	uploader := s3.New(newSession)

	if i.Hash == "default" {
		fmt.Println("Item Hash is required")
		return
	}
	if i.RenderJson.ItemType == "none" {
		fmt.Println("Item String is required")
		return
	}

	// Get itemJson from the URL query parameters
	itemConfig := i.RenderJson
	itemData := itemConfig.Item

	// Check the inner 'Item' field
	if itemData.Item == "none" {
		log.Println("Warning: No item specified in RenderJson for item event.")
		http.Error(w, "No item specified to render", http.StatusBadRequest)
		return
	}

	// ... (call generateObjects and GenerateScene with user specific logic)
	start := time.Now()
	fmt.Println("Drawing Object for item:", itemData.Item) // Access the inner 'Item' field
	var objects []*aeno.Object

	// Generate the list of objects using the function
	objects = generatePreview(itemConfig)
	fmt.Println("Exporting to", env("TEMP_DIR"), "thumbnails")

	if i.RenderJson.PathMod {
		outputFile = path.Join("thumbnails", i.Hash+"_preview.png") // Assign to outputFile
		fullPath = path.Join(env("TEMP_DIR"), i.Hash+".png")        // Construct the full path
	} else {
		outputFile = path.Join("thumbnails", i.Hash+".png")  // Assign to outputFile
		fullPath = path.Join(env("TEMP_DIR"), i.Hash+".png") // Construct the full path
	}

	aeno.GenerateScene(
		true,
		fullPath,
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

	fmt.Println("Uploading to the", env("S3_BUCKET"), "s3 bucket")

	f, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", fullPath, err) // Log the error
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	f.Read(buffer)

	object := s3.PutObjectInput{
		Bucket:             aws.String(env("S3_BUCKET")),
		Key:                aws.String(outputFile),
		Body:               bytes.NewReader(buffer),
		ContentLength:      aws.Int64(size),
		ContentType:        aws.String("image/png"),
		ContentDisposition: aws.String("attachment"),
		ACL:                aws.String("public-read"),
	}

	fmt.Println("File uploaded")
	fmt.Printf("%v\n", object)
	_, err = uploader.PutObject(&object)
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = os.Remove(fullPath)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")

}

func renderItem(i ItemEvent, w http.ResponseWriter) {
	// Delegate user avatar rendering logic here
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(env("S3_ACCESS_KEY"), env("S3_SECRET_KEY"), ""),
		Endpoint:         aws.String(env("S3_ENDPOINT")),
		Region:           aws.String(env("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)

	// Create an uploader with the session and default options
	uploader := s3.New(newSession)

	if i.Hash == "" {
		fmt.Println("itemstring is required")
		return
	}

	fmt.Println("Getting itemstring", i.Hash)

	// Get itemJson from the URL query parameters
	itemConfig := i.RenderJson
	itemData := itemConfig.Item

	// Check if UserJson is present
	if reflect.ValueOf(itemConfig).IsZero() {
		log.Println("Warning: itemJson query parameter is missing, the item will not render !")
		http.Error(w, "itemJson query parameter is missing", http.StatusBadRequest)
		return
	}
	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	var objects []*aeno.Object

	renderedObject := RenderItem(itemData)
	objects = []*aeno.Object{renderedObject}

	fmt.Println("Exporting to", env("TEMP_DIR"), "thumbnails")
	outputFile := path.Join("thumbnails", i.Hash+".png")
	outputPath := path.Join(env("TEMP_DIR"), i.Hash+".png")

	aeno.GenerateScene(
		true,
		outputPath,
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

	fmt.Println("Uploading to the", env("S3_BUCKET"), "s3 bucket")

	f, err := os.Open(outputPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", outputPath, err) // Log the error
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	f.Read(buffer)

	object := s3.PutObjectInput{
		Bucket:             aws.String(env("S3_BUCKET")),
		Key:                aws.String(outputFile),
		Body:               bytes.NewReader(buffer),
		ContentLength:      aws.Int64(size),
		ContentType:        aws.String("image/png"),
		ContentDisposition: aws.String("attachment"),
		ACL:                aws.String("public-read"),
	}

	fmt.Println("File uploaded")
	fmt.Printf("%v\n", object)
	_, err = uploader.PutObject(&object)
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = os.Remove(outputPath)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")
	// ... (call generateObjects and GenerateScene with item specific logic)
}

func renderHeadshot(e RenderEvent, w http.ResponseWriter) {
	// Delegate user avatar rendering logic here
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(env("S3_ACCESS_KEY"), env("S3_SECRET_KEY"), ""),
		Endpoint:         aws.String(env("S3_ENDPOINT")),
		Region:           aws.String(env("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)

	// Create an uploader with the session and default options
	uploader := s3.New(newSession)

	// Delegate headshot rendering logic here
	fmt.Println("Rendering Headshot...")
	var (
		headshot_fovy   = 15.5
		headshot_near   = 1.0    // Much smaller near plane for close-ups
		headshot_far    = 1000.0 // Can be smaller for headshots as well
		headshot_eye    = aeno.V(4, 7, 13)
		headshot_center = aeno.V(-0.5, 6.8, 0)
		headshot_up     = aeno.V(0, 4, 0)
	)

	// Get UserJson from the URL query parameters
	userJson := e.RenderJson

	// Check if UserJson is present
	if reflect.ValueOf(userJson).IsZero() {
		log.Println("Warning: UserJson query parameter is missing, the avatar will not render !")
		http.Error(w, "UserJson query parameter is missing", http.StatusBadRequest)
		return
	}

	start := time.Now()
	fmt.Println("Drawing Objects...")
	// Generate the list of objects using the function
	objects := generateObjects(userJson)

	fmt.Println("Exporting to", env("TEMP_DIR"), "thumbnails")
	outputFile := path.Join("thumbnails", e.Hash+"_headshot.png")

	path := path.Join(env("TEMP_DIR"), e.Hash+"_headshot.png")
	aeno.GenerateScene(
		false,
		path,
		objects,
		headshot_eye,
		headshot_center,
		headshot_up,
		headshot_fovy,
		Dimentions,
		scale,
		light,
		amb,
		lightcolor,
		headshot_near,
		headshot_far,
	)

	fmt.Println("Uploading to the", env("S3_BUCKET"), "s3 bucket")

	f, err := os.Open(path)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", path, err) // Log the error
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	f.Read(buffer)

	object := s3.PutObjectInput{
		Bucket:             aws.String(env("S3_BUCKET")),
		Key:                aws.String(outputFile),
		Body:               bytes.NewReader(buffer),
		ContentLength:      aws.Int64(size),
		ContentType:        aws.String("image/png"),
		ContentDisposition: aws.String("attachment"),
		ACL:                aws.String("public-read"),
	}

	fmt.Println("File uploaded")
	fmt.Printf("%v\n", object)
	_, err = uploader.PutObject(&object)
	if err != nil {
		fmt.Println(err.Error())
	}
	_ = os.Remove(path)

	fmt.Println("Completed in", time.Since(start))

	// Set the response content type to image/png
	w.Header().Set("Content-Type", "image/png")
}

func RenderItem(itemData ItemData) *aeno.Object {
	if itemData.Item == "none" {
		return nil // No item to render for this slot
	}

	cdnURL := env("CDN_URL")
	meshURL := fmt.Sprintf("%s/uploads/%s.obj", cdnURL, itemData.Item)
	textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.Item)

	if itemData.EditStyle != nil {
		if itemData.EditStyle.IsModel {
			meshURL = fmt.Sprintf("%s/uploads/%s.obj", env("CDN_URL"), itemData.EditStyle.Hash)
			log.Printf("DEBUG: Applying model override for item %s with style %s\n", itemData.Item, itemData.EditStyle.Hash)
		}
		if itemData.EditStyle.IsTexture {
			textureURL = fmt.Sprintf("%s/uploads/%s.png", env("CDN_URL"), itemData.EditStyle.Hash)
			log.Printf("DEBUG: Applying texture override for item %s with style %s\n", itemData.Item, itemData.EditStyle.Hash)
		}
	}

	var texture aeno.Texture
	resp, err := http.Head(textureURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		texture = aeno.LoadTextureFromURL(textureURL)
	} else {
		fmt.Printf("Info: No texture found for item at %s. Rendering with color only.\n", textureURL)
	}

	return &aeno.Object{
		Mesh:    aeno.LoadObjectFromURL(meshURL),
		Color:   aeno.Transparent,
		Texture: texture,
		Matrix:  aeno.Identity(),
	}
}

func ToolClause(toolData ItemData, leftArmColor string, shirtTextureHash string) []*aeno.Object {
	objects := []*aeno.Object{}
	cdnUrl := env("CDN_URL")

	var shirtTexture aeno.Texture
	if shirtTextureHash != "none" {
		textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnUrl, shirtTextureHash)
		resp, err := http.Head(textureURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			shirtTexture = aeno.LoadTextureFromURL(textureURL)
		}
	}

	var armMesh *aeno.Mesh

	if toolData.Item != "none" {
		armMesh = aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_tool.obj", cdnUrl))

		if toolObj := RenderItem(toolData); toolObj != nil {
			objects = append(objects, toolObj)
		}
	} else {
		// If no tool is equipped, use the default arm mesh.
		armMesh = aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_left.obj", cdnUrl))
	}

	armObject := &aeno.Object{
		Mesh:    armMesh,
		Color:   aeno.HexColor(leftArmColor),
		Texture: shirtTexture, 
		Matrix:  aeno.Identity(),
	}
	objects = append(objects, armObject)

	return objects
}

func generateObjects(userConfig UserConfig) []*aeno.Object {
	fmt.Printf("generateObjects: Starting. UserConfig: %+v\n", userConfig)

	var allObjects []*aeno.Object
	cdnURL := env("CDN_URL")

	// --- DYNAMICALLY LOAD HEAD MESH ---
	headMeshName := userConfig.BodyParts.Head
	if headMeshName == "" {
		headMeshName = "cranium"
	}

	var headMeshPath string
	if headMeshName == "cranium" {
		headMeshPath = fmt.Sprintf("%s/assets/%s.obj", cdnURL, headMeshName)
	} else {
		headMeshPath = fmt.Sprintf("%s/uploads/%s.obj", cdnURL, headMeshName)
	}
	cachedCraniumMesh := aeno.LoadObjectFromURL(headMeshPath)

	fmt.Printf("generateObjects: Cached Head Mesh Pointer: %p\n", cachedCraniumMesh)

	bodyAndApparelObjects := Texturize(userConfig)
	allObjects = append(allObjects, bodyAndApparelObjects...)

	allObjects = append(allObjects, &aeno.Object{
		Mesh:   cachedCraniumMesh,
		Color:  aeno.HexColor(userConfig.Colors["Head"]),
		Matrix: aeno.Identity(),
	})

	fmt.Printf("generateObjects: Cranium mesh added. Total objects: %d\n", len(allObjects))

	fmt.Printf("generateObjects: Applying Face. Face Item: %s\n", userConfig.Items.Face.Item)
	for _, obj := range allObjects {
		if obj.Mesh != nil {
			fmt.Printf("  generateObjects: Checking obj mesh %p against cached cranium mesh %p for face texture application.\n", obj.Mesh, cachedCraniumMesh)
        	if obj.Mesh == cachedCraniumMesh {
				fmt.Printf("  generateObjects: FOUND cranium mesh for face! Applying face texture.\n")
				obj.Texture = AddFace(userConfig.Items.Face.Item)
				break
			}
		}
	}

	fmt.Printf("generateObjects: Processing Addon. Addon Item: %s\n", userConfig.Items.Addon.Item)
	if obj := RenderItem(userConfig.Items.Addon); obj != nil {
		allObjects = append(allObjects, obj)
		fmt.Printf("generateObjects: Addon object added. Total objects: %d\n", len(allObjects))
	}

	fmt.Printf("generateObjects: Processing Hats. Count: %d\n", len(userConfig.Items.Hats))
	for hatKey, hatItemData := range userConfig.Items.Hats {
		if !hatKeyPattern.MatchString(hatKey) {
			log.Printf("Warning: Invalid hat key format: '%s'. Skipping hat.\n", hatKey)
			continue
		}

		fmt.Printf("  Hat Key: %s, Item: %s\n", hatKey, hatItemData.Item)
		if hatItemData.Item != "none" {
			if obj := RenderItem(hatItemData); obj != nil {
				allObjects = append(allObjects, obj)
				fmt.Printf("Hat object for %s added. Total objects: %d\n", hatKey, len(allObjects))
			}
		}
	}
	fmt.Printf("generateObjects: Finished. Final object count: %d\n", len(allObjects))
	return allObjects
}

func Texturize(config UserConfig) []*aeno.Object {
	objects := []*aeno.Object{}
	cdnUrl := env("CDN_URL")

	// Helper function to build the correct path
	getMeshPath := func(partName, defaultName string) string {
		if partName == "" {
			partName = defaultName
		}
		if partName == defaultName {
			return fmt.Sprintf("%s/assets/%s.obj", cdnUrl, partName)
		}
		return fmt.Sprintf("%s/uploads/%s.obj", cdnUrl, partName)
	}

	// --- CACHED MESH REFERENCES FOR TEXTURIZE'S INTERNAL USE ---
	// These are loaded once within Texturize to ensure consistent pointers for its internal slice indexing.
	torsoPath := getMeshPath(config.BodyParts.Torso, "chesticle")
	rightArmPath := getMeshPath(config.BodyParts.RightArm, "arm_right")
	leftLegPath := getMeshPath(config.BodyParts.LeftLeg, "leg_left")
	rightLegPath := getMeshPath(config.BodyParts.RightLeg, "leg_right")

	cachedChesticleMesh := aeno.LoadObjectFromURL(torsoPath)
	cachedArmRightMesh := aeno.LoadObjectFromURL(rightArmPath)
	cachedLegLeftMesh := aeno.LoadObjectFromURL(leftLegPath)
	cachedLegRightMesh := aeno.LoadObjectFromURL(rightLegPath)
	cachedTeeMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/tee.obj", cdnUrl))

	objects = append(objects, &aeno.Object{
		Mesh:   cachedChesticleMesh,
		Color:  aeno.HexColor(config.Colors["Torso"]),
		Matrix: aeno.Identity(),
	})

	fmt.Printf("Texturize: Added Chesticle Mesh Pointer: %p\n", cachedChesticleMesh)

	objects = append(objects, &aeno.Object{
		Mesh:   cachedArmRightMesh,
		Color:  aeno.HexColor(config.Colors["RightArm"]),
		Matrix: aeno.Identity(),
	})
	fmt.Printf("Texturize: Added Right Arm Mesh Pointer: %p\n", cachedArmRightMesh)

	objects = append(objects,
		&aeno.Object{
			Mesh:   cachedLegLeftMesh,
			Color:  aeno.HexColor(config.Colors["LeftLeg"]),
			Matrix: aeno.Identity(),
		},
		&aeno.Object{
			Mesh:   cachedLegRightMesh,
			Color:  aeno.HexColor(config.Colors["RightLeg"]),
			Matrix: aeno.Identity(),
		},
	)
	fmt.Printf("Texturize: Added Left Leg Mesh Pointer: %p\n", cachedLegLeftMesh)
	fmt.Printf("Texturize: Added Right Leg Mesh Pointer: %p\n", cachedLegRightMesh)

	fmt.Println("Texturize: Initial body meshes created in order for slicing:")
	for i, obj := range objects {
		if obj.Mesh != nil {
			fmt.Printf("  Texturize Index %d: Mesh Pointer %p\n", i, obj.Mesh)
		}
	}

	fmt.Printf("Texturize: Processing Shirt. Shirt Item: %s\n", config.Items.Shirt.Item)
	if config.Items.Shirt.Item != "none" {
		shirtTextureURL := fmt.Sprintf("%s/uploads/%s.png", cdnUrl, config.Items.Shirt.Item)
		fmt.Printf("Texturize: Loading shirt texture from URL: %s\n", shirtTextureURL)
		shirtTexture := aeno.LoadTextureFromURL(shirtTextureURL)

		fmt.Printf("Texturize: Shirt texture loaded. Applying to objects[0:2] (torso, right arm).\n")
		for _, obj := range objects[0:2] {
			fmt.Printf("  Texturize: Applying shirt texture to mesh %p\n", obj.Mesh)
			obj.Texture = shirtTexture
		}
	}

	fmt.Printf("Texturize: Processing Pants. Pants Item: %s\n", config.Items.Pants.Item)
	if config.Items.Pants.Item != "none" {
		pantsTextureURL := fmt.Sprintf("%s/uploads/%s.png", cdnUrl, config.Items.Pants.Item)
		fmt.Printf("Texturize: Loading pants texture from URL: %s\n", pantsTextureURL)
		pantsTexture := aeno.LoadTextureFromURL(pantsTextureURL)

		fmt.Printf("Texturize: Pants texture loaded. Applying to objects[2:] (legs).\n")
		for _, obj := range objects[2:] {
			fmt.Printf("Texturize: Applying pants texture to mesh %p\n", obj.Mesh)
			obj.Texture = pantsTexture
		}
	}

	fmt.Printf("Texturize: Processing T-shirt. T-shirt Item: %s\n", config.Items.Tshirt.Item)
	if config.Items.Tshirt.Item != "none" {
		tshirtTextureURL := fmt.Sprintf("%s/uploads/%s.png", cdnUrl, config.Items.Tshirt.Item)
		fmt.Printf("Texturize: Loading T-shirt texture from URL: %s\n", tshirtTextureURL)
		tshirtTexture := aeno.LoadTextureFromURL(tshirtTextureURL)

		fmt.Printf("Texturize: T-shirt texture loaded. Adding as new object.\n")
		TshirtLoader := &aeno.Object{
			Mesh:    cachedTeeMesh,
			Color:   aeno.Transparent,
			Texture: tshirtTexture,
			Matrix:  aeno.Identity(),
		}
		objects = append(objects, TshirtLoader)
		fmt.Printf("Texturize: T-shirt object added. Total objects: %d\n", len(objects))
	}

	fmt.Printf("Texturize: Processing Tool. Tool Item: %s\n", config.Items.Tool.Item)
	armObjects := ToolClause(
		config.Items.Tool,
		config.Colors["LeftArm"],
		config.Items.Shirt.Item,
	)
	objects = append(objects, armObjects...)
	fmt.Printf("Texturize: Tool/Arm objects added. Total objects: %d\n", len(objects))

	return objects
}

// --- Adapted generatePreview Function ---
func generatePreview(itemConfig ItemConfig) []*aeno.Object {
	fmt.Printf("generatePreview: Starting for ItemType: %s, Item: %+v\n", itemConfig.ItemType, itemConfig.Item)

	previewConfig := useDefault

	itemType := itemConfig.ItemType
	itemData := itemConfig.Item

	switch itemType {
	case "face":
		previewConfig.Items.Face = itemData
	case "hat":
		previewConfig.Items.Hats = make(HatsCollection)
		previewConfig.Items.Hats["hat_1"] = itemData
	case "addon":
		previewConfig.Items.Addon = itemData
	case "tool":
		previewConfig.Items.Tool = itemData
	case "pants":
		previewConfig.Items.Pants = itemData
	case "shirt":
		previewConfig.Items.Shirt = itemData
	case "tshirt":
		previewConfig.Items.Tshirt = itemData
	case "head":
		if itemData.Item != "none" {
			previewConfig.BodyParts.Head = itemData.Item
		}
	default:
		fmt.Printf("generatePreview: Unhandled item type '%s'. Showing default avatar.\n", itemType)
	}
	return generateObjects(previewConfig)
}

func AddFace(faceHash string) aeno.Texture {
	var face aeno.Texture

	faceURL := ""
	if faceHash != "none" {
		faceURL = fmt.Sprintf("%s/uploads/%s.png", env("CDN_URL"), faceHash)
	} else {
		faceURL = fmt.Sprintf("%s/assets/default.png", env("CDN_URL"))
	}

	fmt.Printf("AddFace: Loading face texture from URL: %s\n", faceURL)
	face = aeno.LoadTextureFromURL(faceURL)

	fmt.Printf("AddFace: Loaded texture for %s. (No nil check possible for value type)\n", faceURL)

	return face
}
