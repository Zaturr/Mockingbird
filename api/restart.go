package api

import (
	"context"
	"log"
	"sync"
	"time"
)

// RestartManager manages server restart operations with improved error handling and context support
type RestartManager struct {
	restartChan chan string
	restartFunc func(string) error
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	running     bool
	timeout     time.Duration
	retryCount  int
	retryDelay  time.Duration
}

// RestartOptions configures the RestartManager behavior
type RestartOptions struct {
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
}

// DefaultRestartOptions returns default options for RestartManager
func DefaultRestartOptions() *RestartOptions {
	return &RestartOptions{
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
	}
}

// NewRestartManager creates a new RestartManager instance
func NewRestartManager(restartChan chan string, restartFunc func(string) error, opts ...*RestartOptions) *RestartManager {
	ctx, cancel := context.WithCancel(context.Background())

	options := DefaultRestartOptions()
	if len(opts) > 0 && opts[0] != nil {
		options = opts[0]
	}

	return &RestartManager{
		restartChan: restartChan,
		restartFunc: restartFunc,
		ctx:         ctx,
		cancel:      cancel,
		timeout:     options.Timeout,
		retryCount:  options.RetryCount,
		retryDelay:  options.RetryDelay,
		running:     false,
	}
}

// Start begins the restart manager
func (rm *RestartManager) Start() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.running {
		return ErrManagerAlreadyRunning
	}

	rm.running = true
	rm.wg.Add(1)

	go rm.run()
	return nil
}

// Stop gracefully stops the restart manager
func (rm *RestartManager) Stop() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.running {
		return nil
	}

	rm.cancel()
	rm.wg.Wait()
	rm.running = false

	return nil
}

// IsRunning returns whether the manager is currently running
func (rm *RestartManager) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// run is the main loop for processing restart signals
func (rm *RestartManager) run() {
	defer rm.wg.Done()

	log.Printf("RestartManager: Started")

	for {
		select {
		case serverName, ok := <-rm.restartChan:
			if !ok {
				log.Printf("RestartManager: restart channel closed")
				return
			}

			log.Printf("RestartManager: Restart signal received for server: %s", serverName)

			// Process restart with retry logic
			rm.processRestart(serverName)

		case <-rm.ctx.Done():
			log.Printf("RestartManager: Context cancelled, stopping")
			return
		}
	}
}

// processRestart handles a single restart request with retry logic
func (rm *RestartManager) processRestart(serverName string) {
	ctx, cancel := context.WithTimeout(rm.ctx, rm.timeout)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= rm.retryCount; attempt++ {
		select {
		case <-ctx.Done():
			log.Printf("RestartManager: Timeout waiting for restart of server: %s", serverName)
			return
		default:
		}

		// Add small delay before restart (as in original code)
		time.Sleep(100 * time.Millisecond)

		// Execute restart function
		if err := rm.restartFunc(serverName); err != nil {
			lastErr = err
			log.Printf("RestartManager: Restart attempt %d failed for server %s: %v", attempt, serverName, err)

			if attempt < rm.retryCount {
				log.Printf("RestartManager: Retrying restart for server %s in %v", serverName, rm.retryDelay)
				time.Sleep(rm.retryDelay)
			}
		} else {
			log.Printf("RestartManager: Successfully restarted server: %s", serverName)
			return
		}
	}

	log.Printf("RestartManager: All restart attempts failed for server %s: %v", serverName, lastErr)
}

// UpdateOptions updates the restart manager options
func (rm *RestartManager) UpdateOptions(opts *RestartOptions) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if opts.Timeout > 0 {
		rm.timeout = opts.Timeout
	}
	if opts.RetryCount > 0 {
		rm.retryCount = opts.RetryCount
	}
	if opts.RetryDelay > 0 {
		rm.retryDelay = opts.RetryDelay
	}
}

// GetStats returns current manager statistics
func (rm *RestartManager) GetStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"running":     rm.running,
		"timeout":     rm.timeout.String(),
		"retry_count": rm.retryCount,
		"retry_delay": rm.retryDelay.String(),
	}
}
