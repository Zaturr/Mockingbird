package database

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// QueueManager maneja todas las colas del sistema
type QueueManager struct {
	InputQueue  chan *Mockdata
	BatchQueue  chan *Batch
	ResultQueue chan error
	Ctx         context.Context
	Cancel      context.CancelFunc
	WaitGroup   sync.WaitGroup
	Running     bool
	Mutex       sync.RWMutex
}

// NewQueueManager crea un nuevo manager de colas
func NewQueueManager(config BatchConfig) *QueueManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &QueueManager{
		InputQueue:  make(chan *Mockdata, config.MaxQueueSize),
		BatchQueue:  make(chan *Batch, config.MaxBatchQueue),
		ResultQueue: make(chan error, config.MaxBatchQueue),
		Ctx:         ctx,
		Cancel:      cancel,
		Running:     false,
	}
}

// Start inicia el manager de colas
func (qm *QueueManager) Start() error {
	qm.Mutex.Lock()
	defer qm.Mutex.Unlock()

	if qm.Running {
		return nil
	}

	qm.Running = true
	log.Println("QueueManager started")
	return nil
}

// Stop detiene el manager de colas
func (qm *QueueManager) Stop() {
	qm.Mutex.Lock()
	defer qm.Mutex.Unlock()

	if !qm.Running {
		return
	}

	qm.Cancel()
	close(qm.InputQueue)
	close(qm.BatchQueue)
	close(qm.ResultQueue)
	qm.Running = false

	log.Println("QueueManager stopped")
}

// AddRequest agrega una petición a la cola de entrada
func (qm *QueueManager) AddRequest(operation *Mockdata) error {
	qm.Mutex.RLock()
	if !qm.Running {
		qm.Mutex.RUnlock()
		return ErrQueueNotRunning
	}
	qm.Mutex.RUnlock()

	select {
	case qm.InputQueue <- operation:
		return nil
	case <-qm.Ctx.Done():
		return qm.Ctx.Err()
	default:
		return ErrQueueFull
	}
}

// AddBatch agrega un batch a la cola de procesamiento
func (qm *QueueManager) AddBatch(batch *Batch) error {
	select {
	case qm.BatchQueue <- batch:
		return nil
	case <-qm.Ctx.Done():
		return qm.Ctx.Err()
	default:
		return ErrQueueFull
	}
}

// SendResult envía un resultado a la cola de resultados
func (qm *QueueManager) SendResult(err error) error {
	select {
	case qm.ResultQueue <- err:
		return nil
	case <-qm.Ctx.Done():
		return qm.Ctx.Err()
	default:
		return ErrQueueFull
	}
}

// GetResult obtiene un resultado de la cola de resultados
func (qm *QueueManager) GetResult() (error, bool) {
	select {
	case err, ok := <-qm.ResultQueue:
		return err, ok
	case <-qm.Ctx.Done():
		return nil, false
	}
}

// GetStats retorna estadísticas de las colas
func (qm *QueueManager) GetStats() map[string]interface{} {
	qm.Mutex.RLock()
	defer qm.Mutex.RUnlock()

	return map[string]interface{}{
		"is_running":        qm.Running,
		"input_queue_size":  len(qm.InputQueue),
		"batch_queue_size":  len(qm.BatchQueue),
		"result_queue_size": len(qm.ResultQueue),
	}
}

// IsRunning retorna si el manager está ejecutándose
func (qm *QueueManager) IsRunning() bool {
	qm.Mutex.RLock()
	defer qm.Mutex.RUnlock()
	return qm.Running
}

// Errores de cola
var (
	ErrQueueNotRunning = fmt.Errorf("queue manager not running")
	ErrQueueFull       = fmt.Errorf("queue is full")
)
