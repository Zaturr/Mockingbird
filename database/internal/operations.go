package internal

import (
	"database/sql"
	"time"
)

// Mockdata representa los datos de una transacción mock
type Mockdata struct {
	UUID               string    `json:"uuid" db:"uuid"`
	RecepcionID        string    `json:"recepcion_id" db:"recepcion_id"`
	SenderID           string    `json:"sender_id" db:"sender_id"`
	RequestHeaders     string    `json:"request_headers" db:"request_headers"`
	RequestMethod      string    `json:"request_method" db:"request_method"`
	RequestEndpoint    string    `json:"request_endpoint" db:"request_endpoint"`
	RequestBody        string    `json:"request_body" db:"request_body"`
	ResponseHeaders    string    `json:"response_headers" db:"response_headers"`
	ResponseBody       string    `json:"response_body" db:"response_body"`
	ResponseStatusCode int       `json:"response_status_code" db:"response_status_code"`
	Timestamp          time.Time `json:"timestamp" db:"timestamp"`
}

// InsertOperation inserta una nueva operación en la base de datos
func InsertOperation(db *sql.DB, operation *Mockdata) error {
	query := `
	INSERT INTO mock_transactions (
		uuid, recepcion_id, sender_id, request_headers, request_method, 
		request_endpoint, request_body, response_headers, response_body, 
		response_status_code, timestamp
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := db.Exec(query,
		operation.UUID,
		operation.RecepcionID,
		operation.SenderID,
		operation.RequestHeaders,
		operation.RequestMethod,
		operation.RequestEndpoint,
		operation.RequestBody,
		operation.ResponseHeaders,
		operation.ResponseBody,
		operation.ResponseStatusCode,
		operation.Timestamp,
	)

	return err
}

// UpdateOperationResponse actualiza solo la respuesta de una operación
func UpdateOperationResponse(db *sql.DB, uuid string, responseHeaders, responseBody string, statusCode int) error {
	query := `UPDATE mock_transactions SET 
		response_headers = ?, response_body = ?, response_status_code = ?
		WHERE uuid = ?`

	_, err := db.Exec(query, responseHeaders, responseBody, statusCode, uuid)
	return err
}
