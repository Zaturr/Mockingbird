package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Catalyst/internal/models"
)

func TestCreateServer(t *testing.T) {
	// Create a server manager
	manager := NewManager()

	// Create a test server configuration
	logger := true
	serverConfig := models.Server{
		Listen: 8080,
		Logger: &logger,
		Location: []models.Location{
			{
				Path:       "/api/test",
				Method:     "GET",
				Response:   `{"message":"test"}`,
				StatusCode: 200,
			},
		},
	}

	// Create the server
	err := manager.CreateServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Check that the server was created
	server, exists := manager.servers[8080]
	if !exists {
		t.Fatal("Server was not created")
	}

	// Check server properties
	if server.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", server.Port)
	}

	if server.Router == nil {
		t.Fatal("Router is nil")
	}

	if server.handler == nil {
		t.Fatal("Handler is nil")
	}

	if len(server.locations) != 1 {
		t.Fatalf("Expected 1 location, got %d", len(server.locations))
	}

	// Test the server with a request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/test", nil)
	server.Router.ServeHTTP(w, req)

	// Check the response
	if w.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if message, ok := response["message"]; !ok || message != "test" {
		t.Errorf("Expected message 'test', got %v", message)
	}
}

func TestCreateServers(t *testing.T) {
	// Create a server manager
	manager := NewManager()

	// Create a test configuration with multiple servers
	logger := true
	config := &models.MockServer{
		Http: models.Http{
			Servers: []models.Server{
				{
					Listen: 8080,
					Logger: &logger,
					Location: []models.Location{
						{
							Path:       "/api/server1",
							Method:     "GET",
							Response:   `{"server":"1"}`,
							StatusCode: 200,
						},
					},
				},
				{
					Listen: 8081,
					Location: []models.Location{
						{
							Path:       "/api/server2",
							Method:     "GET",
							Response:   `{"server":"2"}`,
							StatusCode: 200,
						},
					},
				},
			},
		},
	}

	// Create the servers
	err := manager.CreateServers(config)
	if err != nil {
		t.Fatalf("Failed to create servers: %v", err)
	}

	// Check that both servers were created
	if len(manager.servers) != 2 {
		t.Fatalf("Expected 2 servers, got %d", len(manager.servers))
	}

	// Check that servers have the correct ports
	if _, exists := manager.servers[8080]; !exists {
		t.Error("Server on port 8080 was not created")
	}

	if _, exists := manager.servers[8081]; !exists {
		t.Error("Server on port 8081 was not created")
	}

	// Test the first server
	server1 := manager.servers[8080]
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/api/server1", nil)
	server1.Router.ServeHTTP(w1, req1)

	// Check the response
	if w1.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w1.Code)
	}

	var response1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &response1); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if server, ok := response1["server"]; !ok || server != "1" {
		t.Errorf("Expected server '1', got %v", server)
	}

	// Test the second server
	server2 := manager.servers[8081]
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/server2", nil)
	server2.Router.ServeHTTP(w2, req2)

	// Check the response
	if w2.Code != 200 {
		t.Errorf("Expected status code 200, got %d", w2.Code)
	}

	var response2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &response2); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if server, ok := response2["server"]; !ok || server != "2" {
		t.Errorf("Expected server '2', got %v", server)
	}
}

func TestStartStop(t *testing.T) {
	// Create a server manager
	manager := NewManager()

	// Create a test server configuration
	serverConfig := models.Server{
		Listen: 8082, // Use a different port to avoid conflicts
		Location: []models.Location{
			{
				Path:       "/api/test",
				Method:     "GET",
				Response:   `{"message":"test"}`,
				StatusCode: 200,
			},
		},
	}

	// Create the server
	err := manager.CreateServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start the server in a goroutine
	go func() {
		if err := manager.Start(); err != nil {
			t.Errorf("Failed to start server: %v", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	manager.Stop()

	// Wait for the server to stop
	manager.Wait()
}