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

const (
	Scale         = 4
	FovY          = float64(15)
	Near          = 1
	Far           = 10
	AmbColor      = "#666666"
	LightColor    = "#999999"
	Dimensions    = 512
	RenderTimeout = 20 * time.Second
	UploadTimeout = 10 * time.Second
)

var (
	eye    = aeno.V(0.75, 0.85, 2)
	center = aeno.V(0, 0.06, 0)
	up     = aeno.V(0, 1, 0)
	light  = aeno.V(-3, 7, 10).Normalize()
)

type ItemData struct {
	Item      string     `json:"item"`
	EditStyle *EditStyle `json:"edit_style"`
}

type EditStyle struct {
	Hash      string `json:"hash"`
	IsModel   bool   `json:"is_model"`
	IsTexture bool   `json:"is_texture"`
}

type BodyParts struct {
	Head     string `json:"head"`
	Torso    string `json:"torso"`
	LeftArm  string `json:"left_arm"`
	RightArm string `json:"right_arm"`
	LeftLeg  string `json:"left_leg"`
	RightLeg string `json:"right_leg"`
	ToolArm  string `json:"tool_arm"`
}

// ItemsCollection groups all apparel.
// 'items' => array_merge(['hats' => ...], $apparelForRender)
type ItemsCollection struct {
	Hats   map[string]ItemData `json:"hats"`
	Face   ItemData            `json:"face"`
	Addon  ItemData            `json:"addon"`
	Tool   ItemData            `json:"tool"`
	Pants  ItemData            `json:"pants"`
	Shirt  ItemData            `json:"shirt"`
	Tshirt ItemData            `json:"tshirt"`
}

type UserConfig struct {
	BodyParts BodyParts         `json:"body_parts"`
	Items     ItemsCollection   `json:"items"`
	Colors    map[string]string `json:"colors"`
}

type ItemConfig struct {
	ItemType string   `json:"ItemType"`
	Item     ItemData `json:"Item"`
}

type RenderRequest struct {
	RenderType string          `json:"RenderType"`
	Hash       string          `json:"Hash"`
	RenderJson json.RawMessage `json:"RenderJson"` // Delay parsing until we know type
}

type AssetCache struct {
	mu         sync.RWMutex
	meshes     map[string]*aeno.Mesh
	textures   map[string]aeno.Texture
	httpClient *http.Client
}

func NewAssetCache(client *http.Client) *AssetCache {
	return &AssetCache{
		meshes:     make(map[string]*aeno.Mesh),
		textures:   make(map[string]aeno.Texture),
		httpClient: client,
	}
}

type SceneNode struct {
	Name        string
	Object      *aeno.Object
	LocalMatrix aeno.Matrix
	Children    []*SceneNode
}

func NewSceneNode(name string, obj *aeno.Object, matrix aeno.Matrix) *SceneNode {
	return &SceneNode{Name: name, Object: obj, LocalMatrix: matrix, Children: make([]*SceneNode, 0)}
}

func (n *SceneNode) AddChild(child *SceneNode) {
	n.Children = append(n.Children, child)
}

func (n *SceneNode) Flatten(parentMatrix aeno.Matrix, objects *[]*aeno.Object) {
	worldMatrix := parentMatrix.Mul(n.LocalMatrix)
	if n.Object != nil {
		n.Object.Matrix = worldMatrix
		*objects = append(*objects, n.Object)
	}
	for _, child := range n.Children {
		child.Flatten(worldMatrix, objects)
	}
}

type Config struct {
	PostKey       string
	ServerAddress string
	S3Bucket      string
	CDNURL        string
	S3Uploader    *s3.S3
}

type Server struct {
	config     *Config
	cache      *AssetCache
	httpClient *http.Client
}

var hatKeyPattern = regexp.MustCompile(`^hat_\d+$`)

