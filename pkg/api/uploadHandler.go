package api

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"

	"ipfs-gin-example/config"
	"ipfs-gin-example/pkg/merkledag"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

// UploadHandler handles all upload-related API operations.
type UploadHandler struct {
	Store      storage.Store
	Chunker    *merkledag.Chunker
	DAGBuilder *merkledag.DAGBuilder
	Resolver   *resolver.Resolver
	Config     *config.Config
}

// NewUploadHandler creates a new UploadHandler.
func NewUploadHandler(store storage.Store, chunkSize int, resolver *resolver.Resolver, cfg *config.Config) *UploadHandler {
	dagBuilder := merkledag.NewDAGBuilder(store)
	return &UploadHandler{
		Store:      store,
		Chunker:    merkledag.NewChunker(chunkSize),
		DAGBuilder: dagBuilder,
		Resolver:   resolver,
		Config:     cfg,
	}
}

// RegisterRoutes registers upload-related routes.
func (h *UploadHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/upload", h.UploadHandler)
	group.POST("/upload/multipart", h.MultipartUploadHandler)
	group.POST("/upload/dag", h.DAGUploadHandler)
	group.PUT("/:domain/*path", h.PutHandler)
}

// UploadHandler handles single file upload via request body.
func (h *UploadHandler) UploadHandler(c *gin.Context) {
	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body: %v", err)})
		return
	}

	reader := bytes.NewReader(content)
	leaves, err := h.Chunker.Chunk(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk content: %v", err)})
		return
	}

	rootCID, size, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG: %v", err)})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = fmt.Sprintf("file-%s", rootCID[:8])
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(h.Config.PrivateKey, "0x"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid private key: %v", err)})
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(h.Config.ChainID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to prepare transaction: %v", err)})
		return
	}

	err = h.Resolver.UpdateMapping(auth, name, rootCID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID: %v", err)})
		return
	}

	log.Printf("Registered/Updated CID %s for name %s", rootCID, name)
	c.JSON(http.StatusOK, gin.H{"cid": rootCID, "size": size, "name": name})
}

// MultipartUploadHandler handles uploading multiple files via multipart form.
func (h *UploadHandler) MultipartUploadHandler(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse multipart form: %v", err)})
		return
	}

	files := form.File
	itemCIDs := make(map[string]struct {
		CID  string
		Size uint64
	})

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(h.Config.PrivateKey, "0x"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid private key: %v", err)})
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(h.Config.ChainID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to prepare transaction: %v", err)})
		return
	}

	for _, fileHeaders := range files {
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file %s: %v", fileHeader.Filename, err)})
				return
			}
			defer file.Close()

			leaves, err := h.Chunker.Chunk(file)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk file %s: %v", fileHeader.Filename, err)})
				return
			}

			fileRootCID, fileSize, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG for file %s: %v", fileHeader.Filename, err)})
				return
			}

			name := fileHeader.Filename
			err = h.Resolver.UpdateMapping(auth, name, fileRootCID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID for %s: %v", name, err)})
				return
			}

			itemCIDs[fileHeader.Filename] = struct {
				CID  string
				Size uint64
			}{CID: fileRootCID, Size: fileSize}
		}
	}

	if len(itemCIDs) > 0 {
		dirRootCID, dirSize, err := h.DAGBuilder.BuildDirectoryDAG(itemCIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build directory DAG: %v", err)})
			return
		}

		name := c.Query("name")
		if name == "" {
			name = fmt.Sprintf("dir-%s", dirRootCID[:8])
		}

		err = h.Resolver.UpdateMapping(auth, name, dirRootCID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update directory CID: %v", err)})
			return
		}

		c.JSON(http.StatusOK, gin.H{"directory_cid": dirRootCID, "size": dirSize, "files": itemCIDs, "name": name})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "No files uploaded"})
	}
}

// DAGUploadHandler handles pre-built DAG upload.
func (h *UploadHandler) DAGUploadHandler(c *gin.Context) {
	var uploadData struct {
		Root  string            `json:"root"`
		Nodes []*merkledag.Node `json:"nodes"`
	}

	if err := c.BindJSON(&uploadData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to parse request body: %v", err)})
		return
	}

	if uploadData.Root == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Root CID is required"})
		return
	}
	if len(uploadData.Nodes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node list is empty"})
		return
	}

	storedNodes := make(map[string]bool)
	for _, node := range uploadData.Nodes {
		cid, err := h.DAGBuilder.AddNode(node)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to store node: %v", err)})
			return
		}
		storedNodes[cid] = true
	}

	if !storedNodes[uploadData.Root] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provided root CID was not found in the uploaded nodes"})
		return
	}

	name := c.Query("name")
	if name == "" {
		name = fmt.Sprintf("dag-%s", uploadData.Root[:8])
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(h.Config.PrivateKey, "0x"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid private key: %v", err)})
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(h.Config.ChainID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to prepare transaction: %v", err)})
		return
	}

	err = h.Resolver.UpdateMapping(auth, name, uploadData.Root)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID: %v", err)})
		return
	}

	rootNode, err := h.DAGBuilder.GetNode(uploadData.Root)
	var rootSize uint64
	if err == nil {
		rootSize = h.DAGBuilder.CalculateNodeSize(rootNode)
	}

	c.JSON(http.StatusOK, gin.H{"root_cid": uploadData.Root, "root_size": rootSize, "stored_node_count": len(storedNodes), "name": name})
}

// PutHandler handles putting content at a specific path under a domain.
func (h *UploadHandler) PutHandler(c *gin.Context) {
	domain := c.Param("domain")
	path := c.Param("path")

	if path == "" || path == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must include the target file or directory name"})
		return
	}

	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body: %v", err)})
		return
	}

	reader := bytes.NewReader(content)
	leaves, err := h.Chunker.Chunk(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk content: %v", err)})
		return
	}

	rootCID, size, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG: %v", err)})
		return
	}

	name := domain + path
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(h.Config.PrivateKey, "0x"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Invalid private key: %v", err)})
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(h.Config.ChainID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to prepare transaction: %v", err)})
		return
	}

	err = h.Resolver.UpdateMapping(auth, name, rootCID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cid": rootCID, "size": size, "name": name})
}
