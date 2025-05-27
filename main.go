package main

import (
        "aeno" // if there is no aeno, use fauxgl
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
        accessKey     = "DO002FLDZLCZVPGTU37L"
        secretKey     = "pFZlZX6YfLHHB6GcCQOhsPnswyywBkHE7JEmdxpalvQ"
        bucket        = "netisu" // set this to your s3 bucket
        serverAddress = ":4316"  // do not put links like (renderer.example.com) until after pentesting
        postKey       = "furrysesurio2929"
)

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

type ItemConfig struct {
        ItemType string `json:"ItemType"`
        Item     string `json:"Item"`
        PathMod  bool   `json:"PathMod"`
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
        fmt.Println("Exporting to", tempDir, "thumbnails")
        outputFile := path.Join("thumbnails", e.Hash+".png")

        path := path.Join(tempDir, e.Hash+".png")

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
                ContentType:        "image/png",
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
        itemJson := i.RenderJson

        // Check if UserJson is present
        if reflect.ValueOf(itemJson).IsZero() {
                log.Println("Warning: itemJson query parameter is missing, the preview will not render !")
                http.Error(w, "itemJson query parameter is missing", http.StatusBadRequest)
                return
        }

        // ... (call generateObjects and GenerateScene with user specific logic)
        start := time.Now()
        fmt.Println("Drawing Objects...")
        // Generate the list of objects using the function
        objects := generatePreview(i.RenderJson)
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
                ContentType:        "image/png",
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

        itemJson := i.RenderJson

        // Check if UserJson is present
        if reflect.ValueOf(itemJson).IsZero() {
                log.Println("Warning: itemJson query parameter is missing, the item will not render !")
                http.Error(w, "itemJson query parameter is missing", http.StatusBadRequest)
                return
        }
        start := time.Now()
        fmt.Println("Drawing Objects...")
        // Generate the list of objects using the function
        var objects []*aeno.Object

        if itemJson.ItemType == "head" {
                objects = RenderHead(itemJson.Item)
        } else if itemJson.ItemType != "face" {
                objects = RenderHats(itemJson.Item)
        }

        fmt.Println("Exporting to", tempDir, "thumbnails")
        outputFile := path.Join("thumbnails", i.Hash+".png")
        path := path.Join(tempDir, i.Hash+".png")

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
                ContentType:        "image/png",
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

        start := time.Now()
        fmt.Println("Drawing Objects...")
        // Generate the list of objects using the function
        objects := generateObjects(updatedUserConfig)

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
                ContentType:        "image/png",
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

func RenderHats(hats ...string) []*aeno.Object {
        var objects []*aeno.Object

        for _, hat := range hats {
                if hat != "none" {
                        obj := &aeno.Object{
                                Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+hat+".obj")),
                                Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+hat+".png")),
                        }
                        objects = append(objects, obj)
                }
        }

        return objects
}
func RenderHead(hats ...string) []*aeno.Object {
        var objects []*aeno.Object

        for _, hat := range hats {
                if hat != "none" {
                        obj := &aeno.Object{
                                Mesh: aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+hat+".obj")),
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
                                Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_tool.obj")),
                                Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirt+".png")),
                                Color:   aeno.HexColor(leftArmColor),
                        })
                } else {
                        armObjects = append(armObjects, &aeno.Object{
                                Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_tool.obj")),
                                Color: aeno.HexColor(leftArmColor),
                        })
                }

                // Load tool object based on tool name
                toolObj := &aeno.Object{
                        Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+tool+".png")),
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+tool+".obj")),
                }

                // Append tool objects based on if theres a tool
                armObjects = append(armObjects, toolObj)
        } else {
                if shirt != "none" {
                        // Append regular left arm if theres no tool
                        armObjects = append(armObjects, &aeno.Object{
                                Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_left.obj")),
                                Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirt+".png")),
                                Color:   aeno.HexColor(leftArmColor),
                        })
                } else {
                        // Append regular left arm if theres no tool
                        armObjects = append(armObjects, &aeno.Object{
                                Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_left.obj")),
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
        head := userConfig.Items.Head

        objects := Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, tool, rightArmColor, pants, shirt, tshirt)

        // Render and append the face object if a face texture is available
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

        // Render and append the hat objects
        hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, addon)
        objects = append(objects, hatObjects...)

        return objects
}

