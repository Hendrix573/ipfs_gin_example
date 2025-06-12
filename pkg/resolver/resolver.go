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
	// TODO name -> file content
	name := domain
	if name == "" {
		return "", errors.New("domain name cannot be empty")
	}

	// Check cache first
	if cid, ok := r.cache.Get(name); ok {
		log.Printf("Cache hit for name %s: %s", name, cid)
		return cid, nil
	}

	log.Printf("Cache miss for name %s, querying contract", name)
	cid, err := r.contractClient.ResolveCID(domain)
	if err != nil {
		return "", errors.New("failed to resolve CID: " + err.Error())
	}
	if cid == "" {
		return "", nil
	}

	// Store in cache
	//r.cache.Add(name, contentCid)
	log.Printf("Cached name %s: %s", name, cid)
	return cid, nil
}

// UpdateMapping registers or updates the CID for a given domain/path combination.
// It checks ownership and decides whether to register or update the CID.
func (r *Resolver) UpdateMapping(auth *bind.TransactOpts, name, cid string) error {
	if name == "" || cid == "" {
		return errors.New("name and CID cannot be empty")
	}

	// Check if name exists and get owner
	owner, err := r.contractClient.GetOwner(name)
	if err == nil && owner != (common.Address{}) {
		// Name exists, check ownership
		if owner != auth.From {
			return errors.New("not authorized to update this name")
		}
		// Update existing CID
		err = r.contractClient.UpdateCID(auth, name, cid)
		if err != nil {
			return err
		}
	} else {
		// TODO 没有domain直接注册方便测试，后续应该由合约控制
		// Name does not exist, register it
		err = r.contractClient.RegisterName(auth, name, cid)
		if err != nil {
			return err
		}
	}

	// Update cache
	r.cache.Add(name, cid)
	return nil
}

// GetMapping retrieves the current CID and existence status for a name.
func (r *Resolver) GetMapping(name string) (string, bool, error) {
	// Check cache first
	if cid, ok := r.cache.Get(name); ok {
		return cid, true, nil
	}

	// Query smart contract
	cid, err := r.contractClient.ResolveCID(name)
	if err != nil {
		return "", false, errors.New("failed to get mapping: " + err.Error())
	}

	// Store in cache if found
	if cid != "" {
		r.cache.Add(name, cid)
		return cid, true, nil
	}
	return "", false, nil
}
