package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"time"
        "net/http"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/netisu/aeno"
)

const (
	scale      = 1
	fovy       = 15.5
	near       = 1.1
	far        = 1000
	amb        = "606060" // d4d4d4
	lightcolor = "dadada" // 696969
	Dimentions = 512      // april fools (15)
)

var (
	eye           = aeno.V(0.82, 0.85, 2)
	center        = aeno.V(0, 0, 0)
	up            = aeno.V(0, 1.3, 0)
	light         = aeno.V(16, 22, 25).Normalize()
	endpoint      = "https://nyc3.digitaloceanspaces.com"
	cdnUrl        = "https://cdn.netisu.com"
	tempDir       = "/tmp" // temporary directory
	region        = "us-east-1"
	accessKey     = "accessKey"
	secretKey     = "secretKey"
	bucket        = "netisu" // set this to your s3 bucket
	serverAddress = ":4316"  // do not put links like (renderer.example.com) until after pentesting
	postKey       = "key"
)

// ItemData represents the new structure for individual items sent from Laravel.
type ItemData struct {
	Item      string `json:"item"`
	EditStyle string `json:"edit_style"`
	IsModel   bool   `json:"is_model"`
	IsTexture bool   `json:"is_texture"`
}

// UserConfig now reflects the nested structure sent by the Laravel controller.
type UserConfig struct {
	Items struct {
		Face   ItemData   `json:"face"`
		Hats   []ItemData `json:"hats"`
		Addon  ItemData   `json:"addon"`
		Tool   ItemData   `json:"tool"`
		Head   ItemData   `json:"head"`
		Pants  ItemData   `json:"pants"`
		Shirt  ItemData   `json:"shirt"`
		Tshirt ItemData   `json:"tshirt"`
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

type RenderEvent struct {
	RenderType string     `json:"RenderType"`
	Hash       string     `json:"Hash"`
	RenderJson UserConfig `json:"RenderJson"` // Use UserConfig for user type
}

type ItemConfig struct {
	ItemType string `json:"ItemType"`
	Item     string `json:"item"`   // Renamed to match Laravel's 'item' field
	PathMod  bool   `json:"PathMod"`
}

type ItemEvent struct {
	RenderType string     `json:"RenderType"`
	Hash       string     `json:"Hash"`
	RenderJson ItemConfig `json:"RenderJson"` // Use ItemConfig for item/item_preview types
}

var useDefault UserConfig = UserConfig{
	Items: struct {
		Face   ItemData   `json:"face"`
		Hats   []ItemData `json:"hats"`
		Addon  ItemData   `json:"addon"`
		Tool   ItemData   `json:"tool"`
		Head   ItemData   `json:"head"`
		Pants  ItemData   `json:"pants"`
		Shirt  ItemData   `json:"shirt"`
		Tshirt ItemData   `json:"tshirt"`
	}{
		Face:   ItemData{Item: "none"},
		Hats:   []ItemData{{Item: "none"}, {Item: "none"}, {Item: "none"}, {Item: "none"}, {Item: "none"}, {Item: "none"}}, // Initialize all 6 hat slots
		Addon:  ItemData{Item: "none"},
		Head:   ItemData{Item: "none"},
		Tool:   ItemData{Item: "none"},
		Pants:  ItemData{Item: "none"},
		Shirt:  ItemData{Item: "none"},
		Tshirt: ItemData{Item: "none"},
	},
	Colors: struct {
		HeadColor     string `json:"head_color"`
		TorsoColor    string `json:"torso_color"`
		LeftLegColor  string `json:"leftLeg_color"`
		RightLegColor string `json:"rightLeg_color"`
		LeftArmColor  string `json:"leftArm_color"`
		RightArmColor string `json:"rightArm_color"`
	}{
		HeadColor:     "d3d3d3",
		TorsoColor:    "a08bd0",
		LeftLegColor:  "232323",
		RightLegColor: "232323",
		LeftArmColor:  "d3d3d3",
		RightArmColor: "d3d3d3",
	},
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		if postKey != "" && r.Header.Get("Aeo-Access-Key") != postKey {
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

	// Try to unmarshal as RenderEvent (for user renders)
	var userEvent RenderEvent
	err = json.Unmarshal(body, &userEvent)
	if err == nil && userEvent.RenderType == "user" {
		fmt.Println("Running Function", userEvent.RenderType)
		renderUser(userEvent, w)
		renderHeadshot(userEvent, w) // Headshot also uses UserConfig
		return
	}

	// If not a user event, try to unmarshal as ItemEvent
	var itemEvent ItemEvent
	err = json.Unmarshal(body, &itemEvent)
	if err != nil {
		fmt.Println("Error decoding request:", err)
		fmt.Println("Request Body:", string(body)) // For debugging
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	fmt.Println(itemEvent.RenderType)

	// Extract query parameters with default values
	fmt.Println("Running Function", itemEvent.RenderType)
	switch itemEvent.RenderType {
	case "item":
		renderItem(itemEvent, w)
	case "item_preview":
		renderItemPreview(itemEvent, w)
	default:
		fmt.Println("Invalid renderType:", itemEvent.RenderType)
		http.Error(w, "Invalid renderType", http.StatusBadRequest)
		return
	}
}

func renderUser(e RenderEvent, w http.ResponseWriter) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)
	uploader := s3.New(newSession)

	fmt.Println("Getting userstring", e.Hash)

	userJson := e.RenderJson

	if reflect.ValueOf(userJson).IsZero() {
		log.Println("Warning: UserJson query parameter is missing, the avatar will not render!")
		http.Error(w, "UserJson query parameter is missing", http.StatusBadRequest)
		return
	}

	start := time.Now()
	fmt.Println("Drawing Objects...")
	objects := generateObjects(userJson, true) // Pass the full UserConfig
	fmt.Println("Exporting to", tempDir, "thumbnails")
	outputFile := path.Join("thumbnails", e.Hash+".png")
	fullPath := path.Join(tempDir, e.Hash+".png") // Renamed to avoid confusion with path package

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

	fmt.Println("Uploading to the", bucket, "s3 bucket")

	f, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", fullPath, err)
		http.Error(w, "Failed to open rendered image", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	_, err = f.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read file %q: %v", fullPath, err)
		http.Error(w, "Failed to read rendered image", http.StatusInternalServerError)
		return
	}

	object := s3.PutObjectInput{
		Bucket:             aws.String(bucket),
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
		http.Error(w, "Failed to upload image to S3", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(fullPath)

	fmt.Println("Completed in", time.Since(start))

	w.Header().Set("Content-Type", "image/png")
	fmt.Fprintf(w, "Rendered image uploaded to %s/%s", cdnUrl, outputFile)
}

func renderItemPreview(i ItemEvent, w http.ResponseWriter) {
	var outputFile string
	var fullPath string

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)
	uploader := s3.New(newSession)

	if i.Hash == "default" || i.Hash == "" {
		fmt.Println("Item Hash is required")
		http.Error(w, "Item Hash is required", http.StatusBadRequest)
		return
	}
	if i.RenderJson.Item == "none" || i.RenderJson.Item == "" { // Check item hash
		fmt.Println("Item String is required")
		http.Error(w, "Item String is required", http.StatusBadRequest)
		return
	}

	itemJson := i.RenderJson

	if reflect.ValueOf(itemJson).IsZero() {
		log.Println("Warning: itemJson query parameter is missing, the preview will not render!")
		http.Error(w, "itemJson query parameter is missing", http.StatusBadRequest)
		return
	}

	start := time.Now()
	fmt.Println("Drawing Objects...")
	objects := generatePreview(i.RenderJson) // Pass the ItemConfig
	fmt.Println("Exporting to", tempDir, "thumbnails")

	if i.RenderJson.PathMod {
		outputFile = path.Join("thumbnails", i.Hash+"_preview.png")
		fullPath = path.Join(tempDir, i.Hash+"_preview.png")
	} else {
		outputFile = path.Join("thumbnails", i.Hash+".png")
		fullPath = path.Join(tempDir, i.Hash+".png")
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

	fmt.Println("Uploading to the", bucket, "s3 bucket")

	f, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", fullPath, err)
		http.Error(w, "Failed to open rendered image", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	_, err = f.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read file %q: %v", fullPath, err)
		http.Error(w, "Failed to read rendered image", http.StatusInternalServerError)
		return
	}

	object := s3.PutObjectInput{
		Bucket:             aws.String(bucket),
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
		http.Error(w, "Failed to upload image to S3", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(fullPath)

	fmt.Println("Completed in", time.Since(start))

	w.Header().Set("Content-Type", "image/png")
	fmt.Fprintf(w, "Rendered image uploaded to %s/%s", cdnUrl, outputFile)
}

func renderItem(i ItemEvent, w http.ResponseWriter) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)
	uploader := s3.New(newSession)

	if i.Hash == "" {
		fmt.Println("itemstring is required")
		http.Error(w, "Item hash is required", http.StatusBadRequest)
		return
	}

	fmt.Println("Getting itemstring", i.Hash)

	itemJson := i.RenderJson

	if reflect.ValueOf(itemJson).IsZero() {
		log.Println("Warning: itemJson query parameter is missing, the item will not render!")
		http.Error(w, "itemJson query parameter is missing", http.StatusBadRequest)
		return
	}
	start := time.Now()
	fmt.Println("Drawing Objects...")
	var objects []*aeno.Object

	// The Laravel code sends `item_type` and `item` (hash)
	if itemJson.ItemType == "head" {
		objects = RenderHead(itemJson.Item)
	} else if itemJson.ItemType == "hat" || itemJson.ItemType == "addon" {
		objects = RenderHats(itemJson.Item)
	} else if itemJson.ItemType == "face" {
		objects = append(objects, &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemJson.Item+".png")),
			Color:   aeno.HexColor(useDefault.Colors.HeadColor),
		})
	} else if itemJson.ItemType == "tshirt" {
		objects = append(objects, &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/tee.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemJson.Item+".png")),
			Color:   aeno.Transparent,
		})
	} else if itemJson.ItemType == "shirt" {
		objects = append(objects, &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemJson.Item+".png")),
			Color:   aeno.HexColor(useDefault.Colors.TorsoColor),
		})
	} else if itemJson.ItemType == "pants" {
		objects = append(objects,
			&aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_left.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemJson.Item+".png")),
				Color:   aeno.HexColor(useDefault.Colors.LeftLegColor),
			},
			&aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_right.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemJson.Item+".png")),
				Color:   aeno.HexColor(useDefault.Colors.RightLegColor),
			},
		)
	} else if itemJson.ItemType == "tool" {
		objects = ToolClause(itemJson.Item, useDefault.Colors.LeftArmColor, useDefault.Colors.RightArmColor, "none")
	}

	fmt.Println("Exporting to", tempDir, "thumbnails")
	outputFile := path.Join("thumbnails", i.Hash+".png")
	fullPath := path.Join(tempDir, i.Hash+".png")

	aeno.GenerateScene(
		true,
		fullPath,
		objects,
		aeno.V(1, 2, 3), // note: camera position might need adjustment for item renders
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

	fmt.Println("Uploading to the", bucket, "s3 bucket")

	f, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", fullPath, err)
		http.Error(w, "Failed to open rendered image", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	_, err = f.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read file %q: %v", fullPath, err)
		http.Error(w, "Failed to read rendered image", http.StatusInternalServerError)
		return
	}

	object := s3.PutObjectInput{
		Bucket:             aws.String(bucket),
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
		http.Error(w, "Failed to upload image to S3", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(fullPath)

	fmt.Println("Completed in", time.Since(start))

	w.Header().Set("Content-Type", "image/png")
	fmt.Fprintf(w, "Rendered image uploaded to %s/%s", cdnUrl, outputFile)
}

func renderHeadshot(e RenderEvent, w http.ResponseWriter) {
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}

	newSession := session.New(s3Config)
	uploader := s3.New(newSession)

	fmt.Println("Rendering Headshot...")
	var (
		headshot_eye    = aeno.V(4, 7, 13)
		headshot_center = aeno.V(-0.5, 6.8, 0)
		headshot_up     = aeno.V(0, 4, 0)
	)

	userJson := e.RenderJson

	if reflect.ValueOf(userJson).IsZero() {
		log.Println("Warning: UserJson query parameter is missing, the avatar will not render!")
		http.Error(w, "UserJson query parameter is missing", http.StatusBadRequest)
		return
	}

	start := time.Now()
	fmt.Println("Drawing Objects...")
	objects := generateObjects(userJson, false)

	fmt.Println("Exporting to", tempDir, "thumbnails")
	outputFile := path.Join("thumbnails", e.Hash+"_headshot.png")
	fullPath := path.Join(tempDir, e.Hash+"_headshot.png")

	aeno.GenerateScene(
		false,
		fullPath,
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

	fmt.Println("Uploading to the", bucket, "s3 bucket")

	f, err := os.Open(fullPath)
	if err != nil {
		fmt.Printf("Failed to open file %q: %v", fullPath, err)
		http.Error(w, "Failed to open rendered image", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	fileInfo, _ := f.Stat()
	var size int64 = fileInfo.Size()
	buffer := make([]byte, size)
	_, err = f.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read file %q: %v", fullPath, err)
		http.Error(w, "Failed to read rendered image", http.StatusInternalServerError)
		return
	}

	object := s3.PutObjectInput{
		Bucket:             aws.String(bucket),
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
		http.Error(w, "Failed to upload image to S3", http.StatusInternalServerError)
		return
	}
	_ = os.Remove(fullPath)

	fmt.Println("Completed in", time.Since(start))

	w.Header().Set("Content-Type", "image/png")
	fmt.Fprintf(w, "Rendered image uploaded to %s/%s", cdnUrl, outputFile)
}

// RenderHats now accepts a slice of ItemData to handle edit styles if needed.
// For now, it only uses the 'Item' hash.
func RenderHats(hats ...string) []*aeno.Object { // Still accepting strings for simplicity, convert to ItemData.Item
	var objects []*aeno.Object

	for _, hatHash := range hats {
		if hatHash != "none" {
			obj := &aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+hatHash+".obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+hatHash+".png")),
			}
			objects = append(objects, obj)
		}
	}
	return objects
}

func RenderHead(heads ...string) []*aeno.Object { // Still accepting strings
	var objects []*aeno.Object

	for _, headHash := range heads {
		if headHash != "none" {
			obj := &aeno.Object{
				Mesh: aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+headHash+".obj")),
			}
			objects = append(objects, obj)
		}
	}
	return objects
}