func NewDefaultUserConfig() UserConfig {
	return UserConfig{
		BodyParts: BodyParts{
			Head:     "cranium",
			Torso:    "chesticle",
			LeftArm:  "arm_left",
			RightArm: "arm_right",
			LeftLeg:  "leg_left",
			RightLeg: "leg_right",
			ToolArm:  "arm_tool",
		},
		Items: ItemsCollection{
			Hats:   make(map[string]ItemData),
			Face:   ItemData{Item: "none"},
			Addon:  ItemData{Item: "none"},
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
}

func getTextureHash(itemData ItemData) string {
	if itemData.EditStyle != nil && itemData.EditStyle.IsTexture {
		return itemData.EditStyle.Hash
	}
	return itemData.Item
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	rootDir := getEnv("RENDERER_ROOT_DIR", "/var/www/renderer")
	_ = godotenv.Load(path.Join(rootDir, ".env"))

	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(os.Getenv("S3_ACCESS_KEY"), os.Getenv("S3_SECRET_KEY"), ""),
		Endpoint:         aws.String(os.Getenv("S3_ENDPOINT")),
		Region:           aws.String(os.Getenv("S3_REGION")),
		S3ForcePathStyle: aws.Bool(true),
	}
	sess, err := session.NewSession(s3Config)
	if err != nil {
		log.Fatalf("Failed to create S3 session: %v", err)
	}

	httpClient := &http.Client{Timeout: 10 * time.Second}

	server := &Server{
		config: &Config{
			PostKey:       os.Getenv("POST_KEY"),
			ServerAddress: os.Getenv("SERVER_ADDRESS"),
			S3Bucket:      os.Getenv("S3_BUCKET"),
			CDNURL:        os.Getenv("CDN_URL"),
			S3Uploader:    s3.New(sess),
		},
		cache:      NewAssetCache(httpClient),
		httpClient: httpClient,
	}

	http.HandleFunc("/", server.handleRender)

	fmt.Printf("Starting server on %s\n", server.config.ServerAddress)
	if err := http.ListenAndServe(server.config.ServerAddress, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

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

	var req RenderRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("Received RenderType: %s | Hash: %s", req.RenderType, req.Hash)

	switch req.RenderType {
	case "user":
		var u UserConfig
		if err := json.Unmarshal(req.RenderJson, &u); err != nil {
			log.Printf("User JSON error: %v", err)
			http.Error(w, "Invalid user render body", http.StatusBadRequest)
			return
		}
		s.handleUserRender(w, req.Hash, u)

	case "item_preview":
		var i ItemConfig
		if err := json.Unmarshal(req.RenderJson, &i); err != nil {
			log.Printf("Item JSON error: %v", err)
			http.Error(w, "Invalid item render body", http.StatusBadRequest)
			return
		}
		s.handleItemPreviewRender(w, r, req.Hash, i)

	case "item":
		var i ItemConfig
		if err := json.Unmarshal(req.RenderJson, &i); err != nil {
			log.Printf("Item Object JSON error: %v", err)
			http.Error(w, "Invalid item render body", http.StatusBadRequest)
			return
		}
		s.handleItemObjectRender(w, r, req.Hash, i)

	default:
		http.Error(w, "Unknown RenderType", http.StatusBadRequest)
	}
}

func (s *Server) handleUserRender(w http.ResponseWriter, hash string, config UserConfig) {
	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		rootNode, _ := s.buildCharacterTree(config, true)
		var objects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &objects)

		buf, err := s.runRenderWithTimeout(objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
		if err != nil {
			log.Printf("User render failed: %v", err)
			return
		}
		_ = s.uploadToS3(context.Background(), buf, path.Join("thumbnails", hash+".png"))
	}()

	go func() {
		defer wg.Done()
		var (
			hsFovy   = 25.5
			hsEye    = aeno.V(4, 7, 13)
			hsCenter = aeno.V(-0.5, 6.8, 0)
			hsUp     = aeno.V(0, 1, 0)
		)
		rootNode, _ := s.buildCharacterTree(config, false) // No tool for headshot
		var objects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &objects)

		buf, err := s.runRenderWithTimeout(objects, hsEye, hsCenter, hsUp, hsFovy, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, false)
		if err != nil {
			log.Printf("Headshot render failed: %v", err)
			return
		}
		_ = s.uploadToS3(context.Background(), buf, path.Join("thumbnails", hash+"_headshot.png"))
	}()

	wg.Wait()
	log.Printf("Completed user render for %s in %v", hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "User render processed.")
}

