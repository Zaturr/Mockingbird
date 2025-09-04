package server

import (
	"log"
	"os"
	"sync"

	"Mockingbird/network/handler"

	"github.com/gin-gonic/gin"
)

type ServiceServer struct {
	router          *gin.Engine
	port            string
	name            string
	externalHandler *handler.ExternalHandler
}

type MultiPortServer struct {
	services []*ServiceServer
}

func NewServiceServer(name, port string) *ServiceServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Crear el handler externo que maneja el mapeo de JSON
	externalHandler := handler.NewExternalHandler()

	server := &ServiceServer{
		router:          router,
		port:            port,
		name:            name,
		externalHandler: externalHandler,
	}

	// Configurar rutas usando el handler externo
	externalHandler.SetupExternalRoutes(router)

	return server
}

func (s *ServiceServer) Start() error {
	log.Printf("Servicio %s iniciando en puerto %s", s.name, s.port)
	return s.router.Run(":" + s.port)
}

func Multiport() *MultiPortServer {
	// Obtener puertos desde variables de entorno o usar valores por defecto
	jsonplaceholderPort := getEnv("JSONPLACEHOLDER_PORT", "8080")
	sypagoPort := getEnv("SYPAGO_PORT", "8081")
	usersPort := getEnv("USERS_PORT", "9090")

	return &MultiPortServer{
		services: []*ServiceServer{
			NewServiceServer("jsonplaceholder", jsonplaceholderPort),
			NewServiceServer("sypago", sypagoPort),
			NewServiceServer("users", usersPort),
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
