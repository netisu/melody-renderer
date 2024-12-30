package main

import (
        "aeno" // if there is no aeno, use fauxgl
        "fmt"
        "net/http"
        "path/filepath"
        "strconv"
        "time"
)

const (
        scale      = 1
        fovy       = 15.5
        near       = 1
        far        = 1000
        amb        = "d4d4d4" // d4d4d4
        lightcolor = "696969" // 696969
        Dimentions = 512 // april fools (15)
)

var (
        eye           = aeno.V(0.72, 0.82, 2)
        center        = aeno.V(0, 0.06, 0)
        up            = aeno.V(0, 1, 0)
        light         = aeno.V(0, 6, 4).Normalize()
        cdnDirectory  = "/var/www/html/public/cdn" // set this to your storage root
        serverAddress = ":4315" // do not put links like (renderer.example.com) until after pentesting
)

func main() {
        // HTTP endpoint for rendering avatars
        http.HandleFunc("/", renderCommand)

        // Set up and start the HTTP server
        fmt.Printf("Starting server on %s\n", serverAddress)

        if err := http.ListenAndServe(serverAddress, nil); err != nil {
                fmt.Println("HTTP server error:", err)
        }
}

func renderCommand(w http.ResponseWriter, r *http.Request) {
        // Extract query parameters from the HTTP request with default values
        renderType := r.URL.Query().Get("renderType")

        switch renderType {
        case "user":
                renderUser(w, r)
                renderHeadshot(w, r)

        case "item":
                renderItem(w, r)

        case "item_preview":
                renderItemPreview(w, r)
        default:
                fmt.Println("Invalid renderType:", renderType)
                return
        }
}

func renderUser(w http.ResponseWriter, r *http.Request) {

        hash := r.URL.Query().Get("hash")
        if hash == "" {
                hash = "default"
        }

        // Delegate user avatar rendering logic here
        fmt.Println("Getting userstring", hash)

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
                face = "none"
        }

        tool := r.URL.Query().Get("tool")
        if tool == "" {
                tool = "none"
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
        objects := generateObjects(
                torso_color, leftLeg_color, rightLeg_color, rightArm_color, head_color,
                faceTexture,
                shirt, pants, tshirt,
                hat1, hat2, hat3, hat4, hat5, hat6,
                tool, leftArm_color,
        )
        fmt.Println("Exporting to", cdnDirectory, "thumbnails")
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
                path := filepath.Join(cdnDirectory, "thumbnails", hash + "_preview.png")
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
                path := filepath.Join(cdnDirectory, "thumbnails", hash + ".png")
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
                eye,
                center,
                aeno.V(0,1,0),
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
                headshot_eye    = aeno.V(0, 10, 19) // V(0, 10, 19)
                headshot_center = aeno.V(-0.5, 6.8, 0) // V(-0.5, 6.8, 0)
                headshot_up     = aeno.V(0, 4, 0) // V(0, 4, 0)
        )

        hash := r.URL.Query().Get("hash")
        if hash == "" {
                hash = "default"
        }

        // Delegate user avatar rendering logic here
        fmt.Println("Getting userstring", hash)

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
                hat1, hat2, hat3, hat4, hat5, hat6,
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
                24,
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

        // ... (call generateObjects and GenerateScene for user and headshot together)
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
                                Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/toolarm.obj")),
                                Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+shirt+".png")),
                                Color: aeno.HexColor(leftArmColor),
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

func generateObjects(
        torsoColor, leftLegColor, rightLegColor, rightArmColor, headColor string,
        faceTexture aeno.Texture,
        shirt, pants, tshirt,
        hat1, hat2, hat3, hat4, hat5, hat6 string,
        tool, leftArmColor string,
) []*aeno.Object {
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
        hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, hat6)
        objects = append(objects, hatObjects...)

        return objects
}
func generateHeadshot(
        torsoColor, leftLegColor, rightLegColor, rightArmColor, headColor string,
        faceTexture aeno.Texture,
        shirt, pants, tshirt,
        hat1, hat2, hat3, hat4, hat5, hat6 string,
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
        hatObjects := RenderHats(hat1, hat2, hat3, hat4, hat5, hat6)
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
                        Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/tshirt.obj")),
                        Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+tshirt+".png")),
                        Color: aeno.HexColor(torsoColor),
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
                        Mesh:  aeno.LoadObject(filepath.Join(cdnDirectory, "/assets/tshirt.obj")),
                        Texture: aeno.LoadTexture(filepath.Join(cdnDirectory, "/uploads/"+item+".png")),
                        Color: aeno.HexColor("5579C6"),
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
