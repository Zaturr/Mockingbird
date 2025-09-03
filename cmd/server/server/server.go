package server

import (
	"log"
	"os"
	"sync"

	"Mockingbird/network/route/services"

	"github.com/gin-gonic/gin"
)

type ServiceServer struct {
	router *gin.Engine
	port   string
	name   string
}

type MultiPortServer struct {
	services []*ServiceServer
}

func NewServiceServer(name, port string) *ServiceServer {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	server := &ServiceServer{
		router: router,
		port:   port,
		name:   name,
	}

	// Configurar rutas específicas del servicio
	switch name {
	case "jsonplaceholder":
		services.SetupJsonplaceholderRoutes(router)
	case "sypago":
		services.SetupSypagoRoutes(router)
	}

	return server
}

func (s *ServiceServer) Start() error {
	log.Printf("Servicio %s iniciando en puerto %s", s.name, s.port)
	return s.router.Run(":" + s.port)
}

func NewMultiPortServer() *MultiPortServer {
	// Obtener puertos desde variables de entorno o usar valores por defecto
	jsonplaceholderPort := getEnv("JSONPLACEHOLDER_PORT", "8081")
	sypagoPort := getEnv("SYPAGO_PORT", "8080")

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
