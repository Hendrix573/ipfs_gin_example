package resolver

import "errors"

// Resolver resolves domain/subdomain to a root CID
type Resolver struct {
	// domain -> root CID map
	domainMap map[string]string
}

// NewResolver creates a new mock Resolver
func NewResolver() *Resolver {
	// Hardcoded mapping for demonstration
	mockMap := map[string]string{
		"example.com":       "f3a4b5c6d7e8f90123456789abc0def1234567890abcdef1234567890abcdef",  // Example CID
		"files.example.com": "a1b2c3d4e5f678901234567890abcdef1234567890abcdef1234567890abcdef", // Example CID for a directory
		"hello.com":         "20efe42492eeae040ec75d1b9b4a5decfbe2dc97d153038f57a7b5f4471f6edc",
	}
	return &Resolver{domainMap: mockMap}
}

// ResolveDomain looks up the root CID for a given domain
func (r *Resolver) ResolveDomain(domain string) (string, error) {
	cid, ok := r.domainMap[domain]
	if !ok {
		return "", errors.New("domain not found")
	}
	return cid, nil
}

// You would need methods to add/update these mappings in a real system
// AddMapping(domain, cid string) error
// RemoveMapping(domain string) error
