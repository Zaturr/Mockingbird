package main

import (
	"Mockingbird/cmd"
	"log"
)

func main() {
	// Crear y configurar el servidor multi-puerto
	srv := cmd.Multiport()
	if err := srv.StartAll(); err != nil {
		log.Fatal("Error al iniciar los servicios:", err)
	}
}
