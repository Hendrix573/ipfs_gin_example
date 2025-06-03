package config

import (
	"log"
	"os"
	"path/filepath"
)

type Config struct {
	BadgerDBPath string
	ServerPort   string
	ChunkSize    int // Size for content chunking
}

func LoadConfig() *Config {

	dbPath := filepath.Join(".", "data", "badger")
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		log.Fatalf("Failed to create database directory: %v", err)
	}

	return &Config{
		BadgerDBPath: dbPath,
		ServerPort:   ":8080",
		ChunkSize:    256 * 1024, // 256KB
	}
}
