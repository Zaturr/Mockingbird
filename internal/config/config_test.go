package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalyst/internal/models"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	// Sample configuration
	configData := `http:
  servers:
    - listen: 8080
      logger: true
      location:
        - path: /api/test
          method: GET
          response: '{"test": true}'
          status_code: 200
        - path: /api/echo
          method: POST
          schema: |
            {
              "type": "object",
              "properties": {
                "message": { "type": "string" }
              },
              "required": ["message"]
            }
          response: '{"echo": "{{.message}}"}'
          status_code: 200
`

	// Write the test file
	if err := os.WriteFile(testFile, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test loading the configuration
	config, err := LoadConfig(testFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify the configuration
	if config == nil {
		t.Fatal("LoadConfig returned nil config")
	}

	if len(config.Http.Servers) != 1 {
		t.Fatalf("Expected 1 server, got %d", len(config.Http.Servers))
	}

	server := config.Http.Servers[0]
	if server.Listen != 8080 {
		t.Errorf("Expected listen port 8080, got %d", server.Listen)
	}

	if server.Logger == nil || !*server.Logger {
		t.Errorf("Expected logger to be true")
	}

	if len(server.Location) != 2 {
		t.Fatalf("Expected 2 locations, got %d", len(server.Location))
	}

	// Test first location
	location1 := server.Location[0]
	if location1.Path != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", location1.Path)
	}

	if location1.Method != "GET" {
		t.Errorf("Expected method GET, got %s", location1.Method)
	}

	if location1.Response != `{"test": true}` {
		t.Errorf("Expected response '{\"test\": true}', got %s", location1.Response)
	}

	if location1.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", location1.StatusCode)
	}

	// Test second location with schema
	location2 := server.Location[1]
	if location2.Path != "/api/echo" {
		t.Errorf("Expected path /api/echo, got %s", location2.Path)
	}

	if location2.Method != "POST" {
		t.Errorf("Expected method POST, got %s", location2.Method)
	}

	if location2.Response != `{"echo": "{{.message}}"}` {
		t.Errorf("Expected response '{\"echo\": \"{{.message}}\"}', got %s", location2.Response)
	}

	if location2.StatusCode != 200 {
		t.Errorf("Expected status code 200, got %d", location2.StatusCode)
	}

	// Verify schema is present and contains required fields
	if location2.Schema == "" {
		t.Error("Schema should not be empty")
	}

	// Check that schema contains required JSON schema fields
	if !strings.Contains(location2.Schema, `"type": "object"`) {
		t.Error("Schema should contain 'type: object'")
	}
	if !strings.Contains(location2.Schema, `"message"`) {
		t.Error("Schema should contain 'message' property")
	}
	if !strings.Contains(location2.Schema, `"required"`) {
		t.Error("Schema should contain 'required' field")
	}
}

func TestLoadConfigFromDir(t *testing.T) {
	// Create a temporary test directory
	tempDir := t.TempDir()

	// Create multiple test files
	testFiles := []struct {
		name string
		data string
	}{
		{
			name: "server1.yaml",
			data: `http:
  servers:
    - listen: 8080
      location:
        - path: /api/test1
          method: GET
          response: '{"test": 1}'
          status_code: 200
`,
		},
		{
			name: "server2.yml",
			data: `http:
  servers:
    - listen: 8081
      location:
        - path: /api/test2
          method: POST
          response: '{"test": 2}'
          status_code: 201
`,
		},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tempDir, tf.name)
		if err := os.WriteFile(filePath, []byte(tf.data), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", tf.name, err)
		}
	}

	// Test loading configurations from directory
	configs, err := LoadConfigFromDir(tempDir)
	if err != nil {
		t.Fatalf("LoadConfigFromDir failed: %v", err)
	}

	// Verify the configurations
	if len(configs) != 2 {
		t.Fatalf("Expected 2 configs, got %d", len(configs))
	}

	// Check that we have servers with the expected ports
	ports := make(map[int]bool)
	for _, config := range configs {
		for _, server := range config.Http.Servers {
			ports[server.Listen] = true
		}
	}

	if !ports[8080] {
		t.Errorf("Expected server with port 8080")
	}

	if !ports[8081] {
		t.Errorf("Expected server with port 8081")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    *models.MockServer
		expectErr bool
	}{
		{
			name: "Valid config",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen: 8080,
							Location: []models.Location{
								{
									Path:       "/api/test",
									Method:     "GET",
									StatusCode: 200,
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "No servers",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid listen port",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen: 0,
							Location: []models.Location{
								{
									Path:       "/api/test",
									Method:     "GET",
									StatusCode: 200,
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "No locations",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen:   8080,
							Location: []models.Location{},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Empty path",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen: 8080,
							Location: []models.Location{
								{
									Path:       "",
									Method:     "GET",
									StatusCode: 200,
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Empty method",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen: 8080,
							Location: []models.Location{
								{
									Path:       "/api/test",
									Method:     "",
									StatusCode: 200,
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Invalid status code",
			config: &models.MockServer{
				Http: models.Http{
					Servers: []models.Server{
						{
							Listen: 8080,
							Location: []models.Location{
								{
									Path:       "/api/test",
									Method:     "GET",
									StatusCode: 0,
								},
							},
						},
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateConfig() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}
