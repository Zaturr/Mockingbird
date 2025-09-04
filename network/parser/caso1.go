package parser

import (
	"Mockingbird/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Handler para obtener la configuración de la API
func GetConfigHandler(w http.ResponseWriter, r *http.Request) {

	resp, err := http.Get("http://localhost:8084/api/config")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al conectar con la API: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al leer la respuesta: %v", err), http.StatusInternalServerError)
		return
	}

	var config models.Http
	if err := json.Unmarshal(body, &config); err != nil {
		http.Error(w, fmt.Sprintf("Error al parsear la respuesta: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(config); err != nil {
		http.Error(w, fmt.Sprintf("Error al codificar la respuesta: %v", err), http.StatusInternalServerError)
		return
	}
}
