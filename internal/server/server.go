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
	prom "catalyst/prometheus"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

type Server struct {
	Port       int
	Router     *gin.Engine
	httpServer *http.Server
	handler    *handler.Handler
	locations  []models.Location
	logger     *scribe.Scribe
}

type Manager struct {
	servers        map[int]*Server
	apiServer      *Server
	metricsServer  *Server
	restartChan    chan string
	wg             sync.WaitGroup
	configs        []*models.MockServer
	configDir      string
	restartManager *api.RestartManager
	logger         *scribe.Scribe
}

func NewManager() *Manager {

	m := models.LogDescriptor{
		Name:    "",
		Version: "",
		Path:    "",
		File:    false,
		Logger:  true,
	}

	logCtx, _ := logger.GetLoggerContext(m)

	return &Manager{
		servers:     make(map[int]*Server),
		restartChan: make(chan string, 10),
		configs:     make([]*models.MockServer, 0),
		logger:      logCtx,
	}
}

func (m *Manager) CreateServers(config *models.MockServer) error {

	m.configs = append(m.configs, config)

	for _, serverConfig := range config.Http.Servers {
		if err := m.CreateServer(serverConfig); err != nil {
			return fmt.Errorf("error creating server on port %d: %w", serverConfig.Listen, err)
		}
	}
	return nil
}

func (m *Manager) CreateServer(config models.Server) error {
	if _, exists := m.servers[config.Listen]; exists {
		return fmt.Errorf("server on port %d already exists", config.Listen)
	}

	gin.SetMode(gin.ReleaseMode)

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
	router.Use(gin.Recovery())

	db, err := database.InitDB("./database.db")
	if err != nil {
		log.Error().AnErr("error initializing database:", err).Msg("error initializing database")
		return err
	}

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

	if err := batchManager.Start(); err != nil {
		log.Error().AnErr("error initializing batch nanager:", err).Msg("error initializing database")
		return fmt.Errorf("error starting batch manager: %v", err)
	}

	h := handler.NewHandler(log, batchManager)

	h.Logger = log

	server := &Server{
		Port:      config.Listen,
		Router:    router,
		handler:   h,
		locations: config.Location,
		logger:    log,
	}

	if err := server.registerRoutes(); err != nil {
		return fmt.Errorf("error registering routes: %w", err)
	}

	m.servers[config.Listen] = server

	return nil
}

