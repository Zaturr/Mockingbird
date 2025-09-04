package main

import (
	"Mockingbird/network/handler"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Configurar Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Crear handler externo
	externalHandler := handler.NewExternalHandler()

	// Configurar rutas
	externalHandler.SetupExternalRoutes(router)

	// Ruta de información
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":     "Test Handler Server",
			"endpoint":    "POST /api/config",
			"description": "Envía JSON para mapear a la estructura",
		})
	})

	// Configurar puerto
	port := ":8080"
	fmt.Printf("🚀 Servidor de prueba iniciando en puerto %s\n", port)
	fmt.Printf("📋 Endpoint: POST /api/config\n")
	fmt.Printf("🔗 URL: http://localhost%s\n", port)

	// Iniciar servidor
	if err := router.Run(port); err != nil {
		log.Fatal("Error iniciando servidor:", err)
	}
}
