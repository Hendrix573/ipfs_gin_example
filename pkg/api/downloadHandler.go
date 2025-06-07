package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"ipfs-gin-example/pkg/merkledag"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"
	"net/http"
	"path/filepath"
	"strings"
)

// DownloadHandler handles all download-related API operations.
type DownloadHandler struct {
	DAGBuilder *merkledag.DAGBuilder
	Resolver   *resolver.Resolver
}

// NewDownloadHandler creates a new DownloadHandler.
func NewDownloadHandler(store storage.Store, resolver *resolver.Resolver) *DownloadHandler {
	dagBuilder := merkledag.NewDAGBuilder(store)
	return &DownloadHandler{
		DAGBuilder: dagBuilder,
		Resolver:   resolver,
	}
}

// RegisterRoutes registers download-related routes.
func (h *DownloadHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/:domain/*path", h.DownloadHandler)
}

// DownloadHandler handles content retrieval based on domain and path.
func (h *DownloadHandler) DownloadHandler(c *gin.Context) {
	domain := c.Param("domain")
	path := c.Param("path")

	name := domain + path
	cid, err := h.Resolver.ResolveDomain(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Failed to resolve CID for %s: %v", name, err)})
		return
	}

	targetNode, err := h.DAGBuilder.GetNode(cid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve node %s: %v", cid, err)})
		return
	}

	if len(targetNode.Data) == 0 && len(targetNode.Links) > 0 && targetNode.Links[0].Name != "" {
		links, err := h.DAGBuilder.ListDirectory(cid)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list directory %s: %v", cid, err)})
			return
		}
		c.HTML(http.StatusOK, "directory_listing.tmpl", gin.H{
			"Path":  path,
			"Links": links,
		})
		return
	}

	fileData, err := h.DAGBuilder.GetFileData(cid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file data for %s: %v", cid, err)})
		return
	}

	contentType := "application/octet-stream"
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	switch strings.ToLower(ext) {
	case ".txt":
		contentType = "text/plain"
	case ".html", ".htm":
		contentType = "text/html"
	case ".json":
		contentType = "application/json"
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".pdf":
		contentType = "application/pdf"
	}

	c.Data(http.StatusOK, contentType, fileData)
}