func (s *Server) registerRoutes() error {
	for _, location := range s.locations {
		if err := s.handler.RegisterLocation(location); err != nil {
			s.logger.Error().AnErr(fmt.Sprintf("error registering location %s: %w", location.Path, err), err)
			return err
		}

		if location.StaticFilesDir != "" {
			s.logger.Info().Msg(fmt.Sprintf("registering static files at %s", location.StaticFilesDir))
			//currentPath, _ := os.Getwd()
			s.Router.Static(location.Path, "/Users/quintero/GolandProjects/Catalyst/config/samplesite")
		} else {
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

func (s *Server) Start() error {
	addr := ":" + strconv.Itoa(s.Port)
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.Router,
	}

	s.logger.Info().Msg(fmt.Sprintf("Starting server on port %d", s.Port))
	return s.httpServer.ListenAndServe()
}

func (m *Manager) CreateAPIServer(batchManager *database.BatchManager, configDir string) error {
	m.configDir = configDir

	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	api.SetupRoutes(router, batchManager, configDir, m.restartChan)

	m.apiServer = &Server{
		Port:   8282,
		Router: router,
	}

	m.restartManager = api.NewRestartManager(m.restartChan, func(serverName string) error {
		m.RestartMainServer(serverName)
		return nil
	})

	return nil
}

func (m *Manager) CreateMetricsServer(port int) error {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	// Setup metrics endpoint
	router.GET("/metrics", gin.WrapH(prom.PromHTTPHandler()))

	m.metricsServer = &Server{
		Port:   port,
		Router: router,
	}

	return nil
}

func (m *Manager) StartMetricsServer() error {
	if m.metricsServer == nil {
		return fmt.Errorf("metrics server not created")
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		addr := ":" + strconv.Itoa(m.metricsServer.Port)
		m.metricsServer.httpServer = &http.Server{
			Addr:    addr,
			Handler: m.metricsServer.Router,
		}

		log.Printf("Starting metrics server on %s", addr)
		if err := m.metricsServer.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error starting metrics server: %v", err)
		}
	}()

	return nil
}

func (m *Manager) StartAPIServer() error {
	if m.apiServer == nil {
		return fmt.Errorf("API server not created")
	}

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

func (m *Manager) RestartMainServer(serverName string) {
	log.Printf("Restarting server: %s", serverName)

	if strings.EqualFold(serverName, "api") || strings.EqualFold(serverName, "api_server") {
		if err := m.RestartAPIServer(); err != nil {
			log.Printf("Error reiniciando servidor API: %v", err)
		} else {
			log.Printf("Servidor API reiniciado exitosamente")
		}
		return
	}

	if err := m.RestartSpecificServer(serverName); err != nil {
		log.Printf("Error reiniciando servidor %s: %v", serverName, err)
		log.Printf("Server %s restart failed. Manual restart required to apply new configuration.", serverName)
	} else {
		log.Printf("Server %s restarted successfully with new configuration.", serverName)
	}
}

func (m *Manager) GetRestartChan() chan string {
	return m.restartChan
}

func (m *Manager) ReloadConfig(serverName string) (*models.MockServer, error) {
	var configFile string

	if m.configDir != "" {
		extensions := []string{".yml", ".yaml"}
		for _, ext := range extensions {
			configFile = filepath.Join(m.configDir, serverName+ext)
			if _, err := os.Stat(configFile); err == nil {
				break
			}
		}
	}

	if configFile == "" {
		return nil, fmt.Errorf("configuración no encontrada para el servidor: %s", serverName)
	}

	config, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("error cargando configuración actualizada: %w", err)
	}

	log.Printf("Configuración recargada exitosamente para servidor: %s", serverName)
	return config, nil
}

func (m *Manager) RestartSpecificServer(serverName string) error {
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
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return fmt.Errorf("servidor %s no pudo reiniciarse después de %d intentos", serverName, maxRetries)
}

func (m *Manager) restartServerAttempt(serverName string) error {
	config, err := m.ReloadConfig(serverName)
	if err != nil {
		return fmt.Errorf("error recargando configuración: %w", err)
	}

	var targetServerConfig models.Server
	var found bool
	for _, serverConfig := range config.Http.Servers {
		if strings.EqualFold(*serverConfig.Name, serverName) {
			targetServerConfig = serverConfig
			found = true
			log.Printf("DEBUG: Nueva configuración encontrada - nombre: %s, puerto: %d", *serverConfig.Name, serverConfig.Listen)
			break
		}
	}

	if !found {
		return fmt.Errorf("servidor %s no encontrado en configuración recargada", serverName)
	}

	var targetPort int
	var targetServer *Server
	newPort := targetServerConfig.Listen

	if server, exists := m.servers[newPort]; exists {
		targetServer = server
		targetPort = newPort
		log.Printf("DEBUG: Servidor encontrado en puerto de configuración recargada - puerto: %d", newPort)
	} else {
		for _, storedConfig := range m.configs {
			for _, serverConfig := range storedConfig.Http.Servers {
				if strings.EqualFold(*serverConfig.Name, serverName) {
					oldPort := serverConfig.Listen
					if oldPort != newPort {
						if server, exists := m.servers[oldPort]; exists {
							targetServer = server
							targetPort = oldPort
							log.Printf("DEBUG: Servidor encontrado en puerto antiguo - nombre: %s, puerto antiguo: %d, puerto nuevo: %d", *serverConfig.Name, oldPort, newPort)
							break
						}
					}
				}
			}
			if targetServer != nil {
				break
			}
		}
	}

	if targetServer != nil {
		log.Printf("DEBUG: Deteniendo servidor en puerto %d", targetPort)
		targetServer.Stop()
		delete(m.servers, targetPort)

		if targetPort == newPort {
			if !waitForPortToBeFree(targetPort, 5*time.Second) {
				return fmt.Errorf("puerto %d no se liberó después de 5 segundos", targetPort)
			}
		} else {
			// Si el puerto cambió, esperar un momento para que se libere el puerto antiguo
			time.Sleep(500 * time.Millisecond)
		}
	} else {
		log.Printf("DEBUG: Servidor no encontrado en ejecución. Puertos disponibles: %v", func() []int {
			ports := make([]int, 0, len(m.servers))
			for port := range m.servers {
				ports = append(ports, port)
			}
			return ports
		}())
		log.Printf("DEBUG: Servidor no está en ejecución, se creará en puerto %d", newPort)
	}

	// 5. Verificar que el puerto nuevo esté libre antes de crear
	if targetPort != newPort {
		// Si el puerto cambió, esperar a que el puerto nuevo esté libre
		if !waitForPortToBeFree(newPort, 5*time.Second) {
			return fmt.Errorf("puerto nuevo %d no está disponible después de 5 segundos", newPort)
		}
	}

	// 6. Crear nuevo servidor con configuración actualizada
	log.Printf("DEBUG: Creando servidor con configuración actualizada - nombre: %s, puerto: %d", *targetServerConfig.Name, targetServerConfig.Listen)

	if !isPortAvailable(targetServerConfig.Listen) {
		return fmt.Errorf("puerto %d aún está ocupado", targetServerConfig.Listen)
	}

	if err := m.CreateServer(targetServerConfig); err != nil {
		return fmt.Errorf("error creando servidor actualizado: %w", err)
	}

	newServer := m.servers[targetServerConfig.Listen]
	if newServer == nil {
		return fmt.Errorf("error: nuevo servidor no se creó correctamente")
	}

	m.wg.Add(1)
	go func(s *Server, p int, name string) {
		defer m.wg.Done()
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("Error iniciando servidor actualizado en puerto %d: %v", p, err)
		}
	}(newServer, targetServerConfig.Listen, serverName)

	log.Printf("Servidor %s reiniciado exitosamente en puerto %d", serverName, targetServerConfig.Listen)
	m.updateStoredConfig(serverName, config)
	return nil
}

