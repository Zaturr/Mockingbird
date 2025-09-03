package handler

// AST structure for mock HTTP server configuration
// Estructura exacta según especificación del usuario

type Http struct {
	Servers []Server
}

type Server struct {
	Listen         int
	Logger         *bool
	ChaosInjection *ChaosInjection
	Location       []Location
}

type Location struct {
	Method         string
	Body           interface{}
	Response       interface{}
	Async          *Async
	Headers        *Headers
	StatusCode     string
	ChaosInjection *ChaosInjection
}

type Async struct {
	Url        string
	Body       interface{}
	Method     string
	Headers    *Headers
	Timeout    *string
	Retries    *int
	RetryDelay *string
}

type Headers map[string]string

type ChaosInjection struct {
	Latency string
	Abort   string
	Error   string
}
