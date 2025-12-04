package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

func NewBatchManager(db *sql.DB, config BatchConfig) *BatchManager {

	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 2 * time.Second
	}
	if config.MaxQueueSize <= 0 {
		config.MaxQueueSize = 10000
	}
	if config.MaxBatchQueue <= 0 {
		config.MaxBatchQueue = 1000
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.RetryAttempts <= 0 {
		config.RetryAttempts = 3
	}

	return &BatchManager{
		DB:       db,
		Config:   config,
		QueueMgr: NewQueueManager(config),
		Running:  false,
		CurrentBatch: &Batch{
			ID:         generateBatchID(),
			Operations: make([]*Mockdata, 0, config.BatchSize),
			CreatedAt:  time.Now(),
		},
		LastFlush: time.Now(),
	}
}

func (bm *BatchManager) Start() error {
	bm.Mutex.Lock()
	defer bm.Mutex.Unlock()

	if bm.Running {
		return nil
	}

	bm.Running = true

	// Iniciar QueueManager
	if err := bm.QueueMgr.Start(); err != nil {
		bm.Running = false
		return err
	}

	// Iniciar workers para procesar batches
	for i := 0; i < bm.Config.MaxWorkers; i++ {
		bm.WaitGroup.Add(1)
		go bm.batchWorker(i)
	}

	// Iniciar worker para agrupar peticiones en batches
	bm.WaitGroup.Add(1)
	go bm.batchAggregator()

	// Iniciar flush automático si está habilitado
	if bm.Config.FlushInterval > 0 {
		bm.FlushTicker = time.NewTicker(bm.Config.FlushInterval)
		bm.WaitGroup.Add(1)
		go bm.autoFlush()
	}

	log.Printf("BatchManager started with %d workers, batch size: %d",
		bm.Config.MaxWorkers, bm.Config.BatchSize)
	return nil
}

// Stop detiene el batch manager
func (bm *BatchManager) Stop() {
	bm.Mutex.Lock()
	defer bm.Mutex.Unlock()

	if !bm.Running {
		return
	}

	bm.QueueMgr.Stop()

	if bm.FlushTicker != nil {
		bm.FlushTicker.Stop()
	}

	// Flush del batch actual si tiene datos
	bm.flushCurrentBatch()

	bm.WaitGroup.Wait()
	bm.Running = false

	log.Println("BatchManager stopped")
}

// AddOperation agrega una operación al batch
func (bm *BatchManager) AddOperation(operation *Mockdata) error {
	bm.Mutex.RLock()
	if !bm.Running {
		bm.Mutex.RUnlock()
		return bm.insertSync(operation) // Fallback a inserción directa
	}
	bm.Mutex.RUnlock()

	if err := bm.QueueMgr.AddRequest(operation); err != nil {
		if err == ErrQueueFull {
			// Si la cola está llena, insertar directamente
			return bm.insertSync(operation)
		}
		return err
	}
	return nil
}

// batchAggregator agrupa peticiones en batches
func (bm *BatchManager) batchAggregator() {
	defer bm.WaitGroup.Done()

	for {
		select {
		case <-bm.QueueMgr.Ctx.Done():
			bm.flushCurrentBatch()
			return
		case operation, ok := <-bm.QueueMgr.InputQueue:
			if !ok {
				bm.flushCurrentBatch()
				return
			}

			bm.BatchMutex.Lock()
			bm.CurrentBatch.Operations = append(bm.CurrentBatch.Operations, operation)
			bm.CurrentBatch.Size++

			// Si el batch está completo, enviarlo
			if bm.CurrentBatch.Size >= bm.Config.BatchSize {
				bm.sendBatch()
			}
			bm.BatchMutex.Unlock()
		}
	}
}

// batchWorker procesa batches completos
func (bm *BatchManager) batchWorker(id int) {
	defer bm.WaitGroup.Done()

	log.Printf("Batch worker %d started", id)

	for {
		select {
		case <-bm.QueueMgr.Ctx.Done():
			log.Printf("Batch worker %d stopping", id)
			return
		case batch, ok := <-bm.QueueMgr.BatchQueue:
			if !ok {
				log.Printf("Batch worker %d: batch queue closed", id)
				return
			}

			// Procesar el batch
			err := bm.processBatch(batch)
			if err != nil {
				log.Printf("Batch worker %d: error processing batch %s: %v", id, batch.ID, err)
				atomic.AddInt64(&bm.TotalErrors, 1)
			} else {
				atomic.AddInt64(&bm.TotalProcessed, int64(batch.Size))
			}

			// Enviar resultado
			if sendErr := bm.QueueMgr.SendResult(err); sendErr != nil {
				log.Printf("Batch worker %d: error sending result: %v", id, sendErr)
			}
		}
	}
}

