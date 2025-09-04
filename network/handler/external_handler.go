package handler

import (
	"Mockingbird/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ExternalHandler maneja las conexiones a APIs externas
type ExternalHandler struct{}

// NewExternalHandler crea una nueva instancia del handler externo
func NewExternalHandler() *ExternalHandler {
	return &ExternalHandler{}
}

// SetupExternalRoutes configura las rutas
func (h *ExternalHandler) SetupExternalRoutes(router *gin.Engine) {
	// Endpoint POST para recibir configuración y mapear
	router.POST("/api/config", h.handleConfigMapping)
}

// handleConfigMapping maneja el mapeo de configuración
func (h *ExternalHandler) handleConfigMapping(c *gin.Context) {
	var requestBody map[string]interface{}
	
	// Leer el body de la request
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	// Mapear el JSON a la estructura
	mappedConfig := h.mapJSONToStruct(requestBody)
	
	// Buscar el puerto del servidor que se está llamando
	serverPort := h.findServerPort(requestBody, mappedConfig)
	
	// Respuesta con la configuración mapeada y el puerto encontrado
	response := gin.H{
		"mapped_config": mappedConfig,
		"server_port":   serverPort,
		"message":       "Configuration mapped successfully",
	}
	
	c.JSON(http.StatusOK, response)
}

// findServerPort busca el puerto del servidor basado en la configuración recibida
func (h *ExternalHandler) findServerPort(requestBody map[string]interface{}, mappedConfig *models.Http) int {
	// Usar un for para revisar todos los servidores y encontrar el que coincida
	for _, server := range mappedConfig.Servers {
		// Verificar si este servidor coincide con la request
		if h.serverMatches(requestBody, server) {
			return server.Listen
		}
	}
	
	// Si no se encuentra, retornar puerto por defecto
	return 8080
}

// serverMatches verifica si un servidor coincide con la request
func (h *ExternalHandler) serverMatches(requestBody map[string]interface{}, server models.Server) bool {
	// Buscar en el JSON recibido si hay un servidor que coincida
	if httpData, ok := requestBody["http"]; ok {
		if serversData, ok := httpData.(map[string]interface{}); ok {
			if servers, ok := serversData["server"].([]interface{}); ok {
				for _, serverData := range servers {
					if serverMap, ok := serverData.(map[string]interface{}); ok {
						if listen, ok := serverMap["listen"].(float64); ok {
							requestPort := int(listen)
							if requestPort == server.Listen {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

// mapJSONToStruct mapea el JSON recibido a la estructura del modelo
func (h *ExternalHandler) mapJSONToStruct(jsonData map[string]interface{}) *models.Http {
	// Crear la estructura base
	httpConfig := &models.Http{}
	
	// Mapear servers si existen
	if serversData, ok := jsonData["http"].(map[string]interface{}); ok {
		if servers, ok := serversData["server"].([]interface{}); ok {
			httpConfig.Servers = make([]models.Server, 0)
			
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
func (h *ExternalHandler) mapServer(serverData map[string]interface{}) models.Server {
	server := models.Server{}
	
	// Mapear campos básicos
	if listen, ok := serverData["listen"].(float64); ok {
		server.Listen = int(listen)
	}
	
	if logger, ok := serverData["logger"].(bool); ok {
		server.Logger = &logger
	}
	
	// Mapear chaos injection
	if chaosData, ok := serverData["chaosInjection"].(map[string]interface{}); ok {
		chaos := &models.ChaosInjection{}
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
		server.Location = make([]models.Location, 0)
		
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
func (h *ExternalHandler) mapLocation(locationData map[string]interface{}) models.Location {
	location := models.Location{}
	
	// Mapear campos básicos
	if method, ok := locationData["method"].(string); ok {
		location.Method = method
	}
	
	// Mapear body
	if bodyData, ok := locationData["body"]; ok {
		body := models.Body(bodyData)
		location.Body = body
	}
	
	// Mapear response
	if responseData, ok := locationData["response"]; ok {
		response := models.Response(responseData)
		location.Response = response
	}
	
	// Mapear async (opcional)
	if asyncData, ok := locationData["async"].(map[string]interface{}); ok {
		async := &models.Async{}
		if url, ok := asyncData["url"].(string); ok {
			async.Url = url
		}
		if method, ok := asyncData["method"].(string); ok {
			async.Method = method
		}
		if body, ok := asyncData["body"]; ok {
			bodyMap := models.Body(body)
			async.Body = &bodyMap
		}
		if headers, ok := asyncData["headers"]; ok {
			headersMap := make(models.Headers)
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
	
	// Mapear headers
	if headersData, ok := locationData["headers"]; ok {
		headersMap := make(models.Headers)
		if headers, ok := headersData.(map[string]interface{}); ok {
			for k, v := range headers {
				if str, ok := v.(string); ok {
					headersMap[k] = str
				}
			}
		}
		location.Headers = &headersMap
	}
	
	// Mapear status code
	if statusCode, ok := locationData["statusCode"].(string); ok {
		location.StatusCode = statusCode
	}
	
	// Mapear chaos injection de location
	if chaosData, ok := locationData["chaosInjection"].(map[string]interface{}); ok {
		chaos := &models.ChaosInjection{}
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
