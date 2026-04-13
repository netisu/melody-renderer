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

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/joho/godotenv"
	"github.com/netisu/aeno"
)

const (
	Scale         = 4
	FovY          = 15
	Near          = 1
	Far           = 10
	AmbColor      = "#b0b0b0"
	LightColor    = "#808080"
	Dimensions    = 512
	RenderTimeout = 20 * time.Second
	UploadTimeout = 10 * time.Second
)

var (
	eye    = aeno.V(0.75, 0.85, 2)
	center = aeno.V(0, 0.06, 0)
	up     = aeno.V(0, 1, 0)
	light  = aeno.V(-1, 3, 1).Normalize()
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

type CachedMesh struct {
	Mesh   *aeno.Mesh
	Matrix aeno.Matrix
}

type AssetCache struct {
	mu       sync.RWMutex
	meshes   map[string]CachedMesh
	textures map[string]aeno.Texture
	s3Client *s3.Client
	bucket   string
}

func NewAssetCache(s3Client *s3.Client, bucket string) *AssetCache {
	return &AssetCache{
		meshes:   make(map[string]CachedMesh),
		textures: make(map[string]aeno.Texture),
		s3Client: s3Client,
		bucket:   bucket,
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

func (n *SceneNode) Flatten(parentMatrix aeno.Matrix, objects *[]*aeno.Object, filter func(name string) bool) {
	if filter != nil && filter(n.Name) {
		return
	}
	worldMatrix := parentMatrix.Mul(n.LocalMatrix)
	if n.Object != nil {
		obj := *n.Object
		obj.Matrix = worldMatrix.Mul(obj.Matrix)
		*objects = append(*objects, &obj)
	}
	for _, child := range n.Children {
		child.Flatten(worldMatrix, objects, filter)
	}
}

type Config struct {
	PostKey       string
	ServerAddress string
	S3Bucket      string
	CDNURL        string
	S3Client      *s3.Client
}

type Server struct {
	config *Config
	cache  *AssetCache
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

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           os.Getenv("S3_ENDPOINT"),
			SigningRegion: os.Getenv("S3_REGION"),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(os.Getenv("S3_REGION")),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			os.Getenv("S3_ACCESS_KEY"),
			os.Getenv("S3_SECRET_KEY"),
			"",
		)),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		log.Fatalf("Failed to load AWS v2 config: %v", err)
	}

	s3Client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	bucketName := os.Getenv("S3_BUCKET")
	server := &Server{
		config: &Config{
			PostKey:       os.Getenv("POST_KEY"),
			ServerAddress: os.Getenv("SERVER_ADDRESS"),
			S3Bucket:      bucketName,
			S3Client:      s3Client,
		},
		cache: NewAssetCache(s3Client, bucketName),
	}

	http.HandleFunc("/", server.handleRender)

	fmt.Printf("Starting server on %s\n", server.config.ServerAddress)
	if err := http.ListenAndServe(server.config.ServerAddress, nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	c := r.Context()
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

	case "item":
		var i ItemConfig
		if err := json.Unmarshal(req.RenderJson, &i); err != nil {
			log.Printf("Item Object JSON error: %v", err)
			http.Error(w, "Invalid item render body", http.StatusBadRequest)
			return
		}

		switch i.ItemType {
		case "pants", "shirt", "tshirt":
			s.handleItemPreviewRender(c, w, r, req.Hash, i)
		default:
			s.handleItemObjectRender(c, w, r, req.Hash, i)
		}

	default:
		http.Error(w, "Unknown RenderType", http.StatusBadRequest)
	}
}

