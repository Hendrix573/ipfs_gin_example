package api

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"ipfs-gin-example/config"
	"ipfs-gin-example/pkg/resolver"
	"log"
	"math/big"
	"net/http"
	"strings"
)

type RegisterHandler struct {
	Config   *config.Config
	Resolver *resolver.Resolver
}

func (r *RegisterHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/register", r.RegisterDomain)
}

func NewRegisterHandler(cfg *config.Config, resolver *resolver.Resolver) *RegisterHandler {
	return &RegisterHandler{
		Config:   cfg,
		Resolver: resolver,
	}
}

func (r *RegisterHandler) RegisterDomain(c *gin.Context) {
	// 1. Get domain from query parameter
	domain := c.Query("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing domain query parameter"})
		return
	}

	// 2. Prepare transaction options (auth) using the provided logic
	// Decode the private key
	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(r.Config.PrivateKey, "0x"))
	if err != nil {
		// Log the error internally but return a generic server error to the client
		log.Printf("Error decoding private key: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error: failed to process private key"})
		return
	}

	// Create the transactor options
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(r.Config.ChainID))
	if err != nil {
		log.Printf("Error creating keyed transactor: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error: failed to prepare transaction"})
		return
	}

	placeholderCID := ""
	err = r.Resolver.RegisterDomain(auth, domain, placeholderCID) // Call your blockchain interaction function
	if err != nil {
		log.Printf("Error during domain registration for '%s': %v", domain, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to register domain: %v", err)})
		return
	}

	// 4. Respond with success
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Domain '%s' registration initiated successfully", domain)})
}
