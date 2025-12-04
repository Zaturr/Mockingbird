package database

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// NewWorker crea una nueva instancia del worker
func NewWorker(db *sql.DB, config WorkerConfig) *Worker {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 2
	}
	if config.QueueSize <= 0 {
		config.QueueSize = 100
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Worker{
		DB:          db,
		Config:      config,
		JobQueue:    make(chan *Mockdata, config.QueueSize),
		ResultQueue: make(chan error, config.QueueSize),
		Ctx:         ctx,
		Cancel:      cancel,
		Running:     false,
	}
}

// Start inicia el worker con el número especificado de goroutines
func (w *Worker) Start() error {
	w.TimeStop.Lock()
	defer w.TimeStop.Unlock()

	if w.IsRunning() {
		return nil
	}

	w.Running = true

	// Iniciar workers concurrentes
	for i := 0; i < w.Config.MaxWorkers; i++ {
		w.WaitGroup.Add(1)
		go w.workerRoutine(i)
	}

	log.Printf("Worker started with %d concurrent workers", w.Config.MaxWorkers)
	return nil
}

// Stop detiene el worker y espera a que terminen todas las operaciones
func (w *Worker) Stop() {
	w.TimeStop.Lock()
	defer w.TimeStop.Unlock()

	if !w.IsRunning() {
		return
	}

	w.Cancel()
	close(w.JobQueue)
	w.WaitGroup.Wait()
	close(w.ResultQueue)
	w.Running = false

	log.Println("Worker stopped")
}

// workerRoutine es la rutina principal de cada worker
func (w *Worker) workerRoutine(id int) {
	defer w.WaitGroup.Done()

	log.Printf("Worker %d started", id)

	for {
		select {
		case <-w.Ctx.Done():
			log.Printf("Worker %d stopping", id)
			return
		case operation, ok := <-w.JobQueue:
			if !ok {
				log.Printf("Worker %d: job queue closed", id)
				return
			}

			// Procesar la operación
			err := w.processOperation(operation)
			if err != nil {
				log.Printf("Worker %d: error processing operation %s: %v", id, operation.UUID, err)
			}

			// Enviar resultado
			select {
			case w.ResultQueue <- err:
			case <-w.Ctx.Done():
				return
			}
		}
	}
}

// processOperation procesa una operación individual con reintentos
func (w *Worker) processOperation(operation *Mockdata) error {
	var lastErr error

	for attempt := 1; attempt <= w.Config.RetryAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(w.Ctx, w.Config.Timeout)

		err := w.insertWithContext(ctx, operation)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < w.Config.RetryAttempts {
			log.Printf("Retry attempt %d for operation %s: %v", attempt, operation.UUID, err)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return lastErr
}

// insertWithContext ejecuta la inserción con contexto
func (w *Worker) insertWithContext(ctx context.Context, operation *Mockdata) error {
	done := make(chan error, 1)

	go func() {
		done <- InsertOperation(w.DB, operation)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InsertSync inserta una operación de manera síncrona
func (w *Worker) InsertSync(operation *Mockdata) error {
	ctx, cancel := context.WithTimeout(w.Ctx, w.Config.Timeout)
	defer cancel()

	return w.insertWithContext(ctx, operation)
}

// InsertAsync inserta una operación de manera asíncrona
func (w *Worker) InsertAsync(operation *Mockdata) error {
	w.TimeStop.RLock()
	if !w.IsRunning() {
		w.TimeStop.RUnlock()
		return InsertOperation(w.DB, operation) // Fallback a síncrono
	}
	w.TimeStop.RUnlock()

	select {
	case w.JobQueue <- operation:
		return nil
	case <-w.Ctx.Done():
		return w.Ctx.Err()
	default:
		return InsertOperation(w.DB, operation) // Fallback a síncrono si la cola está llena
	}
}

// GetQueueSize retorna el tamaño actual de la cola
func (w *Worker) GetQueueSize() int {
	return len(w.JobQueue)
}

// IsRunning retorna si el worker está ejecutándose
func (w *Worker) IsRunning() bool {
	w.TimeStop.RLock()
	defer w.TimeStop.RUnlock()
	return w.Running
}

// GetStats retorna estadísticas del worker
func (w *Worker) GetStats() map[string]interface{} {
	w.TimeStop.RLock()
	defer w.TimeStop.RUnlock()

	return map[string]interface{}{
		"is_running":     w.Running,
		"queue_size":     len(w.JobQueue),
		"max_workers":    w.Config.MaxWorkers,
		"queue_capacity": w.Config.QueueSize,
		"timeout":        w.Config.Timeout,
		"retry_attempts": w.Config.RetryAttempts,
	}
}
