package main

import (
        "github.com/netisu/aeno"
        "bytes"
        "encoding/json"
        "fmt"
        "github.com/aws/aws-sdk-go/aws"
        "github.com/aws/aws-sdk-go/aws/credentials"
        "github.com/aws/aws-sdk-go/aws/session"
        "github.com/aws/aws-sdk-go/service/s3"
        "io"
        "log"
        "net/http"
        "os"
        "path"
        "reflect"
        "time"
        "github.com/joho/godotenv"

        
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
        light         = aeno.V(16,22,25).Normalize()
        rootDir       = "/var/www/renderer"
)

type ItemData struct {
	Item      string `json:"item"`
	EditStyle *EditStyle `json:"edit_style"`
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
    	Colors map[string]string `json:"colors"`
}

type ItemConfig struct {
        ItemType string `json:"ItemType"`
        Item     ItemData `json:"item"`
        PathMod  bool   `json:"PathMod"`
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
    Colors: map[string]string{
        "Head":      "d3d3d3",
        "Torso":     "a08bd0",
        "LeftLeg":   "232323",
        "RightLeg":  "232323",
        "LeftArm":   "d3d3d3",
        "RightArm":  "d3d3d3",
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
        objects := generateObjects(userJson, true)
        fmt.Println("Exporting to", env("TEMP_DIR"), "thumbnails")
        outputFile := path.Join("thumbnails", e.Hash+".png")
        outputPath := path.Join(env("TEMP_DIR"), e.Hash+".png") // Renamed 'path' to 'outputPath' to avoid shadowing

        aeno.GenerateScene(
                true,
                outputPath,
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
        fullPath = path.Join(env("TEMP_DIR"), i.Hash+".png") // Construct the full path
    } else {
        outputFile = path.Join("thumbnails", i.Hash+".png")      // Assign to outputFile
        fullPath =      path.Join(env("TEMP_DIR"), i.Hash+".png") // Construct the full path
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
        objects := generateObjects(userJson, false)

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

    meshURL := fmt.Sprintf("%s/uploads/%s.obj", env("CDN_URL"), itemData.Item)
    textureURL := fmt.Sprintf("%s/uploads/%s.png", env("CDN_URL"), itemData.Item)

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

    return &aeno.Object{
        Mesh:    aeno.LoadObjectFromURL(meshURL),
        Color: aeno.Transparent,
        Texture: aeno.LoadTextureFromURL(textureURL),
    }
}

func ToolClause(toolData ItemData, leftArmColor, rightArmColor, shirtTextureHash string) []*aeno.Object {
    objects := []*aeno.Object{}
    cdnUrl := env("CDN_URL")

    defaultLeftArmMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_left.obj", cdnUrl))
    defaultLeftArmObj := &aeno.Object{
        Mesh:  defaultLeftArmMesh,
        Color: aeno.HexColor(leftArmColor),
    }

    if shirtTextureHash != "none" {
        shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnUrl, shirtTextureHash))
        if shirtTexture != nil {
            defaultLeftArmObj.Texture = shirtTexture
        }
    }

    if toolData.Item != "none" {
        armToolMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_tool.obj", cdnUrl))

        if toolMesh := RenderItem(toolData); toolMesh != nil {
		    objects = append(objects, toolMesh)
            fmt.Printf("ToolClause: Added tool mesh %s\n", toolData.Item)
	    }
        if armToolMesh != nil {
            armToolObj := &aeno.Object{
                Mesh:  armToolMesh,
                Color: aeno.HexColor(leftArmColor),
            }
            // Apply shirt texture to the arm_tool if a shirt is worn
            if shirtTextureHash != "none" {
                shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnUrl, shirtTextureHash))
                if shirtTexture != nil { // Check for zero-value texture
                    armToolObj.Texture = shirtTexture
                }
            }
            objects = append(objects, armToolObj)
            fmt.Printf("ToolClause: Added arm_tool mesh.\n")
        }
    } else {
        // If no tool, add the default left arm
        objects = append(objects, defaultLeftArmObj)
        fmt.Printf("ToolClause: No tool, added default left arm.\n")
    }

    return objects
}

