package main

import (
	"Mockingbird/cmd/server/server"
	"log"
)

func main() {
	// Crear y configurar el servidor multi-puerto
	srv := server.NewMultiPortServer()

	// Iniciar todos los servicios en sus respectivos puertos
	if err := srv.StartAll(); err != nil {
		log.Fatal("Error al iniciar los servicios:", err)
	}
}
