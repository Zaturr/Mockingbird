package api

import (
	"catalyst/database"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Middleware for logging requests
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
		)
	})
}

// Middleware for CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Middleware for error recovery
func ErrorRecovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if err, ok := recovered.(string); ok {
			log.Printf("Panic recovered: %s", err)
			c.JSON(http.StatusInternalServerError, NewErrorResponse(
				fmt.Errorf("internal server error: %s", err),
				http.StatusInternalServerError,
				"An unexpected error occurred",
			))
		} else {
			c.JSON(http.StatusInternalServerError, NewErrorResponse(
				fmt.Errorf("internal server error"),
				http.StatusInternalServerError,
				"An unexpected error occurred",
			))
		}
		c.Abort()
	})
}

// Middleware for request validation
func ValidateServerName() gin.HandlerFunc {
	return func(c *gin.Context) {
		serverName := c.Query("server_name")
		if serverName == "" {
			c.JSON(http.StatusBadRequest, NewErrorResponse(
				ErrInvalidServer,
				http.StatusBadRequest,
				"server_name parameter is required",
			))
			c.Abort()
			return
		}

		if len(serverName) > 100 {
			c.JSON(http.StatusBadRequest, NewErrorResponse(
				ErrInvalidServer,
				http.StatusBadRequest,
				"server_name parameter is too long",
			))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RouteGroup represents a group of API routes
type RouteGroup struct {
	handler *APIHandler
}

// NewRouteGroup creates a new RouteGroup instance
func NewRouteGroup(handler *APIHandler) *RouteGroup {
	return &RouteGroup{
		handler: handler,
	}
}

// SetupDataRoutes sets up data-related routes
func (rg *RouteGroup) SetupDataRoutes(router *gin.RouterGroup) {
	data := router.Group("/data")
	{
		data.GET("", rg.handler.GetData)
	}
}

// SetupConfigRoutes sets up configuration-related routes
func (rg *RouteGroup) SetupConfigRoutes(router *gin.RouterGroup) {
	config := router.Group("/config")
	{
		config.GET("", ValidateServerName(), rg.handler.GetConfig)
		config.PUT("", ValidateServerName(), rg.handler.UpdateConfig)
		config.PUT("/yaml", rg.handler.UpdateConfigYaml)
	}
}

// SetupHealthRoutes sets up health check routes
func (rg *RouteGroup) SetupHealthRoutes(router *gin.RouterGroup) {
	health := router.Group("/health")
	{
		health.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, NewSuccessResponse(map[string]interface{}{
				"status":    "healthy",
				"timestamp": time.Now().Format(time.RFC3339),
			}, "Service is healthy"))
		})
	}
}

// SetupRoutes sets up all API routes with middleware and proper organization
func SetupRoutes(router *gin.Engine, batchManager *database.BatchManager, configDir string, restartChan chan string) {
	// Add global middleware
	router.Use(RequestLogger())
	router.Use(CORSMiddleware())
	router.Use(ErrorRecovery())

	// Create API handler
	apiHandler := NewAPIHandler(batchManager, configDir, restartChan)
	routeGroup := NewRouteGroup(apiHandler)

	// Setup API routes
	api := router.Group("/api/mock")
	{
		routeGroup.SetupDataRoutes(api)
		routeGroup.SetupConfigRoutes(api)
		routeGroup.SetupHealthRoutes(api)
	}

	log.Printf("API routes configured successfully")
}

// SetupRoutesWithOptions sets up routes with custom options
func SetupRoutesWithOptions(router *gin.Engine, batchManager *database.BatchManager, configDir string, restartChan chan string, options *RouteOptions) {
	// Add global middleware
	router.Use(RequestLogger())
	router.Use(CORSMiddleware())
	router.Use(ErrorRecovery())

	// Create API handler
	apiHandler := NewAPIHandler(batchManager, configDir, restartChan)
	routeGroup := NewRouteGroup(apiHandler)

	// Setup API routes
	api := router.Group("/api/mock")
	{
		if options.EnableDataRoutes {
			routeGroup.SetupDataRoutes(api)
		}
		if options.EnableConfigRoutes {
			routeGroup.SetupConfigRoutes(api)
		}
		if options.EnableHealthRoutes {
			routeGroup.SetupHealthRoutes(api)
		}
	}

	log.Printf("API routes configured with options: %+v", options)
}

// RouteOptions configures which route groups to enable
type RouteOptions struct {
	EnableDataRoutes   bool
	EnableConfigRoutes bool
	EnableHealthRoutes bool
}

// DefaultRouteOptions returns default route options
func DefaultRouteOptions() *RouteOptions {
	return &RouteOptions{
		EnableDataRoutes:   true,
		EnableConfigRoutes: true,
		EnableHealthRoutes: true,
	}
}
