package server

import (
	"catalyst/api"
	"catalyst/database"
	"catalyst/internal/config"
	"catalyst/internal/logger"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SOLUCIONESSYCOM/scribe"

	"catalyst/internal/handler"
	"catalyst/internal/models"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
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
	servers        map[int]*Server
	apiServer      *Server
	restartChan    chan string
	wg             sync.WaitGroup
	configs        []*models.MockServer
	configDir      string
	restartManager *api.RestartManager // NUEVO: RestartManager integrado
}

// NewManager creates a new server manager
func NewManager() *Manager {
	return &Manager{
		servers:     make(map[int]*Server),
		restartChan: make(chan string, 10),
		configs:     make([]*models.MockServer, 0),
	}
}

// CreateServers creates servers based on the configuration
func (m *Manager) CreateServers(config *models.MockServer) error {

	m.configs = append(m.configs, config)

	for _, serverConfig := range config.Http.Servers {
		if err := m.CreateServer(serverConfig); err != nil {
			return fmt.Errorf("error creating server on port %d: %w", serverConfig.Listen, err)
		}
	}
	return nil
}

func (m *Manager) CreateServer(config models.Server) error { // Check if server already exists
	if _, exists := m.servers[config.Listen]; exists {
		return fmt.Errorf("server on port %d already exists", config.Listen)
	}

	// Set Gin mode to release to disable debug logs
	gin.SetMode(gin.ReleaseMode)

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

	// Initialize database
	db, err := database.InitDB("./database.db")
	if err != nil {
		return fmt.Errorf("error initializing database: %v", err)
	}

	// Create batch manager
	batchConfig := database.BatchConfig{
		BatchSize:     20,
		FlushInterval: 2 * time.Second,
		MaxQueueSize:  50000,
		MaxBatchQueue: 50000,
		MaxWorkers:    3,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
	}
	batchManager := database.NewBatchManager(db, batchConfig)

	// Start batch manager
	if err := batchManager.Start(); err != nil {
		return fmt.Errorf("error starting batch manager: %v", err)
	}

	// Create handler
	h := handler.NewHandler(log, batchManager)

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

// CreateAPIServer creates the API server
func (m *Manager) CreateAPIServer(batchManager *database.BatchManager, configDir string) error {
	// Almacenar directorio de configuración
	m.configDir = configDir

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	api.SetupRoutes(router, batchManager, configDir, m.restartChan)

	m.apiServer = &Server{
		Port:   8282,
		Router: router,
	}

	// Crear e inicializar RestartManager dentro del servidor API
	m.restartManager = api.NewRestartManager(m.restartChan, func(serverName string) error {
		m.RestartMainServer(serverName)
		return nil
	})

	return nil
}

// StartAPIServer starts the API server
func (m *Manager) StartAPIServer() error {
	if m.apiServer == nil {
		return fmt.Errorf("API server not created")
	}

	// Iniciar RestartManager dentro del servidor API
	if m.restartManager != nil {
		m.restartManager.Start()
		log.Printf("RestartManager started within API server")
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		addr := ":" + strconv.Itoa(m.apiServer.Port)
		m.apiServer.httpServer = &http.Server{
			Addr:    addr,
			Handler: m.apiServer.Router,
		}

		log.Printf("Starting API server on %s", addr)
		if err := m.apiServer.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting API server: %v", err)
		}
	}()

	return nil
}

// RestartMainServer restarts a specific main server
func (m *Manager) RestartMainServer(serverName string) {
	log.Printf("Restarting server: %s", serverName)

	// Si es el servidor API, reiniciarlo
	if strings.EqualFold(serverName, "api") || strings.EqualFold(serverName, "api_server") {
		if err := m.RestartAPIServer(); err != nil {
			log.Printf("Error reiniciando servidor API: %v", err)
		} else {
			log.Printf("Servidor API reiniciado exitosamente")
		}
		return
	}

	// Para servidores mock, usar el método específico
	if err := m.RestartSpecificServer(serverName); err != nil {
		log.Printf("Error reiniciando servidor %s: %v", serverName, err)
		log.Printf("Server %s restart failed. Manual restart required to apply new configuration.", serverName)
	} else {
		log.Printf("Server %s restarted successfully with new configuration.", serverName)
	}
}

// GetRestartChan returns the restart channel
func (m *Manager) GetRestartChan() chan string {
	return m.restartChan
}

// ReloadConfig recarga la configuración de un servidor específico
func (m *Manager) ReloadConfig(serverName string) (*models.MockServer, error) {
	// Buscar archivo de configuración por nombre de servidor
	var configFile string

	// Buscar en el directorio de configuración
	if m.configDir != "" {
		// Intentar con extensiones .yml y .yaml
		extensions := []string{".yml", ".yaml"}
		for _, ext := range extensions {
			configFile = filepath.Join(m.configDir, serverName+ext)
			// Verificar si el archivo existe
			if _, err := os.Stat(configFile); err == nil {
				break
			}
		}
	}

	if configFile == "" {
		return nil, fmt.Errorf("configuración no encontrada para el servidor: %s", serverName)
	}

	// Cargar configuración actualizada
	config, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("error cargando configuración actualizada: %w", err)
	}

	log.Printf("Configuración recargada exitosamente para servidor: %s", serverName)
	return config, nil
}

