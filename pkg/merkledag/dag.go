package merkledag

import (
	"bytes"
	"errors"
	"fmt"
	"ipfs-gin-example/pkg/storage"
)

// DAGBuilder handles building Merkle DAGs
type DAGBuilder struct {
	store storage.Store
}

// NewDAGBuilder creates a new DAGBuilder
func NewDAGBuilder(store storage.Store) *DAGBuilder {
	return &DAGBuilder{store: store}
}

// AddNodeCID stores a node and returns its CID
func (b *DAGBuilder) AddNode(node *Node) (string, error) {
	cid, err := node.Cid()
	if err != nil {
		return "", fmt.Errorf("failed to get CID for node: %w", err)
	}

	data, err := node.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("failed to marshal node: %w", err)
	}

	err = b.store.Put([]byte(cid), data)
	if err != nil {
		return "", fmt.Errorf("failed to store node %s: %w", cid, err)
	}

	return cid, nil
}

// GetNode retrieves a node by its CID
func (b *DAGBuilder) GetNode(cid string) (*Node, error) {
	data, err := b.store.Get([]byte(cid))
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s from store: %w", cid, err)
	}

	node := &Node{}
	err = node.UnmarshalBinary(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal node %s: %w", cid, err)
	}

	return node, nil
}

// BuildDAGFromLeaves builds a DAG from a list of leaf nodes (chunks)
// It returns the root CID of the built DAG.
func (b *DAGBuilder) BuildDAGFromLeaves(leaves []*Node) (string, error) {
	if len(leaves) == 0 {
		// An empty file might result in 0 leaves, what should the root be?
		// An empty node? Or a specific empty CID? Let's use an empty node for now.
		emptyNode := &Node{}
		return b.AddNode(emptyNode)
	}

	currentLevelNodes := leaves
	for len(currentLevelNodes) > 1 {
		var nextLevelNodes []*Node
		// Group nodes into new parent nodes
		for i := 0; i < len(currentLevelNodes); i += 174 { // IPFS default fanout is around 174 links per node
			end := i + 174
			if end > len(currentLevelNodes) {
				end = len(currentLevelNodes)
			}
			group := currentLevelNodes[i:end]

			parentNode := &Node{}
			var totalSize uint64
			for _, child := range group {
				childCID, err := b.AddNode(child) // Store child and get its CID
				if err != nil {
					return "", fmt.Errorf("failed to store child node: %w", err)
				}

				childSize := uint64(len(child.Data)) // For chunk nodes, size is data size
				// For intermediate nodes, size is sum of children's sizes
				if len(child.Links) > 0 {
					childNodeInfo, err := b.GetNode(childCID) // Retrieve child to get its total size
					if err != nil {
						return "", fmt.Errorf("failed to get child node info for size: %w", err)
					}
					childSize = b.CalculateNodeSize(childNodeInfo) // Calculate size recursively
				}

				parentNode.Links = append(parentNode.Links, Link{
					Hash: childCID,
					Size: childSize, // Size of the object the link points to
				})
				totalSize += childSize
			}

			// Store the parent node
			nextLevelNodes = append(nextLevelNodes, parentNode)
		}
		currentLevelNodes = nextLevelNodes
	}

	// After the loop, currentLevelNodes should contain only the root node
	if len(currentLevelNodes) != 1 {
		return "", errors.New("failed to build single root node")
	}

	rootNode := currentLevelNodes[0]
	return b.AddNode(rootNode) // Store the final root node and return its CID
}

// Helper to recursively calculate the total size of data under a node
func (b *DAGBuilder) CalculateNodeSize(node *Node) uint64 {
	if len(node.Data) > 0 {
		return uint64(len(node.Data)) // Leaf node, size is data size
	}

	var totalSize uint64
	for _, link := range node.Links {
		// We can trust the size in the link if it was set correctly during build
		// Or we could recursively fetch and calculate if needed, but that's expensive.
		// Assuming Link.Size is correctly set during BuildDAGFromLeaves/BuildDirectoryDAG.
		totalSize += link.Size
	}
	return totalSize
}

