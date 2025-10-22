package api

import (
	"catalyst/database"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// APIHandler handles REST API endpoints with improved structure and error handling
type APIHandler struct {
	batchManager *database.BatchManager
	configDir    string
	restartChan  chan string
	timeout      time.Duration
}

// ConfigService handles configuration operations
type ConfigService struct {
	configDir string
	timeout   time.Duration
}

// DatabaseService handles database operations
type DatabaseService struct {
	batchManager *database.BatchManager
	timeout      time.Duration
}

// NewAPIHandler creates a new APIHandler instance
func NewAPIHandler(batchManager *database.BatchManager, configDir string, restartChan chan string) *APIHandler {
	return &APIHandler{
		batchManager: batchManager,
		configDir:    configDir,
		restartChan:  restartChan,
		timeout:      30 * time.Second,
	}
}

// NewConfigService creates a new ConfigService instance
func NewConfigService(configDir string) *ConfigService {
	return &ConfigService{
		configDir: configDir,
		timeout:   30 * time.Second,
	}
}

// NewDatabaseService creates a new DatabaseService instance
func NewDatabaseService(batchManager *database.BatchManager) *DatabaseService {
	return &DatabaseService{
		batchManager: batchManager,
		timeout:      30 * time.Second,
	}
}

// GetData handles GET /api/mock/data - retrieves all records from database
func (h *APIHandler) GetData(c *gin.Context) {
	log.Printf("GET /api/mock/data - Retrieving all records from database")

	if h.batchManager == nil {
		log.Printf("ERROR: Database not available for GET /api/mock/data")
		c.JSON(http.StatusInternalServerError, NewErrorResponse(ErrConfigNotFound, http.StatusInternalServerError, "Database not available"))
		return
	}

	dbService := NewDatabaseService(h.batchManager)
	records, err := dbService.GetAllRecords()
	if err != nil {
		log.Printf("ERROR: Failed to retrieve data from database: %v", err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err, http.StatusInternalServerError, "Error retrieving data"))
		return
	}

	var apiRecords []map[string]interface{}
	for _, record := range records {
		apiRecords = append(apiRecords, record.ToAPIFormat())
	}

	log.Printf("SUCCESS: Retrieved %d records from database", len(apiRecords))
	c.JSON(http.StatusOK, apiRecords)
}

// GetConfig handles GET /api/mock/config - retrieves configuration with real structure
func (h *APIHandler) GetConfig(c *gin.Context) {
	serverName := strings.TrimSpace(c.Query("server_name"))
	if serverName == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrInvalidServer, http.StatusBadRequest, "server_name parameter is required"))
		return
	}

	configService := NewConfigService(h.configDir)
	config, err := configService.GetConfig(serverName)
	if err != nil {
		log.Printf("ERROR: Failed to get config for server %s: %v", serverName, err)
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, NewErrorResponse(err, http.StatusNotFound, fmt.Sprintf("Configuration file not found: %s", serverName)))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(err, http.StatusInternalServerError, "Error retrieving configuration"))
		}
		return
	}

	log.Printf("SUCCESS: Retrieved configuration for server: %s", serverName)
	c.JSON(http.StatusOK, config)
}

// UpdateConfig handles PUT /api/mock/config - updates configuration
func (h *APIHandler) UpdateConfig(c *gin.Context) {
	serverName := strings.TrimSpace(c.Query("server_name"))
	if serverName == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse(ErrInvalidServer, http.StatusBadRequest, "server_name parameter is required"))
		return
	}

	// Read and parse request body
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err, http.StatusBadRequest, "Error reading request body"))
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err, http.StatusBadRequest, "Invalid JSON format"))
		return
	}

	// Update configuration using service
	configService := NewConfigService(h.configDir)
	updatedConfig, err := configService.UpdateConfig(serverName, config)
	if err != nil {
		log.Printf("ERROR: Failed to update config for server %s: %v", serverName, err)
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, NewErrorResponse(err, http.StatusNotFound, fmt.Sprintf("Configuration file not found: %s", serverName)))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(err, http.StatusInternalServerError, "Error updating configuration"))
		}
		return
	}

	log.Printf("SUCCESS: Updated configuration for server: %s", serverName)
	c.JSON(http.StatusOK, updatedConfig)

	// Notify restart after successful config update
	if err := h.notifyRestart(serverName); err != nil {
		log.Printf("WARNING: Failed to notify restart for server %s: %v", serverName, err)
	}
}

