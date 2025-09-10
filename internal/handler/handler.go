package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"mockingbird/internal/chaos"
	"mockingbird/internal/models"

	"github.com/SOLUCIONESSYCOM/scribe"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Handler manages HTTP request handling based on configuration
type Handler struct {
	chaosEngine *chaos.Engine
	schemas     map[string]*jsonschema.Schema
	Logger      *scribe.Scribe
}

// NewHandler creates a new handler with the given chaos engine
func NewHandler(logger *scribe.Scribe) *Handler {
	return &Handler{
		chaosEngine: chaos.NewEngine(),
		schemas:     make(map[string]*jsonschema.Schema),
		Logger:      logger,
	}
}

// RegisterLocation registers a location with the handler
func (h *Handler) RegisterLocation(location models.Location) error {
	h.Logger.Info().
		Str("path", location.Path).
		Str("method", location.Method).
		Int("status_code", location.StatusCode).
		Msg("Registering location")

	// If schema is provided, compile it
	if location.Schema != "" {
		schema, err := h.compileSchema(location.Schema)
		if err != nil {
			h.Logger.Error().
				Str("path", location.Path).
				Str("method", location.Method).
				AnErr("error", err).
				Msg("Error compiling schema for location")
			return fmt.Errorf("error compiling schema for path %s: %w", location.Path, err)
		}
		h.schemas[location.Path+":"+location.Method] = schema
		h.Logger.Debug().
			Str("path", location.Path).
			Str("method", location.Method).
			Msg("Schema compiled successfully for location")
	}

	return nil
}

// compileSchema compiles a JSON schema
func (h *Handler) compileSchema(schemaStr string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()

	// Parse the schema string as JSON first
	var schemaData interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schemaData); err != nil {
		return nil, fmt.Errorf("error parsing schema JSON: %w", err)
	}

	// Add the schema to the compiler using the parsed data
	if err := compiler.AddResource("schema.json", schemaData); err != nil {
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
	ctx := scribe.WithCtx(c.Request.Context())

	logCtx := scribe.GetLogContext(ctx)

	logCtx.Set("request_trace_id", uuid.New().String())
	r := c.Request.WithContext(ctx)

	c.Request = r

	h.Logger.DebugCtx(ctx).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Str("ip", c.ClientIP()).
		Msg("Handling request")

	// Apply chaos injection if configured
	if location.ChaosInjection != nil {
		if h.chaosEngine.ApplyChaos(c.Writer, location.ChaosInjection) {
			h.Logger.WarnCtx(ctx).Msg("Request aborted by chaos injection")
			return // Request was aborted by chaos injection
		}
	}

	// Validate request body against schema if configured
	if schema, ok := h.schemas[location.Path+":"+location.Method]; ok {
		if err := h.validateRequestBody(c, schema); err != nil {
			h.Logger.ErrorCtx(ctx).AnErr("validation_error", err).Msg("Schema validation failed")
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
		h.Logger.InfoCtx(ctx).
			Str("async_url", location.Async.Url).
			Str("async_method", location.Async.Method).
			Msg("Starting async call")
		go h.handleAsyncCall(location.Async, c)
	}

	// Set response status code
	c.Status(location.StatusCode)

	// Set response body if configured
	if location.Response != "" {
		c.Header("Content-Type", "application/json")

		// Process template if it contains template variables
		responseBody, err := h.processResponseTemplate(c, string(location.Response))
		if err != nil {
			h.Logger.ErrorCtx(ctx).AnErr("template_error", err).Msg("Error processing response template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing response template"})
			return
		}

		h.Logger.InfoCtx(ctx).Str("response", string(responseBody)).Msg("Response processed successfully")
		c.String(location.StatusCode, responseBody)
	}

	h.Logger.InfoCtx(ctx).
		Int("status_code", location.StatusCode).
		Msg("Request completed successfully")
}

// validateRequestBody validates the request body against a JSON schema
func (h *Handler) validateRequestBody(c *gin.Context, schema *jsonschema.Schema) error {
	ctx := c.Request.Context()

	h.Logger.InfoCtx(ctx).Msg("Starting request body validation")

	// Read the request body
	body, err := io.ReadAll(c.Request.Body)

	if err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("error", err).Msg("Error reading request body")
		return fmt.Errorf("error reading request body: %w", err)
	}

	// Restore the request body for later use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	defer c.Request.Body.Close()

	// Parse the JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("error", err).Msg("Error parsing JSON")
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	// Validate against the schema
	if err := schema.Validate(data); err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("validation_error", err).Msg("Schema validation failed")
		return err
	}

	h.Logger.DebugCtx(ctx).Msg("Request body validation successful")

	return nil
}

// handleAsyncCall handles an asynchronous HTTP call
func (h *Handler) handleAsyncCall(async *models.Async, c *gin.Context) {

	ctx := scribe.WithCtx(c.Request.Context())

	lc := scribe.GetLogContext(ctx)
	lc.Set("async_request_trace_id", uuid.New().String())

	r := c.Request.WithContext(ctx)
	c.Request = r

	h.Logger.DebugCtx(ctx).
		Str("url", async.Url).
		Str("method", async.Method).
		Msg("Creating async HTTP request")

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
		h.Logger.ErrorCtx(ctx).
			Str("url", async.Url).
			Str("method", async.Method).
			AnErr("error", err).
			Msg("Error creating async request")
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
			h.Logger.WarnCtx(ctx).
				Str("url", async.Url).
				Int("attempt", i+1).
				Int("max_retries", retries-1).
				AnErr("error", lastErr).
				Msg("Async request failed, retrying")
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	// Handle response
	if lastErr != nil {
		h.Logger.ErrorCtx(ctx).
			Str("url", async.Url).
			Str("method", async.Method).
			Int("retries", retries-1).
			AnErr("error", lastErr).
			Msg("Error executing async request after retries")
		return
	}
	defer resp.Body.Close()

	// Log response status
	h.Logger.InfoCtx(ctx).
		Str("url", async.Url).
		Str("method", async.Method).
		Str("status", resp.Status).
		Int("status_code", resp.StatusCode).
		Msg("Async request completed successfully")
}

// processResponseTemplate processes the response template with request data
func (h *Handler) processResponseTemplate(c *gin.Context, responseTemplate string) (string, error) {
	// Check if template contains template variables
	if !strings.Contains(responseTemplate, "{{") {
		return responseTemplate, nil
	}

	// Parse request body to extract data for template variables
	var requestData interface{}
	if c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return "", fmt.Errorf("error reading request body: %w", err)
		}

		// Restore the request body for potential later use
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) > 0 {
			if err := json.Unmarshal(body, &requestData); err != nil {
				return "", fmt.Errorf("error parsing request JSON: %w", err)
			}
		}
	}

	// Create template with custom functions
	tmpl, err := template.New("response").Funcs(template.FuncMap{
		"toJson": func(v interface{}) string {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(jsonBytes)
		},
		"now": func() string {
			return time.Now().Format(time.RFC3339)
		},
	}).Parse(responseTemplate)

	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Execute template with request data
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, requestData); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}
