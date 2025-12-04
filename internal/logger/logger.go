package logger

import (
	"catalyst/internal/config"
	"catalyst/internal/models"

	"github.com/SOLUCIONESSYCOM/scribe"
	"github.com/google/uuid"
)

func GetLoggerContext(server models.LogDescriptor) (*scribe.Scribe, error) {

	logSettings := config.GetLogSettings()

	loggerConfig := &scribe.ConfigLogger{
		FilePath:          server.Path,                    // FilePath donde se guardarán los logs
		MinLevel:          logSettings.MinLevel,           // Nivel mínimo de log (trace, debug, info, warn, error, fatal)
		RotationMaxSizeMB: logSettings.RotationMaxSizeMB,  // Tamaño máximo del archivo antes de rotar
		MaxBackups:        logSettings.MaxBackups,         // Número máximo de archivos de respaldo
		MaxAgeDay:         logSettings.MaxAgeDay,          // Días máximos para conservar los logs
		Compress:          logSettings.Compress,           // Comprimir logs rotados
		Console:           server.Logger,                  // Mostrar logs en consola
		BeutifyConsoleLog: logSettings.BeautifyConsoleLog, // Formato bonito en consola (false = JSON)
		File:              server.File,                    // Escribir logs en archivo
	}

	globals := map[string]interface{}{
		"service_name":    server.Name,
		"service_version": server.Version,
		"service_id":      uuid.New().String(),
	}

	globalContext := scribe.NewGlobalLogContext(globals, []string{"service_name", "service_version", "service_id"})

	return scribe.New(loggerConfig, globalContext, []string{"service_name", "service_version", "service_id, timestamp"})
}
