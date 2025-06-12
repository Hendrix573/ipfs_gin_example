package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"ipfs-gin-example/pkg/merkledag"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

// DownloadHandler handles all download-related API operations.
type DownloadHandler struct {
	DAGBuilder *merkledag.DAGBuilder
	Resolver   *resolver.Resolver
}

var customCIDRegex = regexp.MustCompile("^[0-9a-fA-F]{64}$")

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

	var rootCID string
	var targetNodeCID string

	if customCIDRegex.MatchString(domain) {
		var err error
		// 解析成功，paramInput 是一个合法的 CID
		rootCID = domain // 直接将输入的 CID 作为根 CID
		targetNodeCID, err = h.DAGBuilder.ResolvePath(rootCID, path)
		if err != nil {
			// Path not found or other resolution error
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Path '%s' not found under CID %s: %v", path, rootCID, err)})
			return
		}
	} else {
		// check cache
		var ok bool
		targetNodeCID, ok = h.Resolver.GetCache(domain, path)
		if !ok {
			rootCID, err := h.Resolver.ResolveDomain(domain)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Failed to resolve CID for %s: %v", domain, err)})
				return
			}

			targetNodeCID, err = h.DAGBuilder.ResolvePath(rootCID, path)
			if err != nil {
				// Path not found or other resolution error
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Path '%s' not found under CID %s: %v", path, rootCID, err)})
				return
			}
		}
	}

	targetNode, err := h.DAGBuilder.GetNode(targetNodeCID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve node: %v", err)})
		return
	}

	if len(targetNode.Data) == 0 && len(targetNode.Links) > 0 && targetNode.Links[0].Name != "" {
		links, err := h.DAGBuilder.ListDirectory(targetNodeCID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list directory: %v", err)})
			return
		}
		c.HTML(http.StatusOK, "directory_listing.tmpl", gin.H{
			"Path":    path,
			"Links":   links,
			"BaseURL": fmt.Sprintf("/%s%s/", domain, path),
		})
		return
	} else {
		// Assume it's a file or a file chunk (node with Data or a node linking to unnamed chunks)
		fileData, err := h.DAGBuilder.GetFileData(targetNodeCID)
		if err != nil {
			// Check if the error indicates it wasn't a file node structure
			if strings.Contains(err.Error(), "is not a data chunk") || strings.Contains(err.Error(), "unexpected structure") {
				// It was a node, but not structured like a file we can read
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Node %s is not a readable file structure: %v", targetNodeCID, err)})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file data for node %s: %v", targetNodeCID, err)})
			}
			return
		}

		// Determine content type (basic guess based on path extension)
		contentType := "application/octet-stream"
		// Get filename from path for extension check
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
		case ".gif":
			contentType = "image/gif"
		case ".pdf":
			contentType = "application/pdf"
		case ".zip":
			contentType = "application/zip"
		case ".tar":
			contentType = "application/x-tar"
		case ".gz":
			contentType = "application/gzip"
			// Add more types as needed
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Data(http.StatusOK, contentType, fileData)
		return
	}
}