// processBatch procesa un batch completo con transacción
func (bm *BatchManager) processBatch(batch *Batch) error {
	var lastErr error

	for attempt := 1; attempt <= bm.Config.RetryAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(bm.QueueMgr.Ctx, bm.Config.Timeout)

		err := bm.insertBatchWithContext(ctx, batch)
		cancel()

		if err == nil {
			atomic.AddInt64(&bm.TotalBatches, 1)
			return nil
		}

		lastErr = err
		if attempt < bm.Config.RetryAttempts {
			log.Printf("Retry attempt %d for batch %s: %v", attempt, batch.ID, err)
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return lastErr
}

// insertBatchWithContext inserta un batch completo en una transacción
func (bm *BatchManager) insertBatchWithContext(ctx context.Context, batch *Batch) error {
	done := make(chan error, 1)

	go func() {
		done <- bm.insertBatchTransaction(batch)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// insertBatchTransaction ejecuta la inserción del batch en una transacción
func (bm *BatchManager) insertBatchTransaction(batch *Batch) error {
	tx, err := bm.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Preparar statement para inserción masiva
	stmt, err := tx.Prepare(`
		INSERT INTO mock_transactions (
			uuid, recepcion_id, sender_id, request_headers, request_method, 
			request_endpoint, request_body, response_headers, response_body, 
			response_status_code, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Insertar todas las operaciones del batch
	for _, operation := range batch.Operations {
		_, err := stmt.Exec(
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
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// insertSync inserción síncrona directa (fallback)
func (bm *BatchManager) insertSync(operation *Mockdata) error {
	return InsertOperation(bm.DB, operation)
}

// sendBatch envía el batch actual a la cola de procesamiento
func (bm *BatchManager) sendBatch() {
	if bm.CurrentBatch.Size > 0 {
		if err := bm.QueueMgr.AddBatch(bm.CurrentBatch); err != nil {
			if err == ErrQueueFull {
				// Si la cola de batches está llena, procesar directamente
				if processErr := bm.processBatch(bm.CurrentBatch); processErr != nil {
					log.Printf("Error processing batch directly: %v", processErr)
				}
			} else {
				log.Printf("Error adding batch to queue: %v", err)
				return
			}
		}

		// Crear nuevo batch
		bm.CurrentBatch = &Batch{
			ID:         generateBatchID(),
			Operations: make([]*Mockdata, 0, bm.Config.BatchSize),
			CreatedAt:  time.Now(),
		}
	}
}

// flushCurrentBatch envía el batch actual aunque no esté completo
func (bm *BatchManager) flushCurrentBatch() {
	bm.BatchMutex.Lock()
	defer bm.BatchMutex.Unlock()

	if bm.CurrentBatch.Size > 0 {
		bm.sendBatch()
	}
}

// autoFlush ejecuta flush automático por tiempo
func (bm *BatchManager) autoFlush() {
	defer bm.WaitGroup.Done()

	for {
		select {
		case <-bm.QueueMgr.Ctx.Done():
			return
		case <-bm.FlushTicker.C:
			bm.flushCurrentBatch()
		}
	}
}

// GetStats retorna estadísticas del batch manager
func (bm *BatchManager) GetStats() map[string]interface{} {
	bm.Mutex.RLock()
	defer bm.Mutex.RUnlock()

	bm.BatchMutex.Lock()
	currentBatchSize := bm.CurrentBatch.Size
	bm.BatchMutex.Unlock()

	return map[string]interface{}{
		"is_running":         bm.Running,
		"input_queue_size":   len(bm.QueueMgr.InputQueue),
		"batch_queue_size":   len(bm.QueueMgr.BatchQueue),
		"current_batch_size": currentBatchSize,
		"total_processed":    atomic.LoadInt64(&bm.TotalProcessed),
		"total_batches":      atomic.LoadInt64(&bm.TotalBatches),
		"total_errors":       atomic.LoadInt64(&bm.TotalErrors),
		"batch_size":         bm.Config.BatchSize,
		"max_workers":        bm.Config.MaxWorkers,
		"flush_interval":     bm.Config.FlushInterval,
	}
}

// IsRunning retorna si el batch manager está ejecutándose
func (bm *BatchManager) IsRunning() bool {
	bm.Mutex.RLock()
	defer bm.Mutex.RUnlock()
	return bm.Running
}

// generateBatchID genera un ID único para el batch
func generateBatchID() string {
	return fmt.Sprintf("batch_%d", time.Now().UnixNano())
}