// RestartSpecificServer reinicia un servidor específico con nueva configuración
func (m *Manager) RestartSpecificServer(serverName string) error {
	// Intentar reinicio hasta 3 veces
	maxRetries := 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Intento %d de %d para reiniciar servidor %s", attempt, maxRetries, serverName)

		err := m.restartServerAttempt(serverName)
		if err == nil {
			log.Printf("Servidor %s reiniciado exitosamente en intento %d", serverName, attempt)
			return nil
		}

		log.Printf("Intento %d falló: %v", attempt, err)
		if attempt < maxRetries {
			time.Sleep(time.Duration(attempt) * time.Second) // Esperar más tiempo en cada intento
		}
	}

	return fmt.Errorf("servidor %s no pudo reiniciarse después de %d intentos", serverName, maxRetries)
}

// restartServerAttempt realiza un intento de reinicio del servidor
func (m *Manager) restartServerAttempt(serverName string) error {
	// 1. Recargar configuración
	config, err := m.ReloadConfig(serverName)
	if err != nil {
		return fmt.Errorf("error recargando configuración: %w", err)
	}

	// 2. Encontrar servidor específico por nombre en configuraciones almacenadas
	var targetPort int
	var targetServer *Server

	// Buscar en configuraciones almacenadas primero
	for _, storedConfig := range m.configs {
		for _, serverConfig := range storedConfig.Http.Servers {
			// Búsqueda case-insensitive
			if strings.EqualFold(*serverConfig.Name, serverName) {
				targetPort = serverConfig.Listen
				log.Printf("DEBUG: Servidor encontrado en configuración - nombre: %s, puerto: %d", *serverConfig.Name, targetPort)

				// Buscar servidor en todos los puertos disponibles
				for port, server := range m.servers {
					if port == targetPort {
						targetServer = server
						log.Printf("DEBUG: Servidor encontrado en puerto %d", port)
						break
					}
				}

				if targetServer != nil {
					break
				} else {
					log.Printf("DEBUG: Servidor no encontrado en puerto %d. Puertos disponibles: %v", targetPort, func() []int {
						ports := make([]int, 0, len(m.servers))
						for port := range m.servers {
							ports = append(ports, port)
						}
						return ports
					}())
				}
			}
		}
		if targetServer != nil {
			break
		}
	}

	if targetServer == nil {
		return fmt.Errorf("servidor %s no encontrado para reiniciar", serverName)
	}

	// 3. Detener servidor específico
	targetServer.Stop()
	delete(m.servers, targetPort)

	// 4. Esperar a que el puerto esté libre
	if !waitForPortToBeFree(targetPort, 5*time.Second) {
		return fmt.Errorf("puerto %d no se liberó después de 5 segundos", targetPort)
	}

	// 5. Crear nuevo servidor con configuración actualizada
	for _, serverConfig := range config.Http.Servers {
		// Búsqueda case-insensitive para encontrar el servidor correcto
		if strings.EqualFold(*serverConfig.Name, serverName) {
			log.Printf("DEBUG: Creando servidor con configuración actualizada - nombre: %s, puerto: %d", *serverConfig.Name, serverConfig.Listen)

			// Verificar que el puerto esté libre antes de crear
			if !isPortAvailable(serverConfig.Listen) {
				return fmt.Errorf("puerto %d aún está ocupado", serverConfig.Listen)
			}

			if err := m.CreateServer(serverConfig); err != nil {
				return fmt.Errorf("error creando servidor actualizado: %w", err)
			}

			// 6. Obtener referencia al nuevo servidor creado
			newServer := m.servers[serverConfig.Listen]
			if newServer == nil {
				return fmt.Errorf("error: nuevo servidor no se creó correctamente")
			}

			// 7. Iniciar nuevo servidor
			m.wg.Add(1)
			go func(s *Server, p int, name string) {
				defer m.wg.Done()
				if err := s.Start(); err != nil && err != http.ErrServerClosed {
					log.Printf("Error iniciando servidor actualizado en puerto %d: %v", p, err)
				}
			}(newServer, serverConfig.Listen, serverName)

			log.Printf("Servidor %s reiniciado exitosamente en puerto %d", serverName, serverConfig.Listen)
			break
		}
	}

	return nil
}

// RestartAPIServer reinicia el servidor API con nueva configuración
func (m *Manager) RestartAPIServer() error {
	log.Printf("Reiniciando servidor API...")

	// Detener servidor API actual
	if m.apiServer != nil {
		m.apiServer.Stop()
		log.Printf("Servidor API detenido")
	}

	// Esperar a que el puerto se libere
	if !waitForPortToBeFree(8282, 5*time.Second) {
		return fmt.Errorf("puerto 8282 no se liberó después de 5 segundos")
	}

	// El servidor API se reiniciará automáticamente
	// ya que está en un goroutine separado
	log.Printf("Servidor API reiniciado exitosamente")
	return nil
}

func (m *Manager) Stop() {
	// Detener RestartManager si existe
	if m.restartManager != nil {
		m.restartManager.Stop()
		log.Printf("RestartManager stopped")
	}

	if m.apiServer != nil {
		m.apiServer.Stop()
	}

	for _, server := range m.servers {
		server.Stop()
	}
	m.wg.Wait()
}

// Stop stops the server
func (s *Server) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down server: %v", err)
		}
		// Esperar un momento adicional para asegurar que el puerto se libere
		time.Sleep(100 * time.Millisecond)
	}
}

// Wait waits for all servers to stop
func (m *Manager) Wait() {
	m.wg.Wait()
}

// isPortAvailable verifica si un puerto está disponible
func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// waitForPortToBeFree espera hasta que un puerto esté libre
func waitForPortToBeFree(port int, maxWait time.Duration) bool {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if isPortAvailable(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
