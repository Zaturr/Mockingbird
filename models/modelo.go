package models

// AST structure for mock HTTP server configuration
// Estructura exacta según especificación del usuario

type Http struct {
	Servers []Server `json:"servers"`
}

type Server struct {
	Path           string          `json:"path"`
	Listen         int             `json:"listen"`
	Logger         *bool           `json:"logger"`
	ChaosInjection *ChaosInjection `json:"chaosInjection"`
	Location       []Location      `json:"location"`
}

type Location struct {
	Method         string          `json:"method"`
	Schema         *Schema         `json:"Schema"`
	Response       *Response       `json:"response"`
	Async          *Async          `json:"async,omitempty"`
	Headers        *Headers        `json:"headers"`
	StatusCode     string          `json:"statusCode"`
	ChaosInjection *ChaosInjection `json:"chaosInjection"`
}

type Async struct {
	Url        string   `json:"url"`
	Body       *Body    `json:"body"`
	Method     string   `json:"method"`
	Headers    *Headers `json:"headers"`
	Timeout    *string  `json:"timeout"`
	Retries    *int     `json:"retries"`
	RetryDelay *string  `json:"retryDelay"`
}

type ChaosInjection struct {
	Latency string `json:"latency"`
	Abort   string `json:"abort"`
	Error   string `json:"error"`
}

type Headers map[string]string

type Body []byte
type Schema []byte

type Response []byte
