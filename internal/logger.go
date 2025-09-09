package logger

import (
	"mockingbird/internal/config"

	"github.com/SOLUCIONESSYCOM/scribe"
	"github.com/google/uuid"
)

// InitLogger initializes the logger with default configuration
func InitLogger() {
	logSettings := config.GetLogSettings()

	loggerConfig := &scribe.ConfigLogger{
		FilePath:          logSettings.Path,              // FilePath donde se guardarán los logs
		MinLevel:          logSettings.MinLevel,          // Nivel mínimo de log (trace, debug, info, warn, error, fatal)
		RotationMaxSizeMB: logSettings.RotationMaxSizeMB, // Tamaño máximo del archivo antes de rotar
		MaxBackups:        logSettings.MaxBackups,        // Número máximo de archivos de respaldo
		MaxAgeDay:         logSettings.MaxAgeDay,         // Días máximos para conservar los logs
		Compress:          logSettings.Compress,          // Comprimir logs rotados
		Console:           logSettings.Console,           // Mostrar logs en consola
		BeutifyConsoleLog: logSettings.BeutifyConsoleLog, // Formato bonito en consola (false = JSON)
		File:              logSettings.File,              // Escribir logs en archivo
	}

	// Campos globales para TODOS los loggers de la aplicación
	// Estos están disponibles a través del contexto global por defecto
	appGlobalFields := map[string]interface{}{
		"service_name":    "mockingbird",
		"service_version": "0.1.0",
		"service_id":      uuid.New().String(),
	}

	scribe.SetGlobalFields(appGlobalFields)

	logger, err := scribe.New(loggerConfig, nil, []string{"request_id", "trace_id", "time"})
	if err != nil {
		panic(err)
	}

	scribe.SetDefaultLogger(logger)
}
