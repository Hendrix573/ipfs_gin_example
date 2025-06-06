package merkledag

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"ipfs-gin-example/pkg/storage"
	"strings"
)

// DAGBuilder handles building Merkle DAGs and path resolution
type DAGBuilder struct {
	store storage.Store
}

// NewDAGBuilder creates a new DAGBuilder
func NewDAGBuilder(store storage.Store) *DAGBuilder {
	return &DAGBuilder{store: store}
}

// AddNode stores a node and returns its CID
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
func (b *DAGBuilder) BuildDAGFromLeaves(leaves []*Node) (string, uint64, error) {
	if len(leaves) == 0 {
		// Handle empty content: create an empty node
		emptyNode := &Node{}
		cid, err := b.AddNode(emptyNode)
		if err != nil {
			return "", 0, err
		}
		return cid, 0, nil // Empty node has size 0
	}

	currentLevelNodes := leaves
	for len(currentLevelNodes) > 1 {
		var nextLevelNodes []*Node
		// Group nodes into new parent nodes
		// IPFS default fanout is around 174 links per node
		fanout := 174
		for i := 0; i < len(currentLevelNodes); i += fanout {
			end := i + fanout
			if end > len(currentLevelNodes) {
				end = len(currentLevelNodes)
			}
			group := currentLevelNodes[i:end]

			parentNode := &Node{}
			var totalSize uint64 // Size of the object this node represents (sum of children's sizes)
			for _, child := range group {
				childCID, err := b.AddNode(child) // Store child and get its CID
				if err != nil {
					return "", 0, fmt.Errorf("failed to store child node: %w", err)
				}

				// For chunk nodes (leaves), size is data size.
				// For intermediate nodes, size is the sum of the sizes of objects they link to.
				// We need the *total size* of the object represented by the childCID.
				// If the child is a leaf, its size is len(child.Data).
				// If the child is an intermediate node, we need its calculated size.
				// To avoid recursive size calculation during build, let's rely on the size calculated *after* the child is built.
				// This requires a slight change in flow: build level, then calculate sizes for parent links.
				// Let's simplify for now and assume child.Size is sum of linked objects for non-leaves.
				// A proper implementation would calculate size bottom-up or store it with the node.

				// Simplified size calculation for this example:
				// If the child is a leaf (no links), size is data length.
				// If the child is an intermediate node, its size is the sum of sizes of nodes it links to.
				// We need to *get* the child node to find its total size if it's not a leaf. This is inefficient.
				// A better approach: Build a level -> get CIDs -> for parent links, calculate size by summing children's *total* sizes.
				// Let's adjust: Build level, store nodes, then create parent links with calculated sizes.

				// Temporary Link without size, calculate size later
				parentNode.Links = append(parentNode.Links, Link{
					Hash: childCID,
					Name: "", // File chunks usually have no names in links from a file node
					Size: 0,  //Placeholder, will calculate later
				})
			}

			// Store the parent node *without* correct sizes yet
			parentNodeCID, err := b.AddNode(parentNode)
			if err != nil {
				return "", 0, fmt.Errorf("failed to store parent node: %w", err)
			}

			// Now retrieve the parent node to update link sizes (inefficient but works for demo)
			updatedParentNode := &Node{}     // Create a new node to avoid modifying the one already added
			*updatedParentNode = *parentNode // Copy data and links

			totalSize = 0
			for i, link := range updatedParentNode.Links {
				// Get the child node to calculate its total size
				childNode, err := b.GetNode(link.Hash)
				if err != nil {
					return "", 0, fmt.Errorf("failed to get child node %s for size calculation: %w", link.Hash, err)
				}
				childSize := b.CalculateNodeSize(childNode) // Recursive size calculation
				updatedParentNode.Links[i].Size = childSize
				totalSize += childSize
			}

			// Store the updated parent node again (overwriting the previous one with same CID)
			// This relies on AddNode overwriting if CID is the same, which BadgerDB Put does.
			// In a real system, you might need a specific "update" or "re-add" with integrity check.
			_, err = b.AddNode(updatedParentNode) // Re-add with correct sizes
			if err != nil {
				return "", 0, fmt.Errorf("failed to re-store parent node with sizes %s: %w", parentNodeCID, err)
			}

			nextLevelNodes = append(nextLevelNodes, updatedParentNode) // Add the node with correct sizes
		}
		currentLevelNodes = nextLevelNodes
	}

	// After the loop, currentLevelNodes should contain only the root node
	if len(currentLevelNodes) != 1 {
		return "", 0, errors.New("failed to build single root node")
	}

	rootNode := currentLevelNodes[0]
	rootCID, err := b.AddNode(rootNode) // Store the final root node and return its CID
	if err != nil {
		return "", 0, err
	}
	rootSize := b.CalculateNodeSize(rootNode)

	return rootCID, rootSize, nil
}

