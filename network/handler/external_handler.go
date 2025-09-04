package handler

import (
	"Mockingbird/chaos"
	"Mockingbird/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type ExternalHandler struct {
	chaosEngine *chaos.ChaosEngine
}

func NewExternalHandler() *ExternalHandler {
	return &ExternalHandler{
		chaosEngine: chaos.NewChaosEngine(),
	}
}

func (h *ExternalHandler) HandleConfigMapping(c *gin.Context) {
	var requestBody map[string]interface{}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
		return
	}

	mappedConfig := h.mapJSONToStruct(requestBody)

	// Aplicar inyección de caos después del mapeo
	h.applyChaosInjection(c, mappedConfig)

	// Si el contexto fue abortado por el caos, no continuar
	if c.IsAborted() {
		return
	}

	serverPort := h.findServerPort(requestBody, mappedConfig)

	response := gin.H{
		"mapped_config": mappedConfig,
		"server_port":   serverPort,
		"message":       "Configuration mapped successfully",
	}

	c.JSON(http.StatusOK, response)
}

func (h *ExternalHandler) findServerPort(requestBody map[string]interface{}, mappedConfig *models.Http) int {
	for _, server := range mappedConfig.Servers {
		if h.serverMatches(requestBody, server) {
			return server.Listen
		}
	}

	return 8080
}

func (h *ExternalHandler) serverMatches(requestBody map[string]interface{}, server models.Server) bool {
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

func (h *ExternalHandler) mapJSONToStruct(jsonData map[string]interface{}) *models.Http {
	httpConfig := &models.Http{}

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

func (h *ExternalHandler) mapServer(serverData map[string]interface{}) models.Server {
	server := models.Server{}

	if path, ok := serverData["path"].(string); ok {
		server.Path = path
	}

	if listen, ok := serverData["listen"].(float64); ok {
		server.Listen = int(listen)
	}

	if logger, ok := serverData["logger"].(bool); ok {
		server.Logger = &logger
	}

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

func (h *ExternalHandler) mapLocation(locationData map[string]interface{}) models.Location {
	location := models.Location{}

	if method, ok := locationData["method"].(string); ok {
		location.Method = method
	}

	if bodyData, ok := locationData["body"]; ok {
		body := models.Body(bodyData)
		location.Body = &body
	}

	if responseData, ok := locationData["response"]; ok {
		response := models.Response(responseData)
		location.Response = &response
	}

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

	if statusCode, ok := locationData["statusCode"].(string); ok {
		location.StatusCode = statusCode
	}

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

// applyChaosInjection aplica la inyección de caos basada en la configuración mapeada
func (h *ExternalHandler) applyChaosInjection(c *gin.Context, config *models.Http) {
	// Buscar configuración de caos en todos los servidores
	for _, server := range config.Servers {
		// Aplicar caos del servidor si existe
		if server.ChaosInjection != nil {
			h.applyServerChaos(c, server.ChaosInjection)
			if c.IsAborted() { // Si el caos a nivel de servidor aborta, no continuar
				return
			}
		}

		// Aplicar caos de las locations si existe
		for _, location := range server.Location {
			if location.ChaosInjection != nil {
				h.applyLocationChaos(c, location.ChaosInjection)
				if c.IsAborted() { // Si el caos a nivel de location aborta, no continuar
					return
				}
			}
		}
	}
}

// applyServerChaos aplica caos a nivel de servidor
func (h *ExternalHandler) applyServerChaos(c *gin.Context, chaosConfig *models.ChaosInjection) {
	// Aplicar latency
	if chaosConfig.Latency != "" {
		if latency := h.chaosEngine.ApplyLatency(chaosConfig.Latency); latency > 0 {
			time.Sleep(latency)
		}
	}

	// Aplicar abort
	if chaosConfig.Abort != "" {
		if statusCode := h.chaosEngine.ApplyAbort(chaosConfig.Abort); statusCode > 0 {
			c.JSON(statusCode, gin.H{"error": "Chaos injection: abort triggered"})
			c.Abort()
			return
		}
	}

	// Aplicar error
	if chaosConfig.Error != "" {
		if statusCode := h.chaosEngine.ApplyError(chaosConfig.Error); statusCode > 0 {
			c.JSON(statusCode, gin.H{"error": "Chaos injection: error triggered"})
			c.Abort()
			return
		}
	}
}

// applyLocationChaos aplica caos a nivel de location
func (h *ExternalHandler) applyLocationChaos(c *gin.Context, chaosConfig *models.ChaosInjection) {
	// Aplicar latency
	if chaosConfig.Latency != "" {
		if latency := h.chaosEngine.ApplyLatency(chaosConfig.Latency); latency > 0 {
			time.Sleep(latency)
		}
	}

	// Aplicar abort
	if chaosConfig.Abort != "" {
		if statusCode := h.chaosEngine.ApplyAbort(chaosConfig.Abort); statusCode > 0 {
			c.JSON(statusCode, gin.H{"error": "Chaos injection: abort triggered"})
			c.Abort()
			return
		}
	}

	// Aplicar error
	if chaosConfig.Error != "" {
		if statusCode := h.chaosEngine.ApplyError(chaosConfig.Error); statusCode > 0 {
			c.JSON(statusCode, gin.H{"error": "Chaos injection: error triggered"})
			c.Abort()
			return
		}
	}
}
