package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ASTHandler handles requests based on AST configuration
type ASTHandler struct {
	config *Http
}

// NewASTHandler creates a new handler with AST configuration
func NewASTHandler(config *Http) *ASTHandler {
	return &ASTHandler{
		config: config,
	}
}

// SetupRoutes configures all routes based on AST configuration
func (h *ASTHandler) SetupRoutes(router *gin.Engine) {
	for _, server := range h.config.Servers {
		for i, location := range server.Location {
			// Apply server-level chaos injection if exists
			serverChaos := server.ChaosInjection

			// Apply location-specific chaos injection if exists
			locationChaos := location.ChaosInjection

			// Create handler for this location
			handler := h.createLocationHandler(location, serverChaos, locationChaos)

			// Generate a meaningful path based on method and index
			path := fmt.Sprintf("/api/%s/%d", location.Method, i)

			// Add specific paths for common endpoints
			if location.Method == "GET" && i == 4 { // health check
				path = "/health"
			} else if location.Method == "GET" && i == 3 { // get by id
				path = "/api/posts/:id"
			}

			// Register route based on method
			switch location.Method {
			case "GET":
				router.GET(path, handler)
			case "POST":
				router.POST(path, handler)
			case "PUT":
				router.PUT(path, handler)
			case "DELETE":
				router.DELETE(path, handler)
			case "PATCH":
				router.PATCH(path, handler)
			}
		}
	}
}

// createLocationHandler creates a handler function for a specific location
func (h *ASTHandler) createLocationHandler(location Location, serverChaos, locationChaos *ChaosInjection) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Apply chaos injection (server level first, then location level)
		if serverChaos != nil {
			h.applyChaosInjection(c, serverChaos)
		}
		if locationChaos != nil {
			h.applyChaosInjection(c, locationChaos)
		}

		// Handle async requests if configured
		if location.Async != nil {
			go h.handleAsyncRequest(location.Async)
		}

		// Determine response status code
		statusCode := h.determineStatusCode(location.StatusCode)

		// Set response headers
		if location.Headers != nil {
			for key, value := range *location.Headers {
				c.Header(key, value)
			}
		}

		// Send response
		c.JSON(statusCode, location.Response)
	}
}

// applyChaosInjection applies chaos engineering features
func (h *ASTHandler) applyChaosInjection(c *gin.Context, chaos *ChaosInjection) {
	// TODO: Implementar funcionalidad de chaos engineering
	// Por ahora solo se registra que debe implementarse

	if chaos.Latency != "" {
		// TODO: Implementar inyección de latencia basada en string
		// Parsear el string de latencia (ej: "100ms 30%")
	}

	if chaos.Abort != "" {
		// TODO: Implementar inyección de abort basada en string
		// Parsear el string de abort (ej: "503 10%")
	}

	if chaos.Error != "" {
		// TODO: Implementar inyección de error basada en string
		// Parsear el string de error (ej: "500 5%")
	}
}

// determineStatusCode determines response status code
func (h *ASTHandler) determineStatusCode(statusCode string) int {
	if statusCode == "" {
		return http.StatusOK
	}

	// TODO: Parsear el string de statusCode (ej: "200 80% 500 20%")
	// Por ahora, intentar convertir a int directamente
	if code, err := strconv.Atoi(statusCode); err == nil {
		return code
	}

	// Si no se puede parsear, devolver 200 por defecto
	return http.StatusOK
}

// handleAsyncRequest handles asynchronous requests
func (h *ASTHandler) handleAsyncRequest(async *Async) {
	// TODO: Implementar manejo de requests asíncronos
	// - HTTP client con timeout
	// - Lógica de reintentos
	// - Manejo de errores
	// - Logging de resultados
}

// TODO: Implementar métodos auxiliares para chaos engineering:
// - parseLatencyString(latency string) time.Duration
// - parseAbortString(abort string) bool
// - parseErrorString(error string) bool
// - handleRetries(async *Async) error