// BuildDirectoryDAG builds a DAG node representing a directory
// links map: key is item name (filename/dirname), value is item's root CID and size
func (b *DAGBuilder) BuildDirectoryDAG(items map[string]struct {
	CID  string
	Size uint64
}) (string, uint64, error) {
	dirNode := &Node{}
	var totalSize uint64 // Directory size is typically sum of linked object sizes
	for name, item := range items {
		dirNode.Links = append(dirNode.Links, Link{
			Name: name,
			Hash: item.CID,
			Size: item.Size,
		})
		totalSize += item.Size
	}
	// Note: Directory nodes typically have no Data field.

	cid, err := b.AddNode(dirNode) // Store the directory node and return its CID
	if err != nil {
		return "", 0, err
	}
	return cid, totalSize, nil
}

// ResolvePath traverses the DAG from a root CID to find the node at the given path
func (b *DAGBuilder) ResolvePath(rootCID string, path string) (string, error) {
	currentNodeCID := rootCID
	pathComponents := splitPath(path) // Helper function to split path e.g., "/foo/bar" -> ["foo", "bar"]

	// If path is just "/", resolve to the root CID itself
	if len(pathComponents) == 0 && (path == "/" || path == "") {
		return rootCID, nil
	}

	for _, component := range pathComponents {
		if component == "" {
			continue // Should not happen with splitPath, but safe check
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
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")

	if path == "" { // Handle cases like "//"
		return []string{}
	}

	return strings.Split(path, "/")
}

// GetFileData retrieves and concatenates data for a file node or a node linking to chunks
func (b *DAGBuilder) GetFileData(fileNodeCID string) ([]byte, error) {
	fileNode, err := b.GetNode(fileNodeCID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file node %s: %w", fileNodeCID, err)
	}

	if len(fileNode.Data) > 0 && len(fileNode.Links) == 0 {
		// This is a single-chunk file node (or raw data node)
		return fileNode.Data, nil
	}

	if len(fileNode.Links) == 0 && len(fileNode.Data) == 0 {
		// Empty file?
		return []byte{}, nil
	}

	// It's a file represented by multiple chunks linked from this node
	var fileData bytes.Buffer
	for _, link := range fileNode.Links {
		// Ensure the link points to a data chunk node (node with Data, no Links)
		chunkNode, err := b.GetNode(link.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to get chunk node %s: %w", link.Hash, err)
		}
		if len(chunkNode.Data) == 0 || len(chunkNode.Links) > 0 {
			// This linked node is not a simple data chunk. This might indicate a non-file node
			// or a more complex file structure not handled here.
			// For this simplified example, we expect file data nodes to link directly to chunks.
			// If a link has a Name, it's likely a directory link. File chunks usually don't have names in links from the file root.
			if link.Name != "" {
				return nil, fmt.Errorf("linked node %s ('%s') is not a data chunk for file", link.Hash, link.Name)
			}
			// If no name, but not a data chunk node structure, still an issue
			return nil, fmt.Errorf("linked node %s is not a data chunk (unexpected structure)", link.Hash)

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

	// A directory node should ideally have no Data and only Links with Names.
	// Our simple model allows nodes with Data OR Links. Let's explicitly check for directory characteristics.
	// Assume a node is a directory if it has links and those links have names.
	if len(dirNode.Links) > 0 && dirNode.Links[0].Name != "" {
		return dirNode.Links, nil
	}
	if len(dirNode.Links) == 0 && len(dirNode.Data) == 0 {
		// Empty node, could be an empty directory
		return []Link{}, nil
	}

	return nil, errors.New("node is not a directory node")
}

// CalculateNodeSize recursively calculates the total size of data under a node
func (b *DAGBuilder) CalculateNodeSize(node *Node) uint64 {
	if len(node.Data) > 0 {
		return uint64(len(node.Data)) // Leaf node, size is data size
	}

	var totalSize uint64
	for _, link := range node.Links {
		// In a properly built DAG (like in BuildDAGFromLeaves or BuildDirectoryDAG),
		// the Link.Size should already contain the total size of the object the link points to.
		// Relying on Link.Size avoids deep recursion here.
		totalSize += link.Size
		/*
			// Alternative (recursive, expensive):
			childNode, err := b.GetNode(link.Hash)
			if err == nil { // Ignore error? Or propagate? Propagate might be better.
				totalSize += b.CalculateNodeSize(childNode)
			}
		*/
	}
	return totalSize
}

// PutNodeAtPath updates or creates the DAG path from rootCID, linking targetCID at the final path component.
// It returns the new root CID of the updated DAG.
// path must be absolute (e.g., "/home/user1/file.txt").
func (b *DAGBuilder) PutNodeAtPath(currentRootCID string, path string, targetCID string, targetSize uint64) (string, error) {
	if path == "" || path == "/" {
		// Cannot "put" content at the root path itself using this method.
		// This method is for putting *into* the tree structure.
		// Putting *as* the root would be a direct update to the domain's CID mapping.
		return "", errors.New("cannot put content at the root path '/'")
	}

	pathComponents := splitPath(path)
	if len(pathComponents) == 0 {
		return "", errors.New("invalid path") // Should not happen with the check above
	}

	// The last component is the name of the item being put (file or directory name)
	itemName := pathComponents[len(pathComponents)-1]
	// The preceding components form the path to the parent directory
	parentPathComponents := pathComponents[:len(pathComponents)-1]

	// Start the recursive update from the root
	newRootCID, err := b.updateDirRecursive(currentRootCID, parentPathComponents, itemName, targetCID, targetSize)
	if err != nil {
		return "", fmt.Errorf("failed to update DAG path: %w", err)
	}

	return newRootCID, nil
}

// updateDirRecursive is a helper to recursively build/update directory nodes upwards from the target.
// It takes the CID of the current directory being processed (starting with the root),
// the remaining path components *to the parent directory*, the name of the item to link,
// and the CID/Size of the item being linked.
// It returns the CID of the *new* node for the current directory level.
func (b *DAGBuilder) updateDirRecursive(currentDirCID string, parentPathComponents []string, itemName string, targetCID string, targetSize uint64) (string, error) {

	// Base case: We are at the level of the direct parent directory
	if len(parentPathComponents) == 0 {
		// Get the current parent directory node
		parentDirNode, err := b.GetNode(currentDirCID)
		if err != nil {
			// If the currentDirCID doesn't exist, it means the path was invalid or a parent didn't exist.
			// For simplicity, let's assume the initial rootCID exists. If intermediate paths didn't exist,
			// this recursive function needs to handle creating them.
			// Let's modify the logic: If currentDirCID is empty or not found, assume it's an empty directory.
			if errors.Is(err, badger.ErrKeyNotFound) {
				parentDirNode = &Node{} // Start with an empty directory node if current doesn't exist
			} else {
				return "", fmt.Errorf("failed to get parent directory node %s: %w", currentDirCID, err)
			}
		}

		// Create a new node for the parent directory
		newParentDirNode := &Node{}
		// Copy existing links, but replace or add the link for itemName
		linkExists := false
		for _, link := range parentDirNode.Links {
			if link.Name == itemName {
				// Replace existing link
				newParentDirNode.Links = append(newParentDirNode.Links, Link{Name: itemName, Hash: targetCID, Size: targetSize})
				linkExists = true
			} else {
				// Keep other links
				newParentDirNode.Links = append(newParentDirNode.Links, link)
			}
		}
		if !linkExists {
			// Add the new link if it didn't exist
			newParentDirNode.Links = append(newParentDirNode.Links, Link{Name: itemName, Hash: targetCID, Size: targetSize})
		}

		// Store the new parent directory node
		newParentDirCID, err := b.AddNode(newParentDirNode)
		if err != nil {
			return "", fmt.Errorf("failed to store new parent directory node: %w", err)
		}

		// The new parent directory node is the result of this step
		return newParentDirCID, nil
	}

	// Recursive case: We are at an intermediate directory level.
	// We need to find/create the next directory in the path (the first component in parentPathComponents),
	// recursively update *that* directory, and then link the *new* version of that next directory
	// into a *new* node for the current directory.

	currentComponentName := parentPathComponents[0]  // e.g., "home"
	restOfPathComponents := parentPathComponents[1:] // e.g., ["user1"]

	// Get the node for the current directory level
	currentDirNode, err := b.GetNode(currentDirCID)
	var nextDirCID string // The CID of the directory node for currentComponentName

	if err != nil {
		// If the currentDirCID doesn't exist, it means a parent path component was not found.
		// We need to create the missing intermediate directories recursively.
		// Let's assume currentDirCID exists based on the initial root lookup.
		// If an intermediate link is missing, we create a new empty directory for the next component.
		if errors.Is(err, badger.ErrKeyNotFound) {
			// This case should ideally be handled by finding the link in the parent, not getting the current node failing.
			return "", fmt.Errorf("internal error: current directory node %s not found during recursive update", currentDirCID)
		} else {
			return "", fmt.Errorf("failed to get current directory node %s during recursive update: %w", currentDirCID, err)
		}
	}

	// Find the existing link for the next component (e.g., "home") in the current directory's links
	var existingLink *Link
	for _, link := range currentDirNode.Links {
		if link.Name == currentComponentName {
			existingLink = &link
			break
		}
	}

	if existingLink != nil {
		// The next directory node already exists, get its CID
		nextDirCID = existingLink.Hash
	} else {
		// The next directory node does not exist. Create an empty one for now.
		// The recursive call will populate it or traverse deeper.
		emptyNextDirNode := &Node{}
		var addErr error
		nextDirCID, addErr = b.AddNode(emptyNextDirNode)
		if addErr != nil {
			return "", fmt.Errorf("failed to create new intermediate directory node for '%s': %w", currentComponentName, addErr)
		}
		// Note: The size of this newly created empty directory node is 0.
	}

	// Recursively update the next directory level down
	newNextDirCID, err := b.updateDirRecursive(nextDirCID, restOfPathComponents, itemName, targetCID, targetSize)
	if err != nil {
		return "", fmt.Errorf("recursive update failed for component '%s': %w", currentComponentName, err)
	}

	// Get the newly updated next directory node to calculate its size for the link
	newNextDirNode, err := b.GetNode(newNextDirCID)
	if err != nil {
		return "", fmt.Errorf("failed to get new next directory node %s after recursive update: %w", newNextDirCID, err)
	}
	newNextDirSize := b.CalculateNodeSize(newNextDirNode)

	// Create a new node for the current directory level
	newCurrentDirNode := &Node{}
	// Copy existing links from the original current directory node, but replace or add the link for currentComponentName
	linkExists := false
	for _, link := range currentDirNode.Links {
		if link.Name == currentComponentName {
			// Replace existing link with the new version of the next directory
			newCurrentDirNode.Links = append(newCurrentDirNode.Links, Link{Name: currentComponentName, Hash: newNextDirCID, Size: newNextDirSize})
			linkExists = true
		} else {
			// Keep other links
			newCurrentDirNode.Links = append(newCurrentDirNode.Links, link)
		}
	}
	if !linkExists {
		// Add the new link if it didn't exist (this handles creating missing intermediate directories)
		newCurrentDirNode.Links = append(newCurrentDirNode.Links, Link{Name: currentComponentName, Hash: newNextDirCID, Size: newNextDirSize})
	}

	// Store the new node for the current directory level
	newCurrentDirCID, err := b.AddNode(newCurrentDirNode)
	if err != nil {
		return "", fmt.Errorf("failed to store new current directory node: %w", err)
	}

	// Return the CID of the new node for this level
	return newCurrentDirCID, nil
}

// GetNodeSize retrieves a node and calculates its total size
func (b *DAGBuilder) GetNodeSize(cid string) (uint64, error) {
	node, err := b.GetNode(cid)
	if err != nil {
		return 0, err
	}
	return b.CalculateNodeSize(node), nil
}
