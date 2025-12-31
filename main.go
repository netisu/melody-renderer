package main

import (
	"bytes"
	"context"
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
	Scale         = 1
	FovY          = 50
	Near          = 0.1
	Far           = 1000.0
	AmbColor      = "#b0b0b0"
	LightColor    = "#808080"
	Dimensions    = 512
	RenderTimeout = 20 * time.Second
	UploadTimeout = 10 * time.Second
)

var (
	eye    = aeno.V(4, 7, 13)
	center = aeno.V(0, 0.06, 0)
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

// SceneNode represents a node in the hierarchical scene graph
type SceneNode struct {
	Name        string
	Object      *aeno.Object // The renderable object (can be nil for empty joints)
	LocalMatrix aeno.Matrix  // Transformation relative to the parent
	Children    []*SceneNode
}

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
	httpClient *http.Client
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
		ToolArm:  "arm_tool",
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

func NewAssetCache(client *http.Client) *AssetCache {
	return &AssetCache{
		meshes:   make(map[string]*aeno.Mesh),
		textures: make(map[string]aeno.Texture),
		httpClient: client,
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
	// Double check after acquiring lock
	if mesh, ok = c.meshes[url]; ok {
		return mesh
	}

	// Verify existence via HEAD first to save bandwidth if missing
	resp, err := c.httpClient.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Printf("Warning: Mesh inaccessible at %s", url)
		c.meshes[url] = nil
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	mesh, err = aeno.LoadOBJFromReader(resp.Body)
	if err != nil {
		log.Printf("Warning: Failed to parse OBJ from %s: %v", url, err)
		c.meshes[url] = nil
		return nil
	}

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

	texture = aeno.LoadTextureFromURL(url)
	c.textures[url] = texture
	return texture
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
	httpClient *http.Client
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
	_ = godotenv.Load(path.Join(rootDir, ".env"))

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
	sess, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatalf("Failed to create S3 session: %v", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	server := &Server{
		config:     cfg,
		s3Uploader: s3.New(sess),
		cache:      NewAssetCache(httpClient),
		httpClient: httpClient,
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Peek at the RenderType
	var reqType RenderRequestType
	if err := json.Unmarshal(body, &reqType); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received RenderType: %s", reqType.RenderType)

	switch reqType.RenderType {
	case "user":
		var e RenderEvent
		if err := json.Unmarshal(body, &e); err != nil {
			http.Error(w, "Invalid user render body", http.StatusBadRequest)
			return
		}
		s.handleUserRender(w, e)
	case "item", "style":
		var i ItemEvent
		if err := json.Unmarshal(body, &i); err != nil {
			http.Error(w, "Invalid item render body", http.StatusBadRequest)
			return
		}
		s.handleItemRender(w, r, i)
	default:
		http.Error(w, "Unknown RenderType", http.StatusBadRequest)
	}
}

func (s *Server) runRenderWithTimeout(
	objects []*aeno.Object,
	eye, center, up aeno.Vector,
	fovy float64,
	dim, scale int,
	light aeno.Vector,
	ambStr, lightColorStr string,
	near, far float64,
	fit bool,
) ([]byte, error) {

	ctx, cancel := context.WithTimeout(context.Background(), RenderTimeout)
	defer cancel()

	type result struct {
		data []byte
		err  error
	}
	resChan := make(chan result, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resChan <- result{nil, fmt.Errorf("panic in renderer: %v", r)}
			}
		}()
		
		var buf bytes.Buffer

		err := aeno.GenerateSceneToWriter(
			&buf,
			objects,
			eye,
			center,
			up,
			fovy,
			dim,
			scale,
			light,
			ambStr,
			lightColorStr,
			near,
			far,
			fit,
		)

		resChan <- result{data: buf.Bytes(), err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("render timeout")
	case res := <-resChan:
		return res.data, res.err
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
		var objects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &objects)

		buf, err := s.runRenderWithTimeout(objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
		if err != nil {
			log.Printf("User render failed: %v", err)
			return
		}
		if err := s.uploadToS3(context.Background(), buf, path.Join("thumbnails", e.Hash+".png")); err != nil {
			log.Printf("User upload failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		var (
			hsFovy   = 25.5
			hsEye    = aeno.V(4, 7, 13)
			hsCenter = aeno.V(-0.5, 6.8, 0)
			hsUp     = aeno.V(0, 1, 0)
		)
		rootNode, _ := s.buildCharacterTree(e.RenderJson, RenderConfig{IncludeTool: false})
		var objects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &objects)

		buf, err := s.runRenderWithTimeout(objects, hsEye, hsCenter, hsUp, hsFovy, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, false)
		if err != nil {
			log.Printf("Headshot render failed: %v", err)
			return
		}
		if err := s.uploadToS3(context.Background(), buf, path.Join("thumbnails", e.Hash+"_headshot.png")); err != nil {
			log.Printf("Headshot upload failed: %v", err)
		}
	}()

	wg.Wait()
	log.Printf("Completed user render for %s in %v", e.Hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User render and headshot processed successfully.")
}

func (s *Server) handleItemRender(w http.ResponseWriter, r *http.Request, i ItemEvent) {
	start := time.Now()
	rootNode, _ := s.generatePreview(i.RenderJson, RenderConfig{IncludeTool: true})

	var objects []*aeno.Object
	rootNode.Flatten(aeno.Identity(), &objects)

	if len(objects) == 0 {
		http.Error(w, "No objects to render", http.StatusBadRequest)
		return
	}

	outputKey := path.Join("thumbnails", i.Hash+".png")
	if i.RenderJson.PathMod {
		outputKey = path.Join("thumbnails", i.Hash+"_preview.png")
	}

	buf, err := s.runRenderWithTimeout(objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
	if err != nil {
		log.Printf("Item render failed: %v", err)
		http.Error(w, "Render failed", http.StatusGatewayTimeout)
		return
	}

	if err := s.uploadToS3(r.Context(), buf, outputKey); err != nil {
		log.Printf("Item upload failed: %v", err)
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Item render %s finished in %v", i.Hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Item processed.")
}

func (s *Server) uploadToS3(ctx context.Context, data []byte, key string) error {
	ctx, cancel := context.WithTimeout(ctx, UploadTimeout)
	defer cancel()

	size := int64(len(data))
	_, err := s.s3Uploader.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.config.S3Bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("image/png"),
		ACL:           aws.String("public-read"),
	})

	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}
	
	log.Printf("Uploaded %s to S3 (%d bytes)", key, size)
	return nil
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
		toolArmPath = fmt.Sprintf("%s/assets/arm_tool.obj", cdnURL)
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

func (s *Server) buildCharacterTree(userConfig UserConfig, config RenderConfig) (*SceneNode, bool) {
	cdnURL := s.config.CDNURL
	isToolEquipped := config.IncludeTool && userConfig.Items.Tool.Item != "none"
	
	defaults := map[string]string{
		"Head": "cranium", "Torso": "chesticle", "LeftArm": "arm_left",
		"RightArm": "arm_right", "LeftLeg": "leg_left", "RightLeg": "leg_right",
	}

	parts := map[string]string{
		"Head": userConfig.BodyParts.Head, "Torso": userConfig.BodyParts.Torso,
		"LeftArm": userConfig.BodyParts.LeftArm, "RightArm": userConfig.BodyParts.RightArm,
		"LeftLeg": userConfig.BodyParts.LeftLeg, "RightLeg": userConfig.BodyParts.RightLeg,
	}

	// Create the root node for the character
	rootNode := NewSceneNode("Character", nil, aeno.Identity())

	// Load Torso (This is the central part of the body)
	torsoMesh := s.cache.GetMesh(s.getMeshPath(parts["Torso"], defaults["Torso"]))
	if torsoMesh == nil {
		log.Println("CRITICAL: Torso mesh missing, aborting.")
		return rootNode, isToolEquipped
	}

	torsoObj := &aeno.Object{
		Mesh:   torsoMesh.Copy(),
		Color:  aeno.HexColor(userConfig.Colors["Torso"]),
		Matrix: aeno.Identity(),
	}
	
	// Apply shirt texture to Torso
	if userConfig.Items.Shirt.Item != "none" {
		url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
		torsoObj.Texture = s.cache.GetTexture(url)
	}
	torsoNode := NewSceneNode("Torso", torsoObj, aeno.Identity())
	rootNode.AddChild(torsoNode)


	// Head (Child of Torso)
	headMesh := s.cache.GetMesh(s.getMeshPath(parts["Head"], defaults["Head"]))
	headMatrix := aeno.Translate(aeno.V(0, 0, 0)) // Guesstimate offset
	var headNode *SceneNode

	if headMesh != nil {
		headObj := &aeno.Object{
			Mesh:    headMesh.Copy(),
			Color:   aeno.HexColor(userConfig.Colors["Head"]),
			Texture: s.AddFace(userConfig.Items.Face),
			Matrix:  aeno.Identity(),
		}
		headNode = NewSceneNode("Head", headObj, headMatrix)
	} else {
		headNode = NewSceneNode("Head", nil, headMatrix)
	}
	torsoNode.AddChild(headNode)
	
	// Hats (Children of Head)
	for key, hatData := range userConfig.Items.Hats {
		if hatKeyPattern.MatchString(key) && hatData.Item != "none" {
			if hatObj := s.RenderItem(hatData); hatObj != nil {
				headNode.AddChild(NewSceneNode(key, hatObj, aeno.Identity()))
			}
		}
	}

	// Legs (Children of Torso)
	legOffsets := map[string]aeno.Vector{"LeftLeg": aeno.V(0, 0, 0), "RightLeg": aeno.V(0, 0, 0)}
	for _, name := range []string{"LeftLeg", "RightLeg"} {
		mesh := s.cache.GetMesh(s.getMeshPath(parts[name], defaults[name]))
		if mesh == nil {
			continue
		}
		legObj := &aeno.Object{Mesh: mesh.Copy(), Color: aeno.HexColor(userConfig.Colors[name]), Matrix: aeno.Identity()}
		if userConfig.Items.Pants.Item != "none" {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Pants))
			legObj.Texture = s.cache.GetTexture(url)
		}
		torsoNode.AddChild(NewSceneNode(name, legObj, aeno.Translate(legOffsets[name])))
	}


	// Right Arm
	rightArmNode := NewSceneNode("RightArm", nil, aeno.Identity()) // Joint
	torsoNode.AddChild(rightArmNode)
	
	rArmMesh := s.cache.GetMesh(s.getMeshPath(parts["RightArm"], defaults["RightArm"]))
	if rArmMesh != nil {
		rArmObj := &aeno.Object{Mesh: rArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["RightArm"]), Matrix: aeno.Identity()}
		if userConfig.Items.Shirt.Item != "none" {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
			rArmObj.Texture = s.cache.GetTexture(url)
		}
		rightArmNode.AddChild(NewSceneNode("RightArmMesh", rArmObj, aeno.Identity()))
	}
	
	// Left Arm & Tool
	leftArmNode := NewSceneNode("LeftArm", nil, aeno.Identity()) // Joint
	torsoNode.AddChild(leftArmNode)

	var lArmMesh *aeno.Mesh
	if isToolEquipped {
		// Load tool arm mesh
		toolArmName := userConfig.BodyParts.ToolArm
		path := fmt.Sprintf("%s/assets/arm_tool.obj", cdnURL)
		if toolArmName != "" && toolArmName != "arm_tool" {
			path = fmt.Sprintf("%s/uploads/%s.obj", cdnURL, toolArmName)
		}
		lArmMesh = s.cache.GetMesh(path)
	} else {
		// Load standard left arm
		lArmMesh = s.cache.GetMesh(s.getMeshPath(parts["LeftArm"], defaults["LeftArm"]))
	}

	if lArmMesh != nil {
		lArmObj := &aeno.Object{Mesh: lArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["LeftArm"]), Matrix: aeno.Identity()}
		if userConfig.Items.Shirt.Item != "none" {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
			lArmObj.Texture = s.cache.GetTexture(url)
		}
		lArmMeshNode := NewSceneNode("LeftArmMesh", lArmObj, aeno.Identity())
		leftArmNode.AddChild(lArmMeshNode)

		// Attach Tool if equipped
		if isToolEquipped && userConfig.Items.Tool.Item != "none" {
			if toolObj := s.RenderItem(userConfig.Items.Tool); toolObj != nil {
				lArmMeshNode.AddChild(NewSceneNode("Tool", toolObj, aeno.Identity()))
			}
		}
	}

	// T-Shirt & Addon
	if userConfig.Items.Tshirt.Item != "none" {
		if teeMesh := s.cache.GetMesh(fmt.Sprintf("%s/assets/tee.obj", cdnURL)); teeMesh != nil {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Tshirt))
			teeObj := &aeno.Object{Mesh: teeMesh.Copy(), Color: aeno.Transparent, Texture: s.cache.GetTexture(url), Matrix: aeno.Identity()}
			torsoNode.AddChild(NewSceneNode("Tshirt", teeObj, aeno.Identity()))
		}
	}
	if obj := s.RenderItem(userConfig.Items.Addon); obj != nil {
		torsoNode.AddChild(NewSceneNode("Addon", obj, aeno.Identity()))
	}

	return rootNode, isToolEquipped
}

