package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"catalyst/database"

	"catalyst/internal/chaos"
	"catalyst/internal/invalid"
	"catalyst/internal/models"
	prom "catalyst/prometheus"

	"github.com/SOLUCIONESSYCOM/scribe"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// randomCache almacena los valores aleatorios generados por transacción
var randomCache = make(map[string]map[string]interface{})
var randomCacheMutex sync.RWMutex

// Handler manages HTTP request handling based on configuration
type Handler struct {
	chaosEngine  *chaos.Engine
	schemas      map[string]*jsonschema.Schema
	Logger       *scribe.Scribe
	BatchManager *database.BatchManager
}

// NewHandler creates a new handler with the given chaos engine
func NewHandler(logger *scribe.Scribe, batchManager *database.BatchManager) *Handler {
	return &Handler{
		chaosEngine:  chaos.NewEngine(),
		schemas:      make(map[string]*jsonschema.Schema),
		Logger:       logger,
		BatchManager: batchManager,
	}
}

// RegisterLocation registers a location with the handler
func (h *Handler) RegisterLocation(location models.Location) error {
	h.Logger.Info().
		Str("path", location.Path).
		Str("method", location.Method).
		Int("status_code", location.StatusCode).
		Msg("Registering location")

	// If schema is provided, compile it
	if location.Schema != "" {
		schema, err := h.compileSchema(location.Schema)
		if err != nil {
			h.Logger.Error().
				Str("path", location.Path).
				Str("method", location.Method).
				AnErr("error", err).
				Msg("Error compiling schema for location")
			return fmt.Errorf("error compiling schema for path %s: %w", location.Path, err)
		}
		h.schemas[location.Path+":"+location.Method] = schema
		h.Logger.Debug().
			Str("path", location.Path).
			Str("method", location.Method).
			Msg("Schema compiled successfully for location")
	}

	return nil
}

// compileSchema compiles a JSON schema
func (h *Handler) compileSchema(schemaStr string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()

	// Parse the schema string as JSON first
	var schemaData interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schemaData); err != nil {
		return nil, fmt.Errorf("error parsing schema JSON: %w", err)
	}

	// Add the schema to the compiler using the parsed data
	if err := compiler.AddResource("schema.json", schemaData); err != nil {
		return nil, fmt.Errorf("error adding schema resource: %w", err)
	}

	// Compile the schema
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return nil, fmt.Errorf("error compiling schema: %w", err)
	}

	return schema, nil
}

