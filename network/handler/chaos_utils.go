package handler

import (
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// ChaosEngine maneja la lógica de chaos engineering
type ChaosEngine struct {
	random *rand.Rand
}

// NewChaosEngine crea una nueva instancia del motor de chaos
func NewChaosEngine() *ChaosEngine {
	source := rand.NewSource(time.Now().UnixNano())
	return &ChaosEngine{
		random: rand.New(source),
	}
}

// ParseLatencyString parsea strings de latencia como "100ms 30%"
// Retorna la duración de latencia si debe aplicarse, o 0 si no
func (ce *ChaosEngine) ParseLatencyString(latencyStr string) time.Duration {
	if latencyStr == "" {
		return 0
	}

	parts := strings.Fields(latencyStr)
	if len(parts) < 2 {
		return 0
	}

	// Parsear duración (ej: "100ms")
	duration, err := time.ParseDuration(parts[0])
	if err != nil {
		return 0
	}

	// Parsear porcentaje (ej: "30%")
	percentageStr := strings.TrimSuffix(parts[1], "%")
	percentage, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return 0
	}

	// Aplicar probabilidad
	if ce.shouldApply(percentage) {
		return duration
	}

	return 0
}

// ParseAbortString parsea strings de abort como "503 10%"
// Retorna true si debe abortar la request, false si no
func (ce *ChaosEngine) ParseAbortString(abortStr string) bool {
	if abortStr == "" {
		return false
	}

	parts := strings.Fields(abortStr)
	if len(parts) < 2 {
		return false
	}

	// Parsear porcentaje (ej: "10%")
	percentageStr := strings.TrimSuffix(parts[1], "%")
	percentage, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return false
	}

	return ce.shouldApply(percentage)
}

// ParseErrorString parsea strings de error como "500 5%"
// Retorna true si debe retornar error, false si no
func (ce *ChaosEngine) ParseErrorString(errorStr string) bool {
	if errorStr == "" {
		return false
	}

	parts := strings.Fields(errorStr)
	if len(parts) < 2 {
		return false
	}

	// Parsear porcentaje (ej: "5%")
	percentageStr := strings.TrimSuffix(parts[1], "%")
	percentage, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return false
	}

	return ce.shouldApply(percentage)
}

// ParseStatusCodeString parsea strings de status code como "200 80% 500 20%"
// Retorna el código de estado apropiado basado en probabilidades
func (ce *ChaosEngine) ParseStatusCodeString(statusCodeStr string) int {
	if statusCodeStr == "" {
		return 200
	}

	// Si es solo un número, retornarlo directamente
	if code, err := strconv.Atoi(statusCodeStr); err == nil {
		return code
	}

	parts := strings.Fields(statusCodeStr)
	if len(parts) < 2 {
		return 200
	}

	// Parsear pares de código y porcentaje
	var codes []int
	var percentages []float64

	for i := 0; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}

		code, err := strconv.Atoi(parts[i])
		if err != nil {
			continue
		}

		percentageStr := strings.TrimSuffix(parts[i+1], "%")
		percentage, err := strconv.ParseFloat(percentageStr, 64)
		if err != nil {
			continue
		}

		codes = append(codes, code)
		percentages = append(percentages, percentage)
	}

	if len(codes) == 0 {
		return 200
	}

	// Seleccionar código basado en probabilidades
	return ce.selectByProbability(codes, percentages)
}

// shouldApply determina si debe aplicarse una acción basada en porcentaje
func (ce *ChaosEngine) shouldApply(percentage float64) bool {
	if percentage <= 0 {
		return false
	}
	if percentage >= 100 {
		return true
	}

	// Generar número aleatorio entre 0 y 100
	randomValue := ce.random.Float64() * 100
	return randomValue <= percentage
}

// selectByProbability selecciona un elemento basado en probabilidades
func (ce *ChaosEngine) selectByProbability(items []int, probabilities []float64) int {
	if len(items) == 0 || len(probabilities) == 0 {
		return 200
	}

	// Normalizar probabilidades
	total := 0.0
	for _, prob := range probabilities {
		total += prob
	}

	if total == 0 {
		return items[0]
	}

	// Generar número aleatorio
	randomValue := ce.random.Float64() * total

	// Seleccionar elemento basado en probabilidades acumulativas
	currentSum := 0.0
	for i, prob := range probabilities {
		currentSum += prob
		if randomValue <= currentSum {
			return items[i]
		}
	}

	// Fallback al último elemento
	return items[len(items)-1]
}

// ApplyLatency aplica latencia si es necesario
func (ce *ChaosEngine) ApplyLatency(latencyStr string) {
	if latency := ce.ParseLatencyString(latencyStr); latency > 0 {
		time.Sleep(latency)
	}
}

// ShouldAbort determina si debe abortar la request
func (ce *ChaosEngine) ShouldAbort(abortStr string) bool {
	return ce.ParseAbortString(abortStr)
}

// ShouldReturnError determina si debe retornar error
func (ce *ChaosEngine) ShouldReturnError(errorStr string) bool {
	return ce.ParseErrorString(errorStr)
}

// GetStatusCode determina el código de estado apropiado
func (ce *ChaosEngine) GetStatusCode(statusCodeStr string) int {
	return ce.ParseStatusCodeString(statusCodeStr)
}
