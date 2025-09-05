package models

type MockServer struct {
	Http Http `yaml:"http" json:"http"`
}
type Http struct {
	Servers []Server `yaml:"servers" json:"servers"`
}

type Server struct {
	Listen         int             `yaml:"listen" json:"listen"`
	Logger         *bool           `yaml:"logger" json:"logger"`
	ChaosInjection *ChaosInjection `yaml:"chaos_injection" json:"chaosInjection"`
	Location       []Location      `yaml:"location" json:"location"`
}

type Location struct {
	Path           string          `yaml:"path" json:"path"`
	Method         string          `yaml:"method" json:"method"`
	Schema         string          `yaml:"schema" json:"schema"`
	Response       string          `yaml:"response" json:"response"`
	Async          *Async          `yaml:"async" json:"async"`
	Headers        *Headers        `yaml:"headers" json:"headers"`
	StatusCode     int             `yaml:"status_code" json:"statusCode"`
	ChaosInjection *ChaosInjection `yaml:"chaos_injection" json:"chaosInjection"`
}

type Headers map[string]string

type Async struct {
	Url        string   `yaml:"url" json:"url"`
	Body       string   `yaml:"body" json:"body"`
	Method     string   `yaml:"method" json:"method"`
	Headers    *Headers `yaml:"headers" json:"headers"`
	Timeout    *int     `yaml:"timeout" json:"timeout"`
	Retries    *int     `yaml:"retries" json:"retries"`
	RetryDelay *int     `yaml:"retry_delay" json:"retryDelay"`
}

type ChaosInjection struct {
	Latency Latency `yaml:"latency" json:"latency"`
	Abort   Abort   `yaml:"abort" json:"abort"`
	Error   Error   `yaml:"error" json:"error"`
}

type Latency struct {
	Time        int    `yaml:"time" json:"time"`
	Probability string `yaml:"probability" json:"probability"`
}

type Abort struct {
	Code        int    `yaml:"code" json:"code"`
	Probability string `yaml:"probability" json:"probability"`
}

type Error struct {
	Code        int    `yaml:"code" json:"code"`
	Probability string `yaml:"probability" json:"probability"`
}