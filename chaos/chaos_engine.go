package chaos

import (
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ChaosEngine maneja la inyección de caos en las respuestas HTTP
type ChaosEngine struct {
	rand *rand.Rand
}

// NewChaosEngine crea una nueva instancia del motor de caos
func NewChaosEngine() *ChaosEngine {
	return &ChaosEngine{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ApplyChaos aplica la inyección de caos basada en la configuración
func (ce *ChaosEngine) ApplyChaos(w http.ResponseWriter, chaosConfig string) {
	if chaosConfig == "" {
		return
	}

	if latency := ce.parseLatency(chaosConfig); latency > 0 {
		time.Sleep(latency)
	}

	if statusCode := ce.parseAbort(chaosConfig); statusCode > 0 {
		w.WriteHeader(statusCode)
		return
	}

	if statusCode := ce.parseError(chaosConfig); statusCode > 0 {
		w.WriteHeader(statusCode)
		return
	}
}

func (ce *ChaosEngine) parseLatency(config string) time.Duration {
	if !strings.Contains(config, "ms") {
		return 0
	}

	parts := strings.Fields(config)
	if len(parts) < 2 {
		return 0
	}

	durationStr := parts[0]
	probabilityStr := parts[1]

	probabilityStr = strings.TrimSuffix(probabilityStr, "%")
	probability, err := strconv.ParseFloat(probabilityStr, 64)
	if err != nil {
		return 0
	}

	if ce.rand.Float64()*100 > probability {
		return 0
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return 0
	}

	return duration
}

func (ce *ChaosEngine) parseAbort(config string) int {
	if !strings.Contains(config, "%") {
		return 0
	}

	parts := strings.Fields(config)
	if len(parts) < 2 {
		return 0
	}

	statusCodeStr := parts[0]
	probabilityStr := parts[1]

	probabilityStr = strings.TrimSuffix(probabilityStr, "%")
	probability, err := strconv.ParseFloat(probabilityStr, 64)
	if err != nil {
		return 0
	}

	if ce.rand.Float64()*100 > probability {
		return 0
	}

	statusCode, err := strconv.Atoi(statusCodeStr)
	if err != nil {
		return 0
	}

	return statusCode
}

func (ce *ChaosEngine) parseError(config string) int {
	if !strings.Contains(config, "%") {
		return 0
	}

	parts := strings.Fields(config)
	if len(parts) < 2 {
		return 0
	}

	statusCodeStr := parts[0]
	probabilityStr := parts[1]

	probabilityStr = strings.TrimSuffix(probabilityStr, "%")
	probability, err := strconv.ParseFloat(probabilityStr, 64)
	if err != nil {
		return 0
	}

	if ce.rand.Float64()*100 > probability {
		return 0
	}

	statusCode, err := strconv.Atoi(statusCodeStr)
	if err != nil {
		return 0
	}

	return statusCode
}

func (ce *ChaosEngine) ApplyLatency(config string) time.Duration {
	return ce.parseLatency(config)
}

func (ce *ChaosEngine) ApplyAbort(config string) int {
	return ce.parseAbort(config)
}

func (ce *ChaosEngine) ApplyError(config string) int {
	return ce.parseError(config)
}

func (ce *ChaosEngine) ParseChaosConfig(latencyConfig, abortConfig, errorConfig string) (time.Duration, int, int) {
	latency := ce.parseLatency(latencyConfig)
	abort := ce.parseAbort(abortConfig)
	error := ce.parseError(errorConfig)

	return latency, abort, error
}
