package api

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// APIResponse represents a standard API response structure
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
}

// NewSuccessResponse creates a successful API response
func NewSuccessResponse(data interface{}, message ...string) *APIResponse {
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}
	return &APIResponse{
		Success: true,
		Message: msg,
		Data:    data,
		Code:    200,
	}
}

// NewErrorResponse creates an error API response
func NewErrorResponse(err error, code int, message ...string) *APIResponse {
	msg := err.Error()
	if len(message) > 0 {
		msg = message[0]
	}
	return &APIResponse{
		Success: false,
		Error:   msg,
		Code:    code,
	}
}

// ConfigUpdateRequest represents a generic configuration update request
type ConfigUpdateRequest struct {
	ServerName string                 `json:"server_name" binding:"required" validate:"min=1,max=100"`
	Config     map[string]interface{} `json:"config" binding:"required"`
}

// Validate validates the ConfigUpdateRequest
func (r *ConfigUpdateRequest) Validate() error {
	if strings.TrimSpace(r.ServerName) == "" {
		return errors.New("server_name is required and cannot be empty")
	}
	if len(r.ServerName) > 100 {
		return errors.New("server_name cannot exceed 100 characters")
	}
	if r.Config == nil {
		return errors.New("config is required")
	}
	return nil
}

// ServerLocation represents a server location configuration
type ServerLocation struct {
	Path       string            `yaml:"path" json:"path" validate:"required"`
	Method     string            `yaml:"method" json:"method" validate:"required,oneof=GET POST PUT DELETE PATCH"`
	Response   string            `yaml:"response" json:"response"`
	StatusCode int               `yaml:"status_code" json:"status_code" validate:"min=100,max=599"`
	Headers    map[string]string `yaml:"headers" json:"headers"`
	Schema     string            `yaml:"schema" json:"schema"`
}

// ServerConfig represents a server configuration
type ServerConfig struct {
	Listen     int              `yaml:"listen" json:"listen" validate:"min=1,max=65535"`
	Logger     bool             `yaml:"logger" json:"logger"`
	Name       string           `yaml:"name" json:"name" validate:"required"`
	Version    string           `yaml:"version" json:"version" validate:"required"`
	Location   []ServerLocation `yaml:"location" json:"location"`
	LoggerPath string           `yaml:"logger_path" json:"logger_path"`
}

// HTTPConfig represents HTTP configuration
type HTTPConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers" validate:"required,min=1"`
}

// BancrecerConfig represents the complete Bancrecer configuration structure
type BancrecerConfig struct {
	HTTP        HTTPConfig `yaml:"http" json:"http" validate:"required"`
	TestSetting string     `yaml:"test_setting,omitempty" json:"test_setting,omitempty"`
	RestartTest string     `yaml:"restart_test,omitempty" json:"restart_test,omitempty"`
	NewField    string     `yaml:"new_field,omitempty" json:"new_field,omitempty"`
}

// BancrecerConfigUpdateRequest represents a Bancrecer-specific configuration update request
type BancrecerConfigUpdateRequest struct {
	ServerName string          `json:"server_name" binding:"required" validate:"min=1,max=100"`
	Config     BancrecerConfig `json:"config" binding:"required"`
}

// Validate validates the BancrecerConfigUpdateRequest
func (r *BancrecerConfigUpdateRequest) Validate() error {
	if strings.TrimSpace(r.ServerName) == "" {
		return errors.New("server_name is required and cannot be empty")
	}
	if len(r.ServerName) > 100 {
		return errors.New("server_name cannot exceed 100 characters")
	}
	if len(r.Config.HTTP.Servers) == 0 {
		return errors.New("at least one server configuration is required")
	}
	return nil
}

// DatabaseRecord represents a database record for API responses
type DatabaseRecord struct {
	UUID               string    `json:"uuid" validate:"required"`
	RecepcionID        string    `json:"recepcion_id"`
	SenderID           string    `json:"sender_id"`
	RequestMethod      string    `json:"request_method" validate:"required"`
	RequestEndpoint    string    `json:"request_endpoint" validate:"required"`
	RequestBody        string    `json:"request_body"`
	ResponseBody       string    `json:"response_body"`
	ResponseStatusCode int       `json:"response_status_code" validate:"min=100,max=599"`
	Timestamp          time.Time `json:"timestamp" validate:"required"`
}

// ToAPIFormat converts DatabaseRecord to API format with string timestamp
func (dr *DatabaseRecord) ToAPIFormat() map[string]interface{} {
	return map[string]interface{}{
		"uuid":                 dr.UUID,
		"recepcion_id":         dr.RecepcionID,
		"sender_id":            dr.SenderID,
		"request_method":       dr.RequestMethod,
		"request_endpoint":     dr.RequestEndpoint,
		"request_body":         dr.RequestBody,
		"response_body":        dr.ResponseBody,
		"response_status_code": dr.ResponseStatusCode,
		"timestamp":            dr.Timestamp.Format("2006-01-02 15:04:05"),
	}
}

// Error definitions for better error handling
var (
	ErrControllerClosed      = errors.New("controller is closed")
	ErrChannelFull           = errors.New("restart channel is full")
	ErrInvalidServer         = errors.New("invalid server name")
	ErrConfigNotFound        = errors.New("configuration file not found")
	ErrConfigInvalid         = errors.New("invalid configuration")
	ErrManagerAlreadyRunning = errors.New("restart manager is already running")
)

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", ve.Field, ve.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	var messages []string
	for _, err := range ve {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}
