package main

import (
	"flag"
	"log"
	"mockingbird/cmd"
)

func main() {
	// Parse command line flags
	configDir := flag.String("config", "", "Directory containing YAML configuration files")
	configFile := flag.String("file", "", "Path to a specific YAML configuration file")
	flag.Parse()

	// Crear y configurar el servidor multi-puerto
	srv := cmd.Multiport()

	// Pasar las flags al servidor
	if *configDir != "" {
		srv.SetConfigDir(*configDir)
	}
	if *configFile != "" {
		srv.SetConfigFile(*configFile)
	}

	if err := srv.StartAll(); err != nil {
		log.Fatal("Error al iniciar los servicios:", err)
	}
}
