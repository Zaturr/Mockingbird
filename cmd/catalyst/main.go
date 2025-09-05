package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"Catalyst/internal/config"
	"Catalyst/internal/models"
	"Catalyst/internal/server"
)

func main() {
	// Parse command line flags
	configDir := flag.String("config", "", "Directory containing YAML configuration files")
	configFile := flag.String("file", "", "Path to a specific YAML configuration file")
	flag.Parse()

	// Determine configuration source
	var (
		configs []*models.MockServer
		err     error
	)

	if *configFile != "" {
		// Load a specific configuration file
		cfg, err := config.LoadConfig(*configFile)
		if err != nil {
			log.Fatalf("Error loading configuration file: %v", err)
		}
		configs = []*models.MockServer{cfg}
	} else {
		// Load all configuration files from directory
		dir := *configDir
		if dir == "" {
			dir = config.GetConfigDir()
		}

		configs, err = config.LoadConfigFromDir(dir)
		if err != nil {
			log.Fatalf("Error loading configuration files: %v", err)
		}
	}

	// Create server manager
	manager := server.NewManager()

	// Create servers from configurations
	for _, cfg := range configs {
		if err := manager.CreateServers(cfg); err != nil {
			log.Fatalf("Error creating servers: %v", err)
		}
	}

	// Start all servers
	if err := manager.Start(); err != nil {
		log.Fatalf("Error starting servers: %v", err)
	}

	log.Println("All servers started successfully")

	// Wait for interrupt signal to gracefully shut down the servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")
	manager.Stop()
	log.Println("Servers stopped")
}
