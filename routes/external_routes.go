package routes

import (
	"Mockingbird/network/handler"

	"github.com/gin-gonic/gin"
)

// SetupExternalRoutes configura las rutas para el handler externo
func SetupExternalRoutes(router *gin.Engine, externalHandler *handler.ExternalHandler) {
	router.POST("/api/config", externalHandler.HandleConfigMapping)
}