func (s *Server) handleItemPreviewRender(w http.ResponseWriter, r *http.Request, hash string, i ItemConfig) {
	start := time.Now()

	previewConfig := NewDefaultUserConfig()
	switch i.ItemType {
	case "face":
		previewConfig.Items.Face = i.Item
	case "hat":
		previewConfig.Items.Hats["hat_1"] = i.Item
	case "addon":
		previewConfig.Items.Addon = i.Item
	case "tool":
		previewConfig.Items.Tool = i.Item
	case "pants":
		previewConfig.Items.Pants = i.Item
	case "shirt":
		previewConfig.Items.Shirt = i.Item
	case "tshirt":
		previewConfig.Items.Tshirt = i.Item
	case "head":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.Head = i.Item.Item
		}
	case "torso":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.Torso = i.Item.Item
		}
	case "left_arm":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.LeftArm = i.Item.Item
		}
	case "right_arm":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.RightArm = i.Item.Item
		}
	case "left_leg":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.LeftLeg = i.Item.Item
		}
	case "right_leg":
		if i.Item.Item != "none" {
			previewConfig.BodyParts.RightLeg = i.Item.Item
		}
	}
	rootNode, _ := s.buildCharacterTree(previewConfig, true)

	var objects []*aeno.Object
	rootNode.Flatten(aeno.Identity(), &objects)

	outputKey := path.Join("thumbnails", hash+".png")

	buf, err := s.runRenderWithTimeout(objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
	if err != nil {
		log.Printf("Preview render failed: %v", err)
		http.Error(w, "Render failed", http.StatusGatewayTimeout)
		return
	}

	if err := s.uploadToS3(r.Context(), buf, outputKey); err != nil {
		log.Printf("Preview upload failed: %v", err)
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Item Preview %s finished in %v", hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Preview processed.")
}

