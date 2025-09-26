package postgres_server

import (
	"catalyst/internal/models"
	"catalyst/internal/logger"
	"context"
	"testing"
	"time"
)

func TestNewPostgresManager(t *testing.T) {
	manager := NewPostgresManager()

	if manager == nil {
		t.Fatal("NewPostgresManager returned nil")
	}

	if manager.servers == nil {
		t.Fatal("Servers map is nil")
	}

	if len(manager.servers) != 0 {
		t.Errorf("Expected empty servers map, got %d servers", len(manager.servers))
	}
}

func TestCreateServer(t *testing.T) {
	manager := NewPostgresManager()

	// Create a test logger path and file
	loggerPath := "/tmp"
	loggerEnabled := true

	// Create a test server configuration
	serverConfig := models.PostgresServer{
		Name:       "test-postgres",
		User:       "postgres",
		Password:   "password",
		Host:       "localhost",
		Port:       5432,
		Database:   "testdb",
		InitScript: "",
		LoggerPath: &loggerPath,
		Logger:     &loggerEnabled,
	}

	// Create the server
	err := manager.CreateServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Check that the server was created
	server, exists := manager.servers[5432]
	if !exists {
		t.Fatal("Server was not created")
	}

	// Check server properties
	if server.Name != "test-postgres" {
		t.Errorf("Expected name 'test-postgres', got %s", server.Name)
	}

	if server.User != "postgres" {
		t.Errorf("Expected user 'postgres', got %s", server.User)
	}

	if server.Password != "password" {
		t.Errorf("Expected password 'password', got %s", server.Password)
	}

	if server.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got %s", server.Host)
	}

	if server.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", server.Port)
	}

	if server.Database != "testdb" {
		t.Errorf("Expected database 'testdb', got %s", server.Database)
	}

	if server.logger == nil {
		t.Fatal("Logger is nil")
	}

	// Test creating a server with the same port (should fail)
	err = manager.CreateServer(serverConfig)
	if err == nil {
		t.Fatal("Expected error when creating server with duplicate port, got nil")
	}
}

func TestCreateServers(t *testing.T) {
	manager := NewPostgresManager()

	// Create a test logger path and file
	loggerPath := "/tmp"
	loggerEnabled := true

	// Create a test configuration with multiple servers
	config := &models.MockServer{
		PostgresServers: models.PostgresServers{
			Postgres: []models.PostgresServer{
				{
					Name:       "test-postgres-1",
					User:       "postgres",
					Password:   "password1",
					Host:       "localhost",
					Port:       5432,
					Database:   "testdb1",
					LoggerPath: &loggerPath,
					Logger:     &loggerEnabled,
				},
				{
					Name:       "test-postgres-2",
					User:       "postgres",
					Password:   "password2",
					Host:       "localhost",
					Port:       5433,
					Database:   "testdb2",
					LoggerPath: &loggerPath,
					Logger:     &loggerEnabled,
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
	if _, exists := manager.servers[5432]; !exists {
		t.Error("Server on port 5432 was not created")
	}

	if _, exists := manager.servers[5433]; !exists {
		t.Error("Server on port 5433 was not created")
	}

	// Check properties of the first server
	server1 := manager.servers[5432]
	if server1.Name != "test-postgres-1" {
		t.Errorf("Expected name 'test-postgres-1', got %s", server1.Name)
	}

	if server1.Database != "testdb1" {
		t.Errorf("Expected database 'testdb1', got %s", server1.Database)
	}

	// Check properties of the second server
	server2 := manager.servers[5433]
	if server2.Name != "test-postgres-2" {
		t.Errorf("Expected name 'test-postgres-2', got %s", server2.Name)
	}

	if server2.Database != "testdb2" {
		t.Errorf("Expected database 'testdb2', got %s", server2.Database)
	}

	// Test error case: duplicate port
	config.PostgresServers.Postgres[1].Port = 5432
	err = manager.CreateServers(config)
	if err == nil {
		t.Fatal("Expected error when creating servers with duplicate port, got nil")
	}
}

// This test is more complex as it involves actual container creation
// It's marked as a short test so it can be skipped with -short flag
func TestStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	manager := NewPostgresManager()

	// Create a test logger path and file
	loggerPath := "/tmp"
	loggerEnabled := true

	// Create a test server configuration with a unique port to avoid conflicts
	serverConfig := models.PostgresServer{
		Name:       "test-postgres-start-stop",
		User:       "postgres",
		Password:   "password",
		Host:       "localhost",
		Port:       5434, // Use a different port to avoid conflicts
		Database:   "testdb",
		LoggerPath: &loggerPath,
		Logger:     &loggerEnabled,
	}

	// Create the server
	err := manager.CreateServer(serverConfig)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start the server
	err = manager.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Give the server time to start
	time.Sleep(2 * time.Second)

	// Check that the container was created
	server := manager.servers[5434]
	if server.PostgresContainer == nil {
		t.Fatal("PostgresContainer is nil after Start()")
	}

	// Check container state
	ctx := context.Background()
	state, err := server.PostgresContainer.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get container state: %v", err)
	}

	if !state.Running {
		t.Fatal("Container is not running after Start()")
	}

	// Stop the server
	manager.Stop()

	// Give the server time to stop
	time.Sleep(2 * time.Second)

	// Try to get state again (should fail or show not running)
	state, err = server.PostgresContainer.State(ctx)
	if err == nil && state.Running {
		t.Fatal("Container is still running after Stop()")
	}
}

func TestServerStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create a test server
	server := &Server{
		Name:       "test-postgres-start",
		User:       "postgres",
		Password:   "password",
		Host:       "localhost",
		Port:       5435, // Use a different port to avoid conflicts
		Database:   "testdb",
		LoggerPath: "/tmp",
	}

	// Get a logger for the server
	log, err := logger.GetLoggerContext(models.LogDescriptor{
		Name:   server.Name,
		Path:   server.LoggerPath,
		File:   true,
		Logger: true,
	})

	if err != nil {
		t.Fatalf("Failed to get logger: %v", err)
	}

	server.logger = log

	// Start the server
	container, err := server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Check that the container was created
	if container == nil {
		t.Fatal("Container is nil after Start()")
	}

	// Check container state
	ctx := context.Background()
	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get container state: %v", err)
	}

	if !state.Running {
		t.Fatal("Container is not running after Start()")
	}

	// Stop the container
	server.PostgresContainer = container
	server.Stop()

	// Give the server time to stop
	time.Sleep(2 * time.Second)

	// Try to get state again (should fail or show not running)
	state, err = container.State(ctx)
	if err == nil && state.Running {
		t.Fatal("Container is still running after Stop()")
	}
}
