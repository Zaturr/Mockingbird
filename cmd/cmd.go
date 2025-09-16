package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"catalyst/internal/config"
	"catalyst/internal/models"
	"catalyst/internal/server"
)

// ServerManager wraps the server manager
type ServerManager struct {
	manager    *server.Manager
	configDir  string
	configFile string
}

// Multiport creates a new server manager
func Multiport() *ServerManager {
	return &ServerManager{
		manager: server.NewManager(),
	}
}

// SetConfigDir sets the configuration directory
func (sm *ServerManager) SetConfigDir(dir string) {
	sm.configDir = dir
}

// SetConfigFile sets the configuration file
func (sm *ServerManager) SetConfigFile(file string) {
	sm.configFile = file
}

// StartAll starts all servers
func (sm *ServerManager) StartAll() error {
	// Determine configuration source
	var (
		configs []*models.MockServer
		err     error
	)

	if sm.configFile != "" {
		// Load a specific configuration file
		cfg, err := config.LoadConfig(sm.configFile)
		if err != nil {
			return err
		}
		configs = []*models.MockServer{cfg}
	} else {
		// Load all configuration files from directory
		dir := sm.configDir
		if dir == "" {
			dir = config.GetConfigDir()
		}

		configs, err = config.LoadConfigFromDir(dir)
		if err != nil {
			return err
		}
	}

	// Create servers from configurations
	for _, cfg := range configs {
		if err := sm.manager.CreateServers(cfg); err != nil {
			return err
		}
	}

	// Start all servers
	if err := sm.manager.Start(); err != nil {
		return err
	}

	log.Println("All servers started successfully")

	// Wait for interrupt signal to gracefully shut down the servers
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")
	sm.manager.Stop()
	log.Println("Servers stopped")

	return nil
}