func ToolClause(toolHash, leftArmColor, rightArmColorParam, shirtHash string) []*aeno.Object {
	armObjects := []*aeno.Object{}
	if toolHash != "none" {
		if shirtHash != "none" {
			armObjects = append(armObjects, &aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_tool.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirtHash+".png")),
				Color:   aeno.HexColor(leftArmColor),
			})
		} else {
			armObjects = append(armObjects, &aeno.Object{
				Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_tool.obj")),
				Color: aeno.HexColor(leftArmColor),
			})
		}

		toolObj := &aeno.Object{
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+toolHash+".png")),
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+toolHash+".obj")),
		}
		armObjects = append(armObjects, toolObj)
	} else {
		if shirtHash != "none" {
			armObjects = append(armObjects, &aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_left.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirtHash+".png")),
				Color:   aeno.HexColor(leftArmColor),
			})
		} else {
			armObjects = append(armObjects, &aeno.Object{
				Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_left.obj")),
				Color: aeno.HexColor(leftArmColor),
			})
		}
	}
	return armObjects
}

func generateObjects(userConfig UserConfig, toolNeeded bool) []*aeno.Object {
	// Extract relevant data from the UserConfig struct
	torsoColor := userConfig.Colors.TorsoColor
	leftLegColor := userConfig.Colors.LeftLegColor
	rightLegColor := userConfig.Colors.RightLegColor
	rightArmColor := userConfig.Colors.RightArmColor

	leftArmColor := userConfig.Colors.LeftArmColor
	headColor := userConfig.Colors.HeadColor

	faceTexture := AddFace(userConfig.Items.Face.Item) // Pass only the item hash

	shirt := userConfig.Items.Shirt.Item
	pants := userConfig.Items.Pants.Item
	tshirt := userConfig.Items.Tshirt.Item

        var tool string
        if (toolNeeded){
	        tool = userConfig.Items.Tool.Item
        } else {
                tool = "none"
        }
	head := userConfig.Items.Head.Item

	var hatHashes []string
	for _, hatItem := range userConfig.Items.Hats {
		hatHashes = append(hatHashes, hatItem.Item)
	}
	addon := userConfig.Items.Addon.Item // Addon is now part of the hats slice for rendering flexibility

	objects := Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, tool, rightArmColor, pants, shirt, tshirt)

	if head != "none" {
		HeadLoader := &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+head+".obj")),
			Texture: faceTexture, 
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, HeadLoader)
	} else if faceTexture != nil {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
			Texture: faceTexture,
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, faceObject)
	}

	allHatsAndAddons := append(hatHashes, addon)
	hatObjects := RenderHats(allHatsAndAddons...)
	objects = append(objects, hatObjects...)

	return objects
}

