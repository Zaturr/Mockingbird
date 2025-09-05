package externa

import (
	"Mockingbird/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Handler para manejar cualquier método HTTP y devolver respuestas mock
func MockHandler(w http.ResponseWriter, r *http.Request) {
	// Leer el cuerpo de la petición que contiene la estructura models.Http
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al leer el cuerpo de la petición: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Mapear los datos a la estructura Http según modelo.go
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

	// Buscar la location que coincida con el método y path de la petición
	location := findMatchingLocation(r, config)
	if location == nil {
		http.Error(w, fmt.Sprintf("No se encontró configuración para %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		return
	}

	// Aplicar headers configurados
	if location.Headers != nil {
		for key, value := range *location.Headers {
			w.Header().Set(key, value)
		}
	}

	statusCode := 200
	if location.StatusCode != "" {
		if code, err := fmt.Sscanf(location.StatusCode, "%d", &statusCode); err != nil || code != 1 {
			statusCode = 200 // fallback
		}
	}

	w.WriteHeader(statusCode)
	if location.Response != nil {
		w.Write(*location.Response)
	}

	if location.Async != nil {
		go handleAsyncCall(location.Async)
	}
}

// Función para encontrar la location que coincida con la petición
func findMatchingLocation(r *http.Request, config models.Http) *models.Location {
	requestPath := r.URL.Path
	requestMethod := r.Method

	fmt.Printf("Buscando configuración para: %s %s\n", requestMethod, requestPath)

	for i, server := range config.Servers {
		fmt.Printf("Servidor %d: path='%s', listen=%d\n", i, server.Path, server.Listen)

		// Verificar si el path de la petición coincide con el path del servidor
		// o si el path del servidor es un prefijo de la petición
		if strings.HasPrefix(requestPath, server.Path) || server.Path == "" {
			fmt.Printf("Path coincide para servidor %d\n", i)
			for j, location := range server.Location {
				fmt.Printf("  Location %d: method='%s', statusCode='%s'\n", j, location.Method, location.StatusCode)
				// Verificar si el método coincide
				if strings.EqualFold(location.Method, requestMethod) {
					fmt.Printf("  ¡Encontrada location coincidente!\n")
					return &location
				}
			}
		}
	}

	fmt.Printf("No se encontró configuración coincidente\n")
	return nil
}

// Función para manejar llamadas asíncronas
func handleAsyncCall(async *models.Async) {

	fmt.Printf("Llamada asíncrona programada para: %s %s\n", async.Method, async.Url)

}

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
