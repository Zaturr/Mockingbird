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
	Postgres []PostgresServer `yaml:"servers" json:"servers"`
}

type Server struct {
	Listen         int             `yaml:"listen" json:"listen"`
	Logger         *bool           `yaml:"logger" json:"logger"`
	LoggerPath     *string         `yaml:"logger_path" json:"logger_path"`
	Name           *string         `yaml:"name" json:"name"`
	Version        *string         `yaml:"version" json:"version"`
	ChaosInjection *ChaosInjection `yaml:"chaos_injection" json:"chaos_injection"`
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
	StaticFilesDir string          `yaml:"static_dir" json:"static_dir"`
	Schema         string          `yaml:"schema" json:"schema"`
	Response       string          `yaml:"response" json:"response"`
	Async          []Async         `yaml:"async" json:"async"`
	Headers        *Headers        `yaml:"headers" json:"headers"`
	StatusCode     int             `yaml:"status_code" json:"statusCode"`
	ChaosInjection *ChaosInjection `yaml:"chaos_injection" json:"chaos_injection"`
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
	Name              string                      `yaml:"name" json:"name"`
	User              string                      `yaml:"user" json:"user"`
	Password          string                      `yaml:"password" json:"password"`
	Host              string                      `yaml:"host" json:"host"`
	Port              int                         `yaml:"port" json:"port"`
	Database          string                      `yaml:"database" json:"database"`
	InitScript        string                      `yaml:"init_script" json:"init_script"`
	Seed              []Seed                      `yaml:"seed" json:"seed"`
	PostgresContainer *postgres.PostgresContainer `yaml:"postgres_container" json:"postgres_container"`
	Logger            *bool                       `yaml:"logger" json:"logger"`
	LoggerPath        *string                     `yaml:"logger_path" json:"logger_path"`
	File              *bool                       `yaml:"file" json:"file"`
}

type Seed struct {
	Table     string      `yaml:"table" json:"table"`
	Schema    string      `yaml:"schema" json:"schema"`
	Rows      int         `yaml:"rows" json:"rows"`
	Overrides []Overrides `yaml:"overrides" json:"overrides"`
}

type Overrides struct {
	Column string `yaml:"column" json:"column"`
	Value  string `yaml:"value" json:"value"`
}