func Texturize(torsoColor, leftLegColor, rightLegColor, leftArmColor, tool, rightArmColorParam, pants, shirt, tshirt string) []*aeno.Object {
        objects := []*aeno.Object{}

        // Load torso object
        objects = append(objects, &aeno.Object{
                Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/chesticle.obj")),
                Color: aeno.HexColor(torsoColor),
        })

        // Load right arm object
        // Render and append the arm objects
        objects = append(objects, &aeno.Object{
                Mesh:  aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/arm_right.obj")),
                Color: aeno.HexColor(rightArmColorParam),
        })

        // Load leg objects (always loaded)
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

        // Load shirt texture if provided
        if shirt != "none" {
                shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+shirt+".png"))
                for _, obj := range objects[0:2] { // Skip torso and right arm
                        obj.Texture = shirtTexture
                }
        }

        // Load pants texture if provided (similar to shirt shii)
        if pants != "none" {
                pantsTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+pants+".png"))
                for _, obj := range objects[2:] { // Skip torso and right arm
                        obj.Texture = pantsTexture
                }
        }

        if tshirt != "none" {
                texture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+tshirt+".png"))

                TshirtLoader := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/tee.obj")),
                        Color:   aeno.Transparent,
                        Texture: texture,
                }
                objects = append(objects, TshirtLoader)
        }

        // Handle tool logic

        armObjects := ToolClause(tool, leftArmColor, rightArmColorParam, shirt)
        objects = append(objects, armObjects...)

        return objects
}

func generatePreview(itemConfig ItemConfig) []*aeno.Object {
        // Extract relevant data from the useDefault struct
        torsoColor := useDefault.Colors.TorsoColor
        leftLegColor := useDefault.Colors.LeftLegColor
        rightLegColor := useDefault.Colors.RightLegColor
        rightArmColor := useDefault.Colors.RightArmColor
        leftArmColor := useDefault.Colors.LeftArmColor
        headColor := useDefault.Colors.HeadColor
        faceTexture := AddFace(useDefault.Items.Face)

        itemType := itemConfig.ItemType
        item := itemConfig.Item

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
                // Render and append the arm objects
                armObject := ToolClause(item, "d3d3d3", "d3d3d3", "none")
                objects = append(objects, armObject...)
        }

        if itemType == "head" {
                HeadLoader := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".obj")),
                        Texture: faceTexture,
                        Color:   aeno.HexColor(headColor),
                }
                objects = append(objects, HeadLoader)
        }

        // Render and append the face object if a face texture is available
        if itemType == "face" {
                faceObject := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
                        Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".png")),
                        Color:   aeno.HexColor(headColor),
                }
                objects = append(objects, faceObject)
        } else if itemType == "head" {
                HeadLoader := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".obj")),
                        Texture: faceTexture,
                        Color:   aeno.HexColor(headColor),
                }
                objects = append(objects, HeadLoader)
        } else {
                faceObject := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/cranium.obj")),
                        Texture: faceTexture,
                        Color:   aeno.HexColor(headColor),
                }
                objects = append(objects, faceObject)
        }

        if itemType == "tshirt" {
                TshirtLoader := &aeno.Object{
                        Mesh:    aeno.LoadObjectFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/tee.obj")),
                        Texture: aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".png")),
                }
                objects = append(objects, TshirtLoader)
        }

        if itemType == "shirt" {
                shirtTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".png"))
                for _, obj := range objects[0:3] { // Skip torso and right arm
                        obj.Texture = shirtTexture
                }
        }

        if itemType == "pants" {
                pantsTexture := aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+item+".png"))
                for _, obj := range objects[3:5] { // Skip torso and right arm
                        obj.Texture = pantsTexture
                }
        }

        if itemType == "hat" {
                hatObject := RenderHats(item)
                objects = append(objects, hatObject...)
        }

        if itemType == "addon" {
                hatObject := RenderHats(item)
                objects = append(objects, hatObject...)
        }

        if itemType == "tool" {
                armObjects := ToolClause(item, leftArmColor, rightArmColor, "none")
                objects = append(objects, armObjects...)
        }

        return objects
}

func AddFace(facePath string) aeno.Texture {
        var face aeno.Texture

        if facePath != "none" {
                face = aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/uploads/"+facePath+".png"))
        } else {
                face = aeno.LoadTextureFromURL(fmt.Sprintf("%s%s", cdnUrl, "/assets/default.png"))
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
