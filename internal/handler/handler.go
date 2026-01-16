package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
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
	var requestBodyXML string
	if c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return "", fmt.Errorf("error reading request body: %w", err)
		}

		// Restore the request body for potential later use
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) > 0 {
			// Intentamos hacer Unmarshal en un mapa para facilitar el acceso por nombre de campo
			// Si falla (por ejemplo, si es XML), intentamos parsear como XML
			if err := json.Unmarshal(body, &requestData); err != nil {
				// Si no es JSON válido, intentar parsear como XML
				contentType := c.GetHeader("Content-Type")
				if strings.Contains(contentType, "xml") || strings.Contains(string(body), "<?xml") || strings.Contains(string(body), "<") {
					// Guardar el body XML como string para extracción posterior
					requestBodyXML = string(body)
					requestData = make(map[string]interface{})
				} else {
					// Si no es JSON ni XML, simplemente continuamos sin requestData
					requestData = make(map[string]interface{})
				}
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
		// Función helper para valores por defecto
		"default": func(value, defaultValue interface{}) interface{} {
			if value == nil {
				return defaultValue
			}
			if str, ok := value.(string); ok && str == "" {
				return defaultValue
			}
			return value
		},
		// Función helper para printf
		"printf": fmt.Sprintf,
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
		// Función helper para extraer valores del JSON del request
		// Uso: {{ jsonValue "PmtInf.0.Dbtr.Nm" }} o {{ jsonValue "PmtInf.0.Dbtr.Id" }}
		"jsonValue": func(path string) string {
			if requestData == nil {
				return ""
			}

			// Navegar por la ruta del JSON (ej: "PmtInf.0.Dbtr.Nm")
			// Primero buscar en CstmrCdtTrfInitn si existe
			var current interface{} = requestData
			if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
				current = cstmr
			}

			parts := strings.Split(path, ".")
			for _, part := range parts {
				switch v := current.(type) {
				case map[string]interface{}:
					if val, ok := v[part]; ok {
						current = val
					} else {
						return ""
					}
				case []interface{}:
					// Si es un array, intentar convertir el índice
					if idx, err := strconv.Atoi(part); err == nil && idx >= 0 && idx < len(v) {
						current = v[idx]
					} else {
						return ""
					}
				default:
					return ""
				}
			}

			// Convertir el valor final a string
			if str, ok := current.(string); ok {
				return str
			}
			if num, ok := current.(float64); ok {
				return fmt.Sprintf("%.0f", num)
			}
			if num, ok := current.(int); ok {
				return strconv.Itoa(num)
			}

			return ""
		},
		// Función helper para extraer valores del JSON con valor por defecto
		// Uso: {{ jsonValueOr "PmtInf.0.Dbtr.Nm" "Default Name" }}
		"jsonValueOr": func(path string, defaultValue string) string {
			value := ""
			if requestData != nil {
				// Navegar por la ruta del JSON
				var current interface{} = requestData
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					current = cstmr
				}

				parts := strings.Split(path, ".")
				for _, part := range parts {
					switch v := current.(type) {
					case map[string]interface{}:
						if val, ok := v[part]; ok {
							current = val
						} else {
							return defaultValue
						}
					case []interface{}:
						if idx, err := strconv.Atoi(part); err == nil && idx >= 0 && idx < len(v) {
							current = v[idx]
						} else {
							return defaultValue
						}
					default:
						return defaultValue
					}
				}

				// Convertir el valor final a string
				if str, ok := current.(string); ok {
					value = str
				} else if num, ok := current.(float64); ok {
					value = fmt.Sprintf("%.0f", num)
				} else if num, ok := current.(int); ok {
					value = strconv.Itoa(num)
				}
			}

			if value == "" {
				return defaultValue
			}
			return value
		},
		// Función helper para obtener EndToEndId del JSON o generar uno
		"endToEndId": func() string {
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
						if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
							if endToEndId, ok := pmtInf0["EndToEndId"].(string); ok && endToEndId != "" {
								return endToEndId
							}
						}
					}
				}
			}
			// Generar usando randNumericString existente
			key := fmt.Sprintf("randNumericString_8")
			var randomPart string
			if val, exists := randomValues[key]; exists {
				randomPart = val.(string)
			} else {
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				result := make([]byte, 8)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				randomPart = string(result)
				randomValues[key] = randomPart
			}
			return "0114" + time.Now().Format("20060102150405") + randomPart
		},
		// Función helper para obtener TxId del JSON o generar uno de 30 dígitos
		"txId": func() string {
			// Intentar obtener del JSON primero
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
						if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
							// Obtener DbtrAgt como código de banco
							bankCode := "0001"
							if dbtrAgt, ok := pmtInf0["DbtrAgt"].(string); ok && dbtrAgt != "" {
								bankCode = dbtrAgt
							}
							// Si hay TxId en el JSON, usarlo
							if txId, ok := pmtInf0["TxId"].(string); ok && txId != "" {
								// Asegurar que tenga 30 dígitos
								if len(txId) < 30 {
									return strings.Repeat("0", 30-len(txId)) + txId
								} else if len(txId) > 30 {
									return txId[len(txId)-30:]
								}
								return txId
							}
							// Generar TxId de 30 dígitos: código_banco (4) + fecha/hora (14) + aleatorios (12)
							bankCodePadded := bankCode
							if len(bankCode) > 4 {
								bankCodePadded = bankCode[:4]
							} else if len(bankCode) < 4 {
								bankCodePadded = strings.Repeat("0", 4-len(bankCode)) + bankCode
							}
							// Generar parte aleatoria
							key := fmt.Sprintf("randNumericString_12")
							var randomPart string
							if val, exists := randomValues[key]; exists {
								randomPart = val.(string)
							} else {
								rand.Seed(time.Now().UnixNano())
								digits := "0123456789"
								result := make([]byte, 12)
								for i := range result {
									result[i] = digits[rand.Intn(len(digits))]
								}
								randomPart = string(result)
								randomValues[key] = randomPart
							}
							return bankCodePadded + time.Now().Format("20060102150405") + randomPart
						}
					}
				}
			}
			// Fallback: generar uno de 30 dígitos
			key := fmt.Sprintf("randNumericString_12")
			var randomPart string
			if val, exists := randomValues[key]; exists {
				randomPart = val.(string)
			} else {
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				result := make([]byte, 12)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				randomPart = string(result)
				randomValues[key] = randomPart
			}
			return "0001" + time.Now().Format("20060102150405") + randomPart
		},
		// Función helper para obtener OrgnlTxId (30 dígitos) - mismo formato que TxId
		"orgnlTxId": func() string {
			// Reutilizar la lógica de txId directamente
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
						if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
							bankCode := "0001"
							if dbtrAgt, ok := pmtInf0["DbtrAgt"].(string); ok && dbtrAgt != "" {
								bankCode = dbtrAgt
							}
							if txId, ok := pmtInf0["TxId"].(string); ok && txId != "" {
								if len(txId) < 30 {
									return strings.Repeat("0", 30-len(txId)) + txId
								} else if len(txId) > 30 {
									return txId[len(txId)-30:]
								}
								return txId
							}
							bankCodePadded := bankCode
							if len(bankCode) > 4 {
								bankCodePadded = bankCode[:4]
							} else if len(bankCode) < 4 {
								bankCodePadded = strings.Repeat("0", 4-len(bankCode)) + bankCode
							}
							key := fmt.Sprintf("randNumericString_12")
							var randomPart string
							if val, exists := randomValues[key]; exists {
								randomPart = val.(string)
							} else {
								rand.Seed(time.Now().UnixNano())
								digits := "0123456789"
								result := make([]byte, 12)
								for i := range result {
									result[i] = digits[rand.Intn(len(digits))]
								}
								randomPart = string(result)
								randomValues[key] = randomPart
							}
							return bankCodePadded + time.Now().Format("20060102150405") + randomPart
						}
					}
				}
			}
			key := fmt.Sprintf("randNumericString_12")
			var randomPart string
			if val, exists := randomValues[key]; exists {
				randomPart = val.(string)
			} else {
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				result := make([]byte, 12)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				randomPart = string(result)
				randomValues[key] = randomPart
			}
			return "0001" + time.Now().Format("20060102150405") + randomPart
		},
		// Función helper para obtener OrgnlEndToEndId
		"orgnlEndToEndId": func() string {
			// Reutilizar la lógica de endToEndId directamente
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
						if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
							if endToEndId, ok := pmtInf0["EndToEndId"].(string); ok && endToEndId != "" {
								return endToEndId
							}
						}
					}
				}
			}
			key := fmt.Sprintf("randNumericString_8")
			var randomPart string
			if val, exists := randomValues[key]; exists {
				randomPart = val.(string)
			} else {
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				result := make([]byte, 8)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				randomPart = string(result)
				randomValues[key] = randomPart
			}
			return "0114" + time.Now().Format("20060102150405") + randomPart
		},
		// Función helper para obtener OrgnlMsgId del JSON o generar uno
		"orgnlMsgId": func() string {
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if grpHdr, ok := cstmr["GrpHdr"].(map[string]interface{}); ok {
						if msgId, ok := grpHdr["MsgId"].(string); ok && msgId != "" {
							return msgId
						}
					}
				}
			}
			// Generar uno: código_banco + fecha/hora + aleatorios
			bankCode := "0172"
			if requestData != nil {
				if cstmr, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
						if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
							if cdtrAgt, ok := pmtInf0["CdtrAgt"].(string); ok && cdtrAgt != "" {
								bankCode = cdtrAgt
							}
						}
					}
				}
			}
			if len(bankCode) > 4 {
				bankCode = bankCode[:4]
			} else if len(bankCode) < 4 {
				bankCode = strings.Repeat("0", 4-len(bankCode)) + bankCode
			}
			key := fmt.Sprintf("randNumericString_8")
			var randomPart string
			if val, exists := randomValues[key]; exists {
				randomPart = val.(string)
			} else {
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				result := make([]byte, 8)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				randomPart = string(result)
				randomValues[key] = randomPart
			}
			return bankCode + "01" + time.Now().Format("20060102150405") + randomPart
		},
		// Función helper para extraer valores del XML del request
		// Uso: {{ xmlValue "Id" }} o {{ xmlValue "Dbtr.Id" }} para rutas anidadas
		"xmlValue": func(path string) string {
			if requestBodyXML == "" {
				return ""
			}

			// Si la ruta contiene puntos, buscar el último elemento (ej: "Dbtr.Id" -> buscar "Id" dentro de "Dbtr")
			parts := strings.Split(path, ".")
			tagName := parts[len(parts)-1]

			// Crear expresión regular para encontrar el tag y su contenido
			// Buscar <TagName>valor</TagName> o <TagName atributos>valor</TagName>
			pattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([^<]*)</%s>`, regexp.QuoteMeta(tagName), regexp.QuoteMeta(tagName))
			re := regexp.MustCompile(pattern)

			// Si hay una ruta anidada, buscar primero el tag padre
			xmlToSearch := requestBodyXML
			if len(parts) > 1 {
				// Buscar el tag padre y usar solo ese fragmento
				parentTag := parts[len(parts)-2]
				parentPattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([\s\S]*?)</%s>`, regexp.QuoteMeta(parentTag), regexp.QuoteMeta(parentTag))
				parentRe := regexp.MustCompile(parentPattern)
				matches := parentRe.FindStringSubmatch(requestBodyXML)
				if len(matches) > 1 {
					xmlToSearch = matches[1]
				}
			}

			matches := re.FindStringSubmatch(xmlToSearch)
			if len(matches) > 1 {
				return strings.TrimSpace(matches[1])
			}

			return ""
		},
		// Función helper para construir ClrSysRef con formato: VES + código_banco (6) + TEST + TxId (30 dígitos)
		// Uso: {{ clrSysRef "000101" }} o {{ clrSysRef "0114" }}
		"clrSysRef": func(bankCode string) string {
			// Extraer el TxId del request (primero intentar JSON, luego XML)
			txId := ""

			// Intentar extraer del JSON primero
			if requestData != nil {
				// Buscar en PmtInf[0].TxId o similar
				if pmtInf, ok := requestData["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
					if pmtInfArr, ok := pmtInf["PmtInf"].([]interface{}); ok && len(pmtInfArr) > 0 {
						if pmtInf0, ok := pmtInfArr[0].(map[string]interface{}); ok {
							if txIdVal, ok := pmtInf0["TxId"].(string); ok && txIdVal != "" {
								txId = txIdVal
							}
						}
					}
				}
			}

			// Si no se encontró en JSON, intentar XML
			if txId == "" && requestBodyXML != "" {
				// Buscar TxId dentro de PmtId
				pmtIdPattern := regexp.MustCompile(`<PmtId(?:\s[^>]*)?>([\s\S]*?)</PmtId>`)
				pmtIdMatches := pmtIdPattern.FindStringSubmatch(requestBodyXML)
				if len(pmtIdMatches) > 1 {
					txIdPattern := regexp.MustCompile(`<TxId(?:\s[^>]*)?>([^<]*)</TxId>`)
					txIdMatches := txIdPattern.FindStringSubmatch(pmtIdMatches[1])
					if len(txIdMatches) > 1 {
						txId = strings.TrimSpace(txIdMatches[1])
					}
				}
			}

			// Si no se encontró TxId, generar uno de 30 dígitos como fallback
			if txId == "" {
				// Usar el mismo formato que en el template: código_banco (4) + fecha/hora (14) + aleatorios (12)
				// Usar el código del banco proporcionado o "0001" por defecto
				codePrefix := bankCode
				if len(bankCode) > 4 {
					codePrefix = bankCode[:4]
				} else if len(bankCode) < 4 {
					codePrefix = strings.Repeat("0", 4-len(bankCode)) + bankCode
				}

				dateTimeStr := time.Now().Format("20060102150405") // 14 dígitos
				// Generar 12 dígitos aleatorios para completar 30
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				randomPart := make([]byte, 12)
				for i := range randomPart {
					randomPart[i] = digits[rand.Intn(len(digits))]
				}
				txId = codePrefix + dateTimeStr + string(randomPart) // 4 + 14 + 12 = 30 dígitos
			}

			// Asegurar que el código del banco tenga 6 dígitos (rellenar con ceros a la izquierda)
			bankCodePadded := bankCode
			if len(bankCode) < 6 {
				bankCodePadded = strings.Repeat("0", 6-len(bankCode)) + bankCode
			} else if len(bankCode) > 6 {
				bankCodePadded = bankCode[len(bankCode)-6:]
			}

			// Construir ClrSysRef: VES + código_banco (6) + TEST + TxId (exactamente 33 caracteres - Hard33Text)
			// Formato: "VES" (3) + código_banco (6) + "TEST" (4) + TxId (20) = 33 caracteres
			// Del ejemplo: "VES000101TEST00000428119747823414" = 33 caracteres
			// - "VES" = 3
			// - "000101" = 6
			// - "TEST" = 4
			// - "00000428119747823414" = 20
			// Total = 33

			// Asegurar que el TxId tenga exactamente 20 dígitos para que el total sea 33
			// Formato ClrSysRef: "VES" (3) + código_banco (6) + "TEST" (4) + TxId (20) = 33 caracteres
			// Del ejemplo: "VES000101TEST00000428119747823414"
			// El TxId de 30 dígitos se trunca a los últimos 20 para ClrSysRef
			txIdPadded := txId
			if len(txId) == 0 {
				// Si no hay TxId, generar uno de 20 dígitos (fecha/hora 14 + aleatorios 6)
				dateTimeStr := time.Now().Format("20060102150405") // 14 dígitos
				rand.Seed(time.Now().UnixNano())
				digits := "0123456789"
				randomPart := make([]byte, 6)
				for i := range randomPart {
					randomPart[i] = digits[rand.Intn(len(digits))]
				}
				txIdPadded = dateTimeStr + string(randomPart) // 14 + 6 = 20 dígitos
			} else {
				// Siempre tomar exactamente los últimos 20 dígitos del TxId
				// Si tiene menos de 20, rellenar con ceros a la izquierda
				// Si tiene 20 o más, tomar los últimos 20
				if len(txId) < 20 {
					txIdPadded = strings.Repeat("0", 20-len(txId)) + txId
				} else {
					// Tomar los últimos 20 dígitos (funciona para 20, 22, 30, etc.)
					txIdPadded = txId[len(txId)-20:]
				}
			}

			// Asegurar que txIdPadded tenga exactamente 20 dígitos
			if len(txIdPadded) != 20 {
				if len(txIdPadded) < 20 {
					txIdPadded = strings.Repeat("0", 20-len(txIdPadded)) + txIdPadded
				} else {
					txIdPadded = txIdPadded[len(txIdPadded)-20:]
				}
			}

			// Construir ClrSysRef: VES (3) + bankCodePadded (6) + TEST (4) + txIdPadded (20) = 33
			clrSysRef := "VES" + bankCodePadded + "TEST" + txIdPadded
			// Validación final: debe ser exactamente 33 caracteres (Hard33Text)
			// Si excede, truncar a 33 (esto no debería pasar si todo está correcto)
			if len(clrSysRef) > 33 {
				clrSysRef = clrSysRef[:33]
			}
			return clrSysRef
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
