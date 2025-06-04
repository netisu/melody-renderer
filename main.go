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
        "HeadColor":      "d3d3d3",
        "TorsoColor":     "a08bd0",
        "LeftLegColor":   "232323",
        "RightLegColor":  "232323",
        "LeftArmColor":   "d3d3d3",
        "RightArmColor":  "d3d3d3",
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
                Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
                Endpoint:         aws.String(endpoint),
                Region:           aws.String(region),
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
        fmt.Println("Exporting to", tempDir, "thumbnails")
       outputFile := path.Join("thumbnails", e.Hash+".png")
    outputPath := path.Join(tempDir, e.Hash+".png") // Renamed 'path' to 'outputPath' to avoid shadowing

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

        fmt.Println("Uploading to the", bucket, "s3 bucket")

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
                Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
                Endpoint:         aws.String(endpoint),
                Region:           aws.String(region),
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
        fmt.Println("Exporting to", tempDir, "thumbnails")

        if i.RenderJson.PathMod {
        outputFile = path.Join("thumbnails", i.Hash+"_preview.png") // Assign to outputFile
        fullPath = path.Join(tempDir, i.Hash+".png") // Construct the full path
    } else {
        outputFile = path.Join("thumbnails", i.Hash+".png")      // Assign to outputFile
        fullPath =      path.Join(tempDir, i.Hash+".png") // Construct the full path
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
                fmt.Printf("Failed to open file %q: %v", fullPath, err) // Log the error
        }
        defer f.Close()

        fileInfo, _ := f.Stat()
        var size int64 = fileInfo.Size()
        buffer := make([]byte, size)
        f.Read(buffer)

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
        }
        _ = os.Remove(fullPath)

        fmt.Println("Completed in", time.Since(start))

        // Set the response content type to image/png
        w.Header().Set("Content-Type", "image/png")

}

func renderItem(i ItemEvent, w http.ResponseWriter) {
        // Delegate user avatar rendering logic here
        s3Config := &aws.Config{
                Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
                Endpoint:         aws.String(endpoint),
                Region:           aws.String(region),
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

        fmt.Println("Exporting to", tempDir, "thumbnails")
        outputFile := path.Join("thumbnails", i.Hash+".png")
        outputPath := path.Join(tempDir, i.Hash+".png")

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


        fmt.Println("Uploading to the", bucket, "s3 bucket")

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
                Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
                Endpoint:         aws.String(endpoint),
                Region:           aws.String(region),
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

        fmt.Println("Exporting to", tempDir, "thumbnails")
        outputFile := path.Join("thumbnails", e.Hash+"_headshot.png")

        path := path.Join(tempDir, e.Hash+"_headshot.png")
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

        fmt.Println("Uploading to the", bucket, "s3 bucket")

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

    meshURL := fmt.Sprintf("%s/uploads/%s.obj", cdnUrl, itemData.Item)
    textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnUrl, itemData.Item)

    if itemData.EditStyle != nil {
        if itemData.EditStyle.IsModel {
            meshURL = fmt.Sprintf("%s/uploads/%s.obj", cdnUrl, itemData.EditStyle.Hash)
            log.Printf("DEBUG: Applying model override for item %s with style %s\n", itemData.Item, itemData.EditStyle.Hash)
        }
        if itemData.EditStyle.IsTexture {
            textureURL = fmt.Sprintf("%s/uploads/%s.png", cdnUrl, itemData.EditStyle.Hash)
            log.Printf("DEBUG: Applying texture override for item %s with style %s\n", itemData.Item, itemData.EditStyle.Hash)
        }
    }

    return &aeno.Object{
        Mesh:    aeno.LoadObjectFromURL(meshURL),
        Texture: aeno.LoadTextureFromURL(textureURL),
    }
}

func ToolClause(toolData ItemData, leftArmColor, shirtTextureHash string) []*aeno.Object {
    armObjects := []*aeno.Object{}

    var leftArmTexture string
    if shirtTextureHash != "none" {
        leftArmTexture = fmt.Sprintf("%s/uploads/%s.png", cdnUrl, shirtTextureHash)
    } else {
        leftArmTexture = ""
    }


    if toolData.Item != "none" {
        armMeshURL := fmt.Sprintf("%s/assets/arm_tool.obj", cdnUrl)
        
        // Load the tool object itself
        toolObj := RenderItem(toolData)
        if toolObj != nil {
            armObjects = append(armObjects, toolObj)
        }

        // Add the left arm itself (the one holding the tool)
        armObjects = append(armObjects, &aeno.Object{
            Mesh:    aeno.LoadObjectFromURL(armMeshURL),
            Texture: aeno.LoadTextureFromURL(leftArmTexture), // Apply shirt texture if present
            Color:   aeno.HexColor(leftArmColor),
        })

    } else {
        armObjects = append(armObjects, &aeno.Object{
            Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_left.obj", cdnUrl)),
            Texture: aeno.LoadTextureFromURL(leftArmTexture), // Apply shirt texture if present
            Color:   aeno.HexColor(leftArmColor),
        })
    }

    return armObjects
}

