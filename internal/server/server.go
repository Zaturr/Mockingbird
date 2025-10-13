package server

import (
	"catalyst/internal/logger"
	"context"
	"fmt"
	"github.com/SOLUCIONESSYCOM/scribe"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"catalyst/internal/handler"
	"catalyst/internal/models"

	"github.com/gin-gonic/gin"
)

// Server represents a single HTTP server instance
type Server struct {
	Port       int
	Router     *gin.Engine
	httpServer *http.Server
	handler    *handler.Handler
	locations  []models.Location
	logger     *scribe.Scribe
}

// Manager manages multiple server instances
type Manager struct {
	servers map[int]*Server
	wg      sync.WaitGroup
}

// NewManager creates a new server manager
func NewManager() *Manager {
	return &Manager{
		servers: make(map[int]*Server),
	}
}

// CreateServers creates servers based on the configuration
func (m *Manager) CreateServers(config *models.MockServer) error {
	for _, serverConfig := range config.Http.Servers {
		if err := m.CreateServer(serverConfig); err != nil {
			return fmt.Errorf("error creating server on port %d: %w", serverConfig.Listen, err)
		}
	}
	return nil
}

// CreateServer creates a single server based on the configuration

func (m *Manager) CreateServer(config models.Server) error { // Check if server already exists
	if _, exists := m.servers[config.Listen]; exists {
		return fmt.Errorf("server on port %d already exists", config.Listen)
	}

	// Set Gin mode
	if config.Logger != nil && *config.Logger {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	var log *scribe.Scribe
	var err error

	log, err = logger.GetLoggerContext(models.LogDescriptor{
		Name:    *config.Name,
		Version: *config.Version,
		Path:    *config.LoggerPath,
		File:    *config.Logger,
		Logger:  *config.Logger,
	})

	if err != nil {
		log = &scribe.Scribe{}
	}
	// Add middleware
	router.Use(gin.Recovery())
	if config.Logger != nil && *config.Logger {
		router.Use(gin.Logger())
	}

	// Create handler
	h := handler.NewHandler(log)

	h.Logger = log

	// Create server
	server := &Server{
		Port:      config.Listen,
		Router:    router,
		handler:   h,
		locations: config.Location,
		logger:    log,
	}

	// Register routes
	if err := server.registerRoutes(); err != nil {
		return fmt.Errorf("error registering routes: %w", err)
	}

	// Store server
	m.servers[config.Listen] = server

	return nil
}

// registerRoutes registers all routes for the server
func (s *Server) registerRoutes() error {
	for _, location := range s.locations {
		// Register location with handler for schema compilation
		if err := s.handler.RegisterLocation(location); err != nil {
			return fmt.Errorf("error registering location %s: %w", location.Path, err)
		}

		if location.StaticFilesDir != "" {
			s.logger.Info().Msg(fmt.Sprintf("registering static files at %s", location.StaticFilesDir))
			//currentPath, _ := os.Getwd()
			s.Router.Static(location.Path, "/Users/quintero/GolandProjects/Catalyst/config/samplesite")
		} else {
			// Create route handler
			s.Router.Handle(location.Method, location.Path, func(loc models.Location) gin.HandlerFunc {
				return func(c *gin.Context) {
					s.handler.HandleRequest(c, loc)
				}
			}(location))
		}

		s.logger.Info().Msg(fmt.Sprintf("Registered route: %s %s", location.Method, location.Path))
	}

	return nil
}

// Start starts all servers
func (m *Manager) Start() error {
	for port, server := range m.servers {
		m.wg.Add(1)
		go func(s *Server, p int) {
			defer m.wg.Done()
			if err := s.Start(); err != nil && err != http.ErrServerClosed {
				log.Printf("Error starting server on port %d: %v", p, err)
			}
		}(server, port)
	}

	return nil
}

// Start starts the server
func (s *Server) Start() error {
	addr := ":" + strconv.Itoa(s.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Router,
	}

	log.Printf("Starting server on %s", addr)
	return s.httpServer.ListenAndServe()
}

// Stop stops all servers
func (m *Manager) Stop() {
	for _, server := range m.servers {
		server.Stop()
	}
	m.wg.Wait()
}

// Stop stops the server
func (s *Server) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
	}
}

// Wait waits for all servers to stop
func (m *Manager) Wait() {
	m.wg.Wait()
}