func (s *Server) handleItemObjectRender(w http.ResponseWriter, r *http.Request, hash string, i ItemConfig) {
	start := time.Now()

	var rootNode *SceneNode
	switch i.ItemType {
	case "head", "torso", "left_arm", "right_arm", "left_leg", "right_leg", "tool_arm":
		rootNode = s.generateBodyPartObject(i)
	default:
		rootNode = s.generateItemObject(i)
	}

	var objects []*aeno.Object
	rootNode.Flatten(aeno.Identity(), &objects)

	if len(objects) == 0 {
		log.Println("Warning: No objects generated for ItemObjectRender")
	}

	outputKey := path.Join("thumbnails", hash+".png")

	buf, err := s.runRenderWithTimeout(objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
	if err != nil {
		log.Printf("Object render failed: %v", err)
		http.Error(w, "Render failed", http.StatusGatewayTimeout)
		return
	}

	if err := s.uploadToS3(r.Context(), buf, outputKey); err != nil {
		http.Error(w, "Upload failed", http.StatusInternalServerError)
		return
	}

	log.Printf("Item Object %s finished in %v", hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Object render processed.")
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
		err := aeno.GenerateSceneToWriter(&buf, objects, eye, center, up, fovy, dim, scale, light, ambStr, lightColorStr, near, far, fit)
		resChan <- result{data: buf.Bytes(), err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("render timeout")
	case res := <-resChan:
		return res.data, res.err
	}
}

func (s *Server) buildCharacterTree(userConfig UserConfig, includeTool bool) (*SceneNode, bool) {
	cdnURL := s.config.CDNURL
	isToolEquipped := includeTool && userConfig.Items.Tool.Item != "none"

	getMesh := func(hash, defaultName string) *aeno.Mesh {
		if hash == "" || hash == defaultName {
			return s.cache.GetMesh(fmt.Sprintf("%s/assets/%s.glb", cdnURL, defaultName))
		}
		return s.cache.GetMesh(fmt.Sprintf("%s/uploads/%s.obj", cdnURL, hash))
	}

	rootNode := NewSceneNode("Character", nil, aeno.Identity())

	torsoMesh := getMesh(userConfig.BodyParts.Torso, "chesticle")
	if torsoMesh == nil {
		return rootNode, false
	}
	torsoObj := &aeno.Object{
		Mesh:   torsoMesh.Copy(),
		Color:  aeno.HexColor(userConfig.Colors["Torso"]),
		Matrix: aeno.Identity(),
	}
	if userConfig.Items.Shirt.Item != "none" {
		url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
		torsoObj.Texture = s.cache.GetTexture(url)
	}
	torsoNode := NewSceneNode("Torso", torsoObj, aeno.Identity())
	rootNode.AddChild(torsoNode)

	headMesh := getMesh(userConfig.BodyParts.Head, "cranium")
	headMatrix := aeno.Translate(aeno.V(0, 0, 0))
	if headMesh != nil {
		headObj := &aeno.Object{
			Mesh:    headMesh.Copy(),
			Color:   aeno.HexColor(userConfig.Colors["Head"]),
			Texture: s.AddFace(userConfig.Items.Face),
			Matrix:  aeno.Identity(),
		}
		headNode := NewSceneNode("Head", headObj, headMatrix)
		torsoNode.AddChild(headNode)

		for key, hatData := range userConfig.Items.Hats {
			if hatData.Item != "none" {
				if hatObj := s.RenderItem(hatData); hatObj != nil {
					headNode.AddChild(NewSceneNode(key, hatObj, aeno.Identity()))
				}
			}
		}
	}

	legs := []struct{ Key, Default string }{
		{"LeftLeg", "leg_left"}, {"RightLeg", "leg_right"},
	}
	for _, leg := range legs {
		hash := userConfig.BodyParts.LeftLeg
		color := userConfig.Colors["LeftLeg"]
		if leg.Key == "RightLeg" {
			hash = userConfig.BodyParts.RightLeg
			color = userConfig.Colors["RightLeg"]
		}

		mesh := getMesh(hash, leg.Default)
		if mesh != nil {
			legObj := &aeno.Object{Mesh: mesh.Copy(), Color: aeno.HexColor(color), Matrix: aeno.Identity()}
			if userConfig.Items.Pants.Item != "none" {
				url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Pants))
				legObj.Texture = s.cache.GetTexture(url)
			}
			torsoNode.AddChild(NewSceneNode(leg.Key, legObj, aeno.Identity()))
		}
	}

	rArmMesh := getMesh(userConfig.BodyParts.RightArm, "arm_right")
	if rArmMesh != nil {
		rObj := &aeno.Object{Mesh: rArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["RightArm"]), Matrix: aeno.Identity()}
		if userConfig.Items.Shirt.Item != "none" {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
			rObj.Texture = s.cache.GetTexture(url)
		}
		torsoNode.AddChild(NewSceneNode("RightArm", rObj, aeno.Identity()))
	}

	shoulderPos := aeno.V(-2.4342, 5.2510, 0.0132)
	jointMatrix := aeno.Translate(shoulderPos)
	if isToolEquipped && userConfig.Items.Tool.Item != "none" {
		rot := aeno.Rotate(aeno.V(1, 0, 0), aeno.Radians(90))
		jointMatrix = jointMatrix.Mul(rot)
	}
	leftArmNode := NewSceneNode("LeftArm", nil, jointMatrix) // Joint
	torsoNode.AddChild(leftArmNode)
	
	var lArmMesh *aeno.Mesh
		lArmMesh = getMesh(userConfig.BodyParts.LeftArm, "arm_left")

	if lArmMesh != nil {
		lArmObj := &aeno.Object{Mesh: lArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["LeftArm"]), Matrix: aeno.Identity()}
		if userConfig.Items.Shirt.Item != "none" {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, getTextureHash(userConfig.Items.Shirt))
			lArmObj.Texture = s.cache.GetTexture(url)
		}
		meshMatrix := aeno.Translate(shoulderPos.Negate())
		lArmMeshNode := NewSceneNode("LeftArmMesh", lArmObj, meshMatrix)
		leftArmNode.AddChild(lArmMeshNode)

		if isToolEquipped && userConfig.Items.Tool.Item != "none" {
        	if toolObj := s.RenderItem(userConfig.Items.Tool); toolObj != nil {
            	torsoNode.AddChild(NewSceneNode("Tool", toolObj, aeno.Identity()))
        	}
    	}
	}

	if userConfig.Items.Tshirt.Item != "none" {
		teeHash := getTextureHash(userConfig.Items.Tshirt)
		teeMesh := s.cache.GetMesh(fmt.Sprintf("%s/assets/tee.glb", cdnURL))
		if teeMesh != nil {
			url := fmt.Sprintf("%s/uploads/%s.png", cdnURL, teeHash)
			teeObj := &aeno.Object{Mesh: teeMesh.Copy(), Color: aeno.Transparent, Texture: s.cache.GetTexture(url), Matrix: aeno.Identity()}
			torsoNode.AddChild(NewSceneNode("Tshirt", teeObj, aeno.Identity()))
		}
	}

	if obj := s.RenderItem(userConfig.Items.Addon); obj != nil {
		torsoNode.AddChild(NewSceneNode("Addon", obj, aeno.Identity()))
	}

	return rootNode, isToolEquipped
}

