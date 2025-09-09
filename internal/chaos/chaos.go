package chaos

import (
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"mockingbird/internal/models"
)

// Engine manages chaos injection in HTTP responses
type Engine struct {
	rand *rand.Rand
}

// NewEngine creates a new instance of the chaos engine
func NewEngine() *Engine {
	return &Engine{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ApplyChaos applies chaos injection based on the configuration
func (e *Engine) ApplyChaos(w http.ResponseWriter, chaosConfig *models.ChaosInjection) bool {
	if chaosConfig == nil {
		return false
	}

	// Apply latency if configured
	latency := e.applyLatency(chaosConfig.Latency)
	if latency > 0 {
		time.Sleep(latency)
	}

	// Apply abort if configured
	abortCode := e.applyAbort(chaosConfig.Abort)
	if abortCode > 0 {
		w.WriteHeader(abortCode)
		return true
	}

	// Apply error if configured
	errorCode := e.applyError(chaosConfig.Error)
	if errorCode > 0 {
		w.WriteHeader(errorCode)
		return true
	}

	return false
}

// applyLatency returns a duration to delay the response based on the latency configuration
func (e *Engine) applyLatency(latency models.Latency) time.Duration {
	if latency.Time <= 0 {
		return 0
	}

	probability, err := strconv.ParseFloat(latency.Probability, 64)
	if err != nil || probability <= 0 {
		return 0
	}

	if e.rand.Float64()*100 > probability {
		return 0
	}

	return time.Duration(latency.Time) * time.Millisecond
}

// applyAbort returns an HTTP status code to abort the request based on the abort configuration
func (e *Engine) applyAbort(abort models.Abort) int {
	if abort.Code <= 0 {
		return 0
	}

	probability, err := strconv.ParseFloat(abort.Probability, 64)
	if err != nil || probability <= 0 {
		return 0
	}

	if e.rand.Float64()*100 > probability {
		return 0
	}

	return abort.Code
}

// applyError returns an HTTP error status code based on the error configuration
func (e *Engine) applyError(errorConfig models.Error) int {
	if errorConfig.Code <= 0 {
		return 0
	}

	probability, err := strconv.ParseFloat(errorConfig.Probability, 64)
	if err != nil || probability <= 0 {
		return 0
	}

	if e.rand.Float64()*100 > probability {
		return 0
	}

	return errorConfig.Code
}