func AddFace(faceHash string) aeno.Texture { // <--- Changed return type to aeno.Texture (no asterisk)
    if faceHash != "none" && faceHash != "" {
        return aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+faceHash+".png"))
    }
    return nil
}

func Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, toolHash, rightArmColorParam, pantsHash, shirtHash, tshirtHash string) []*aeno.Object {
	objects := []*aeno.Object{}

	objects = append(objects, &aeno.Object{
		Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
		Color: aeno.HexColor(torsoColor),
	})

	objects = append(objects, &aeno.Object{
		Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_right.obj")),
		Color: aeno.HexColor(rightArmColorParam),
	})

	objects = append(objects,
		&aeno.Object{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_left.obj")),
			Color: aeno.HexColor(leftLegColor),
		},
		&aeno.Object{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_right.obj")),
			Color: aeno.HexColor(rightLegColor),
		},
	)

	if shirtHash != "none" {
		shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirtHash+".png"))
		for _, obj := range objects[0:2] { // Torso and right arm
			obj.Texture = shirtTexture
		}
	}

	if pantsHash != "none" {
		pantsTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+pantsHash+".png"))
		for _, obj := range objects[2:] { // Legs
			obj.Texture = pantsTexture
		}
	}

	if tshirtHash != "none" {
		texture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+tshirtHash+".png"))
		TshirtLoader := &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/tee.obj")),
			Color:   aeno.Transparent,
			Texture: texture,
		}
		objects = append(objects, TshirtLoader)
	}

	armObjects := ToolClause(toolHash, leftArmColor, rightArmColorParam, shirtHash)
	objects = append(objects, armObjects...)

	return objects
}

