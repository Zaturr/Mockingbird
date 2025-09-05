package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"Catalyst/internal/models"
	"github.com/gin-gonic/gin"
)

func TestHandleRequest(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new handler
	h := NewHandler()

	// Test cases
	tests := []struct {
		name           string
		location       models.Location
		method         string
		requestBody    string
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "Simple GET request",
			location: models.Location{
				Path:       "/api/test",
				Method:     "GET",
				Response:   `{"message":"test"}`,
				StatusCode: 200,
			},
			method:         "GET",
			expectedStatus: 200,
			expectedBody:   `{"message":"test"}`,
		},
		{
			name: "Request with headers",
			location: models.Location{
				Path:       "/api/headers",
				Method:     "GET",
				Response:   `{"message":"headers"}`,
				StatusCode: 200,
				Headers: &models.Headers{
					"X-Test-Header": "test-value",
				},
			},
			method:         "GET",
			expectedStatus: 200,
			expectedBody:   `{"message":"headers"}`,
		},
		{
			name: "POST request with valid JSON",
			location: models.Location{
				Path:       "/api/post",
				Method:     "POST",
				Response:   `{"message":"post"}`,
				StatusCode: 201,
			},
			method:         "POST",
			requestBody:    `{"data":"test"}`,
			expectedStatus: 201,
			expectedBody:   `{"message":"post"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Register the location
			if err := h.RegisterLocation(tt.location); err != nil {
				t.Fatalf("Failed to register location: %v", err)
			}

			// Create a test request
			var req *http.Request
			if tt.requestBody != "" {
				req = httptest.NewRequest(tt.method, tt.location.Path, bytes.NewBufferString(tt.requestBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.location.Path, nil)
			}

			// Create a test response recorder
			w := httptest.NewRecorder()

			// Create a Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Handle the request
			h.HandleRequest(c, tt.location)

			// Check the response
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}

			// Check headers if specified
			if tt.location.Headers != nil {
				for key, value := range *tt.location.Headers {
					if w.Header().Get(key) != value {
						t.Errorf("Expected header %s=%s, got %s", key, value, w.Header().Get(key))
					}
				}
			}
		})
	}
}

func TestSchemaValidation(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new handler
	h := NewHandler()

	// Define a location with schema validation
	location := models.Location{
		Path:   "/api/validate",
		Method: "POST",
		Schema: `{
			"type": "object",
			"properties": {
				"name": { "type": "string" },
				"age": { "type": "integer", "minimum": 18 }
			},
			"required": ["name", "age"]
		}`,
		Response:   `{"message":"valid"}`,
		StatusCode: 200,
	}

	// Register the location
	if err := h.RegisterLocation(location); err != nil {
		t.Fatalf("Failed to register location: %v", err)
	}

	// Test cases
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "Valid request",
			requestBody:    `{"name":"John","age":25}`,
			expectedStatus: 200,
		},
		{
			name:           "Missing required field",
			requestBody:    `{"name":"John"}`,
			expectedStatus: 400,
		},
		{
			name:           "Invalid type",
			requestBody:    `{"name":"John","age":"25"}`,
			expectedStatus: 400,
		},
		{
			name:           "Value below minimum",
			requestBody:    `{"name":"John","age":17}`,
			expectedStatus: 400,
		},
		{
			name:           "Invalid JSON",
			requestBody:    `{"name":"John","age":25`,
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest("POST", "/api/validate", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")

			// Create a test response recorder
			w := httptest.NewRecorder()

			// Create a Gin context
			c, _ := gin.CreateTestContext(w)
			c.Request = req

			// Handle the request
			h.HandleRequest(c, location)

			// Check the response status
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status code %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}