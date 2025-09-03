package server

import (
	"log"
	"os"
	"sync"

	"Mockingbird/network/handler"

	"github.com/gin-gonic/gin"
)

type ServiceServer struct {
	router     *gin.Engine
	port       string
	name       string
	astHandler *handler.ASTHandler
}

type MultiPortServer struct {
	services []*ServiceServer
}

func NewServiceServer(name, port string) *ServiceServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Configuración AST específica para cada servicio
	var config *handler.Http

	switch name {
	case "jsonplaceholder":
		config = getJsonplaceholderConfig()
	case "sypago":
		config = getSypagoConfig()
	default:
		config = getDefaultConfig()
	}

	astHandler := handler.NewASTHandler(config)

	server := &ServiceServer{
		router:     router,
		port:       port,
		name:       name,
		astHandler: astHandler,
	}

	// Configurar rutas usando AST específico del servicio
	astHandler.SetupRoutes(router)

	return server
}

// getJsonplaceholderConfig retorna la configuración AST para Jsonplaceholder
func getJsonplaceholderConfig() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Post created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "User created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Comment created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"message": "Post retrieved successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Jsonplaceholder service is healthy", "port": 8080},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"message": "Chaos test endpoint", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "100ms 30%",
							Abort:   "503 10%",
							Error:   "500 5%",
						},
					},
				},
			},
		},
	}
}

// getSypagoConfig retorna la configuración AST para Sypago
func getSypagoConfig() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8081,
				Logger: &logger,
				Location: []handler.Location{
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Payment processed successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Transaction created successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Sypago service is healthy", "port": 8081},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

// getDefaultConfig retorna una configuración por defecto
func getDefaultConfig() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Service is healthy"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

func (s *ServiceServer) Start() error {
	log.Printf("Servicio %s iniciando en puerto %s", s.name, s.port)
	return s.router.Run(":" + s.port)
}

func NewMultiPortServer() *MultiPortServer {
	// Obtener puertos desde variables de entorno o usar valores por defecto
	jsonplaceholderPort := getEnv("JSONPLACEHOLDER_PORT", "8080") // Cambiado a 8080
	sypagoPort := getEnv("SYPAGO_PORT", "8081")

	return &MultiPortServer{
		services: []*ServiceServer{
			NewServiceServer("jsonplaceholder", jsonplaceholderPort),
			NewServiceServer("sypago", sypagoPort),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (m *MultiPortServer) StartAll() error {
	var wg sync.WaitGroup
	var errors []error

	for _, service := range m.services {
		wg.Add(1)
		go func(s *ServiceServer) {
			defer wg.Done()
			if err := s.Start(); err != nil {
				log.Printf("Error en servicio %s: %v", s.name, err)
				errors = append(errors, err)
			}
		}(service)
	}

	wg.Wait()

	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}
