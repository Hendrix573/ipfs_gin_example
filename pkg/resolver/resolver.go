package resolver

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/lru"
	"log"

	"ipfs-gin-example/pkg/contract"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

// Resolver resolves domain/subdomain to a root CID by interacting with the smart contract,
// with an LRU cache for performance optimization.
type Resolver struct {
	contractClient *contract.Client
	cache          *lru.Cache[string, string] // LRU cache for name -> CID mappings
}

// NewResolver creates a new Resolver with a contract client and an LRU cache of size 2^16.
func NewResolver(contractClient *contract.Client) *Resolver {
	// Initialize LRU cache with capacity 2^16 (65,536)
	cache := lru.NewCache[string, string](1 << 16)
	//if err != nil {
	//	// Should not happen with valid capacity
	//	panic("failed to initialize LRU cache: " + err.Error())
	//}
	return &Resolver{
		contractClient: contractClient,
		cache:          cache,
	}
}

// ResolveDomain looks up the root CID for a given domain/path combination.
func (r *Resolver) ResolveDomain(domain string) (string, error) {
	cid, err := r.contractClient.ResolveCID(domain)
	if err != nil {
		return "", errors.New("failed to resolve CID: " + err.Error())
	}
	if cid == "" {
		return "", nil
	}
	return cid, nil
}

func (r *Resolver) GetCache(domain string, path string) (string, bool) {
	name := domain + "/" + path
	// Check cache first
	if cid, ok := r.cache.Get(name); ok {
		log.Printf("Cache hit for name %s: %s", name, cid)
		return cid, true
	}
	log.Printf("Cache miss for name %s, querying contract", name)
	return "", false
}

func (r *Resolver) AddCache(domain string, path string, contentCID string) {
	name := domain + "/" + path
	r.cache.Add(name, contentCID)
}

// UpdateMapping registers or updates the CID for a given domain/path combination.
// It checks ownership and decides whether to register or update the CID.
func (r *Resolver) UpdateMapping(auth *bind.TransactOpts, domain, cid string) error {
	if domain == "" || cid == "" {
		return errors.New("name and CID cannot be empty")
	}

	// Check if name exists and get owner
	owner, err := r.contractClient.GetOwner(domain)
	if err == nil && owner != (common.Address{}) {
		// Name exists, check ownership
		if owner != auth.From {
			return errors.New("not authorized to update this name")
		}
		// Update existing CID
		err = r.contractClient.UpdateCID(auth, domain, cid)
		if err != nil {
			return err
		}
	} else {
		return errors.New("domain does not exist")
	}

	return nil
}

func (r *Resolver) RegisterDomain(auth *bind.TransactOpts, domain, cid string) error {
	err := r.contractClient.RegisterName(auth, domain, cid)
	return err
}
