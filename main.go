package main

import (
	"html/template"
	"log"
	"net/http" // Import http for status codes

	"ipfs-gin-example/config"
	"ipfs-gin-example/pkg/api"
	"ipfs-gin-example/pkg/resolver"
	"ipfs-gin-example/pkg/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()

	// Initialize BadgerDB storage
	store, err := storage.NewBadgerStore(cfg.BadgerDBPath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	log.Printf("BadgerDB initialized at %s", cfg.BadgerDBPath)

	// Initialize Resolver
	mockResolver := resolver.NewResolver()
	log.Println("Mock resolver initialized. Mappings are in-memory and not persistent.")

	// Initialize API Handlers
	handlers := api.NewHandlers(store, cfg.ChunkSize, mockResolver)

	// Setup Gin router
	router := gin.Default()

	// Load HTML templates for directory listing
	tmpl, err := template.ParseFiles("templates/directory_listing.tmpl")
	if err != nil {
		// Handle the case where the template file is missing more gracefully
		log.Printf("Warning: Failed to load template 'templates/directory_listing.tmpl': %v. Directory listing will not work.", err)
		// Set a dummy template to prevent panic if directory listing is attempted
		router.SetHTMLTemplate(template.Must(template.New("dummy").Parse("<h1>Directory Listing Not Available</h1>")))

	} else {
		router.SetHTMLTemplate(tmpl)
	}

	// Define routes

	// Basic Uploads (return CID of content)
	router.POST("/upload", handlers.UploadHandler)
	router.POST("/upload/multipart", handlers.MultipartUploadHandler)
	router.POST("/upload/dag", handlers.DAGUploadHandler)

	// Put content at a specific path under a domain (updates domain's root CID)
	// PUT /:domain/*path
	router.PUT("/:domain/*path", handlers.PutHandler)

	// Download route with domain and path parameters
	// The /*path parameter captures everything after /:domain/
	router.GET("/:domain/*path", handlers.DownloadHandler)

	// Add a root route or health check
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "IPFS-like Gin Example Server is running!")
	})

	// Run the server
	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := router.Run(cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