// GetAllRecords retrieves all records from the database
func (ds *DatabaseService) GetAllRecords() ([]DatabaseRecord, error) {
	if ds.batchManager == nil || ds.batchManager.DB == nil {
		return nil, fmt.Errorf("database not available")
	}

	db := ds.batchManager.DB
	query := `SELECT uuid, recepcion_id, sender_id, request_headers, request_method, 
			  request_endpoint, request_body, response_headers, response_body, 
			  response_status_code, timestamp FROM mock_transactions ORDER BY timestamp DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}
	defer rows.Close()

	var records []DatabaseRecord
	for rows.Next() {
		var record DatabaseRecord
		var requestHeaders, responseHeaders string

		err := rows.Scan(
			&record.UUID,
			&record.RecepcionID,
			&record.SenderID,
			&requestHeaders,
			&record.RequestMethod,
			&record.RequestEndpoint,
			&record.RequestBody,
			&responseHeaders,
			&record.ResponseBody,
			&record.ResponseStatusCode,
			&record.Timestamp,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		records = append(records, record)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return records, nil
}

// GetConfig retrieves configuration for a specific server
func (cs *ConfigService) GetConfig(serverName string) (map[string]interface{}, error) {
	if strings.TrimSpace(serverName) == "" {
		return nil, ErrInvalidServer
	}

	configFile, found := cs.findConfigFile(serverName)
	if !found {
		return nil, ErrConfigNotFound
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// UpdateConfig updates configuration for a specific server
func (cs *ConfigService) UpdateConfig(serverName string, config map[string]interface{}) (map[string]interface{}, error) {
	if strings.TrimSpace(serverName) == "" {
		return nil, ErrInvalidServer
	}

	configFile, found := cs.findConfigFile(serverName)
	if !found {
		return nil, ErrConfigNotFound
	}

	// Convert config to YAML
	updatedConfig, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write updated configuration
	if err := os.WriteFile(configFile, updatedConfig, 0644); err != nil {
		return nil, fmt.Errorf("failed to write config file: %w", err)
	}

	return config, nil
}

// findConfigFile finds the configuration file for a server, trying different extensions
func (cs *ConfigService) findConfigFile(serverName string) (string, bool) {
	extensions := []string{".yml", ".yaml"}
	for _, ext := range extensions {
		configFile := filepath.Join(cs.configDir, serverName+ext)
		if _, err := os.Stat(configFile); err == nil {
			return configFile, true
		}
	}
	return "", false
}

// UpdateBancrecerConfig handles specific updates for bancrecer.yml structure
func (h *APIHandler) UpdateBancrecerConfig(c *gin.Context) {
	var req BancrecerConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err, http.StatusBadRequest, "Invalid request format"))
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse(err, http.StatusBadRequest, "Validation failed"))
		return
	}

	configService := NewConfigService(h.configDir)

	// Get current config
	currentConfig, err := configService.GetConfig(req.ServerName)
	if err != nil {
		log.Printf("ERROR: Failed to get current config for server %s: %v", req.ServerName, err)
		if err == ErrConfigNotFound {
			c.JSON(http.StatusNotFound, NewErrorResponse(err, http.StatusNotFound, fmt.Sprintf("Configuration file not found: %s", req.ServerName)))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(err, http.StatusInternalServerError, "Error retrieving current configuration"))
		}
		return
	}

	// Update configuration with new values
	if req.Config.TestSetting != "" {
		currentConfig["test_setting"] = req.Config.TestSetting
	}
	if req.Config.RestartTest != "" {
		currentConfig["restart_test"] = req.Config.RestartTest
	}
	if req.Config.NewField != "" {
		currentConfig["new_field"] = req.Config.NewField
	}

	// Update the configuration
	updatedConfig, err := configService.UpdateConfig(req.ServerName, currentConfig)
	if err != nil {
		log.Printf("ERROR: Failed to update config for server %s: %v", req.ServerName, err)
		c.JSON(http.StatusInternalServerError, NewErrorResponse(err, http.StatusInternalServerError, "Error updating configuration"))
		return
	}

	log.Printf("SUCCESS: Updated Bancrecer configuration for server: %s", req.ServerName)
	c.JSON(http.StatusOK, updatedConfig)

	// Notify restart after successful config update
	if err := h.notifyRestart(req.ServerName); err != nil {
		log.Printf("WARNING: Failed to notify restart for server %s: %v", req.ServerName, err)
	}
}

// notifyRestart sends a restart signal for the specified server
func (h *APIHandler) notifyRestart(serverName string) error {
	select {
	case h.restartChan <- serverName:
		log.Printf("Restart signal sent for server: %s", serverName)
		return nil
	default:
		log.Printf("Restart channel full, dropping signal for server: %s", serverName)
		return ErrChannelFull
	}
}
