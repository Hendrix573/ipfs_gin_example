package resolver

import (
	"errors"
	"sync"
)

// Resolver resolves domain/subdomain to a root CID
// Added mutex for thread safety since mappings will be updated
type Resolver struct {
	// domain -> root CID map
	domainMap map[string]string
	mu        sync.RWMutex
}

// NewResolver creates a new mock Resolver
func NewResolver() *Resolver {
	// Hardcoded mapping for demonstration
	mockMap := map[string]string{
		// These CIDs should ideally point to directory nodes
		"example.com":       "f3a4b5c6d7e8f90123456789abc0def1234567890abcdef1234567890abcdef",  // Example CID
		"files.example.com": "a1b2c3d4e5f678901234567890abcdef1234567890abcdef1234567890abcdef", // Example CID for a directory
		"hello.com":         "3a64c418ea035aeee20d08fd347562e106201f99b639e1c0ac0b5ba1db26ef39",
	}
	return &Resolver{domainMap: mockMap}
}

// ResolveDomain looks up the root CID for a given domain
func (r *Resolver) ResolveDomain(domain string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cid, ok := r.domainMap[domain]
	if !ok {
		// In a real system, you might return a default empty directory CID or an error.
		// Returning an empty string here to indicate not found.
		return "", errors.New("domain not found")
	}
	return cid, nil
}

// UpdateMapping updates the root CID for a given domain.
// If the domain doesn't exist, it will be added.
func (r *Resolver) UpdateMapping(domain string, cid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.domainMap[domain] = cid
}

// GetMapping retrieves the current mapping - useful for initializing or checking existence
func (r *Resolver) GetMapping(domain string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cid, ok := r.domainMap[domain]
	return cid, ok
}

// For a real system, these mappings would be persistent (e.g., in BadgerDB or another store)
// and potentially part of a distributed naming system (like IPNS or DNSLink).
