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

// SetupCaso1Routes configura las rutas para el caso1
func SetupCaso1Routes(router *gin.Engine) {
	router.POST("/api/mock", func(c *gin.Context) {
		externa.MockHandler(c.Writer, c.Request)
	})
}
