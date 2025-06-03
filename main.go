package main

import (
	"html/template"
	"log"

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
	log.Println("Mock resolver initialized with hardcoded domains.")

	// Initialize API Handlers
	handlers := api.NewHandlers(store, cfg.ChunkSize, mockResolver)

	// Setup Gin router
	router := gin.Default()

	// Load HTML templates for directory listing
	tmpl, err := template.ParseFiles("templates/directory_listing.tmpl")
	if err != nil {
		log.Fatalf("Failed to load template: %v", err)
	}
	router.SetHTMLTemplate(tmpl)

	// Define routes
	router.POST("/upload", handlers.UploadHandler)
	router.POST("/upload/multipart", handlers.MultipartUploadHandler)
	router.POST("/upload/dag", handlers.DAGUploadHandler)

	// Download route with domain and path parameters
	// The /*path parameter captures everything after /:domain/
	router.GET("/:domain/*path", handlers.DownloadHandler)

	// Run the server
	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := router.Run(cfg.ServerPort); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