// HandleRequest handles an HTTP request based on the location configuration
func (h *Handler) HandleRequest(c *gin.Context, location models.Location) {
	// Start timing for metrics
	start := time.Now()
	requestPath := location.Path // Usar location.Path para las métricas si es consistente
	requestMethod := c.Request.Method

	// Incrementar el gauge de solicitudes activas para este path/method
	prom.HandlerActiveRequests.WithLabelValues(requestMethod, requestPath).Inc()

	// Asegurarse de que el gauge se decremente al finalizar, sin importar el resultado
	defer prom.HandlerActiveRequests.WithLabelValues(requestMethod, requestPath).Dec()

	ctx := scribe.WithCtx(c.Request.Context())

	logCtx := scribe.GetLogContext(ctx)

	logCtx.Set("request_trace_id", uuid.New().String())
	r := c.Request.WithContext(ctx)

	c.Request = r

	h.Logger.DebugCtx(ctx).
		Str("method", c.Request.Method).
		Str("path", c.Request.URL.Path).
		Str("ip", c.ClientIP()).
		Msg("Handling request")

	// Apply chaos injection if configured
	if location.ChaosInjection != nil {
		if h.chaosEngine.ApplyChaos(c.Writer, location.ChaosInjection) {
			h.Logger.WarnCtx(ctx).Msg("Request aborted by chaos injection")
			// Insertar en BD con el status code modificado por chaos
			h.insertTransactionToDB(c, location)

			// --- FIN DEL HANDLER: CAPTURAR MÉTRICAS DE RESPUESTA ---
			statusCode := strconv.Itoa(c.Writer.Status()) // Obtener el status code real después de chaos
			prom.HandlerResquestTotal.WithLabelValues(requestPath, requestMethod, statusCode).Inc()
			prom.HandlerRequestDuration.WithLabelValues(requestPath, requestMethod, statusCode).Observe(time.Since(start).Seconds())
			prom.HandlerErrorsTotal.WithLabelValues(requestPath, requestMethod, "chaos_aborted").Inc() // Contar el error
			// --- FIN DE CAPTURAR MÉTRICAS DE RESPUESTA ---

			return
		}
	}

	// Validate request body against schema if configured
	if schema, ok := h.schemas[location.Path+":"+location.Method]; ok {
		if err := h.validateRequestBody(c, schema); err != nil {
			h.Logger.ErrorCtx(ctx).AnErr("validation_error", err).Msg("Schema validation failed")
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Schema validation failed: %v", err)})
			// Insertar en BD con el status code real (400)
			h.insertTransactionToDB(c, location)

			// --- FIN DEL HANDLER: CAPTURAR MÉTRICAS DE RESPUESTA ---
			statusCode := strconv.Itoa(c.Writer.Status()) // Debería ser 400
			prom.HandlerResquestTotal.WithLabelValues(requestPath, requestMethod, statusCode).Inc()
			prom.HandlerRequestDuration.WithLabelValues(requestPath, requestMethod, statusCode).Observe(time.Since(start).Seconds())
			prom.HandlerErrorsTotal.WithLabelValues(requestPath, requestMethod, "schema_validation_failed").Inc() // Contar el error
			// --- FIN DE CAPTURAR MÉTRICAS DE RESPUESTA ---

			return
		}
	}

	// Set response headers if configured
	if location.Headers != nil {
		for key, value := range *location.Headers {
			c.Header(key, value)
		}
	}

	// Handle async call if configured
	if location.Async != nil {
		for _, v := range location.Async {
			asyncURL := v.Url
			if v.Path != "" {
				asyncURL = v.Path
			}
			h.Logger.InfoCtx(ctx).
				Str("async_url", asyncURL).
				Str("async_method", v.Method).
				Msg("Starting async call")
			go h.handleAsyncCall(&v, c)
			// Contar las llamadas asíncronas
			prom.HandlerAsyncCallsTotal.WithLabelValues(requestPath, requestMethod, asyncURL).Inc()
		}
	}

	// Set response status code
	c.Status(location.StatusCode)

	// Set response body if configured
	if location.Response != "" {
		// Solo establecer Content-Type si no fue definido en los headers del config
		if location.Headers == nil || (*location.Headers)["Content-Type"] == "" {
			c.Header("Content-Type", "application/json")
		}

		// Process template if it contains template variables
		responseBody, err := h.processResponseTemplate(c, string(location.Response))
		if err != nil {
			h.Logger.ErrorCtx(ctx).AnErr("template_error", err).Msg("Error processing response template")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error processing response template"})
			// Insertar en BD con el status code real (500)
			h.insertTransactionToDB(c, location)

			// --- FIN DEL HANDLER: CAPTURAR MÉTRICAS DE RESPUESTA ---
			statusCode := strconv.Itoa(c.Writer.Status()) // Debería ser 500
			prom.HandlerResquestTotal.WithLabelValues(requestPath, requestMethod, statusCode).Inc()
			prom.HandlerRequestDuration.WithLabelValues(requestPath, requestMethod, statusCode).Observe(time.Since(start).Seconds())
			prom.HandlerErrorsTotal.WithLabelValues(requestPath, requestMethod, "response_template_error").Inc() // Contar el error
			// --- FIN DE CAPTURAR MÉTRICAS DE RESPUESTA ---

			return
		}

		h.Logger.InfoCtx(ctx).Str("response", string(responseBody)).Msg("Response processed successfully")
		c.String(location.StatusCode, responseBody)
	}

	h.Logger.InfoCtx(ctx).
		Int("status_code", location.StatusCode).
		Msg("Request completed successfully")

	// Insertar en BD al finalizar la operación (casos exitosos)
	h.insertTransactionToDB(c, location)

	// --- FIN DEL HANDLER: CAPTURAR MÉTRICAS DE RESPUESTA ---
	// Este es el punto final de ejecución exitosa del handler.
	statusCode := strconv.Itoa(c.Writer.Status()) // Obtener el status code final.
	prom.HandlerResquestTotal.WithLabelValues(requestPath, requestMethod, statusCode).Inc()
	prom.HandlerRequestDuration.WithLabelValues(requestPath, requestMethod, statusCode).Observe(time.Since(start).Seconds())
	// --- FIN DE CAPTURAR MÉTRICAS DE RESPUESTA ---
}

