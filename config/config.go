package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds the application configuration.
type Config struct {
	BadgerDBPath    string // Path to BadgerDB storage directory
	ServerPort      string // Port for the HTTP server
	ChunkSize       int    // Size for content chunking (in bytes)
	EthereumRPC     string // Ethereum node RPC URL
	ContractAddress string // Address of the DecentralizedNamingSystem contract
	PrivateKey      string // Private key for signing transactions
	ChainID         int64  // Ethereum chain ID
}

// LoadConfig loads and returns the application configuration.
func LoadConfig() *Config {
	// Initialize BadgerDB path and ensure directory exists
	dbPath := filepath.Join(".", "data", "badger")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Fatalf("Failed to create database directory %s: %v", dbPath, err)
	}

	// Load server port
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = ":8080"
		log.Println("Warning: SERVER_PORT not set, using default :8080")
	}

	// Load Ethereum RPC URL
	ethereumRPC := os.Getenv("ETHEREUM_RPC")
	if ethereumRPC == "" {
		ethereumRPC = "http://127.0.0.1:8545"
		log.Println("Warning: ETHEREUM_RPC not set, using default Ganache URL")
	}

	// Load contract address
	contractAddress := os.Getenv("CONTRACT_ADDRESS")
	if contractAddress == "" {
		contractAddress = "0x26d1824524320000b66c0f6990c4d15b950b2779"
		log.Println("Warning: CONTRACT_ADDRESS not set, Using default CONTRACT_ADDRESS")
	}

	// Load private key
	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		privateKey = "0x49b44de84c2581e367a66f2f6d5b90a3c8f0cfec4153ca2b36b99f7bd916940e"
		log.Println("Warning: PRIVATE_KEY not set, Using default PRIVATE_KEY")
	}

	// Load chain ID
	chainIDStr := os.Getenv("CHAIN_ID")
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil || chainID == 0 {
		chainID = 1337 // Ganache default chain ID
		log.Println("Warning: CHAIN_ID not set or invalid, using default Ganache chain ID 1337")
	}

	return &Config{
		BadgerDBPath:    dbPath,
		ServerPort:      serverPort,
		ChunkSize:       256 * 1024, // 256KB
		EthereumRPC:     ethereumRPC,
		ContractAddress: contractAddress,
		PrivateKey:      privateKey,
		ChainID:         chainID,
	}
}
