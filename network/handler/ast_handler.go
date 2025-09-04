// package handler
//
// import (
//
//	"fmt"
//	"net/http"
//	"strconv"
//	"strings"
//
//	"github.com/gin-gonic/gin"
//
// )
//
// // ASTHandler handles requests based on AST configuration
//
//	type ASTHandler struct {
//		config      *Http
//		chaosEngine *ChaosEngine
//	}
//
// // NewASTHandler creates a new handler with AST configuration
//
//	func NewASTHandler(config *Http) *ASTHandler {
//		return &ASTHandler{
//			config:      config,
//			chaosEngine: NewChaosEngine(),
//		}
//	}
//
// // SetupRoutes configures all routes based on AST configuration
//
//	func (h *ASTHandler) SetupRoutes(router *gin.Engine) {
//		// Validar que la configuración no sea nil
//		if h.config == nil {
//			fmt.Printf("Warning: ASTHandler config is nil, skipping route setup\n")
//			return
//		}
//
//		// Validar que haya servidores configurados
//		if len(h.config.Servers) == 0 {
//			fmt.Printf("Warning: No servers configured in ASTHandler\n")
//			return
//		}
//
//		for _, server := range h.config.Servers {
//			// Validar que el servidor tenga locations
//			if len(server.Location) == 0 {
//				fmt.Printf("Warning: Server on port %d has no locations configured\n", server.Listen)
//				continue
//			}
//
//			for i, location := range server.Location {
//				// Obtener configuraciones de chaos
//				serverChaos := server.ChaosInjection
//				locationChaos := location.ChaosInjection
//
//				// Crear handler para esta location
//				handler := h.createLocationHandler(location, serverChaos, locationChaos)
//
//				// Generar ruta basada en el método y contexto
//				path := h.generateRoutePath(location, i)
//
//				// Registrar ruta basada en el método HTTP
//				switch location.Method {
//				case "GET":
//					router.GET(path, handler)
//				case "POST":
//					router.POST(path, handler)
//				case "PUT":
//					router.PUT(path, handler)
//				case "DELETE":
//					router.DELETE(path, handler)
//				case "PATCH":
//					router.PATCH(path, handler)
//				default:
//					// Log warning para métodos no soportados
//					fmt.Printf("Warning: Unsupported HTTP method '%s' for path '%s'\n", location.Method, path)
//				}
//			}
//		}
//	}
//
// // generateRoutePath genera la ruta basada en la configuración de la location
//
//	func (h *ASTHandler) generateRoutePath(location Location, index int) string {
//		// Lógica inteligente para generar rutas basada en el contexto
//		switch {
//		case location.Method == "GET" && index == 4: // health check
//			return "/health"
//		case location.Method == "GET" && index == 3: // get by id
//			return "/api/posts/:id"
//		case location.Method == "POST" && h.isOTPEndpoint(location): // OTP endpoint
//			return "/api/v1/transaction/otp"
//		default:
//			// Generar ruta por defecto basada en método y contexto
//			return fmt.Sprintf("/api/%s/%d", location.Method, index)
//		}
//	}
//
// // isOTPEndpoint verifica si es el endpoint de OTP
//
//	func (h *ASTHandler) isOTPEndpoint(location Location) bool {
//		if location.Response == nil {
//			return false
//		}
//
//		// Convertir a string y buscar el indicador
//		responseStr := fmt.Sprintf("%v", location.Response)
//		return strings.Contains(responseStr, "OTP sent successfully")
//	}
//
// // createLocationHandler creates a handler function for a specific location
//
//	func (h *ASTHandler) createLocationHandler(location Location, serverChaos, locationChaos *ChaosInjection) gin.HandlerFunc {
//		return func(c *gin.Context) {
//			// Apply chaos injection (server level first, then location level)
//			if serverChaos != nil {
//				h.applyChaosInjection(c, serverChaos)
//			}
//			if locationChaos != nil {
//				h.applyChaosInjection(c, locationChaos)
//			}
//
//			// Handle async requests if configured
//			if location.Async != nil {
//				go h.handleAsyncRequest(location.Async)
//			}
//
//			// Determine response status code
//			statusCode := h.determineStatusCode(location.StatusCode)
//
//			// Set response headers
//			if location.Headers != nil {
//				for key, value := range *location.Headers {
//					c.Header(key, value)
//				}
//			}
//
//			// Send response
//			if location.Response != nil {
//				c.JSON(statusCode, location.Response)
//			} else {
//				c.JSON(statusCode, map[string]string{"message": "No response configured"})
//			}
//		}
//	}
//
// // applyChaosInjection applies chaos engineering features
//
//	func (h *ASTHandler) applyChaosInjection(c *gin.Context, chaos *ChaosInjection) {
//		if chaos == nil {
//			return
//		}
//
//		// Aplicar latencia si está configurada
//		if chaos.Latency != "" {
//			h.chaosEngine.ApplyLatency(chaos.Latency)
//		}
//
//		// Verificar si debe abortar la request
//		if chaos.Abort != "" && h.chaosEngine.ShouldAbort(chaos.Abort) {
//			// Extraer código de estado del string de abort (ej: "503 10%" -> 503)
//			parts := strings.Fields(chaos.Abort)
//			if len(parts) > 0 {
//				if abortCode, err := strconv.Atoi(parts[0]); err == nil {
//					c.AbortWithStatus(abortCode)
//					return
//				}
//			}
//			// Fallback a 503 si no se puede parsear
//			c.AbortWithStatus(503)
//			return
//		}
//
//		// Verificar si debe retornar error
//		if chaos.Error != "" && h.chaosEngine.ShouldReturnError(chaos.Error) {
//			// Extraer código de estado del string de error (ej: "500 5%" -> 500)
//			parts := strings.Fields(chaos.Error)
//			if len(parts) > 0 {
//				if errorCode, err := strconv.Atoi(parts[0]); err == nil {
//					c.AbortWithStatus(errorCode)
//					return
//				}
//			}
//			// Fallback a 500 si no se puede parsear
//			c.AbortWithStatus(500)
//			return
//		}
//	}
//
// // determineStatusCode determines response status code
//
//	func (h *ASTHandler) determineStatusCode(statusCode string) int {
//		if statusCode == "" {
//			return http.StatusOK
//		}
//
//		// Usar el motor de chaos para determinar el status code
//		return h.chaosEngine.GetStatusCode(statusCode)
//	}
//
// // handleAsyncRequest handles asynchronous requests
//
//	func (h *ASTHandler) handleAsyncRequest(async *Async) {
//		// TODO: Implementar manejo de requests asíncronos
//		// - HTTP client con timeout
//		// - Lógica de reintentos
//		// - Manejo de errores
//		// - Logging de resultados
//	}
//
// // TODO: Implementar métodos auxiliares para chaos engineering:
// // - parseLatencyString(latency string) time.Duration
// // - parseAbortString(abort string) bool
// // - parseErrorString(error string) bool
// // - handleRetries(async *Async) error
package handler
