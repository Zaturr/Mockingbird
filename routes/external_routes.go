package routes

import (
	"Mockingbird/network/handler"
	externa "Mockingbird/network/parser"

	"github.com/gin-gonic/gin"
)

// SetupExternalRoutes configura las rutas para el handler externo
func SetupExternalRoutes(router *gin.Engine, externalHandler *handler.ExternalHandler) {
	router.POST("/api/config", externalHandler.HandleConfigMapping)
}

func SetupCaso1Routes(router *gin.Engine) {
	// Configurar ruta para cualquier método HTTP
	router.Any("/api/mock", func(c *gin.Context) {
		externa.MockHandler(c.Writer, c.Request)
	})
}
