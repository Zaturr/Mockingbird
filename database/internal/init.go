package internal

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DBConfig configuración de conexiones de base de datos
type DBConfig struct {
	MaxOpenConns    int           // Máximo de conexiones abiertas
	MinConn         int           // Mínimo de conexiones inactivas
	ConnMaxLifetime time.Duration // Tiempo máximo de vida de conexión
	ConnMaxIdleTime time.Duration // Tiempo máximo inactivo
}

func InitDB(dbPath string) (*sql.DB, error) {
	return InitDBWithConfig(dbPath, DBConfig{
		MaxOpenConns:    1,
		MinConn:         1,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,
	})
}

func InitDBWithConfig(dbPath string, config DBConfig) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Habilitar WAL mode para mejor concurrencia
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("error setting WAL mode: %v", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, fmt.Errorf("error setting synchronous mode: %v", err)
	}

	// Configurar límites de conexiones para SQLite
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MinConn)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	// Crear tabla unificada para requests y responses
	createUnifiedTable := `
	CREATE TABLE IF NOT EXISTS mock_transactions (
		uuid TEXT PRIMARY KEY,
		recepcion_id TEXT,
		sender_id TEXT,
		request_headers TEXT,
		request_method TEXT NOT NULL,
		request_endpoint TEXT NOT NULL,
		request_body TEXT,
		response_headers TEXT,
		response_body TEXT,
		response_status_code INTEGER,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	createIndexes := `
	CREATE INDEX IF NOT EXISTS idx_transactions_recepcion_id ON mock_transactions(recepcion_id);
	CREATE INDEX IF NOT EXISTS idx_transactions_sender_id ON mock_transactions(sender_id);
	CREATE INDEX IF NOT EXISTS idx_transactions_method ON mock_transactions(request_method);
	CREATE INDEX IF NOT EXISTS idx_transactions_endpoint ON mock_transactions(request_endpoint);
	CREATE INDEX IF NOT EXISTS idx_transactions_method_endpoint ON mock_transactions(request_method, request_endpoint);
	`

	if _, err := db.Exec(createUnifiedTable); err != nil {
		return nil, fmt.Errorf("error creating unified table: %v", err)
	}

	if _, err := db.Exec(createIndexes); err != nil {
		return nil, fmt.Errorf("error creating indexes: %v", err)
	}

	log.Println("Database initialized successfully")
	return db, nil
}
