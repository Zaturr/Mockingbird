package handler

import (
	"Mockingbird/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Conexion struct {
	config *models.Http
}

func NewExternalHandler() *Conexion {
	return &Conexion{
		config: DefaultConfig(),
	}
}

// createDefaultConfig crea la configuración por defecto según la especificación
func DefaultConfig() *Http {
	logger := true
	return &Http{
		Servers: []Server{
			{
				Path:   "/",
				Listen: 8080,
				Logger: &logger,
				ChaosInjection: &ChaosInjection{
					Latency: "0ms 10% 1000ms 50%",
					Abort:   "400 10%",
					Error:   "500 5%",
				},
				Location: []Location{
					{
						Method:   "POST",
						Body:     &Body{},
						Response: &Response{},
						Async: &Async{
							Url:        "http://localhost:9090/health",
							Method:     "POST",
							Body:       &Body{"event": "push_received"},
							Headers:    &Headers{"Content-Type": "application/json"},
							Timeout:    stringPtr("500ms"),
							Retries:    intPtr(3),
							RetryDelay: stringPtr("200ms"),
						},
						Headers:    &Headers{"Content-Type": "application/json"},
						StatusCode: "200 50% 500 50%",
					},
					{
						Method:     "GET",
						Response:   &Response{},
						Headers:    &Headers{"Content-Type": "application/json"},
						StatusCode: "200 80%",
						ChaosInjection: &ChaosInjection{
							Latency: "0ms 500ms 30%",
							Abort:   "503 15%",
							Error:   "500 10%",
						},
					},
				},
			},
			{
				Path:   "/",
				Listen: 9090,
				Location: []Location{
					{
						Method:     "GET",
						Response:   &Response{"status": "ok"},
						Headers:    &Headers{"Content-Type": "application/json"},
						StatusCode: "200 100%",
					},
				},
			},
		},
	}
}

// SetupExternalRoutes configura las rutas
func (h *Conexion) SetupExternalRoutes(router *gin.Engine) {
	// Endpoint POST para recibir configuración y mapear
	router.POST("/api/config", h.handleConfigMapping)
}

// handleConfigMapping maneja el mapeo de configuración
func (h *Conexion) handleConfigMapping(c *gin.Context) {
	var requestBody map[string]interface{}

	// Leer el body de la request
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Buscar el puerto del servidor que se está llamando
	serverPort := h.findServerPort(requestBody)

	// Mapear el JSON a la estructura
	mappedConfig := h.mapJSONToStruct(requestBody)

	// Agregar el puerto encontrado
	response := gin.H{
		"mapped_config": mappedConfig,
		"server_port":   serverPort,
		"message":       "Configuration mapped successfully",
	}

	c.JSON(http.StatusOK, response)
}

// findServerPort busca el puerto del servidor basado en la configuración
func (h *Conexion) findServerPort(requestBody map[string]interface{}) int {
	// Buscar en la configuración existente
	for _, server := range h.config.Servers {
		// Verificar si este servidor coincide con la request
		if h.serverMatches(requestBody, server) {
			return server.Listen
		}
	}

	// Si no se encuentra, retornar puerto por defecto
	return 8080
}

// serverMatches verifica si un servidor coincide con la request
func (h *Conexion) serverMatches(requestBody map[string]interface{}, server Server) bool {
	// Lógica simple: verificar si hay locations que coincidan
	if requestBody["server"] != nil {
		serverData := requestBody["server"].(map[string]interface{})
		if serverData["listen"] != nil {
			requestPort := int(serverData["listen"].(float64))
			return requestPort == server.Listen
		}
	}
	return false
}

// mapJSONToStruct mapea el JSON recibido a la estructura
func (h *Conexion) mapJSONToStruct(jsonData map[string]interface{}) *Http {
	// Crear la estructura base
	httpConfig := &Http{}

	// Mapear servers si existen
	if serversData, ok := jsonData["http"].(map[string]interface{}); ok {
		if servers, ok := serversData["server"].([]interface{}); ok {
			httpConfig.Servers = make([]Server, 0)

			for _, serverData := range servers {
				if serverMap, ok := serverData.(map[string]interface{}); ok {
					server := h.mapServer(serverMap)
					httpConfig.Servers = append(httpConfig.Servers, server)
				}
			}
		}
	}

	return httpConfig
}

