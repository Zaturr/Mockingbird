package models

import "github.com/testcontainers/testcontainers-go/modules/postgres"

type MockServer struct {
	Http            Http            `yaml:"http" json:"http"`
	PostgresServers PostgresServers `yaml:"postgres" json:"postgres"`
}
type Http struct {
	Servers []Server `yaml:"servers" json:"servers"`
}

type PostgresServers struct {
	Postgres []PostgresServer `yaml:"postgres" json:"postgres"`
}

type Server struct {
	Listen         int             `yaml:"listen" json:"listen"`
	Logger         *bool           `yaml:"logger" json:"logger"`
	LoggerPath     *string         `yaml:"logger_path" json:"logger_path"`
	Name           *string         `yaml:"name" json:"name"`
	Version        *string         `yaml:"version" json:"version"`
	ChaosInjection *ChaosInjection `yaml:"chaos_injection" json:"chaosInjection"`
	Location       []Location      `yaml:"location" json:"location"`
}

type LogDescriptor struct {
	Name    string
	Version string
	Path    string
	File    bool
	Logger  bool
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
	Response    string `yaml:"response" json:"response"`
}

type LogSettings struct {
	Console            bool   `yaml:"console"`
	BeautifyConsoleLog bool   `yaml:"beautify_console"`
	File               bool   `yaml:"file"`
	Path               string `yaml:"path"`
	MinLevel           string `yaml:"min_level"`
	RotationMaxSizeMB  int    `yaml:"rotation_max_size_mb"`
	MaxAgeDay          int    `yaml:"max_age_day"`
	MaxBackups         int    `yaml:"max_backups"`
	Compress           bool   `yaml:"compress"`
}

type PostgresServer struct {
	Name              string
	User              string
	Password          string
	Host              string
	Port              int
	Database          string
	InitScript        string
	Seed              string
	PostgresContainer *postgres.PostgresContainer
	Logger            *bool
	LoggerPath        *string
	File              *bool
}

type Seed struct {
	Table     string
	Schema    string
	Rows      int
	Overrides []Overrides
}

type Overrides struct {
	Column string
	Value  string
}