// BuildDirectoryDAG builds a DAG node representing a directory
// links map: key is item name (filename/dirname), value is item's root CID and size
func (b *DAGBuilder) BuildDirectoryDAG(items map[string]struct {
	CID  string
	Size uint64
}) (string, error) {
	dirNode := &Node{}
	for name, item := range items {
		dirNode.Links = append(dirNode.Links, Link{
			Name: name,
			Hash: item.CID,
			Size: item.Size,
		})
	}
	// Note: Directory nodes typically have no Data field.

	return b.AddNode(dirNode) // Store the directory node and return its CID
}

// ResolvePath traverses the DAG from a root CID to find the node at the given path
func (b *DAGBuilder) ResolvePath(rootCID string, path string) (string, error) {
	currentNodeCID := rootCID
	pathComponents := splitPath(path) // Helper function to split path e.g., "/foo/bar" -> ["foo", "bar"]

	for _, component := range pathComponents {
		if component == "" {
			continue // Skip empty components from leading/trailing slashes
		}

		node, err := b.GetNode(currentNodeCID)
		if err != nil {
			return "", fmt.Errorf("failed to get node %s during path resolution: %w", currentNodeCID, err)
		}

		found := false
		for _, link := range node.Links {
			if link.Name == component {
				currentNodeCID = link.Hash // Move to the next node
				found = true
				break
			}
		}

		if !found {
			return "", fmt.Errorf("path component '%s' not found in node %s", component, currentNodeCID)
		}
	}

	return currentNodeCID, nil // Return the CID of the final node
}

// Helper to split path components
func splitPath(path string) []string {
	// Simple split by '/'
	if path == "/" || path == "" {
		return []string{} // Root path has no components
	}
	// Remove leading/trailing slashes before splitting
	if path[0] == '/' {
		path = path[1:]
	}
	if path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	// Use bytes.Split to handle potential multiple slashes correctly (produces empty slices)
	byteComponents := bytes.Split([]byte(path), []byte("/"))

	// Convert [][]byte to []string and filter out empty components
	var stringComponents []string
	for _, comp := range byteComponents {
		// Convert each []byte component to a string
		strComp := string(comp)
		// Only include non-empty components
		if strComp != "" {
			stringComponents = append(stringComponents, strComp)
		}
	}

	return stringComponents // Return the slice of strings
}

// GetFileData retrieves and concatenates data for a file node
func (b *DAGBuilder) GetFileData(fileNodeCID string) ([]byte, error) {
	fileNode, err := b.GetNode(fileNodeCID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file node %s: %w", fileNodeCID, err)
	}

	if len(fileNode.Data) > 0 && len(fileNode.Links) == 0 {
		// This is a single-chunk file node
		return fileNode.Data, nil
	}

	if len(fileNode.Links) == 0 && len(fileNode.Data) == 0 {
		// Empty file?
		return []byte{}, nil
	}

	// It's a file represented by multiple chunks linked from this node
	var fileData bytes.Buffer
	for _, link := range fileNode.Links {
		chunkNode, err := b.GetNode(link.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get chunk node %s: %w", link.Hash, err)
		}
		if len(chunkNode.Data) == 0 {
			return nil, fmt.Errorf("linked node %s is not a data chunk", link.Hash)
		}
		fileData.Write(chunkNode.Data)
	}

	// Optional: Verify total size if link.Size was used to sum up
	// if uint64(fileData.Len()) != b.calculateNodeSize(fileNode) {
	// 	return nil, errors.New("file data size mismatch")
	// }

	return fileData.Bytes(), nil
}

// ListDirectory lists the contents of a directory node
func (b *DAGBuilder) ListDirectory(dirNodeCID string) ([]Link, error) {
	dirNode, err := b.GetNode(dirNodeCID)
	if err != nil {
		return nil, fmt.Errorf("failed to get directory node %s: %w", dirNodeCID, err)
	}

	if len(dirNode.Data) > 0 {
		return nil, errors.New("node is not a directory node (has data)")
	}

	return dirNode.Links, nil
}
