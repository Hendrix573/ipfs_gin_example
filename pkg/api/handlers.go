package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"ipfs-gin-example/pkg/merkledag"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"

	"github.com/gin-gonic/gin"
)

// Handlers holds dependencies for API handlers
type Handlers struct {
	Store      storage.Store
	Chunker    *merkledag.Chunker
	DAGBuilder *merkledag.DAGBuilder
	Resolver   *resolver.Resolver
}

// NewHandlers creates new API Handlers
func NewHandlers(store storage.Store, chunkSize int, resolver *resolver.Resolver) *Handlers {
	dagBuilder := merkledag.NewDAGBuilder(store)
	return &Handlers{
		Store:      store,
		Chunker:    merkledag.NewChunker(chunkSize),
		DAGBuilder: dagBuilder,
		Resolver:   resolver,
	}
}

// UploadHandler handles single file upload via request body (basic, returns content CID)
// POST /upload
func (h *Handlers) UploadHandler(c *gin.Context) {
	// Read the entire request body
	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body: %v", err)})
		return
	}

	// Create a bytes.Reader to pass to the chunker
	reader := bytes.NewReader(content)

	// Chunk the content
	leaves, err := h.Chunker.Chunk(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk content: %v", err)})
		return
	}

	// Build the DAG from chunks and store nodes
	rootCID, size, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cid": rootCID, "size": size})
}

// MultipartUploadHandler handles uploading multiple files via multipart form (basic, returns directory CID)
// POST /upload/multipart
func (h *Handlers) MultipartUploadHandler(c *gin.Context) {
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

	for _, fileHeaders := range files {
		// Handle each file field (e.g., multiple files under the same field name)
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file %s: %v", fileHeader.Filename, err)})
				return
			}
			defer file.Close()

			// Chunk the file content
			leaves, err := h.Chunker.Chunk(file)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk file %s: %v", fileHeader.Filename, err)})
				return
			}

			// Build DAG for this file
			fileRootCID, fileSize, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG for file %s: %v", fileHeader.Filename, err)})
				return
			}

			// Use the actual filename from the header
			itemCIDs[fileHeader.Filename] = struct {
				CID  string
				Size uint64
			}{CID: fileRootCID, Size: fileSize}
		}
	}

	// If multiple files were uploaded, create a directory node linking to them
	if len(itemCIDs) > 0 {
		dirRootCID, dirSize, err := h.DAGBuilder.BuildDirectoryDAG(itemCIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build directory DAG: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"directory_cid": dirRootCID, "size": dirSize, "files": itemCIDs})
	} else {
		c.JSON(http.StatusOK, gin.H{"message": "No files uploaded"})
	}
}

// DAGUploadHandler handles pre-built DAG upload (list of nodes + root CID)
// POST /upload/dag
// Expects JSON body: { "root": "root_cid_string", "nodes": [ { ...node1... }, { ...node2... } ] }
func (h *Handlers) DAGUploadHandler(c *gin.Context) {
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
		// Allow uploading just a root CID if the node is already known?
		// Or require at least the root node itself? Let's require nodes for simplicity.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Node list is empty"})
		return
	}

	// Store each node
	storedNodes := make(map[string]bool)
	for _, node := range uploadData.Nodes {
		cid, err := h.DAGBuilder.AddNode(node) // AddNode handles getting CID and storing
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to store node: %v", err)})
			return
		}
		storedNodes[cid] = true
	}

	// Verify the provided root CID was among the stored nodes
	if !storedNodes[uploadData.Root] {
		// Note: In a real system, you might allow adding nodes that link to *already existing* CIDs.
		// This check assumes all nodes in the DAG are being uploaded together.
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provided root CID was not found in the uploaded nodes"})
		return
	}

	// Optional: Get the size of the root object
	rootNode, err := h.DAGBuilder.GetNode(uploadData.Root)
	var rootSize uint64
	if err == nil {
		rootSize = h.DAGBuilder.CalculateNodeSize(rootNode)
	}

	c.JSON(http.StatusOK, gin.H{"root_cid": uploadData.Root, "root_size": rootSize, "stored_node_count": len(storedNodes)})
}

