package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

// ServerConfigUpdateRequest represents a generic server configuration update request
type ServerConfigUpdateRequest struct {
	ServerName string        `json:"server_name" binding:"required" validate:"min=1,max=100"`
	Config     *ServerConfig `json:"config" binding:"required"`
}

// Validate validates the ServerConfigUpdateRequest
func (r *ServerConfigUpdateRequest) Validate() error {

	if r.Config.Location == nil {
		return errors.New("location is required")
	}
	if r.Config.Listen <= 0 {
		return errors.New("listen is required and must be greater than 0")
	}
	if r.Config.Listen == r.Config.Controlport {
		return errors.New("listen and controlport cannot be the same")
	}

	return nil
}

// Headers represents HTTP headers as a map
type Headers map[string]string

// Async represents async request configuration
type Async struct {
	Url        string   `yaml:"url" json:"url"`
	Body       string   `yaml:"body" json:"body"`
	Method     string   `yaml:"method" json:"method"`
	Headers    *Headers `yaml:"headers" json:"headers"`
	Timeout    *int     `yaml:"timeout" json:"timeout"`
	Retries    *int     `yaml:"retries" json:"retries"`
	RetryDelay *int     `yaml:"retry_delay" json:"retryDelay"`
}

// ProbabilityString is a custom type that can unmarshal from both number and string
type ProbabilityString string

// UnmarshalJSON allows ProbabilityString to accept both number and string values
func (p *ProbabilityString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as number first
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*p = ProbabilityString(fmt.Sprintf("%g", num))
		return nil
	}
	// If not a number, try as string
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*p = ProbabilityString(str)
	return nil
}

// MarshalJSON converts ProbabilityString back to JSON
func (p ProbabilityString) MarshalJSON() ([]byte, error) {
	// Try to parse as float to see if it's a number
	if num, err := strconv.ParseFloat(string(p), 64); err == nil {
		return json.Marshal(num)
	}
	return json.Marshal(string(p))
}

// chaosInjection represents chaos injection configuration
type chaos_injection struct {
	Latency *Latency `yaml:"latency,omitempty" json:"latency,omitempty"`
	Abort   *Abort   `yaml:"abort,omitempty" json:"abort,omitempty"`
	Error   *Error   `yaml:"error,omitempty" json:"error,omitempty"`
}

// Latency represents latency configuration for chaos injection
type Latency struct {
	Time        int               `yaml:"time" json:"time"`
	Probability ProbabilityString `yaml:"probability" json:"probability"`
}

// Abort represents abort configuration for chaos injection
type Abort struct {
	Code        int               `yaml:"code" json:"code"`
	Probability ProbabilityString `yaml:"probability" json:"probability"`
}

// Error represents error configuration for chaos injection
type Error struct {
	Code        int               `yaml:"code" json:"code"`
	Probability ProbabilityString `yaml:"probability" json:"probability"`
	Response    string            `yaml:"response" json:"response"`
}

// ServerLocation represents a server location configuration
type ServerLocation struct {
	Path            string           `yaml:"path" json:"path" validate:"required"`
	Method          string           `yaml:"method" json:"method" validate:"required,oneof=GET POST PUT DELETE PATCH"`
	Response        string           `yaml:"response" json:"response,omitempty"`
	Body            string           `yaml:"body" json:"body,omitempty"`
	StatusCode      int              `yaml:"status_code" json:"status_code" validate:"min=100,max=599"`
	Headers         *Headers         `yaml:"headers" json:"headers"`
	Schema          string           `yaml:"schema" json:"schema"`
	Chaos_injection *chaos_injection `yaml:"chaos_injection" json:"chaos_injection"`
	Async           []Async          `yaml:"async,omitempty" json:"async,omitempty"`
}

// ServerConfig represents a server configuration
type ServerConfig struct {
	Listen      int              `yaml:"listen" json:"listen" validate:"min=1,max=65535"`
	Controlport int              `yaml:"port" json:"port,omitempty" validate:"min=1,max=65535"`
	Logger      bool             `yaml:"logger" json:"logger"`
	Name        string           `yaml:"name" json:"name" validate:"required"`
	Version     string           `yaml:"version" json:"version" validate:"required"`
	Location    []ServerLocation `yaml:"location" json:"location"`
	LoggerPath  string           `yaml:"logger_path" json:"logger_path"`
}

// HTTPConfig represents HTTP configuration
type HTTPConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers" validate:"required,min=1"`
}

// YamlConfig represents the complete YAML configuration structure
type YamlConfig struct {
	HTTP        HTTPConfig `yaml:"http" json:"http" validate:"required"`
	TestSetting string     `yaml:"test_setting,omitempty" json:"test_setting,omitempty"`
	RestartTest string     `yaml:"restart_test,omitempty" json:"restart_test,omitempty"`
	NewField    string     `yaml:"new_field,omitempty" json:"new_field,omitempty"`
}

// ConfigUpdateRequest represents a configuration update request
type ConfigUpdateRequest struct {
	ServerName string     `json:"server_name" binding:"required" validate:"min=1,max=100"`
	Config     YamlConfig `json:"config" binding:"required"`
}

// Validate validates the ConfigUpdateRequest
func (r *ConfigUpdateRequest) Validate() error {
	if strings.TrimSpace(r.ServerName) == "" {
		return errors.New("server_name is required and cannot be empty")
	}
	if len(r.ServerName) > 100 {
		return errors.New("server_name cannot exceed 100 characters")
	}
	if len(r.Config.HTTP.Servers) == 0 {
		return errors.New("at least one server configuration is required")
	}

	// Valida que no haya puertos duplicados entre servidores
	portMap := make(map[int]bool)
	for i, server := range r.Config.HTTP.Servers {
		// Validar Listen
		if portMap[server.Listen] {
			return fmt.Errorf("duplicate listen port %d found in server %d", server.Listen, i)
		}
		portMap[server.Listen] = true

		// Validar Controlport
		if server.Controlport > 0 {
			if portMap[server.Controlport] {
				return fmt.Errorf("duplicate controlport %d found in server %d", server.Controlport, i)
			}
			portMap[server.Controlport] = true

			// Validar que Listen y Controlport no sean iguales en el mismo servidor
			if server.Listen == server.Controlport {
				return fmt.Errorf("server %d: listen and controlport cannot be the same (%d)", i, server.Listen)
			}
		}
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
