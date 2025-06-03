package merkledag

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Link represents a link to another Node
type Link struct {
	Name string `json:"name,omitempty"` // Name of the linked object (e.g., filename)
	Hash string `json:"hash"`           // CID of the linked Node
	Size uint64 `json:"size"`           // Size of the linked object
}

// Node represents a Merkle DAG node
type Node struct {
	Data  []byte `json:"data,omitempty"`  // Content data (for leaf nodes)
	Links []Link `json:"links,omitempty"` // Links to children nodes
}

// Cid calculates the CID (SHA256 hex) of the Node's serialized representation
func (n *Node) Cid() (string, error) {
	// We need to serialize the node consistently to get a consistent hash.
	// JSON is simple for this example. Note: Real IPFS uses Protobuf and specific codecs.
	// The serialization should include both Data and Links.
	// Omitempty is used, so we need to be careful when marshalling for hashing.
	// Let's create a temporary struct or marshal explicitly to ensure fields are included.

	// Simple JSON serialization for hashing
	// Note: This might not be identical to IPFS's serialization, but works for this example.
	dataToHash, err := json.Marshal(n)
	if err != nil {
		return "", fmt.Errorf("failed to marshal node for hashing: %w", err)
	}

	hash := sha256.Sum256(dataToHash)
	return hex.EncodeToString(hash[:]), nil
}

// MarshalBinary implements encoding.BinaryMarshaler
func (n *Node) MarshalBinary() ([]byte, error) {
	return json.Marshal(n)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (n *Node) UnmarshalBinary(data []byte) error {
	return json.Unmarshal(data, n)
}
