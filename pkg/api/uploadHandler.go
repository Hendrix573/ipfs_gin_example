package api

import (
	"bytes"
	"fmt"
	"io"
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
	group.PUT("/:domain/*path", h.PutHandler)
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

	contentRootCID, contentSize, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	// add to cache
	h.Resolver.AddCache(domain, path, contentRootCID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG: %v", err)})
		return
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
	// get domain cid
	currentRootCID, err := h.Resolver.ResolveDomain(domain)
	if currentRootCID == "" {
		// If the domain doesn't exist or has no root CID, initialize it with an empty directory
		emptyDirNode := &merkledag.Node{} // Represents an empty directory
		var addErr error
		currentRootCID, addErr = h.DAGBuilder.AddNode(emptyDirNode)
		if addErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize domain with empty directory: %v", addErr)})
			return
		}
		err := h.Resolver.UpdateMapping(auth, domain, currentRootCID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID: %v", err)})
			return
		}
		// log.Printf("Initialized domain '%s' with root CID %s", domain, currentRootCID) // Optional logging
	}
	// 构建path的dag
	newRootCID, err := h.DAGBuilder.PutNodeAtPath(currentRootCID, path, contentRootCID, contentSize)
	if err != nil {
		// This could fail if intermediate path components are files instead of directories, etc.
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to update DAG path '%s': %v", path, err)})
		return
	}
	err = h.Resolver.UpdateMapping(auth, domain, newRootCID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register/update CID: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"domain":              domain,
		"path":                path,
		"content_cid":         contentRootCID,
		"content_size":        contentSize,
		"new_domain_root_cid": newRootCID,
	})
}
