package main

import (
	"html/template"
	"log"
	"net/http"

	"ipfs-gin-example/config"
	"ipfs-gin-example/pkg/api"
	"ipfs-gin-example/pkg/contract"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"

	"github.com/gin-gonic/gin"
)

// main initializes and starts the IPFS-like server.
func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize BadgerDB storage
	store, err := storage.NewBadgerStore(cfg.BadgerDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Printf("BadgerDB initialized at %s", cfg.BadgerDBPath)

	// Validate contract address
	if cfg.ContractAddress == "" {
		log.Fatal("CONTRACT_ADDRESS is required for smart contract interaction")
	}

	// Initialize smart contract client
	contractClient, err := contract.NewClient(cfg.EthereumRPC, cfg.ContractAddress)
	if err != nil {
		log.Fatalf("Failed to initialize contract client: %v", err)
	}
	defer contractClient.Close()
	log.Printf("Smart contract client initialized for address %s", cfg.ContractAddress)

	// Initialize Resolver with contract client
	resolver := resolver.NewResolver(contractClient)
	log.Println("Resolver initialized with smart contract client and LRU cache.")

	// Initialize API Handlers
	uploadHandler := api.NewUploadHandler(store, cfg.ChunkSize, resolver, cfg)
	downloadHandler := api.NewDownloadHandler(store, resolver)
	registerHandler := api.NewRegisterHandler(cfg, resolver)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Load HTML templates for directory listing
	tmpl, err := template.ParseFiles("templates/directory_listing.tmpl")
	if err != nil {
		log.Printf("Error: Failed to load template 'templates/directory_listing.tmpl': %v. Directory listing will not work.", err)
		router.SetHTMLTemplate(template.Must(template.New("dummy").Parse("<h1>Directory Listing Not Available</h1>")))
	} else {
		router.SetHTMLTemplate(tmpl)
	}

	//// Serve static files (frontend)
	//router.Static("/static", "./static")
	//router.GET("/", func(c *gin.Context) {
	//	c.File("./static/index.html")
	//})

	// Define API routes
	apiGroup := router.Group("/")
	{
		registerHandler.RegisterRoutes(apiGroup)
		uploadHandler.RegisterRoutes(apiGroup)
		downloadHandler.RegisterRoutes(apiGroup)
	}
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "IPFS-like Gin Example Server is running!")
	})

	// Run the server
	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := router.Run(cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