// PutHandler handles putting content at a specific path under a domain.
// PUT /:domain/*path
// Expects file content in the request body.
// Example: PUT /example.com/home/user1/hello.txt
func (h *Handlers) PutHandler(c *gin.Context) {
	domain := c.Param("domain")
	// Gin's *path parameter captures everything after /:domain/
	path := c.Param("path") // e.g., "/home/user1/hello.txt"

	if path == "" || path == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path must include the target file or directory name"})
		return
	}

	// Read the entire request body (the content to be put)
	content, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read request body: %v", err)})
		return
	}

	// 1. Chunk the content and build its DAG
	reader := bytes.NewReader(content)
	leaves, err := h.Chunker.Chunk(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to chunk content: %v", err)})
		return
	}

	// Build the DAG for the content
	contentRootCID, contentSize, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build content DAG: %v", err)})
		return
	}

	// 2. Get the current root CID for the domain
	currentRootCID, found := h.Resolver.GetMapping(domain)
	if !found || currentRootCID == "" {
		// If the domain doesn't exist or has no root CID, initialize it with an empty directory
		emptyDirNode := &merkledag.Node{} // Represents an empty directory
		var addErr error
		currentRootCID, addErr = h.DAGBuilder.AddNode(emptyDirNode)
		if addErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to initialize domain with empty directory: %v", addErr)})
			return
		}
		h.Resolver.UpdateMapping(domain, currentRootCID)
		// log.Printf("Initialized domain '%s' with root CID %s", domain, currentRootCID) // Optional logging
	}

	// 3. Update the DAG path from the current root to link the new content
	// This function recursively builds/updates nodes from the target's parent up to the root.
	newRootCID, err := h.DAGBuilder.PutNodeAtPath(currentRootCID, path, contentRootCID, contentSize)
	if err != nil {
		// This could fail if intermediate path components are files instead of directories, etc.
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to update DAG path '%s': %v", path, err)})
		return
	}

	// 4. Update the domain's mapping in the resolver to the new root CID
	h.Resolver.UpdateMapping(domain, newRootCID)

	// 5. Return the new root CID for the domain and the CID of the content that was put
	c.JSON(http.StatusOK, gin.H{
		"domain":              domain,
		"path":                path,
		"content_cid":         contentRootCID,
		"content_size":        contentSize,
		"new_domain_root_cid": newRootCID,
	})
}

// DownloadHandler handles content retrieval based on domain and path
// GET /:domain/*path
// Example: GET /example.com/my/file.txt
func (h *Handlers) DownloadHandler(c *gin.Context) {
	domain := c.Param("domain")
	// Gin's *path parameter includes the leading slash
	path := c.Param("path") // e.g., "/my/file.txt" or "/"

	// 1. Resolve domain to root CID
	rootCID, err := h.Resolver.ResolveDomain(domain)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Domain '%s' not found: %v", domain, err)})
		return
	}

	// 2. Resolve path from root CID to the final node CID
	targetNodeCID, err := h.DAGBuilder.ResolvePath(rootCID, path)
	if err != nil {
		// Path not found or other resolution error
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Path '%s' not found under CID %s: %v", path, rootCID, err)})
		return
	}

	// 3. Get the target node
	targetNode, err := h.DAGBuilder.GetNode(targetNodeCID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve target node %s: %v", targetNodeCID, err)})
		return
	}

	// 4. Determine if it's a file or directory and serve accordingly
	// Check if it's a directory node (no Data, has Links, Links have Names)
	if len(targetNode.Data) == 0 && len(targetNode.Links) > 0 && targetNode.Links[0].Name != "" {
		// It's a directory listing
		links, listErr := h.DAGBuilder.ListDirectory(targetNodeCID)
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to list directory %s: %v", targetNodeCID, listErr)})
			return
		}
		// Render a simple HTML directory listing
		c.HTML(http.StatusOK, "directory_listing.tmpl", gin.H{
			"Path":  path,
			"Links": links,
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

		c.Data(http.StatusOK, contentType, fileData)
		return
	}
}

/*
// CalculateNodeSize helper - moved to DAGBuilder
func (h *Handlers) CalculateNodeSize(node *merkledag.Node) uint64 {
	return h.DAGBuilder.CalculateNodeSize(node) // Call the method on the DAGBuilder instance
}
*/
