package config

import (
	"Mockingbird/network/handler"
)

// EndpointsConfig contiene todas las configuraciones de endpoints para todos los servicios
var EndpointsConfig = map[string]*handler.Http{
	"jsonplaceholder": getJsonplaceholderEndpoints(),
	"sypago":          getSypagoEndpoints(),
	"users":           getUsersEndpoints(),
}

// getJsonplaceholderEndpoints retorna todos los endpoints para Jsonplaceholder
func getJsonplaceholderEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Path:   "/api",
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "Post created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "api/jsonplaceholder/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "User created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "Comment created successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					// GET endpoints
					{
						Method:     "GET",
						Response:   &handler.Response{"message": "Post retrieved successfully", "service": "jsonplaceholder"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   &handler.Response{"status": "Jsonplaceholder service is healthy", "port": 8080},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   &handler.Response{"message": "Chaos test endpoint", "service": "jsonplaceholder"},
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
						Response:   &handler.Response{"message": "Post updated successfully", "service": "jsonplaceholder"},
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
						Response:   &handler.Response{"message": "Post deleted successfully", "service": "jsonplaceholder"},
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
				Path:   "/api/v1",
				Listen: 8081,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "Payment processed successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200 95% 400 5%",
						ChaosInjection: &handler.ChaosInjection{
							Latency: "300ms 15%",
							Error:   "500 3%",
						},
					},
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "Transaction created successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "OTP sent successfully", "service": "sypago", "otp": "123456"},
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
						Response:   &handler.Response{"status": "Sypago service is healthy", "port": 8081},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// PUT endpoints
					{
						Method:     "PUT",
						Response:   &handler.Response{"message": "Payment updated successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// DELETE endpoints
					{
						Method:     "DELETE",
						Response:   &handler.Response{"message": "Transaction cancelled successfully", "service": "sypago"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}

// getUsersEndpoints retorna todos los endpoints para Users
func getUsersEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Path:   "/api",
				Listen: 8082,
				Logger: &logger,
				Location: []handler.Location{
					// POST endpoints
					{
						Method:     "POST",
						Response:   &handler.Response{"message": "User created successfully", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "201",
					},
					// GET endpoints
					{
						Method:     "GET",
						Response:   &handler.Response{"message": "Users retrieved successfully", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					{
						Method:     "GET",
						Response:   &handler.Response{"status": "Users service is healthy", "port": 8082},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// PUT endpoints
					{
						Method:     "PUT",
						Response:   &handler.Response{"message": "User updated successfully", "service": "users"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
					// DELETE endpoints
					{
						Method:     "DELETE",
						Response:   &handler.Response{"message": "User deleted successfully", "service": "users"},
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
	// Retornar configuración por defecto en lugar de nil
	return getDefaultEndpoints()
}

// getDefaultEndpoints retorna una configuración por defecto
func getDefaultEndpoints() *handler.Http {
	logger := true
	return &handler.Http{
		Servers: []handler.Server{
			{
				Path:   "/",
				Listen: 8080,
				Logger: &logger,
				Location: []handler.Location{
					{
						Method:     "GET",
						Response:   &handler.Response{"message": "Default service", "status": "running"},
						Headers:    &handler.Headers{"Content-Type": "application/json"},
						StatusCode: "200",
					},
				},
			},
		},
	}
}
