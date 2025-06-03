package merkledag

import (
	"io"
)

// Chunker splits content into fixed-size chunks
type Chunker struct {
	chunkSize int
}

// NewChunker creates a new Chunker
func NewChunker(chunkSize int) *Chunker {
	return &Chunker{chunkSize: chunkSize}
}

// Chunk reads from an io.Reader and returns a list of Node representing the chunks
func (c *Chunker) Chunk(r io.Reader) ([]*Node, error) {
	var blocks []*Node
	buf := make([]byte, c.chunkSize)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunkData := make([]byte, n)
			copy(chunkData, buf[:n])
			node := &Node{Data: chunkData}
			blocks = append(blocks, node)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return blocks, nil
}
