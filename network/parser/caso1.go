package externa

import (
	"Mockingbird/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Handler para mapear datos entrantes y verificar estructura
func MockHandler(w http.ResponseWriter, r *http.Request) {
	// Leer el cuerpo de la petición
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al leer el cuerpo de la petición: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Mapear los datos a la estructura Http
	var config models.Http
	if err := json.Unmarshal(body, &config); err != nil {
		http.Error(w, fmt.Sprintf("Error al parsear los datos: %v", err), http.StatusBadRequest)
		return
	}

	// Verificar la estructura completa
	if err := validateConfig(config); err != nil {
		http.Error(w, fmt.Sprintf("Error en la validación: %v", err), http.StatusBadRequest)
		return
	}

	// Respuesta mock exitosa
	response := map[string]interface{}{
		"status":  "success",
		"message": "Configuración validada correctamente",
		"data":    config,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Error al codificar la respuesta: %v", err), http.StatusInternalServerError)
		return
	}
}

// Función para validar la estructura completa
func validateConfig(config models.Http) error {
	if len(config.Servers) == 0 {
		return fmt.Errorf("no se encontraron servidores en la configuración")
	}

	for i, server := range config.Servers {
		if server.Path == "" {
			return fmt.Errorf("servidor %d: path es requerido", i)
		}
		if server.Listen == 0 {
			return fmt.Errorf("servidor %d: listen es requerido", i)
		}
		if len(server.Location) == 0 {
			return fmt.Errorf("servidor %d: al menos una location es requerida", i)
		}

		for j, location := range server.Location {
			if location.Method == "" {
				return fmt.Errorf("servidor %d, location %d: method es requerido", i, j)
			}
			if location.StatusCode == "" {
				return fmt.Errorf("servidor %d, location %d: statusCode es requerido", i, j)
			}
		}
	}

	return nil
}