func (s *Server) RenderItem(itemData ItemData) *aeno.Object {
	if itemData.Item == "none" || itemData.Item == "" {
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
		log.Printf("Warning: Could not render item %s", meshURL)
		return nil
	}

	return &aeno.Object{
		Mesh:    finalMesh.Copy(),
		Color:   aeno.Transparent,
		Texture: s.cache.GetTexture(textureURL),
		Matrix:  aeno.Identity(),
	}
}

func (s *Server) AddFace(faceData ItemData) aeno.Texture {
	faceURL := ""
	if faceData.Item != "none" && faceData.Item != "" {
		faceHash := getTextureHash(faceData)
		faceURL = fmt.Sprintf("%s/uploads/%s.png", s.config.CDNURL, faceHash)
	} else {
		faceURL = fmt.Sprintf("%s/assets/default.png", s.config.CDNURL)
	}
	return s.cache.GetTexture(faceURL)
}

func (s *Server) generateItemObject(config ItemConfig) *SceneNode {
	rootNode := NewSceneNode("ItemRoot", nil, aeno.Identity())
	if config.ItemType == "face" {
		headMesh := s.cache.GetMesh(s.config.CDNURL + "/assets/cranium.glb")
		if headMesh != nil {
			headObj := &aeno.Object{
				Mesh:    headMesh.Copy(),
				Color:   aeno.HexColor("d3d3d3"),
				Texture: s.AddFace(config.Item),
				Matrix:  aeno.Identity(),
			}
			rootNode.AddChild(NewSceneNode("HeadForFace", headObj, aeno.Identity()))
		}
		return rootNode
	}

	if obj := s.RenderItem(config.Item); obj != nil {
		rootNode.AddChild(NewSceneNode("ItemObject", obj, aeno.Identity()))
	}
	return rootNode
}

func (s *Server) generateBodyPartObject(config ItemConfig) *SceneNode {
	rootNode := NewSceneNode("BodyPartRoot", nil, aeno.Identity())
	cdnURL := s.config.CDNURL
	partName := config.Item.Item

	textureURL := fmt.Sprintf("%s/assets/error-texture.png", cdnURL)
	if config.ItemType == "head" {
		textureURL = fmt.Sprintf("%s/assets/default.png", cdnURL)
	}

	meshURL := fmt.Sprintf("%s/uploads/%s.obj", cdnURL, partName)

	mesh := s.cache.GetMesh(meshURL)
	if mesh != nil {
		obj := &aeno.Object{
			Mesh:    mesh.Copy(),
			Color:   aeno.HexColor("d3d3d3"),
			Texture: s.cache.GetTexture(textureURL),
			Matrix:  aeno.Identity(),
		}
		rootNode.AddChild(NewSceneNode("BodyPart", obj, aeno.Identity()))
	}
	return rootNode
}

func (s *Server) uploadToS3(ctx context.Context, data []byte, key string) error {
	ctx, cancel := context.WithTimeout(ctx, UploadTimeout)
	defer cancel()

	size := int64(len(data))
	_, err := s.config.S3Uploader.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.config.S3Bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("image/png"),
		ACL:           aws.String("public-read"),
	})

	if err != nil {
		log.Printf("S3 Upload Error for key %s: %v", key, err)
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}

	log.Printf("Uploaded %s to S3 (%d bytes)", key, size)
	return nil
}

func (c *AssetCache) GetMesh(url string) *aeno.Mesh {
	c.mu.RLock()
	mesh, ok := c.meshes[url]
	c.mu.RUnlock()
	if ok {
		return mesh
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if mesh, ok = c.meshes[url]; ok {
		return mesh
	}

	resp, err := c.httpClient.Get(url)
	if err != nil || resp.StatusCode != 200 {
		log.Printf("Warning: Mesh inaccessible at %s (Status: %v)", url, resp.StatusCode)
		c.meshes[url] = nil
		if resp != nil {
			resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	if path.Ext(url) == ".glb" {
		mesh, _ = aeno.LoadGLTFFromReader(resp.Body)
	} else {
		mesh, _ = aeno.LoadOBJFromReader(resp.Body)
	}
	c.meshes[url] = mesh
	return mesh
}

func (c *AssetCache) GetTexture(url string) aeno.Texture {
	c.mu.RLock()
	tex, ok := c.textures[url]
	c.mu.RUnlock()
	if ok {
		return tex
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if tex, ok = c.textures[url]; ok {
		return tex
	}

	tex = aeno.LoadTextureFromURL(url)
	c.textures[url] = tex
	return tex
}
