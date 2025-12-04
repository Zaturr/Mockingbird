package database

import (
	"catalyst/database/internal"
	"context"
	"database/sql"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// WorkerConfig configuración del worker
type WorkerConfig struct {
	MaxWorkers    int           `json:"max_workers"`    // Número máximo de workers concurrentes
	QueueSize     int           `json:"queue_size"`     // Tamaño de la cola de trabajos
	Timeout       time.Duration `json:"timeout"`        // Timeout para operaciones
	RetryAttempts int           `json:"retry_attempts"` // Número de reintentos en caso de error
}

// Worker maneja las operaciones de inserción síncronas y asíncronas
type Worker struct {
	DB          *sql.DB
	Config      WorkerConfig
	JobQueue    chan *Mockdata
	ResultQueue chan error
	Ctx         context.Context
	Cancel      context.CancelFunc
	WaitGroup   sync.WaitGroup
	TimeStop    sync.RWMutex
	Running     bool
}

// BatchConfig configuración del sistema de batch
type BatchConfig struct {
	BatchSize     int           `json:"batch_size"`      // Tamaño del batch (default: 20)
	FlushInterval time.Duration `json:"flush_interval"`  // Intervalo para flush automático
	MaxQueueSize  int           `json:"max_queue_size"`  // Tamaño máximo de la cola de entrada
	MaxBatchQueue int           `json:"max_batch_queue"` // Tamaño máximo de la cola de batches
	MaxWorkers    int           `json:"max_workers"`     // Número de workers para procesar batches
	Timeout       time.Duration `json:"timeout"`         // Timeout para operaciones
	RetryAttempts int           `json:"retry_attempts"`  // Número de reintentos
	EnableMetrics bool          `json:"enable_metrics"`  // Habilitar métricas
}

// Batch representa un lote de operaciones
type Batch struct {
	ID         string      `json:"id"`
	Operations []*Mockdata `json:"operations"`
	CreatedAt  time.Time   `json:"created_at"`
	Size       int         `json:"size"`
}

// Mockdata representa los datos de una transacción mock
type Mockdata struct {
	UUID               string    `json:"uuid" db:"uuid"`
	RecepcionID        string    `json:"recepcion_id" db:"recepcion_id"`
	SenderID           string    `json:"sender_id" db:"sender_id"`
	RequestHeaders     string    `json:"request_headers" db:"request_headers"`
	RequestMethod      string    `json:"request_method" db:"request_method"`
	RequestEndpoint    string    `json:"request_endpoint" db:"request_endpoint"`
	RequestBody        string    `json:"request_body" db:"request_body"`
	ResponseHeaders    string    `json:"response_headers" db:"response_headers"`
	ResponseBody       string    `json:"response_body" db:"response_body"`
	ResponseStatusCode int       `json:"response_status_code" db:"response_status_code"`
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
}

// BatchManager maneja el sistema de batch con alta concurrencia
type BatchManager struct {
	DB        *sql.DB
	Config    BatchConfig
	QueueMgr  *QueueManager
	WaitGroup sync.WaitGroup
	Running   bool
	Mutex     sync.RWMutex

	TotalProcessed int64
	TotalBatches   int64
	TotalErrors    int64
	CurrentBatch   *Batch
	BatchMutex     sync.Mutex
	LastFlush      time.Time
	FlushTicker    *time.Ticker
}

// InsertOperation inserta una nueva operación en la base de datos
func InsertOperation(db *sql.DB, operation *Mockdata) error {
	query := `
	INSERT INTO mock_transactions (
		uuid, recepcion_id, sender_id, request_headers, request_method, 
		request_endpoint, request_body, response_headers, response_body, 
		response_status_code, timestamp
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query,
		operation.UUID,
		operation.RecepcionID,
		operation.SenderID,
		operation.RequestHeaders,
		operation.RequestMethod,
		operation.RequestEndpoint,
		operation.RequestBody,
		operation.ResponseHeaders,
		operation.ResponseBody,
		operation.ResponseStatusCode,
		operation.Timestamp,
	)

	return err
}

// InitDB inicializa la base de datos usando la función interna
func InitDB(dbPath string) (*sql.DB, error) {
	return internal.InitDB(dbPath)
}