func generateObjects(userConfig UserConfig, toolNeeded bool) []*aeno.Object { // toolNeeded might be unused now
    fmt.Printf("generateObjects: Starting. UserConfig: %+v\n", userConfig)

    var allObjects []*aeno.Object

    // Load cranium mesh once for face application in generateObjects
    cdnURL := env("CDN_URL")
    cachedCraniumMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/cranium.obj", cdnURL))
    fmt.Printf("generateObjects: Cached Cranium Mesh Pointer (for cranium): %p\n", cachedCraniumMesh)

    // 1. Call Texturize to get the core body parts with colors, shirt, pants, T-shirt, and tool-related arms.
    // Texturize now encapsulates most of the base avatar rendering.
    bodyAndApparelObjects := Texturize(userConfig)
    allObjects = append(allObjects, bodyAndApparelObjects...)
    fmt.Printf("generateObjects: Objects from Texturize received. Total objects: %d\n", len(allObjects))

    // 2. Add Cranium mesh and apply head color.
    // This is added here because Texturize, does not include cranium.obj as we need it for later.
    allObjects = append(allObjects, &aeno.Object{
        Mesh:  cachedCraniumMesh,
        Color: aeno.HexColor(userConfig.Colors["Head"]),
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
    for _, hatItemData := range userConfig.Items.Hats {
        fmt.Printf("  Hat Item: %s\n", hatItemData.Item)
        if obj := RenderItem(hatItemData); obj != nil {
            allObjects = append(allObjects, obj)
            fmt.Printf("Hat object added. Total objects: %d\n", len(allObjects))
        }
    }

    fmt.Printf("generateObjects: Finished. Final object count: %d\n", len(allObjects))
    return allObjects
}

func Texturize(config UserConfig) []*aeno.Object {
	objects := []*aeno.Object{}
    cdnUrl := env("CDN_URL")

    // --- CACHED MESH REFERENCES FOR TEXTURIZE'S INTERNAL USE ---
    // These are loaded once within Texturize to ensure consistent pointers for its internal slice indexing.
    cachedChesticleMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/chesticle.obj", cdnUrl))
    cachedArmRightMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_right.obj", cdnUrl))
    cachedLegLeftMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_left.obj", cdnUrl))
    cachedLegRightMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_right.obj", cdnUrl))
    cachedTeeMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/tee.obj", cdnUrl))

	objects = append(objects, &aeno.Object{
		Mesh:  cachedChesticleMesh,
		Color: aeno.HexColor(config.Colors["Torso"]),
	})
    fmt.Printf("Texturize: Added Chesticle Mesh Pointer: %p\n", cachedChesticleMesh)

	objects = append(objects, &aeno.Object{
		Mesh:  cachedArmRightMesh,
		Color: aeno.HexColor(config.Colors["RightArm"]),
	})
    fmt.Printf("Texturize: Added Right Arm Mesh Pointer: %p\n", cachedArmRightMesh)

	objects = append(objects,
		&aeno.Object{
			Mesh:  cachedLegLeftMesh,
			Color: aeno.HexColor(config.Colors["LeftLeg"]),
		},
		&aeno.Object{
			Mesh:  cachedLegRightMesh,
			Color: aeno.HexColor(config.Colors["RightLeg"]),
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
		}
		objects = append(objects, TshirtLoader)
        fmt.Printf("Texturize: T-shirt object added. Total objects: %d\n", len(objects))
	}

    fmt.Printf("Texturize: Processing Tool. Tool Item: %s\n", config.Items.Tool.Item)
	armObjects := ToolClause(
		config.Items.Tool,
		config.Colors["LeftArm"],
		config.Colors["RightArm"],
		config.Items.Shirt.Item,
	)
	objects = append(objects, armObjects...)
    fmt.Printf("Texturize: Tool/Arm objects added. Total objects: %d\n", len(objects))

	return objects
}

// --- Adapted generatePreview Function ---
func generatePreview(itemConfig ItemConfig) []*aeno.Object {
    fmt.Printf("generatePreview: Starting for ItemType: %s, Item: %+v\n", itemConfig.ItemType, itemConfig.Item)
	var allObjects []*aeno.Object
    cdnURL := env("CDN_URL")

    // These calls must be outside the switch to ensure consistent pointers.
    cachedCraniumObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/cranium.obj", cdnURL))
    cachedChesticleObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/chesticle.obj", cdnURL))
    cachedArmRightObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_right.obj", cdnURL))
    cachedArmLeftObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_left.obj", cdnURL))
    cachedLegLeftObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_left.obj", cdnURL))
    cachedLegRightObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_right.obj", cdnURL))
    cachedTeeObjMesh := aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/tee.obj", cdnURL))

    
    // Since we're using `useDefault` here, it will be a plain colored body without specific apparel textures initially.
    coloredBodyParts := Texturize(useDefault)
    allObjects = append(allObjects, coloredBodyParts...)

    // This is needed for face application.
    allObjects = append(allObjects, &aeno.Object{
        Mesh:  cachedCraniumObjMesh,
        Color: aenoHexColor(useDefault.Colors["Head"]), // Use default head color
    })

    itemType := itemConfig.ItemType
    itemData := itemConfig.Item

    // --- Handle different item types for preview ---
    switch itemType {
    case "tool":
        // Fixed ToolClause parameters: toolData, leftArmColor, rightArmColor, shirtTextureHash
        armObjects := ToolClause(
            itemData,
            useDefault.Colors["LeftArm"],  // Default left arm color
            useDefault.Colors["RightArm"], // Default right arm color
            "none",                        // Pass "none" for shirt texture if we want a bare arm for tool preview
        )
        allObjects = append(allObjects, armObjects...)
        fmt.Printf("generatePreview: Added tool and arm objects for '%s'.\n", itemData.Item)

    case "head":
        // If 'head' is a custom model, render it.
        // This will likely replace or sit on top of the default cranium.obj
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
            fmt.Printf("generatePreview: Added custom head object for '%s'.\n", itemData.Item)
        }

    case "face":
        // Find the cranium or generic head mesh in the existing objects and apply the face texture.
        faceTexture := AddFace(itemData.Item) // AddFace handles its own nil checks
        if faceTexture != nil { // Check if AddFace actually loaded a texture
            foundHeadMesh := false
            for _, obj := range allObjects {
                if obj.Mesh != nil && (obj.Mesh == cachedCraniumObjMesh) {
                    obj.Texture = faceTexture
                    foundHeadMesh = true
                    fmt.Printf("generatePreview: Applied face texture to head/cranium mesh.\n")
                    break
                }
            }
            if !foundHeadMesh {
                fmt.Printf("generatePreview: WARNING: No head/cranium mesh found to apply face texture for '%s'.\n", itemData.Item)
            }
        } else {
            fmt.Printf("generatePreview: WARNING: Face texture for '%s' failed to load. Not applying.\n", itemData.Item)
        }

    case "tshirt":
        // T-shirt is an overlay mesh with its own texture.
        // It's not a texture applied to the base body, but a separate "tee" mesh.
        tshirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.Item))
        if tshirtTexture != nil { 
            tshirtObj := &aeno.Object{
                Mesh:    cachedTeeObjMesh, // Use the cached tee mesh
                Color:   aeno.Transparent,
                Texture: tshirtTexture,
            }
            allObjects = append(allObjects, tshirtObj)
            fmt.Printf("generatePreview: Added T-shirt object for '%s'.\n", itemData.Item)
        } else {
            fmt.Printf("generatePreview: WARNING! T-shirt texture for '%s' failed to load. Not adding.\n", itemData.Item)
        }


    case "shirt":
        shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.Item))
        if shirtTexture != nil { 
            // Apply to torso, right arm, and left arm (if exists)
            appliedCount := 0
            for _, obj := range allObjects {
                if obj.Mesh != nil && (obj.Mesh == cachedChesticleObjMesh ||
                                       obj.Mesh == cachedArmRightObjMesh ||
                                       obj.Mesh == cachedArmLeftObjMesh) { // Include left arm
                    obj.Texture = shirtTexture
                    appliedCount++
                }
            }
            fmt.Printf("generatePreview: Applied shirt texture for '%s' to %d body part(s).\n", itemData.Item, appliedCount)
        } else {
            fmt.Printf("generatePreview: WARNING! Shirt texture for '%s' failed to load. Not applying.\n", itemData.Item)
        }


    case "pants":
        pantsTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.Item))
        if pantsTexture != nil { 
            // Apply to left and right legs
            appliedCount := 0
            for _, obj := range allObjects {
                if obj.Mesh != nil && (obj.Mesh == cachedLegLeftObjMesh ||
                                       obj.Mesh == cachedLegRightObjMesh) {
                    obj.Texture = pantsTexture
                    appliedCount++
                }
            }
            fmt.Printf("generatePreview: Applied pants texture for '%s' to %d leg(s).\n", itemData.Item, appliedCount)
        } else {
            fmt.Printf("generatePreview: WARNING! Pants texture for '%s' failed to load. Not applying.\n", itemData.Item)
        }

    case "hat":
        // Hats are usually distinct models.
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
            fmt.Printf("generatePreview: Added hat object for '%s'.\n", itemData.Item)
        }

    case "addon":
        // Addons are also distinct models.
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
            fmt.Printf("generatePreview: Added addon object for '%s'.\n", itemData.Item)
        }

    default:
        // This case is a fallback for any item type not explicitly handled.
        // It simply tries to render it as a generic item.
        fmt.Printf("generatePreview: Unhandled item type '%s'. Attempting generic RenderItem.\n", itemType)
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
            fmt.Printf("generatePreview: Added generic object for '%s'.\n", itemData.Item)
        }
    }

    fmt.Printf("generatePreview: Finished. Final object count: %d\n", len(allObjects))
    return allObjects
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