func generateObjects(userConfig UserConfig, toolNeeded bool) []*aeno.Object {
	    var allObjects []*aeno.Object
        // Extract relevant data from the UserConfig struct
	    coloredBodyParts := Texturize(userConfig.Colors)
    	allObjects = append(allObjects, coloredBodyParts...)

		if obj := RenderItem(userConfig.Items.Face); obj != nil {
        	allObjects = append(allObjects, obj)
    	}
    	if obj := RenderItem(userConfig.Items.Addon); obj != nil {
        	allObjects = append(allObjects, obj)
    	}
		if obj := RenderItem(userConfig.Items.Shirt); obj != nil {
        	allObjects = append(allObjects, obj)
    	}
    	if obj := RenderItem(userConfig.Items.Pants); obj != nil {
        	allObjects = append(allObjects, obj)
    	}
    	if obj := RenderItem(userConfig.Items.Tshirt); obj != nil {
        	allObjects = append(allObjects, obj)
    	}

    	for _, hatItemData := range userConfig.Items.Hats {
        	if obj := RenderItem(hatItemData); obj != nil {
            	allObjects = append(allObjects, obj)
        	}
   		}

		shirtTextureHash := userConfig.Items.Shirt.Item

		leftArmObjects := ToolClause(
        	userConfig.Items.Tool,
        	userConfig.Colors["left_arm"], // Left arm color
        	shirtTextureHash, // Shirt hash (can be "none")
    	)
    	allObjects = append(allObjects, leftArmObjects...)

        return allObjects
}

func Texturize(colors map[string]string) []*aeno.Object {
    objects := []*aeno.Object{}

    headColor := colors["head"]
    objects = append(objects, &aeno.Object{
        Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
        Color: aeno.HexColor(headColor),
    })

    torsoColor := colors["torso"]
    objects = append(objects, &aeno.Object{
        Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
        Color: aeno.HexColor(torsoColor),
    })

    rightArmColor := colors["right_arm"] 
    objects = append(objects, &aeno.Object{
        Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_right.obj")),
        Color: aeno.HexColor(rightArmColor),
    })

    leftLegColor := colors["left_leg"]
    rightLegColor := colors["right_leg"]
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

    return objects
}

func generatePreview(itemConfig ItemConfig) []*aeno.Object {
		baseUserConfig := useDefault
	  	coloredBodyParts := Texturize(baseUserConfig)
    	allObjects = append(allObjects, coloredBodyParts...)

        itemType := itemConfig.ItemType
        item := itemConfig.Item

    coloredBodyParts := Texturize(baseUserConfig)
    allObjects = append(allObjects, coloredBodyParts...)

    // 2. Render the specific item from itemConfig
    itemType := itemConfig.ItemType
    itemData := itemConfig.Item

    // --- Handle different item types for preview ---
    switch itemType {
    case "tool":
        armObjects := ToolClause(
            itemData,
            useDefault.Colors["LeftArmColor"],  // Default left arm color
            "none",                             // No default shirt texture for tool preview
        )
        allObjects = append(allObjects, armObjects...)

    case "head":
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
        }

    case "face":
        faceTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnUrl, itemData.Item))
        for _, obj := range allObjects {
            if obj.Mesh != nil && (obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/head.obj", cdnUrl)) ||
                                   obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/cranium.obj", cdnUrl))) {
                obj.Texture = faceTexture
                break
            }
        }

    case "tshirt":
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
        }

    case "shirt":
        shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnUrl, itemData.Item))
        // Apply to torso, left arm and right arm
        for _, obj := range allObjects {
            if obj.Mesh != nil && (obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/chesticle.obj", cdnUrl)) ||
                                   obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_right.obj", cdnUrl)) || 
								   obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/arm_left.obj", cdnUrl))) {
                obj.Texture = shirtTexture
            }
        }

    case "pants":
        pantsTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s/uploads/%s.png", cdnUrl, itemData.Item))
        // Apply to left and right legs
        for _, obj := range allObjects {
            if obj.Mesh != nil && (obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_left.obj", cdnUrl)) ||
                                   obj.Mesh == aeno.LoadObjectFromURL(fmt.Sprintf("%s/assets/leg_right.obj", cdnUrl))) {
                obj.Texture = pantsTexture
            }
        }

    case "hat":
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
        }

    case "addon":
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
        }

    default:
        if obj := RenderItem(itemData); obj != nil {
            allObjects = append(allObjects, obj)
        }
    }


        return objects
}

func AddFace(faceHash  string) aeno.Texture {
        var face aeno.Texture

        if faceHash != "none" {
                face = aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+facePath+".png"))
        } else {
                face = aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/default.png"))
        }

        return face
}