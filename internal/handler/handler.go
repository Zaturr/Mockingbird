package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"Catalyst/internal/chaos"
	"Catalyst/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler manages HTTP request handling based on configuration
type Handler struct {
	chaosEngine *chaos.Engine
	schemas     map[string]*jsonschema.Schema
}

// NewHandler creates a new handler with the given chaos engine
func NewHandler() *Handler {
	return &Handler{
		chaosEngine: chaos.NewEngine(),
		schemas:     make(map[string]*jsonschema.Schema),
	}
}

// RegisterLocation registers a location with the handler
func (h *Handler) RegisterLocation(location models.Location) error {
	// If schema is provided, compile it
	if location.Schema != "" {
		schema, err := h.compileSchema(location.Schema)
		if err != nil {
			return fmt.Errorf("error compiling schema for path %s: %w", location.Path, err)
		}
		h.schemas[location.Path+":"+location.Method] = schema
	}

	return nil
}

// compileSchema compiles a JSON schema
func (h *Handler) compileSchema(schemaStr string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()

	// Add the schema to the compiler
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaStr)); err != nil {
		return nil, fmt.Errorf("error adding schema resource: %w", err)
	}

	// Compile the schema
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("error compiling schema: %w", err)
	}

	return schema, nil
}

// HandleRequest handles an HTTP request based on the location configuration
func (h *Handler) HandleRequest(c *gin.Context, location models.Location) {
	// Apply chaos injection if configured
	if location.ChaosInjection != nil {
		if h.chaosEngine.ApplyChaos(c.Writer, location.ChaosInjection) {
			return // Request was aborted by chaos injection
		}
	}

	// Validate request body against schema if configured
	if schema, ok := h.schemas[location.Path+":"+location.Method]; ok {
		if err := h.validateRequestBody(c, schema); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Schema validation failed: %v", err)})
			return
		}
	}

	// Set response headers if configured
	if location.Headers != nil {
		for key, value := range *location.Headers {
			c.Header(key, value)
		}
	}

	// Handle async call if configured
	if location.Async != nil {
		go h.handleAsyncCall(location.Async)
	}

	// Set response status code
	c.Status(location.StatusCode)

	// Set response body if configured
	if location.Response != "" {
		c.Header("Content-Type", "application/json")
		c.String(location.StatusCode, location.Response)
	}
}

// validateRequestBody validates the request body against a JSON schema
func (h *Handler) validateRequestBody(c *gin.Context, schema *jsonschema.Schema) error {
	// Read the request body
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		return fmt.Errorf("error reading request body: %w", err)
	}

	// Restore the request body for later use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	defer c.Request.Body.Close()

	// Parse the JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	// Validate against the schema
	if err := schema.Validate(data); err != nil {
		return err
	}

	return nil
}

// handleAsyncCall handles an asynchronous HTTP call
func (h *Handler) handleAsyncCall(async *models.Async) {
	// Create HTTP client with timeout
	client := &http.Client{}
	if async.Timeout != nil {
		client.Timeout = time.Duration(*async.Timeout) * time.Millisecond
	}

	// Create request
	var body io.Reader
	if async.Body != "" {
		body = strings.NewReader(async.Body)
	}

	req, err := http.NewRequest(async.Method, async.Url, body)
	if err != nil {
		fmt.Printf("Error creating async request: %v\n", err)
		return
	}

	// Set headers
	if async.Headers != nil {
		for key, value := range *async.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set default content type if not specified
	if async.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request with retries
	var resp *http.Response
	var lastErr error
	retries := 1
	if async.Retries != nil {
		retries = *async.Retries + 1
	}

	retryDelay := 100 // Default retry delay in milliseconds
	if async.RetryDelay != nil {
		retryDelay = *async.RetryDelay
	}

	for i := 0; i < retries; i++ {
		resp, lastErr = client.Do(req)
		if lastErr == nil {
			break
		}

		if i < retries-1 {
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	// Handle response
	if lastErr != nil {
		fmt.Printf("Error executing async request after %d retries: %v\n", retries-1, lastErr)
		return
	}
	defer resp.Body.Close()

	// Log response status
	fmt.Printf("Async request completed with status: %s\n", resp.Status)
}