// validateRequestBody validates the request body against a JSON schema
func (h *Handler) validateRequestBody(c *gin.Context, schema *jsonschema.Schema) error {
	ctx := c.Request.Context()

	h.Logger.InfoCtx(ctx).Msg("Starting request body validation")

	// Read the request body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("error", err).Msg("Error reading request body")
		return fmt.Errorf("error reading request body: %w", err)
	}

	// Restore the request body for later use
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// Parse the JSON
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("error", err).Msg("Error parsing JSON")
		return fmt.Errorf("error parsing JSON: %w", err)
	}

	// Validate against the schema
	if err := schema.Validate(data); err != nil {
		h.Logger.ErrorCtx(ctx).AnErr("validation_error", err).Msg("Schema validation failed")
		return err
	}

	h.Logger.DebugCtx(ctx).Msg("Request body validation successful")

	return nil
}

// handleAsyncCall handles an asynchronous HTTP call
func (h *Handler) handleAsyncCall(async *models.Async, c *gin.Context) {

	ctx := scribe.WithCtx(c.Request.Context())

	lc := scribe.GetLogContext(ctx)
	lc.Set("async_request_trace_id", uuid.New().String())

	r := c.Request.WithContext(ctx)
	c.Request = r

	// Construir la URL: si hay Path, construir URL completa; si hay Url, usarla directamente
	asyncURL := async.Url
	if async.Path != "" {
		// Es un path relativo, construir la URL completa basándose en el request actual
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host
		if host == "" {
			host = "localhost"
		}
		asyncURL = fmt.Sprintf("%s://%s%s", scheme, host, async.Path)
	}

	h.Logger.DebugCtx(ctx).
		Str("url", asyncURL).
		Str("method", async.Method).
		Msg("Creating async HTTP request")

	// Create HTTP client with timeout
	client := &http.Client{}
	if async.Timeout != nil {
		client.Timeout = time.Duration(*async.Timeout) * time.Millisecond
	}

	// Create request
	var body io.Reader
	if async.Body != "" {
		// Procesar el template del body async con los mismos valores aleatorios del request principal
		processedBody, err := h.processResponseTemplate(c, async.Body)
		if err != nil {
			h.Logger.ErrorCtx(ctx).
				Str("url", asyncURL).
				Str("method", async.Method).
				AnErr("error", err).
				Msg("Error processing async body template")
			// Si hay error, usar el body original sin procesar
			body = strings.NewReader(async.Body)
		} else {
			body = strings.NewReader(processedBody)
		}
	}

	req, err := http.NewRequest(async.Method, asyncURL, body)
	if err != nil {
		h.Logger.ErrorCtx(ctx).
			Str("url", asyncURL).
			Str("method", async.Method).
			AnErr("error", err).
			Msg("Error creating async request")
		return
	}

	// Set headers
	if async.Headers != nil {
		for key, value := range *async.Headers {
			req.Header.Set(key, value)
		}
	}

	// Pasar el transaction ID a los async requests para que puedan usar los mismos valores aleatorios
	if transactionID, exists := c.Get("transactionID"); exists {
		req.Header.Set("X-Transaction-ID", transactionID.(string))
	}

	// Set default content type if not specified
	if async.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Execute request with retries
	var resp *http.Response
	var lastErr error
	retries := 1
	if async.Retries != nil {
		retries = *async.Retries + 1
	}

	retryDelay := 100 // Default retry delay in milliseconds
	if async.RetryDelay != nil {
		retryDelay = *async.RetryDelay
	}

	for i := 0; i < retries; i++ {
		resp, lastErr = client.Do(req)
		if lastErr == nil {
			break
		}

		if i < retries-1 {
			h.Logger.WarnCtx(ctx).
				Str("url", asyncURL).
				Int("attempt", i+1).
				Int("max_retries", retries-1).
				AnErr("error", lastErr).
				Msg("Async request failed, retrying")
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}

	// Handle response
	if lastErr != nil {
		h.Logger.ErrorCtx(ctx).
			Str("url", asyncURL).
			Str("method", async.Method).
			Int("retries", retries-1).
			AnErr("error", lastErr).
			Msg("Error executing async request after retries")
		return
	}
	defer resp.Body.Close()

	// Log response status
	h.Logger.InfoCtx(ctx).
		Str("url", asyncURL).
		Str("method", async.Method).
		Str("status", resp.Status).
		Int("status_code", resp.StatusCode).
		Msg("Async request completed successfully")
}

// processResponseTemplate processes the response template with request data
func (h *Handler) processResponseTemplate(c *gin.Context, responseTemplate string) (string, error) {
	// Check if template contains template variables
	if !strings.Contains(responseTemplate, "{{") {
		return responseTemplate, nil
	}

	// Parse request body to extract data for template variables
	// Utilizamos map[string]interface{} para que las propiedades del JSON (como .Amount) sean accesibles
	var requestData map[string]interface{}
	if c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return "", fmt.Errorf("error reading request body: %w", err)
		}

		// Restore the request body for potential later use
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) > 0 {
			// Intentamos hacer Unmarshal en un mapa para facilitar el acceso por nombre de campo
			// Si falla (por ejemplo, si es XML), simplemente continuamos sin requestData
			// Las funciones de aleatorización seguirán funcionando desde el caché
			if err := json.Unmarshal(body, &requestData); err != nil {
				// Si no es JSON válido (puede ser XML u otro formato), simplemente continuamos
				// sin requestData. Las funciones de aleatorización usarán el caché compartido.
				requestData = make(map[string]interface{})
			}
		}
	}

	// Agregar query parameters al contexto del template
	if requestData == nil {
		requestData = make(map[string]interface{})
	}
	// Agregar query params como .Query.paramName en el template
	queryParams := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0] // Tomar el primer valor
		}
	}
	requestData["Query"] = queryParams

	// Obtener o crear el mapa de valores aleatorios compartidos
	// Usar un ID de transacción para compartir valores entre diferentes requests
	transactionID := c.GetHeader("X-Transaction-ID")
	if transactionID == "" {
		// Si no hay ID de transacción, generar uno nuevo
		transactionID = uuid.New().String()
		c.Header("X-Transaction-ID", transactionID)
	}

	// Obtener o crear el mapa de valores aleatorios para esta transacción
	randomCacheMutex.Lock()
	var randomValues map[string]interface{}
	if val, exists := randomCache[transactionID]; exists {
		randomValues = val
	} else {
		randomValues = make(map[string]interface{})
		randomCache[transactionID] = randomValues
		// Limpiar el caché después de 5 minutos para evitar memory leaks
		go func(id string) {
			time.Sleep(5 * time.Minute)
			randomCacheMutex.Lock()
			delete(randomCache, id)
			randomCacheMutex.Unlock()
		}(transactionID)
	}
	randomCacheMutex.Unlock()

	// También almacenar en el contexto de Gin para acceso rápido
	c.Set("randomValues", randomValues)
	c.Set("transactionID", transactionID)

	// Agregar los valores aleatorios al contexto del template
	requestData["Random"] = randomValues

	// Create template with custom functions (incluyendo randInt y now que devuelve time.Time)
	tmpl, err := template.New("response").Funcs(template.FuncMap{
		"toJson": func(v interface{}) string {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(jsonBytes)
		},
		// Devuelve un objeto time.Time para que la plantilla pueda llamar a .Format
		"now": func() time.Time {
			return time.Now()
		},
		// Agrega la función randInt necesaria para generar números aleatorios (con caché)
		"randInt": func(min, max int) int {
			key := fmt.Sprintf("randInt_%d_%d", min, max)
			if val, exists := randomValues[key]; exists {
				return val.(int)
			}
			rand.Seed(time.Now().UnixNano())
			value := rand.Intn(max-min) + min
			randomValues[key] = value
			return value
		},
		// Genera un string numérico aleatorio de longitud especificada (con caché)
		"randNumericString": func(length int) string {
			key := fmt.Sprintf("randNumericString_%d", length)
			if val, exists := randomValues[key]; exists {
				return val.(string)
			}
			rand.Seed(time.Now().UnixNano())
			digits := "0123456789"
			result := make([]byte, length)
			for i := range result {
				result[i] = digits[rand.Intn(len(digits))]
			}
			value := string(result)
			randomValues[key] = value
			return value
		},
		// Genera un nombre aleatorio (nombre + apellido) (con caché)
		"randName": func() string {
			key := "randName"
			if val, exists := randomValues[key]; exists {
				return val.(string)
			}
			rand.Seed(time.Now().UnixNano())
			firstNames := []string{"Kathryn", "Rebecca", "John", "Maria", "Carlos", "Ana", "Luis", "Patricia", "Roberto", "Laura", "David", "Sofia", "Michael", "Isabella", "James", "Emily", "William", "Olivia", "Richard", "Emma"}
			lastNames := []string{"Schmitt", "Anderson", "Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore"}
			value := firstNames[rand.Intn(len(firstNames))] + " " + lastNames[rand.Intn(len(lastNames))]
			randomValues[key] = value
			return value
		},
		// Genera un ID venezolano aleatorio (formato: Letra + números) (con caché)
		"randVenezuelanID": func() string {
			key := "randVenezuelanID"
			if val, exists := randomValues[key]; exists {
				return val.(string)
			}
			rand.Seed(time.Now().UnixNano())
			letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
			letter := string(letters[rand.Intn(len(letters))])
			digits := "0123456789"
			result := make([]byte, 8)
			for i := range result {
				result[i] = digits[rand.Intn(len(digits))]
			}
			value := letter + string(result)
			randomValues[key] = value
			return value
		},
		// Genera un número de cuenta aleatorio (formato: código banco + números) (con caché)
		"randAccount": func(bankCode string, length int) string {
			key := fmt.Sprintf("randAccount_%s_%d", bankCode, length)
			if val, exists := randomValues[key]; exists {
				return val.(string)
			}
			rand.Seed(time.Now().UnixNano())
			digits := "0123456789"
			result := make([]byte, length)
			for i := range result {
				result[i] = digits[rand.Intn(len(digits))]
			}
			value := bankCode + string(result)
			randomValues[key] = value
			return value
		},
		// Genera un mensaje aleatorio (con caché)
		"randMessage": func() string {
			key := "randMessage"
			if val, exists := randomValues[key]; exists {
				return val.(string)
			}
			rand.Seed(time.Now().UnixNano())
			messages := []string{"PRUEBA ENVIO", "TRANSFERENCIA", "PAGO SERVICIO", "ABONO CUENTA", "DEBITO AUTOMATICO", "CREDITO AUTOMATICO", "TRANSACCION PRUEBA", "OPERACION TEST"}
			value := messages[rand.Intn(len(messages))]
			randomValues[key] = value
			return value
		},
		// Genera un string alfanumérico aleatorio
		"randString": func(length int) string {
			rand.Seed(time.Now().UnixNano())
			chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			result := make([]byte, length)
			for i := range result {
				result[i] = chars[rand.Intn(len(chars))]
			}
			return string(result)
		},
		// Genera un valor aleatorio de una lista de opciones
		"randChoice": func(choices ...string) string {
			if len(choices) == 0 {
				return ""
			}
			rand.Seed(time.Now().UnixNano())
			return choices[rand.Intn(len(choices))]
		},
		// Genera un valor decimal aleatorio
		"randFloat": func(min, max float64) float64 {
			rand.Seed(time.Now().UnixNano())
			return min + rand.Float64()*(max-min)
		},
		// Genera un valor UTF-8 inválido o válido según query param
		// Si existe query param "utf8_type", genera UTF-8 inválido del tipo especificado
		// Si no existe el query param, genera UTF-8 válido por defecto
		// Uso: {{ invalidUTF8 }} o {{ invalidUTF8 "random" }}
		"invalidUTF8": func(args ...string) string {
			// Leer query param "utf8_type" si existe
			utf8Type := c.Query("utf8_type")

			// Si hay query param, usarlo (tiene prioridad sobre argumentos)
			if utf8Type != "" {
				return invalid.GetInvalidUTF8ByTypeName(utf8Type)
			}

			// Si se pasó un argumento, usarlo
			if len(args) > 0 && args[0] != "" {
				return invalid.GetInvalidUTF8ByTypeName(args[0])
			}

			// Por defecto, generar UTF-8 válido
			return invalid.GenerateValidUTF8()
		},
		// Función helper para obtener query param desde el template
		"query": func(key string) string {
			return c.Query(key)
		},
	}).Parse(responseTemplate)

	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Execute template with request data (map[string]interface{} pasado como contexto raíz)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, requestData); err != nil {
		// Si el error persiste aquí, es probable que la sintaxis de la plantilla (YAML) sea el problema.
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

// insertTransactionToDB inserta la transacción en la base de datos
func (h *Handler) insertTransactionToDB(c *gin.Context, location models.Location) {
	if h.BatchManager == nil {
		h.Logger.Warn().Msg("BatchManager is nil, skipping database insertion")
		return
	}

	// Verificar si BatchManager está corriendo
	if !h.BatchManager.IsRunning() {
		h.Logger.Warn().Msg("BatchManager is not running, skipping database insertion")
		return
	}

	// Extraer datos del request
	requestHeaders, _ := json.Marshal(c.Request.Header)
	requestBody := h.getRequestBody(c)
	responseHeaders, _ := json.Marshal(c.Writer.Header())
	responseBody := h.getActualResponseBody(c, location)

	// Obtener el status code real del response writer
	actualStatusCode := h.getActualStatusCode(c)

	// Crear Mockdata
	recepcionID := c.GetHeader("X-Recepcion-ID")
	if recepcionID == "" {
		recepcionID = uuid.New().String()
	}

	senderID := c.GetHeader("X-Sender-ID")
	if senderID == "" {
		senderID = uuid.New().String()
	}

	operation := &database.Mockdata{
		UUID:               uuid.New().String(),
		RecepcionID:        recepcionID,
		SenderID:           senderID,
		RequestHeaders:     string(requestHeaders),
		RequestMethod:      c.Request.Method,
		RequestEndpoint:    c.Request.URL.Path,
		RequestBody:        requestBody,
		ResponseHeaders:    string(responseHeaders),
		ResponseBody:       responseBody,
		ResponseStatusCode: actualStatusCode,
		Timestamp:          time.Now(),
	}

	// Insertar en batch
	if err := h.BatchManager.AddOperation(operation); err != nil {
		h.Logger.Error().
			Str("uuid", operation.UUID).
			Str("recepcion_id", operation.RecepcionID).
			AnErr("error", err).
			Msg("Error inserting transaction to database")
	} else {
		h.Logger.Info().
			Str("uuid", operation.UUID).
			Str("recepcion_id", operation.RecepcionID).
			Str("method", operation.RequestMethod).
			Str("endpoint", operation.RequestEndpoint).
			Int("status_code", actualStatusCode).
			Msg("Transaction added to batch successfully")
	}
}

// getRequestBody extrae el body del request
func (h *Handler) getRequestBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.Logger.Error().AnErr("error", err).Msg("Error reading request body for database")
		return ""
	}

	// Restaurar el body para uso posterior
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body)
}

