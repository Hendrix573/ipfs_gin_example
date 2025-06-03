package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

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

// UploadHandler handles single file upload via request body
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
	rootCID, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"cid": rootCID})
}

// MultipartUploadHandler handles uploading multiple files via multipart form
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
			fileRootCID, err := h.DAGBuilder.BuildDAGFromLeaves(leaves)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build DAG for file %s: %v", fileHeader.Filename, err)})
				return
			}

			// Store the file's root CID and size (header size is not DAG size, need to calculate)
			// For simplicity now, let's use fileHeader.Size, but a real system needs DAG size calculation
			// Let's calculate the actual DAG size for the file
			fileRootNode, getNodeErr := h.DAGBuilder.GetNode(fileRootCID)
			if getNodeErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file root node %s for size calc: %v", fileRootCID, getNodeErr)})
				return
			}
			fileSize := h.DAGBuilder.CalculateNodeSize(fileRootNode) // Use the helper

			// Use the actual filename from the header
			itemCIDs[fileHeader.Filename] = struct {
				CID  string
				Size uint64
			}{CID: fileRootCID, Size: fileSize}
		}
	}

	// If multiple files were uploaded, create a directory node linking to them
	if len(itemCIDs) > 0 {
		dirRootCID, err := h.DAGBuilder.BuildDirectoryDAG(itemCIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build directory DAG: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"directory_cid": dirRootCID, "files": itemCIDs})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provided root CID was not found in the uploaded nodes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"root_cid": uploadData.Root, "stored_node_count": len(storedNodes)})
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
	if len(targetNode.Data) > 0 || (len(targetNode.Links) > 0 && targetNode.Links[0].Name == "") {
		// It's likely a file node (either single chunk with Data or multi-chunk with unnamed links)
		// Let's assume nodes with Data or links with no names are file data chunks/references.
		// A more robust system would distinguish file nodes from raw data nodes explicitly.
		// In our simple model, a file built from chunks has a root node with links to chunks.
		// A single-chunk file *could* be just the chunk node itself.
		// Let's refine: If the resolved node has Data, serve it directly (single chunk).
		// If it has Links, and the path was to a *file*, those links point to chunks.
		// If the resolved node has Links and the path was to a *directory*, list the links.

		// Let's assume the ResolvePath returns the CID of the *final object* (file or directory).
		// If the final object is a file, its node structure depends on chunking:
		// - Single chunk: Node with Data, no Links.
		// - Multi chunk: Node with Links (to chunk nodes), no Data.
		// If the final object is a directory: Node with Links (to file/dir nodes), no Data.

		// Check if it's a directory node (no Data, has Links, Links have Names)
		if len(targetNode.Data) == 0 && len(targetNode.Links) > 0 && targetNode.Links[0].Name != "" {
			// It's a directory listing
			links := targetNode.Links
			// Render a simple HTML directory listing
			c.HTML(http.StatusOK, "directory_listing.tmpl", gin.H{
				"Path":  path,
				"Links": links,
			})
			return
		} else {
			// Assume it's a file or a file chunk
			fileData, err := h.DAGBuilder.GetFileData(targetNodeCID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get file data for node %s: %v", targetNodeCID, err)})
				return
			}

			// Determine content type (basic guess based on path extension)
			contentType := "application/octet-stream"
			ext := filepath.Ext(path)
			switch ext {
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
				// Add more types as needed
			}

			c.Data(http.StatusOK, contentType, fileData)
			return
		}

	} else {
		// This case might catch empty files or root node of an empty directory
		// An empty file node might have no Data and no Links.
		// An empty directory node might have no Data and no Links.
		// Need to distinguish based on how the node was created or potentially check the path.
		// For simplicity, if no data and no named links, assume it's an empty file/object.
		if len(targetNode.Data) == 0 && len(targetNode.Links) == 0 {
			c.Data(http.StatusOK, "application/octet-stream", []byte{}) // Serve empty content
			return
		}

		// If it has links but no data and links have no names, it should have been handled as a file above.
		// If it has links with names, it should have been handled as a directory above.
		// This might indicate an unexpected node structure.
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Node %s has unexpected structure", targetNodeCID)})
	}
}
