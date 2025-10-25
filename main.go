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
	"math"

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
	near       = 0.1
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
type SceneNode struct {
	Name        string
	Object      *aeno.Object // The renderable object (can be nil for empty joints)
	LocalMatrix aeno.Matrix  // Transformation relative to the parent
	Children    []*SceneNode
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
var shoulderJointOffset = aeno.V(0, 0, 0) 
var leftarmEquippedPose = aeno.Translate(aeno.V(0, 0.34, 0.4)).Mul(aeno.Rotate(aeno.V(90, 0, 0), math.Pi/2))

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

func NewSceneNode(name string, obj *aeno.Object, matrix aeno.Matrix) *SceneNode {
	return &SceneNode{
		Name:        name,
		Object:      obj,
		LocalMatrix: matrix,
		Children:    make([]*SceneNode, 0),
	}
}
func (n *SceneNode) AddChild(child *SceneNode) {
	n.Children = append(n.Children, child)
}

// FindNodeByName recursively searches the tree for a node by its name.
func (n *SceneNode) FindNodeByName(name string) *SceneNode {
	if n.Name == name {
		return n
	}
	for _, child := range n.Children {
		if found := child.FindNodeByName(name); found != nil {
			return found
		}
	}
	return nil
}

func (n *SceneNode) Flatten(parentMatrix aeno.Matrix, objects *[]*aeno.Object) {
	worldMatrix := parentMatrix.Mul(n.LocalMatrix)

	if n.Object != nil {
		n.Object.Matrix = worldMatrix
		*objects = append(*objects, n.Object)
	}

	// Recursively do the same for all children.
	for _, child := range n.Children {
		child.Flatten(worldMatrix, objects)
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

// not so new now lol
func (s *Server) handleUserRender(w http.ResponseWriter, e RenderEvent) {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2) // We are running two render jobs in parallel

	go func() {
		defer wg.Done()
		rootNode, _ := s.buildCharacterTree(e.RenderJson, RenderConfig{IncludeTool: true})		

		var allObjects []*aeno.Object // This is the flat list the renderer needs
		rootNode.Flatten(aeno.Identity(), &allObjects)

		outputKey := path.Join("thumbnails", e.Hash+".png")
		var buffer bytes.Buffer
		aeno.GenerateSceneToWriter(
			&buffer,
			allObjects,
			eye, center, up, fovy,
			Dimentions, scale, light, amb, lightcolor, near, far, true, // This true actually decides if all objects are fit into a bounding box or not.
		)

		s.uploadToS3(buffer.Bytes(), outputKey)
	}()

	go func() {
		defer wg.Done()
		var (
			headshot_fovy 	= 23.5
			headshot_eye    = aeno.V(-4, 7, 13)
			headshot_center = aeno.V(-0.5, 6.8, 0)
			headshot_up     = aeno.V(0, 4, 0)
		)
		rootNode, _ := s.buildCharacterTree(e.RenderJson, RenderConfig{IncludeTool: false})
		var allObjects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &allObjects)
		
		outputKey := path.Join("thumbnails", e.Hash+"_headshot.png")

		var buffer bytes.Buffer
		aeno.GenerateSceneToWriter(
			&buffer,
			allObjects,
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
	var allObjects []*aeno.Object
	var outputKey string
	if isPreview {
		rootNode, _ := s.generatePreview(i.RenderJson, RenderConfig{IncludeTool: true})
		rootNode.Flatten(aeno.Identity(), &allObjects)
		if i.RenderJson.PathMod {
			outputKey = path.Join("thumbnails", i.Hash+"_preview.png")
		} else {
			outputKey = path.Join("thumbnails", i.Hash+".png")
		}
	} else {
		if renderedObject := s.RenderItem(i.RenderJson.Item); renderedObject != nil {
			allObjects = []*aeno.Object{renderedObject}
		}
		outputKey = path.Join("thumbnails", i.Hash+".png")
	}

	if len(allObjects) == 0 {
		http.Error(w, "No objects to render for this item", http.StatusBadRequest)
		return
	}

	var buffer bytes.Buffer
	aeno.GenerateSceneToWriter(
		&buffer,
		allObjects,
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

func (s *Server) buildCharacterTree(userConfig UserConfig, config RenderConfig) (*SceneNode, bool) {
	cdnURL := s.config.CDNURL
	isToolEquipped := config.IncludeTool && userConfig.Items.Tool.Item != "none"

	parts := map[string]string{
		"Head":     userConfig.BodyParts.Head,
		"Torso":    userConfig.BodyParts.Torso,
		"LeftArm":  userConfig.BodyParts.LeftArm,
		"RightArm": userConfig.BodyParts.RightArm,
		"LeftLeg":  userConfig.BodyParts.LeftLeg,
		"RightLeg": userConfig.BodyParts.RightLeg,
	}
	// This map holds the *default* mesh names
	bodyPartDefaults := map[string]string{
		"Head":     "cranium",
		"Torso":    "chesticle",
		"LeftArm":  "arm_left",
		"RightArm": "arm_right",
		"LeftLeg":  "leg_left",
		"RightLeg": "leg_right",
	}

	// Create the root node for the character
	rootNode := NewSceneNode("Character", nil, aeno.Identity())

	// Load Torso (This is the central part of the body)
	torsoMeshPath := s.getMeshPath(parts["Torso"], bodyPartDefaults["Torso"])
	torsoMesh := s.cache.GetMesh(torsoMeshPath)
	if torsoMesh == nil {
		log.Printf("CRITICAL: Failed to load Torso mesh from '%s'. Aborting tree build.", torsoMeshPath)
		return rootNode, isToolEquipped // Return an empty tree
	}

	torsoObj := &aeno.Object{
		Mesh:   torsoMesh.Copy(),
		Color:  aeno.HexColor(userConfig.Colors["Torso"]),
		Matrix: aeno.Identity(), // Will be set by Flatten()
	}
	// Apply shirt texture to Torso
	if userConfig.Items.Shirt.Item != "none" {
		shirtHash := getTextureHash(userConfig.Items.Shirt)
		textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, shirtHash)
		torsoObj.Texture = s.cache.GetTexture(textureURL)
	}

	// Add the Torso to the root node. We assume the Torso is at the origin.
	torsoNode := NewSceneNode("Torso", torsoObj, aeno.Identity())
	rootNode.AddChild(torsoNode)

	// --- Children of Torso ---

	// 3. Load Head (Child of Torso)
	headMeshPath := s.getMeshPath(parts["Head"], bodyPartDefaults["Head"])
	headMesh := s.cache.GetMesh(headMeshPath)
	var headNode *SceneNode // Declare headNode so we can add hats to it
	if headMesh != nil {
		headObj := &aeno.Object{
			Mesh:    headMesh.Copy(),
			Color:   aeno.HexColor(userConfig.Colors["Head"]),
			Texture: s.AddFace(userConfig.Items.Face),
			Matrix:  aeno.Identity(),
		}

		headMatrix := aeno.Translate(aeno.V(0, 0, 0)) // guesstimate: (0, 1.5, 0)
		headNode = NewSceneNode("Head", headObj, headMatrix)
		torsoNode.AddChild(headNode)

	} else {
		log.Printf("Warning: Failed to load head mesh from '%s'.", headMeshPath)
		headMatrix := aeno.Translate(aeno.V(0, 0, 0)) // guesstimate
		headNode = NewSceneNode("Head", nil, headMatrix)
		torsoNode.AddChild(headNode)
	}

	// 4. Load Hats (Children of Head)
	for hatKey, hatItemData := range userConfig.Items.Hats {
		if !hatKeyPattern.MatchString(hatKey) {
			log.Printf("Warning: Invalid hat key format: '%s'. Skipping hat.\n", hatKey)
			continue
		}
		if hatItemData.Item != "none" {
			if hatObj := s.RenderItem(hatItemData); hatObj != nil {
				hatNode := NewSceneNode(hatKey, hatObj, aeno.Identity())
				headNode.AddChild(hatNode)
			}
		}
	}

	// 5. Load Legs (Children of Torso)
	// These are offsets for the hip joints.
	legOffsets := map[string]aeno.Vector{
		"LeftLeg":  aeno.V(0, 0, 0), // guesstimate
		"RightLeg": aeno.V(0, 0, 0), // guesstimate
	}
	for _, name := range []string{"LeftLeg", "RightLeg"} {
		meshPath := s.getMeshPath(parts[name], bodyPartDefaults[name])
		mesh := s.cache.GetMesh(meshPath)
		if mesh == nil {
			log.Printf("Warning: Failed to load leg mesh for '%s' from '%s'.", name, meshPath)
			continue
		}

		legObj := &aeno.Object{
			Mesh:   mesh.Copy(),
			Color:  aeno.HexColor(userConfig.Colors[name]),
			Matrix: aeno.Identity(),
		}
		if userConfig.Items.Pants.Item != "none" {
			pantHash := getTextureHash(userConfig.Items.Pants)
			textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, pantHash)
			legObj.Texture = s.cache.GetTexture(textureURL)
		}

		legMatrix := aeno.Translate(legOffsets[name])
		legNode := NewSceneNode(name, legObj, legMatrix)
		torsoNode.AddChild(legNode)
	}


	rightArmJointMatrix := aeno.Translate(aeno.V(0, 0, 0)) // change as its a gusstimate....
	rightArmNode := NewSceneNode("RightArm", nil, rightArmJointMatrix) // This is the node you would rotate
	torsoNode.AddChild(rightArmNode)

	rightArmMeshPath := s.getMeshPath(parts["RightArm"], bodyPartDefaults["RightArm"])
	rightArmMesh := s.cache.GetMesh(rightArmMeshPath)
	if rightArmMesh != nil {
		rightArmObj := &aeno.Object{
			Mesh:   rightArmMesh.Copy(),
			Color:  aeno.HexColor(userConfig.Colors["RightArm"]),
			Matrix: aeno.Identity(),
		}
		if userConfig.Items.Shirt.Item != "none" {
			shirtHash := getTextureHash(userConfig.Items.Shirt)
			textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, shirtHash)
			rightArmObj.Texture = s.cache.GetTexture(textureURL)
		}
		// The arm mesh is parented to the joint, with no extra offset
		rightArmMeshNode := NewSceneNode("RightArmMesh", rightArmObj, aeno.Identity())
		rightArmNode.AddChild(rightArmMeshNode)
	} else {
		log.Printf("Warning: Failed to load RightArm mesh from '%s'.", rightArmMeshPath)
	}

	// --- Left Arm (Complex case with Tool) ---
	var leftArmJointMatrix aeno.Matrix
	if isToolEquipped {
		leftArmJointMatrix = leftarmEquippedPose
	} else {
		leftArmJointMatrix = aeno.Translate(shoulderJointOffset) // guesstimate

	}
	leftArmNode := NewSceneNode("LeftArm", nil, leftArmJointMatrix) // This is the node you will rotate!
	torsoNode.AddChild(leftArmNode)

	meshPath := s.getMeshPath(parts["LeftArm"], bodyPartDefaults["LeftArm"])
	mesh := s.cache.GetMesh(meshPath)
	if mesh != nil {
		leftArmObj := &aeno.Object{
			Mesh:   mesh.Copy(),
			Color:  aeno.HexColor(userConfig.Colors["LeftArm"]),
			Matrix: aeno.Identity(),
		}
		if userConfig.Items.Shirt.Item != "none" {
			shirtHash := getTextureHash(userConfig.Items.Shirt)
			textureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, shirtHash)
			leftArmObj.Texture = s.cache.GetTexture(textureURL)
		}

		// Arm mesh is parented to the joint
		leftArmMeshNode := NewSceneNode("LeftArmMesh", leftArmObj, aeno.Identity())
		leftArmNode.AddChild(leftArmMeshNode)
		if isToolEquipped {
			// Load the tool (Child of the Left Arm)
			if toolObj := s.RenderItem(userConfig.Items.Tool); toolObj != nil {
				toolNode := NewSceneNode("Tool", toolObj, aeno.Identity())
				leftArmMeshNode.AddChild(toolNode) // Parent tool to the arm
			}
		} else {
			log.Printf("Warning: Failed to load tool arm mesh from '%s'.", meshPath)
		}
	} else {
		log.Printf("Warning: Failed to load LeftArm mesh from '%s'.", meshPath)
	}

	// 7. Load T-Shirt (Child of Torso)
	if userConfig.Items.Tshirt.Item != "none" {
		teeMeshPath := fmt.Sprintf("%s/assets/tee.obj", cdnURL)
		teeMesh := s.cache.GetMesh(teeMeshPath)
		if teeMesh != nil {
			tshirtHash := getTextureHash(userConfig.Items.Tshirt)
			tshirtTextureURL := fmt.Sprintf("%s/uploads/%s.png", cdnURL, tshirtHash)
			tshirtTexture := s.cache.GetTexture(tshirtTextureURL)

			TshirtLoader := &aeno.Object{
				Mesh:    teeMesh.Copy(),
				Color:   aeno.Transparent,
				Texture: tshirtTexture,
				Matrix:  aeno.Identity(),
			}
			// T-Shirt is an overlay on the Torso, so no offset
			tshirtNode := NewSceneNode("Tshirt", TshirtLoader, aeno.Identity())
			torsoNode.AddChild(tshirtNode)
		} else {
			log.Printf("Warning: Failed to load t-shirt mesh from '%s'.", teeMeshPath)
		}
	}

	// 8. Load Addon (Child of Torso)
	if obj := s.RenderItem(userConfig.Items.Addon); obj != nil {
		addonNode := NewSceneNode("Addon", obj, aeno.Identity())
		torsoNode.AddChild(addonNode)
	}

	return rootNode, isToolEquipped
}

func (s *Server) generatePreview(config ItemConfig, renderConfig RenderConfig) (*SceneNode, bool) {
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
	return s.buildCharacterTree(previewConfig, renderConfig)
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