// getResponseBody extrae el body de la respuesta
func (h *Handler) getResponseBody(c *gin.Context, location models.Location) string {
	if location.Response == "" {
		return ""
	}

	// Procesar template si existe
	responseBody, err := h.processResponseTemplate(c, string(location.Response))
	if err != nil {
		return string(location.Response)
	}

	return responseBody
}

// getActualStatusCode obtiene el status code real del response writer
func (h *Handler) getActualStatusCode(c *gin.Context) int {
	// En Gin, el status code se puede obtener del response writer
	// Si no se ha establecido, devuelve 200 por defecto
	if c.Writer.Status() == 0 {
		return 200
	}
	return c.Writer.Status()
}

// getActualResponseBody obtiene el response body real que se envió al cliente
func (h *Handler) getActualResponseBody(c *gin.Context, location models.Location) string {
	// Verificar si el chaos injection se activó
	// Si el status code es diferente al configurado, significa que hubo chaos injection
	actualStatusCode := h.getActualStatusCode(c)

	// Si hay chaos injection activado (status code diferente al configurado)
	if location.ChaosInjection != nil && actualStatusCode != location.StatusCode {
		// Para casos de chaos injection, devolver el response del chaos config
		if location.ChaosInjection.Error.Response != "" {
			return location.ChaosInjection.Error.Response
		}
		// Si no hay response específico en chaos, devolver string vacío
		return ""
	}

	// Para casos normales (sin chaos injection), usar el response configurado
	if location.Response != "" {
		responseBody, err := h.processResponseTemplate(c, string(location.Response))
		if err != nil {
			return string(location.Response)
		}
		return responseBody
	}

	return ""
}
