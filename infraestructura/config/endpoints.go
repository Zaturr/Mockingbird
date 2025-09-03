package config

import "Mockingbird/network/handler"

// EndpointsConfig contiene todas las configuraciones de endpoints para todos los servicios
var EndpointsConfig = map[string]*handler.Http{
	"jsonplaceholder": getJsonplaceholderEndpoints(),
	"sypago":          getSypagoEndpoints(),
	"users":           getUsersEndpoints(), // Ejemplo de nuevo servicio
	"default":         getDefaultEndpoints(),
}

// getJsonplaceholderEndpoints retorna todos los endpoints para Jsonplaceholder
func getJsonplaceholderEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Post created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "api/jsonplaceholder/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "User created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Comment created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					// GET endpoints
					{
						Method:     "GET",
						Response:   map[string]interface{}{"message": "Post retrieved successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Jsonplaceholder service is healthy", "port": 8080},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"message": "Chaos test endpoint", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 80% 500 20%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "100ms 30%",
							Abort:   "503 10%",
							Error:   "500 5%",
						},
					},
					// PUT endpoints
					{
						Method:     "PUT",
						Response:   map[string]interface{}{"message": "Post updated successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 90% 400 10%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "200ms 20%",
							Error:   "422 5%",
						},
					},
					// DELETE endpoints
					{
						Method:     "DELETE",
						Response:   map[string]interface{}{"message": "Post deleted successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 85% 404 15%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "150ms 25%",
							Abort:   "403 8%",
						},
					},
				},
			},
		},
	}
}

// getSypagoEndpoints retorna todos los endpoints para Sypago
func getSypagoEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8081,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Payment processed successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 95% 400 5%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "300ms 15%",
							Error:   "500 3%",
						},
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "Transaction created successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "OTP sent successfully", "service": "sypago", "otp": "123456"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 90% 400 10%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "150ms 25%",
							Error:   "500 5%",
						},
					},
					// GET endpoints
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Sypago service is healthy", "port": 8081},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// PUT endpoints
					{
						Method:     "PUT",
						Response:   map[string]interface{}{"message": "Payment updated successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// DELETE endpoints
					{
						Method:     "DELETE",
						Response:   map[string]interface{}{"message": "Transaction cancelled successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

// getUsersEndpoints retorna todos los endpoints para el servicio de Usuarios (ejemplo)
func getUsersEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8082,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   map[string]interface{}{"message": "User registered successfully", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					// GET endpoints
					{
						Method:     "GET",
						Response:   map[string]interface{}{"message": "User profile retrieved", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Users service is healthy", "port": 8082},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// PUT endpoints
					{
						Method:     "PUT",
						Response:   map[string]interface{}{"message": "User profile updated", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// DELETE endpoints
					{
						Method:     "DELETE",
						Response:   map[string]interface{}{"message": "User account deleted", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

// getDefaultEndpoints retorna una configuración por defecto
func getDefaultEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					{
						Method:     "GET",
						Response:   map[string]interface{}{"status": "Service is healthy"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

// GetServiceEndpoints retorna la configuración de endpoints para un servicio específico
func GetServiceEndpoints(serviceName string) *handler.Http {
	if config, exists := EndpointsConfig[serviceName]; exists {
		return config
	}
	return EndpointsConfig["default"]
}
