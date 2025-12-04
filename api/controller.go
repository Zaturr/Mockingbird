package api

import (
	"context"
	"log"
	"sync"
	"time"
)

// ServerController manages server restart operations with thread-safe operations
type ServerController struct {
	restartChan chan string
	mu          *sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
}

// NewServerController creates a new ServerController instance
func NewServerController() *ServerController {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServerController{
		restartChan: make(chan string, 10),
		mu:          &sync.RWMutex{},
		ctx:         ctx,
		cancel:      cancel,
		closed:      false,
	}
}

// GetRestartChan returns the restart channel for external use
func (sc *ServerController) GetRestartChan() chan string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.restartChan
}

// NotifyRestart sends a restart signal for the specified server
func (sc *ServerController) NotifyRestart(serverName string) error {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if sc.closed {
		return ErrControllerClosed
	}

	select {
	case sc.restartChan <- serverName:
		log.Printf("Restart signal sent for server: %s", serverName)
		return nil
	case <-sc.ctx.Done():
		return sc.ctx.Err()
	default:
		log.Printf("Restart channel full, dropping signal for server: %s", serverName)
		return ErrChannelFull
	}
}

// Close gracefully shuts down the controller
func (sc *ServerController) Close() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.closed {
		return nil
	}

	sc.closed = true
	sc.cancel()
	close(sc.restartChan)
	return nil
}

// IsClosed returns whether the controller is closed
func (sc *ServerController) IsClosed() bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.closed
}

// ConfigWatcher watches for configuration changes and triggers restarts
type ConfigWatcher struct {
	controller *ServerController
	timeout    time.Duration
}

// NewConfigWatcher creates a new ConfigWatcher instance
func NewConfigWatcher(controller *ServerController) *ConfigWatcher {
	return &ConfigWatcher{
		controller: controller,
		timeout:    30 * time.Second,
	}
}

// WatchForChanges starts watching for configuration changes
func (cw *ConfigWatcher) WatchForChanges(restartHandler func(string)) {
	go func() {
		for {
			select {
			case serverName, ok := <-cw.controller.restartChan:
				if !ok {
					log.Printf("ConfigWatcher: restart channel closed")
					return
				}
				log.Printf("Configuration change detected for server: %s", serverName)

				// Execute restart handler with timeout
				done := make(chan struct{})
				go func() {
					restartHandler(serverName)
					close(done)
				}()

				select {
				case <-done:
					log.Printf("Restart handler completed for server: %s", serverName)
				case <-time.After(cw.timeout):
					log.Printf("Restart handler timeout for server: %s", serverName)
				}

			case <-cw.controller.ctx.Done():
				log.Printf("ConfigWatcher: context cancelled")
				return
			}
		}
	}()
}