// mapServer mapea un servidor individual
func (h *Conexion) mapServer(serverData map[string]interface{}) Server {
	server := Server{}

	if path, ok := serverData["Path"].(string); ok {
		server.Path = path
	}

	if listen, ok := serverData["listen"].(float64); ok {
		server.Listen = int(listen)
	}

	if logger, ok := serverData["logger"].(bool); ok {
		server.Logger = &logger
	}

	if chaosData, ok := serverData["chaosInjection"].(map[string]interface{}); ok {
		chaos := &ChaosInjection{}
		if latency, ok := chaosData["latency"].(string); ok {
			chaos.Latency = latency
		}
		if abort, ok := chaosData["abort"].(string); ok {
			chaos.Abort = abort
		}
		if error, ok := chaosData["error"].(string); ok {
			chaos.Error = error
		}
		server.ChaosInjection = chaos
	}

	// Mapear locations
	if locationsData, ok := serverData["location"].([]interface{}); ok {
		server.Location = make([]Location, 0)

		for _, locationData := range locationsData {
			if locationMap, ok := locationData.(map[string]interface{}); ok {
				location := h.mapLocation(locationMap)
				server.Location = append(server.Location, location)
			}
		}
	}

	return server
}

// mapLocation mapea una location individual
func (h *Conexion) mapLocation(locationData map[string]interface{}) Location {
	location := Location{}

	// Mapear campos básicos
	if method, ok := locationData["method"].(string); ok {
		location.Method = method
	}

	// Mapear body
	if bodyData, ok := locationData["body"]; ok {
		body := Body(bodyData.(map[string]interface{}))
		location.Body = &body
	}

	// Mapear response
	if responseData, ok := locationData["response"]; ok {
		response := Response(responseData.(map[string]interface{}))
		location.Response = &response
	}

	if asyncData, ok := locationData["async"].(map[string]interface{}); ok {
		async := &Async{}
		if url, ok := asyncData["url"].(string); ok {
			async.Url = url
		}
		if method, ok := asyncData["method"].(string); ok {
			async.Method = method
		}
		if body, ok := asyncData["body"]; ok {
			bodyMap := Body(body.(map[string]interface{}))
			async.Body = &bodyMap
		}
		if headers, ok := asyncData["headers"]; ok {
			headersMap := make(Headers)
			if headersData, ok := headers.(map[string]interface{}); ok {
				for k, v := range headersData {
					if str, ok := v.(string); ok {
						headersMap[k] = str
					}
				}
			}
			async.Headers = &headersMap
		}
		if timeout, ok := asyncData["timeout"].(string); ok {
			async.Timeout = &timeout
		}
		if retries, ok := asyncData["retries"].(float64); ok {
			retriesInt := int(retries)
			async.Retries = &retriesInt
		}
		if retryDelay, ok := asyncData["retryDelay"].(string); ok {
			async.RetryDelay = &retryDelay
		}
		location.Async = async
	}

	if headersData, ok := locationData["headers"]; ok {
		headersMap := make(Headers)
		if headers, ok := headersData.(map[string]interface{}); ok {
			for k, v := range headers {
				if str, ok := v.(string); ok {
					headersMap[k] = str
				}
			}
		}
		location.Headers = &headersMap
	}

	if statusCode, ok := locationData["statusCode"].(string); ok {
		location.StatusCode = statusCode
	}

	// Mapear chaos injection de location
	if chaosData, ok := locationData["chaosInjection"].(map[string]interface{}); ok {
		chaos := &ChaosInjection{}
		if latency, ok := chaosData["latency"].(string); ok {
			chaos.Latency = latency
		}
		if abort, ok := chaosData["abort"].(string); ok {
			chaos.Abort = abort
		}
		if error, ok := chaosData["error"].(string); ok {
			chaos.Error = error
		}
		location.ChaosInjection = chaos
	}

	return location
}

func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }
