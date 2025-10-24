package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/joho/godotenv"
	"github.com/netisu/aeno"
)

// --- Constants and Global Variables ---
const (
	scale      = 1
	fovy       = 15.0
	near       = 1.0
	far        = 1000
	amb        = "b0b0b0"
	lightcolor = "808080"
	Dimentions = 512
)

var (
	eye    = aeno.V(-0.75, 0.85, 2)
	center = aeno.V(0, 0, 0)
	up     = aeno.V(0, 1, 0)
	light  = aeno.V(-1, 3, 1).Normalize()
)

type ItemData struct {
	Item      string     `json:"item"`
	EditStyle *EditStyle `json:"edit_style"`
}
type BodyParts struct {
	Head     string `json:"head"`
	Torso    string `json:"torso"`
	ToolArm  string	`json:"tool_arm"`
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
	Hash       string     `json:"Hash"`
	RenderJson UserConfig `json:"RenderJson"` // Use interface{} for flexibility
}

type ItemEvent struct {
	Hash       string     `json:"Hash"`
	RenderJson ItemConfig `json:"RenderJson"` // Use interface{} for flexibility
}

type HatsCollection map[string]ItemData

type UserConfig struct {
	BodyParts BodyParts `json:"body_parts"`
	Items     struct {
		Face   ItemData       `json:"face"`
		Hats   HatsCollection `json:"hats"`
		Addon  ItemData       `json:"addon"`
		Tool   ItemData       `json:"tool"`
	    ToolArm ItemData	  `json:"tool_arm"`
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
		ToolArm:  "tool_arm",
	},
	Items: struct {
		Face   ItemData       `json:"face"`
		Hats   HatsCollection `json:"hats"`
		Addon  ItemData       `json:"addon"`
		Tool   ItemData       `json:"tool"`
		ToolArm ItemData      `json:"tool_arm"`
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
		ToolArm: ItemData{Item: "none"}, // Default for ToolArm
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

// hatKeyPattern is a regular expression to match keys like "hat_1", "hat_123", etc.
var hatKeyPattern = regexp.MustCompile(`^hat_\d+$`)

// Holds all environment variables, loaded once at startup.
type Config struct {
	PostKey       string
	ServerAddress string
	S3AccessKey   string
	S3SecretKey   string
	S3Endpoint    string
	S3Region      string
	S3Bucket      string
	CDNURL        string
	RootDir       string
}

// For headshots or any other place where we need it.
type RenderConfig struct {
    IncludeTool bool
}

// --- NEW: Asset Cache ---
// A thread-safe cache for meshes and textures to avoid redundant downloads.
type AssetCache struct {
	mu       sync.RWMutex
	meshes   map[string]*aeno.Mesh
	textures map[string]aeno.Texture
}

func NewAssetCache() *AssetCache {
	return &AssetCache{
		meshes:   make(map[string]*aeno.Mesh),
		textures: make(map[string]aeno.Texture),
	}
}

// GetMesh fetches a mesh from the cache or loads it from the URL if not present.
func (c *AssetCache) GetMesh(url string) *aeno.Mesh {
	c.mu.RLock()
	mesh, ok := c.meshes[url]
	c.mu.RUnlock()
	if ok {
		return mesh
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Double-check in case another goroutine loaded it while we were waiting for the lock
	if mesh, ok = c.meshes[url]; ok {
		return mesh
	}

	resp, err := http.Head(url)
    if err != nil || resp.StatusCode != http.StatusOK {
        log.Printf("Warning: Mesh not found or inaccessible at %s (Status: %d)", url, resp.StatusCode)
        c.meshes[url] = nil // Cache the failure to avoid repeated checks
        return nil
    }

	mesh = aeno.LoadObjectFromURL(url)
	c.meshes[url] = mesh
	return mesh
}

// GetTexture fetches a texture from the cache or loads it from the URL if not present.
func (c *AssetCache) GetTexture(url string) aeno.Texture {
	c.mu.RLock()
	texture, ok := c.textures[url]
	c.mu.RUnlock()
	if ok {
		return texture
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if texture, ok = c.textures[url]; ok {
		return texture
	}

	// Only load if the texture actually exists
	resp, err := http.Head(url)
	if err == nil && resp.StatusCode == http.StatusOK {
		texture = aeno.LoadTextureFromURL(url)
		c.textures[url] = texture
		return texture
	}
	// Return a nil texture if not found, which can be handled by the renderer
	return nil
}

func getTextureHash(itemData ItemData) string {
	if itemData.EditStyle != nil && itemData.EditStyle.IsTexture {
		return itemData.EditStyle.Hash
	}
	return itemData.Item
}

// Holds shared dependencies like config, S3 client, and cache.
type Server struct {
	config     *Config
	s3Uploader *s3.S3
	cache      *AssetCache
}

// Helper to get environment variables with a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// Initializes everything once.
func main() {
	rootDir := getEnv("RENDERER_ROOT_DIR", "/var/www/renderer")
	if err := godotenv.Load(path.Join(rootDir, ".env")); err != nil {
		log.Println("Warning: .env file not found or could not be loaded.")
	}

	cfg := &Config{
		PostKey:       os.Getenv("POST_KEY"),
		ServerAddress: os.Getenv("SERVER_ADDRESS"),
		S3AccessKey:   os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:   os.Getenv("S3_SECRET_KEY"),
		S3Endpoint:    os.Getenv("S3_ENDPOINT"),
		S3Region:      os.Getenv("S3_REGION"),
		S3Bucket:      os.Getenv("S3_BUCKET"),
		CDNURL:        os.Getenv("CDN_URL"),
		RootDir:       rootDir,
	}

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Endpoint:         aws.String(cfg.S3Endpoint),
		Region:           aws.String(cfg.S3Region),
		S3ForcePathStyle: aws.Bool(true),
	}
	newSession, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatalf("Failed to create S3 session: %v", err)
	}

	server := &Server{
		config:     cfg,
		s3Uploader: s3.New(newSession),
		cache:      NewAssetCache(),
	}

	http.HandleFunc("/", server.handleRender)

	fmt.Printf("Starting server on %s\n", cfg.ServerAddress)
	if err := http.ListenAndServe(cfg.ServerAddress, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

// --- NEW: Request Type Identifier ---
type RenderRequestType struct {
	RenderType string `json:"RenderType"`
}

// --- CHANGED: Central HTTP Handler ---
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	if s.config.PostKey != "" && r.Header.Get("Aeo-Access-Key") != s.config.PostKey {
		http.Error(w, "Unauthorized request", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// OPTIMIZATION: Unmarshal just the type first to decide what to do.
	var reqType RenderRequestType
	if err := json.Unmarshal(body, &reqType); err != nil {
		http.Error(w, "Invalid request body: could not determine RenderType", http.StatusBadRequest)
		return
	}

	log.Printf("Received render request of type: %s", reqType.RenderType)

	switch reqType.RenderType {
	case "user":
		var e RenderEvent
		if err := json.Unmarshal(body, &e); err != nil {
			http.Error(w, "Invalid request body for type 'user'", http.StatusBadRequest)
			return
		}
		s.handleUserRender(w, e)
	case "item":
		var i ItemEvent
		if err := json.Unmarshal(body, &i); err != nil {
			http.Error(w, "Invalid request body for type 'item'", http.StatusBadRequest)
			return
		}
		s.handleItemRender(w, i, false) // isPreview = false
	case "item_preview", "style":
		var i ItemEvent
		if err := json.Unmarshal(body, &i); err != nil {
            http.Error(w, "Invalid request body for type 'item_preview' or 'style'", http.StatusBadRequest)
			return
		}
		s.handleItemRender(w, i, true) // isPreview = true
	default:
		http.Error(w, "Invalid RenderType specified", http.StatusBadRequest)
	}
}

// --- NEW: CONCURRENT User Render Handler ---
func (s *Server) handleUserRender(w http.ResponseWriter, e RenderEvent) {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2) // We are running two render jobs in parallel

	go func() {
		defer wg.Done()
		objects := s.generateObjects(e.RenderJson, RenderConfig{IncludeTool: true})
		outputKey := path.Join("thumbnails", e.Hash+".png")
		
		var buffer bytes.Buffer
		aeno.GenerateSceneToWriter(
			&buffer,
			objects,
			eye, center, up, fovy,
			Dimentions, scale, light, amb, lightcolor, near, far, true, // This true actually decides if all objects are fit into a bounding box or not.
		)

		s.uploadToS3(buffer.Bytes(), outputKey)
	}()

	go func() {
		defer wg.Done()
		var (
			headshot_fovy 	= 15.5
			headshot_eye    = aeno.V(-4, 7, 13)
			headshot_center = aeno.V(-0.5, 6.8, 0)
			headshot_up     = aeno.V(0, 4, 0)
		)
		objects := s.generateObjects(e.RenderJson, RenderConfig{IncludeTool: false})
		outputKey := path.Join("thumbnails", e.Hash+"_headshot.png")

		var buffer bytes.Buffer
		aeno.GenerateSceneToWriter(
			&buffer,
			objects,
			headshot_eye, headshot_center, headshot_up, headshot_fovy,
			Dimentions, scale, light, amb, lightcolor, near, far, false,
		)

		s.uploadToS3(buffer.Bytes(), outputKey)
	}()

	wg.Wait()
	log.Printf("Completed user render for %s in %v", e.Hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User render and headshot processed successfully.")
}

func (s *Server) handleItemRender(w http.ResponseWriter, i ItemEvent, isPreview bool) {
	start := time.Now()
	var objects []*aeno.Object
	var outputKey string
	if isPreview {
		objects = s.generatePreview(i.RenderJson, RenderConfig{IncludeTool: true})
		if i.RenderJson.PathMod {
			outputKey = path.Join("thumbnails", i.Hash+"_preview.png")
		} else {
			outputKey = path.Join("thumbnails", i.Hash+".png")
		}
	} else {
		if renderedObject := s.RenderItem(i.RenderJson.Item); renderedObject != nil {
			objects = []*aeno.Object{renderedObject}
		}
		outputKey = path.Join("thumbnails", i.Hash+".png")
	}

	if len(objects) == 0 {
		http.Error(w, "No objects to render for this item", http.StatusBadRequest)
		return
	}

	var buffer bytes.Buffer
	aeno.GenerateSceneToWriter(
		&buffer,
		objects,
		eye, center, up, fovy,
		Dimentions, scale, light, amb, lightcolor, near, far, true,
	)

	s.uploadToS3(buffer.Bytes(), outputKey)
	
	log.Printf("Completed item render for %s in %v", i.Hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Item processed successfully.")
}


func (s *Server) uploadToS3(buffer []byte, key string) {
	size := int64(len(buffer))

	_, err := s.s3Uploader.PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(s.config.S3Bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(buffer),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("image/png"),
		ACL:           aws.String("public-read"),
	})

	if err != nil {
		log.Printf("Failed to upload %s to S3: %v", key, err)
	} else {
		log.Printf("Successfully uploaded %s to S3.", key)
	}
}

// Helper function to build the correct path
func (s *Server) getMeshPath(partName, defaultName string) string {
	cdnURL := s.config.CDNURL
	if partName == "" {
		partName = defaultName
	}
	if partName == defaultName {
		return fmt.Sprintf("%s/assets/%s.obj", cdnURL, partName)
	}
	return fmt.Sprintf("%s/uploads/%s.obj", cdnURL, partName)
}

func (s *Server) RenderItem(itemData ItemData) *aeno.Object {
	if itemData.Item == "none" {
		return nil
	}
	
	cdnURL := s.config.CDNURL
	meshURL := fmt.Sprintf("%s/uploads/%s.obj", cdnURL, itemData.Item)
	textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.Item)

	if itemData.EditStyle != nil {
		if itemData.EditStyle.IsModel {
			meshURL = fmt.Sprintf("%s/uploads/%s.obj", cdnURL, itemData.EditStyle.Hash)
		}
		if itemData.EditStyle.IsTexture {
			textureURL = fmt.Sprintf("%s/uploads/%s.png", cdnURL, itemData.EditStyle.Hash)
		}
	}

	finalMesh := s.cache.GetMesh(meshURL)

	if finalMesh == nil {
        log.Printf("Error: Could not render item because its mesh failed to load from %s", meshURL)
        return nil
    }
	
	// OPTIMIZATION: Use the cache
	return &aeno.Object{
		Mesh:    finalMesh.Copy(),
		Color:   aeno.Transparent,
		Texture: s.cache.GetTexture(textureURL),
		Matrix:  aeno.Identity(),
	}
}

func (s *Server) ToolClause(toolData ItemData, toolArmMeshName string, leftArmColor string, shirtData ItemData, config RenderConfig) []*aeno.Object {
	objects := []*aeno.Object{}
	cdnURL := s.config.CDNURL
	var shirtTexture aeno.Texture
	if shirtData.Item != "none" {
		shirtHash := getTextureHash(shirtData)
		textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, shirtHash)
		shirtTexture = s.cache.GetTexture(textureURL)
	}
	
	if config.IncludeTool {
		var toolArmPath string
		// If a custom tool arm is specified (and it's not the default placeholder name), use it.
		if toolArmMeshName != "" && toolArmMeshName != "arm_tool" {
			toolArmPath = fmt.Sprintf("%s/uploads/%s.obj", cdnURL, toolArmMeshName)
		} else {
			// Otherwise, use the default asset.
			toolArmPath = fmt.Sprintf("%s/assets/arm_tool.obj", cdnURL)
		}
		armMesh := s.cache.GetMesh(toolArmPath)

		if toolObj := s.RenderItem(toolData); toolObj != nil {
			objects = append(objects, toolObj)
		}

		if armMesh == nil {
			log.Printf("Warning: Failed to load tool arm mesh from '%s'. Skipping arm.", toolArmPath)
			return objects
		}

		armObject := &aeno.Object{
			Mesh:    armMesh.Copy(),
			Color:   aeno.HexColor(leftArmColor),
			Texture: shirtTexture,
			Matrix:  aeno.Identity(),
		}
		objects = append(objects, armObject)
	}
	
	return objects
}

func (s *Server) generateObjects(userConfig UserConfig, config RenderConfig) []*aeno.Object {
	cdnURL := s.config.CDNURL
	var allObjects []*aeno.Object
	bodyPartDefaults := map[string]string{
		"Head":     "cranium",
		"Torso":    "chesticle",
		"LeftArm":  "arm_left",
		"RightArm": "arm_right",
		"LeftLeg":  "leg_left",
		"RightLeg": "leg_right",
	}

	parts := struct {
		m map[string]string
	}{
		m: map[string]string{
			"Head":     userConfig.BodyParts.Head,
			"Torso":    userConfig.BodyParts.Torso,
			"LeftArm":  userConfig.BodyParts.LeftArm,
			"RightArm": userConfig.BodyParts.RightArm,
			"LeftLeg":  userConfig.BodyParts.LeftLeg,
			"RightLeg": userConfig.BodyParts.RightLeg,
		},
	}
	
	isToolEquipped := config.IncludeTool && userConfig.Items.Tool.Item != "none"

	for name, defaultMesh := range bodyPartDefaults {
		
		if name == "LeftArm" && isToolEquipped {
			continue // Skip to the next body part in the loop
		}
		// Use the helper function to determine the correct path (asset or upload).
		meshPath := s.getMeshPath(parts.m[name], defaultMesh)
		mesh := s.cache.GetMesh(meshPath)

		// If the mesh fails to load, log a warning and skip this part.
		if mesh == nil {
			log.Printf("Warning: Failed to load body part mesh for '%s' from '%s'. Skipping.", name, meshPath)
			continue
		}

		// Create the renderable object for the body part.
		bodyPartObject := &aeno.Object{
			Mesh:   mesh.Copy(),
			Color:  aeno.HexColor(userConfig.Colors[name]),
			Matrix: aeno.Identity(),
		}
		
		if name == "Torso" || name == "LeftArm" || name == "RightArm" && userConfig.Items.Shirt.Item != "none"  {
			shirtHash := getTextureHash(userConfig.Items.Shirt)
			textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, shirtHash)
			bodyPartObject.Texture = s.cache.GetTexture(textureURL)
		}

		if name == "LeftLeg" || name == "RightLeg" && userConfig.Items.Pants.Item != "none"  {
			pantHash := getTextureHash(userConfig.Items.Pants)
			textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, pantHash)
			bodyPartObject.Texture = s.cache.GetTexture(textureURL)
		}
		
		if name == "Head" {
			bodyPartObject.Texture = s.AddFace(userConfig.Items.Face)
		}
		
		// Add the completed object to our list for rendering.
		allObjects = append(allObjects, bodyPartObject)
	}

	// Here, we decide whether to render the normal left arm or the tool-holding arm.
	if isToolEquipped {
		armAndToolObjects := s.ToolClause(
			userConfig.Items.Tool,
			userConfig.BodyParts.ToolArm,
			userConfig.Colors["LeftArm"],
			userConfig.Items.Shirt,
			config,
		)
		allObjects = append(allObjects, armAndToolObjects...)
	}

	if config.Items.Tshirt.Item != "none" {
        teeMeshPath := fmt.Sprintf("%s/assets/tee.obj", cdnURL)
        teeMesh := s.cache.GetMesh(teeMeshPath)

        if teeMesh == nil {
            log.Printf("Warning: Failed to load t-shirt mesh from '%s'. Skipping.", teeMeshPath)
        } else {
            tshirtHash := getTextureHash(config.Items.Tshirt)
            tshirtTextureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, tshirtHash)
            tshirtTexture := s.cache.GetTexture(tshirtTextureURL)
            
            TshirtLoader := &aeno.Object{
                Mesh:    teeMesh.Copy(),
                Color:   aeno.Transparent,
                Texture: tshirtTexture,
                Matrix:  aeno.Identity(),
            }
            allObjects = append(allObjects, TshirtLoader)
        }
    }
	
	if obj := s.RenderItem(userConfig.Items.Addon); obj != nil {
		allObjects = append(allObjects, obj)
	}

	for hatKey, hatItemData := range userConfig.Items.Hats {
		if !hatKeyPattern.MatchString(hatKey) {
			log.Printf("Warning: Invalid hat key format: '%s'. Skipping hat.\n", hatKey)
			continue
		}
		if hatItemData.Item != "none" {
			if obj := s.RenderItem(hatItemData); obj != nil {
				allObjects = append(allObjects, obj)
			}
		}
	}
	
	return allObjects
}

func (s *Server) generatePreview(config ItemConfig, renderConfig RenderConfig) []*aeno.Object {
	fmt.Printf("generatePreview: Starting for ItemType: %s, Item: %+v\n", config.ItemType, config.Item)

	previewConfig := useDefault

	itemType := config.ItemType
	itemData := config.Item

	switch itemType {
	case "face":
		previewConfig.Items.Face = config.Item
	case "hat":
		previewConfig.Items.Hats = make(HatsCollection)
		previewConfig.Items.Hats["hat_1"] =  config.Item
	case "addon":
		previewConfig.Items.Addon =  config.Item
	case "tool":
		previewConfig.Items.Tool =  config.Item
	case "pants":
		previewConfig.Items.Pants =  config.Item
	case "shirt":
		previewConfig.Items.Shirt =  config.Item
	case "tshirt":
		previewConfig.Items.Tshirt =  config.Item
	case "head":
		if itemData.Item != "none" {
			previewConfig.BodyParts.Head = config.Item.Item
		}
	default:
		fmt.Printf("generatePreview: Unhandled item type '%s'. Showing default avatar.\n", config.ItemType)
	}
	return s.generateObjects(previewConfig, renderConfig)
}

func (s *Server) AddFace(faceData ItemData) aeno.Texture {
	faceURL := ""

	if faceData.Item != "none" && faceData.Item != "" {
		faceHash := getTextureHash(faceData)
		faceURL = fmt.Sprintf("%s/uploads/%s.png", s.config.CDNURL, faceHash)
	} else {
		faceURL = fmt.Sprintf("%s/assets/default.png", s.config.CDNURL)
	}

	// Use the cache to load and return the texture.
	return s.cache.GetTexture(faceURL)
}
