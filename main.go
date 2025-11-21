package main

import (
	"catalyst/database"
	postgres_server "catalyst/internal/postgres"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catalyst/internal/config"
	"catalyst/internal/models"
	"catalyst/internal/server"
	prom "catalyst/prometheus"

	_ "modernc.org/sqlite"
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
	prom.InitMetrics()

	// Create server manager
	manager := server.NewManager()
	postgresManager := postgres_server.NewPostgresManager()

	configDirPath := *configDir
	if configDirPath == "" {
		configDirPath = config.GetConfigDir()
	}

	for _, cfg := range configs {
		if err := manager.CreateServers(cfg); err != nil {
			log.Fatalf("Error creating http servers: %v", err)
		}
		if err := postgresManager.CreateServers(cfg); err != nil {
			log.Fatalf("Error creating postgres servers: %v", err)
		}
	}

	// Create batch manager for API server
	db, err := database.InitDB("./database.db")
	if err != nil {
		log.Fatalf("Error initializing database for API: %v", err)
	}

	batchConfig := database.BatchConfig{
		BatchSize:     20,
		FlushInterval: 2 * time.Second,
		MaxQueueSize:  50000,
		MaxBatchQueue: 50000,
		MaxWorkers:    3,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}
	batchManager := database.NewBatchManager(db, batchConfig)

	// Start batch manager
	if err := batchManager.Start(); err != nil {
		log.Fatalf("Error starting batch manager for API: %v", err)
	}

	if err := manager.CreateAPIServer(batchManager, configDirPath); err != nil {
		log.Fatalf("Error creating API server: %v", err)
	}

	// Create metrics server on port 9090 (default Prometheus port)
	if err := manager.CreateMetricsServer(4894); err != nil {
		log.Fatalf("Error creating metrics server: %v", err)
	}

	if err := manager.Start(); err != nil {
		log.Fatalf("Error starting servers: %v", err)
	}

	if err := manager.StartAPIServer(); err != nil {
		log.Fatalf("Error starting API server: %v", err)
	}

	if err := manager.StartMetricsServer(); err != nil {
		log.Fatalf("Error starting metrics server: %v", err)
	}

	log.Println("All HTTP servers started successfully")
	log.Println("API server started on port 8282")
	log.Println("Metrics server started on port 4894")

	// if err := postgresManager.Start(); err != nil {
	// 	log.Fatalf("Error starting postgres servers: %v", err)
	// }

	log.Println("All postgres servers started successfully")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")
	manager.Stop()
	//postgresManager.Stop()
	log.Println("Servers stopped")
}