func generatePreview(itemConfig ItemConfig) []*aeno.Object {
	torsoColor := useDefault.Colors.TorsoColor
	leftLegColor := useDefault.Colors.LeftLegColor
	rightLegColor := useDefault.Colors.RightLegColor
	rightArmColor := useDefault.Colors.RightArmColor
	leftArmColor := useDefault.Colors.LeftArmColor
	headColor := useDefault.Colors.HeadColor
	faceTexture := AddFace(useDefault.Items.Face.Item) 

	itemType := itemConfig.ItemType
	itemHash := itemConfig.Item 

	objects := []*aeno.Object{
		{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_right.obj")),
			Color: aeno.HexColor(rightArmColor),
		},
		{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
			Color: aeno.HexColor(torsoColor),
		},
	}
	if itemType != "tool" {
		leftArm := &aeno.Object{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_left.obj")),
			Color: aeno.HexColor(leftArmColor),
		}
		objects = append(objects, leftArm)
	}
	LeftLeg := &aeno.Object{
		Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_left.obj")),
		Color: aeno.HexColor(leftLegColor),
	}
	RightLeg := &aeno.Object{
		Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_right.obj")),
		Color: aeno.HexColor(rightLegColor),
	}

	objects = append(objects, LeftLeg, RightLeg)

	if itemType == "tool" {
		armObject := ToolClause(itemHash, "d3d3d3", "d3d3d3", "none")
		objects = append(objects, armObject...)
	}

	if itemType == "head" {
		HeadLoader := &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".obj")),
			Texture: faceTexture,
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, HeadLoader)
	}

	if itemType == "face" {
		faceObject := &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".png")),
			Color:   aeno.HexColor(headColor),
		}
		objects = append(objects, faceObject)
	} else if itemType == "hat" || itemType == "addon" {
		objects = append(objects, &aeno.Object{
			Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
			Color: aeno.HexColor(headColor),
		})
		hatObjects := RenderHats(itemHash)
		objects = append(objects, hatObjects...)
	} else if itemType == "tshirt" {
		objects = append(objects, &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/tee.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".png")),
			Color:   aeno.Transparent,
		})
	} else if itemType == "shirt" {
		objects = append(objects, &aeno.Object{
			Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
			Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".png")),
			Color:   aeno.HexColor(useDefault.Colors.TorsoColor),
		})
	} else if itemType == "pants" {
		objects = append(objects,
			&aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_left.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".png")),
				Color:   aeno.HexColor(useDefault.Colors.LeftLegColor),
			},
			&aeno.Object{
				Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/leg_right.obj")),
				Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+itemHash+".png")),
				Color:   aeno.HexColor(useDefault.Colors.RightLegColor),
			},
		)
	}

	return objects
}