func (m *Manager) updateStoredConfig(serverName string, newConfig *models.MockServer) {
	for i, storedConfig := range m.configs {
		for _, serverConfig := range storedConfig.Http.Servers {
			if strings.EqualFold(*serverConfig.Name, serverName) {
				m.configs[i] = newConfig
				log.Printf("DEBUG: Configuración actualizada en memoria para servidor: %s", serverName)
				return
			}
		}
	}

	m.configs = append(m.configs, newConfig)
	log.Printf("DEBUG: Nueva configuración agregada en memoria para servidor: %s", serverName)
}

func (m *Manager) RestartAPIServer() error {
	log.Printf("Reiniciando servidor API...")

	if m.apiServer != nil {
		m.apiServer.Stop()
		log.Printf("Servidor API detenido")
	}

	if !waitForPortToBeFree(8282, 5*time.Second) {
		return fmt.Errorf("puerto 8282 no se liberó después de 5 segundos")
	}

	log.Printf("Servidor API reiniciado exitosamente")
	return nil
}

func (m *Manager) Stop() {
	if m.restartManager != nil {
		m.restartManager.Stop()
		log.Printf("RestartManager stopped")
	}

	if m.apiServer != nil {
		m.apiServer.Stop()
	}

	if m.metricsServer != nil {
		m.metricsServer.Stop()
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
		time.Sleep(100 * time.Millisecond)
	}
}

// Wait waits for all servers to stop
func (m *Manager) Wait() {
	m.wg.Wait()
}

func isPortAvailable(port int) bool {
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

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