func (s *Server) handleUserRender(w http.ResponseWriter, hash string, config UserConfig) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), RenderTimeout)
	defer cancel()

	rootNode, _ := s.buildCharacterTree(ctx, config, true)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		var avatarObjects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &avatarObjects, nil)
		buf, err := s.runRenderWithContext(ctx, avatarObjects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
		if err == nil {
			_ = s.uploadToS3(ctx, buf, path.Join("avatars", hash+".png"))
		}
	}()

	go func() {
		defer wg.Done()
		var headshotObjects []*aeno.Object
		rootNode.Flatten(aeno.Identity(), &headshotObjects, func(name string) bool {
			return name == "Tool"
		})
		var (
			hsFovY   = 25.5
			hsEye    = aeno.V(4.5, 11, 13)
			hsCenter = aeno.V(-0.5, 6.8, 0)
			hsUp     = aeno.V(0, 4, 0)
		)

		buf, err := s.runRenderWithContext(ctx, headshotObjects, hsEye, hsCenter, hsUp, hsFovY, Dimensions, Scale, light, AmbColor, LightColor, 0.1, 1000, false)
		if err == nil {
			_ = s.uploadToS3(ctx, buf, path.Join("thumbnails", hash+"_headshot.png"))
		}
	}()

	wg.Wait()
	log.Printf("Completed user render for %s in %v", hash, time.Since(start))
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleItemPreviewRender(ctx context.Context, w http.ResponseWriter, r *http.Request, hash string, i ItemConfig) {
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
	rootNode, _ := s.buildCharacterTree(ctx, previewConfig, true)

	var objects []*aeno.Object
	rootNode.Flatten(aeno.Identity(), &objects, nil)

	outputKey := path.Join("thumbnails", hash+".png")

	buf, err := s.runRenderWithContext(ctx, objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
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

func (s *Server) handleItemObjectRender(c context.Context, w http.ResponseWriter, r *http.Request, hash string, i ItemConfig) {
	start := time.Now()

	var rootNode *SceneNode
	switch i.ItemType {
	case "head", "torso", "left_arm", "right_arm", "left_leg", "right_leg", "tool_arm":
		rootNode = s.generateBodyPartObject(c, i)
	default:
		rootNode = s.generateItemObject(c, i)
	}

	var objects []*aeno.Object
	rootNode.Flatten(aeno.Identity(), &objects, nil)

	if len(objects) == 0 {
		log.Println("Warning: No objects generated for ItemObjectRender")
	}

	outputKey := path.Join("thumbnails", hash+".png")

	buf, err := s.runRenderWithContext(c, objects, eye, center, up, FovY, Dimensions, Scale, light, AmbColor, LightColor, Near, Far, true)
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

func (s *Server) runRenderWithContext(
	ctx context.Context,
	objects []*aeno.Object,
	eye, center, up aeno.Vector,
	fovy float64,
	dim, scale int,
	light aeno.Vector,
	ambStr, lightColorStr string,
	near, far float64,
	fit bool,
) ([]byte, error) {

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
		return nil, ctx.Err()
	case res := <-resChan:
		return res.data, res.err
	}
}

func (s *Server) buildCharacterTree(ctx context.Context, userConfig UserConfig, includeTool bool) (*SceneNode, bool) {
	isToolEquipped := includeTool && userConfig.Items.Tool.Item != "none"

	getMesh := func(hash, defaultName string) (*aeno.Mesh, aeno.Matrix) {
		if hash == "" || hash == defaultName {
			return s.cache.GetMesh(ctx, fmt.Sprintf("assets/%s.glb", defaultName))
		}
		return s.cache.GetMesh(ctx, fmt.Sprintf("uploads/%s.obj", hash))
	}

	rootNode := NewSceneNode("Character", nil, aeno.Identity())

	torsoMesh, torsoMatrix := getMesh(userConfig.BodyParts.Torso, "chesticle")
	if torsoMesh == nil {
		return rootNode, false
	}
	torsoObj := &aeno.Object{
		Mesh:   torsoMesh.Copy(),
		Color:  aeno.HexColor(userConfig.Colors["Torso"]),
		Matrix: torsoMatrix,
	}
	if userConfig.Items.Shirt.Item != "none" {
		key := fmt.Sprintf("uploads/%s.png", getTextureHash(userConfig.Items.Shirt))
		torsoObj.Texture = s.cache.GetTexture(ctx, key)
	}
	torsoNode := NewSceneNode("Torso", torsoObj, aeno.Identity())
	rootNode.AddChild(torsoNode)

	headMesh, headMatrix := getMesh(userConfig.BodyParts.Head, "cranium")
	if headMesh != nil {
		headObj := &aeno.Object{
			Mesh:    headMesh.Copy(),
			Color:   aeno.HexColor(userConfig.Colors["Head"]),
			Texture: s.AddFace(ctx, userConfig.Items.Face),
			Matrix:  headMatrix,
		}
		headNode := NewSceneNode("Head", headObj, aeno.Identity())
		torsoNode.AddChild(headNode)

		for key, hatData := range userConfig.Items.Hats {
			if hatData.Item != "none" {
				if hatObj := s.RenderItem(ctx, hatData); hatObj != nil {
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

		mesh, meshMatrix := getMesh(hash, leg.Default)
		if mesh != nil {
			legObj := &aeno.Object{Mesh: mesh.Copy(), Color: aeno.HexColor(color), Matrix: meshMatrix}
			if userConfig.Items.Pants.Item != "none" {
				key := fmt.Sprintf("uploads/%s.png", getTextureHash(userConfig.Items.Pants))
				legObj.Texture = s.cache.GetTexture(ctx, key)
			}
			torsoNode.AddChild(NewSceneNode(leg.Key, legObj, aeno.Identity()))
		}
	}

	rArmMesh, rArmMatrix := getMesh(userConfig.BodyParts.RightArm, "arm_right")
	if rArmMesh != nil {
		rObj := &aeno.Object{Mesh: rArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["RightArm"]), Matrix: rArmMatrix}
		if userConfig.Items.Shirt.Item != "none" {
			key := fmt.Sprintf("uploads/%s.png", getTextureHash(userConfig.Items.Shirt))
			rObj.Texture = s.cache.GetTexture(ctx, key)
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
	var lArmMatrix aeno.Matrix
	lArmMesh, lArmMatrix = getMesh(userConfig.BodyParts.LeftArm, "arm_left")

	if lArmMesh != nil {
		lArmObj := &aeno.Object{Mesh: lArmMesh.Copy(), Color: aeno.HexColor(userConfig.Colors["LeftArm"]), Matrix: lArmMatrix}
		if userConfig.Items.Shirt.Item != "none" {
			key := fmt.Sprintf("uploads/%s.png", getTextureHash(userConfig.Items.Shirt))
			lArmObj.Texture = s.cache.GetTexture(ctx, key)
		}
		meshMatrix := aeno.Translate(shoulderPos.Negate())
		lArmMeshNode := NewSceneNode("LeftArmMesh", lArmObj, meshMatrix)
		leftArmNode.AddChild(lArmMeshNode)

		if isToolEquipped && userConfig.Items.Tool.Item != "none" {
			if toolObj := s.RenderItem(ctx, userConfig.Items.Tool); toolObj != nil {
				torsoNode.AddChild(NewSceneNode("Tool", toolObj, aeno.Identity()))
			}
		}
	}

	if userConfig.Items.Tshirt.Item != "none" {
		teeHash := getTextureHash(userConfig.Items.Tshirt)
		teeMesh, teeMatrix := s.cache.GetMesh(ctx, path.Join("assets", "tee.glb"))
		if teeMesh != nil {
			key := fmt.Sprintf("uploads/%s.png", teeHash)
			teeObj := &aeno.Object{Mesh: teeMesh.Copy(), Color: aeno.Transparent, Texture: s.cache.GetTexture(ctx, key), Matrix: teeMatrix}
			torsoNode.AddChild(NewSceneNode("Tshirt", teeObj, aeno.Identity()))
		}
	}

	if obj := s.RenderItem(ctx, userConfig.Items.Addon); obj != nil {
		torsoNode.AddChild(NewSceneNode("Addon", obj, aeno.Identity()))
	}

	return rootNode, isToolEquipped
}

func (s *Server) RenderItem(ctx context.Context, itemData ItemData) *aeno.Object {
	if itemData.Item == "none" || itemData.Item == "" {
		return nil
	}

	meshKey := fmt.Sprintf("uploads/%s.obj", itemData.Item)
	textureKey := fmt.Sprintf("uploads/%s.png", itemData.Item)

	if itemData.EditStyle != nil {
		if itemData.EditStyle.IsModel {
			meshKey = fmt.Sprintf("uploads/%s.obj", itemData.EditStyle.Hash)
		}
		if itemData.EditStyle.IsTexture {
			textureKey = fmt.Sprintf("uploads/%s.png", itemData.EditStyle.Hash)
		}
	}

	finalMesh, finalMatrix := s.cache.GetMesh(ctx, meshKey)

	if finalMesh == nil {
		log.Printf("Warning: Could not render item %s", meshKey)
		return nil
	}

	return &aeno.Object{
		Mesh:    finalMesh.Copy(),
		Color:   aeno.Transparent,
		Texture: s.cache.GetTexture(ctx, textureKey),
		Matrix:  finalMatrix,
	}
}

func (s *Server) AddFace(ctx context.Context, faceData ItemData) aeno.Texture {
	faceKey := "assets/default.png"
	if faceData.Item != "none" && faceData.Item != "" {
		faceHash := getTextureHash(faceData)
		faceKey = fmt.Sprintf("uploads/%s.png", faceHash)
	}
	return s.cache.GetTexture(ctx, faceKey)
}

func (s *Server) generateItemObject(ctx context.Context, config ItemConfig) *SceneNode {
	rootNode := NewSceneNode("ItemRoot", nil, aeno.Identity())
	if config.ItemType == "face" {
		headMesh, headMatrix := s.cache.GetMesh(ctx, "assets/cranium.glb")
		if headMesh != nil {
			headObj := &aeno.Object{
				Mesh:    headMesh.Copy(),
				Color:   aeno.HexColor("d3d3d3"),
				Texture: s.AddFace(ctx, config.Item),
				Matrix:  headMatrix,
			}
			rootNode.AddChild(NewSceneNode("HeadForFace", headObj, aeno.Identity()))
		}
		return rootNode
	}

	if obj := s.RenderItem(ctx, config.Item); obj != nil {
		rootNode.AddChild(NewSceneNode("ItemObject", obj, aeno.Identity()))
	}
	return rootNode
}

func (s *Server) generateBodyPartObject(ctx context.Context, config ItemConfig) *SceneNode {
	rootNode := NewSceneNode("BodyPartRoot", nil, aeno.Identity())

	textureURL := "assets/error-texture.png"
	if config.ItemType == "head" {
		textureURL = "assets/default.png"
	}

	meshURL := fmt.Sprintf("uploads/%s.obj", config.Item.Item)

	mesh, meshMatrix := s.cache.GetMesh(ctx, meshURL)
	if mesh != nil {
		obj := &aeno.Object{
			Mesh:    mesh.Copy(),
			Color:   aeno.HexColor("d3d3d3"),
			Texture: s.cache.GetTexture(ctx, textureURL),
			Matrix:  meshMatrix,
		}
		rootNode.AddChild(NewSceneNode("BodyPart", obj, aeno.Identity()))
	}
	return rootNode
}

func (s *Server) uploadToS3(ctx context.Context, data []byte, key string) error {
	ctx, cancel := context.WithTimeout(ctx, UploadTimeout)
	defer cancel()

	size := int64(len(data))
	_, err := s.config.S3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.config.S3Bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String("image/png"),
		ACL:           types.ObjectCannedACLPublicRead,
	})

	if err != nil {
		log.Printf("S3 Upload Error for key %s: %v", key, err)
		return fmt.Errorf("failed to upload %s: %w", key, err)
	}

	log.Printf("Uploaded %s to S3 (%d bytes)", key, size)
	return nil
}

func (c *AssetCache) GetMesh(ctx context.Context, key string) (*aeno.Mesh, aeno.Matrix) {
	c.mu.RLock()
	cached, ok := c.meshes[key]
	c.mu.RUnlock()
	if ok {
		return cached.Mesh, cached.Matrix
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok = c.meshes[key]; ok {
		return cached.Mesh, cached.Matrix
	}

	req, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Printf("Warning: Mesh inaccessible at S3 key %s (Error: %v)", key, err)
		c.meshes[key] = CachedMesh{nil, aeno.Identity()}
		return nil, aeno.Identity()
	}
	defer req.Body.Close()

	var mesh *aeno.Mesh
	matrix := aeno.Identity()

	ext := path.Ext(key)
	if ext == ".glb" {
		mesh, matrix, _ = aeno.LoadGLTFFromReader(req.Body)
	} else {
		mesh, _ = aeno.LoadOBJFromReader(req.Body)
	}

	c.meshes[key] = CachedMesh{mesh, matrix}
	return mesh, matrix
}

func (c *AssetCache) GetTexture(ctx context.Context, key string) aeno.Texture {
	c.mu.RLock()
	tex, ok := c.textures[key]
	c.mu.RUnlock()
	if ok {
		return tex
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if tex, ok = c.textures[key]; ok {
		return tex
	}

	req, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Printf("Warning: Texture inaccessible at S3 key %s", key)
		c.textures[key] = nil
		return nil
	}
	defer req.Body.Close()

	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil
	}

	tex = aeno.TexFromBytes(data)
	c.textures[key] = tex
	return tex
}