func (s *Server) generatePreview(config ItemConfig, renderConfig RenderConfig) (*SceneNode, bool) {
	previewConfig := useDefault
	switch config.ItemType {
	case "face": previewConfig.Items.Face = config.Item
	case "hat":  previewConfig.Items.Hats = HatsCollection{"hat_1": config.Item}
	case "addon": previewConfig.Items.Addon = config.Item
	case "tool": previewConfig.Items.Tool = config.Item
	case "pants": previewConfig.Items.Pants = config.Item
	case "shirt": previewConfig.Items.Shirt = config.Item
	case "tshirt": previewConfig.Items.Tshirt = config.Item
	case "head": if config.Item.Item != "none" { previewConfig.BodyParts.Head = config.Item.Item }
	case "torso": if config.Item.Item != "none" { previewConfig.BodyParts.Torso = config.Item.Item }
	case "left_arm": if config.Item.Item != "none" { previewConfig.BodyParts.LeftArm = config.Item.Item }
	case "right_arm": if config.Item.Item != "none" { previewConfig.BodyParts.RightArm = config.Item.Item }
	case "left_leg": if config.Item.Item != "none" { previewConfig.BodyParts.LeftLeg = config.Item.Item }
	case "right_leg": if config.Item.Item != "none" { previewConfig.BodyParts.RightLeg = config.Item.Item }
	case "tool_arm":
		if config.Item.Item != "none" {
			previewConfig.BodyParts.ToolArm = config.Item.Item
			previewConfig.Items.Tool = ItemData{Item: "none"}
		}
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
