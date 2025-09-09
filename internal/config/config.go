package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"mockingbird/internal/models"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads a mock server configuration from a YAML file
func LoadConfig(filePath string) (*models.MockServer, error) {
	// Read the YAML file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Parse the YAML into the MockServer struct
	var config models.MockServer
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}

	// Validate the configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// LoadConfigFromDir loads all YAML configuration files from a directory
func LoadConfigFromDir(dirPath string) ([]*models.MockServer, error) {
	// Get all YAML files in the directory
	files, err := filepath.Glob(filepath.Join(dirPath, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error finding YAML files: %w", err)
	}

	// Also check for .yml files
	ymlFiles, err := filepath.Glob(filepath.Join(dirPath, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("error finding YML files: %w", err)
	}

	files = append(files, ymlFiles...)

	if len(files) == 0 {
		return nil, fmt.Errorf("no YAML configuration files found in %s", dirPath)
	}

	// Load each configuration file
	var configs []*models.MockServer
	for _, file := range files {
		config, err := LoadConfig(file)
		if err != nil {
			return nil, fmt.Errorf("error loading config from %s: %w", file, err)
		}
		configs = append(configs, config)
	}

	return configs, nil
}

// SaveConfig saves a mock server configuration to a YAML file
func SaveConfig(config *models.MockServer, filePath string) error {
	// Marshal the config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	// Write the YAML to the file
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// validateConfig validates a mock server configuration
func validateConfig(config *models.MockServer) error {
	if len(config.Http.Servers) == 0 {
		return fmt.Errorf("no servers defined in configuration")
	}

	for i, server := range config.Http.Servers {
		if server.Listen <= 0 {
			return fmt.Errorf("server %d has invalid listen port: %d", i, server.Listen)
		}

		if len(server.Location) == 0 {
			return fmt.Errorf("server %d has no locations defined", i)
		}

		for j, location := range server.Location {
			if location.Path == "" {
				return fmt.Errorf("server %d, location %d has empty path", i, j)
			}

			if location.Method == "" {
				return fmt.Errorf("server %d, location %d has empty method", i, j)
			}

			if location.StatusCode <= 0 {
				return fmt.Errorf("server %d, location %d has invalid status code: %d", i, j, location.StatusCode)
			}
		}
	}

	return nil
}

// GetConfigDir returns the directory where configuration files are stored
func GetConfigDir() string {
	// Check if CONFIG_DIR environment variable is set
	configDir := os.Getenv("CONFIG_DIR")
	if configDir != "" {
		return configDir
	}

	// Default to ./config
	return "./config"
}

// GetLogSettings returns the default logging configuration
func GetLogSettings() *models.LogSettings {
	return &models.LogSettings{
		Console:           true,
		BeutifyConsoleLog: true,
		File:              true,
		Path:              "./logs/mockingbird.log",
		MinLevel:          "info",
		RotationMaxSizeMB: 100,
		MaxAgeDay:         30,
		MaxBackups:        5,
		Compress:          true,
	}
}
