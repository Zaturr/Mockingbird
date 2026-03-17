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

	// Guardar body en contexto para que las plantillas (response y async) puedan usarlo sin race
	if c.Request.Body != nil {
		body, err := io.ReadAll(c.Request.Body)
		if err == nil {
			c.Set("requestBody", string(body))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
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

	// Extracción limpia de datos y caché (mismo patrón que handler.go)
	data, xmlBody, txID := h.analistDataForTemplate(c)
	cache := h.getOrCreateRandomCache(txID)

	c.Set("transactionID", txID)
	c.Set("randomValues", cache)
	data["Random"] = cache

	funcMap := h.createFuncMap(c, data, cache, xmlBody)
	tmpl, err := template.New("response").Funcs(funcMap).Parse(responseTemplate)

	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	// Execute template with request data (map[string]interface{} pasado como contexto raíz)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		// Si el error persiste aquí, es probable que la sintaxis de la plantilla (YAML) sea el problema.
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

// analistDataForTemplate prepara el entorno de datos del request .
func (h *Handler) analistDataForTemplate(c *gin.Context) (map[string]interface{}, string, string) {
	data := make(map[string]interface{})
	var xmlStr string
	var body []byte
	if b, ok := c.Get("requestBody"); ok {
		if s, ok := b.(string); ok {
			body = []byte(s)
		}
	}
	if len(body) == 0 && c.Request.Body != nil {
		var err error
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			return data, xmlStr, uuid.New().String()
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			contentType := c.GetHeader("Content-Type")
			if strings.Contains(contentType, "xml") || strings.Contains(string(body), "<?xml") || strings.Contains(string(body), "<") {
				xmlStr = string(body)
			}
			data = make(map[string]interface{})
		}
	}
	q := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			q[k] = v[0]
		}
	}
	data["Query"] = q
	txID := c.GetHeader("X-Transaction-ID")
	if txID == "" {
		txID = uuid.New().String()
		c.Header("X-Transaction-ID", txID)
	}
	return data, xmlStr, txID
}

// getOrCreateRandomCache obtiene o crea el caché de valores aleatorios por transacción (mismo patrón que handler.getOrCreateCache).
func (h *Handler) getOrCreateRandomCache(txID string) map[string]interface{} {
	randomCacheMutex.Lock()
	defer randomCacheMutex.Unlock()
	if _, exists := randomCache[txID]; !exists {
		randomCache[txID] = make(map[string]interface{})
		go func(id string) {
			time.Sleep(5 * time.Minute)
			randomCacheMutex.Lock()
			delete(randomCache, id)
			randomCacheMutex.Unlock()
		}(txID)
	}
	return randomCache[txID]
}

// getCached devuelve un valor del caché o lo genera y guarda (helper para createFuncMap).
func getCached(cache map[string]interface{}, key string, generator func() interface{}) interface{} {
	if val, exists := cache[key]; exists {
		return val
	}
	newVal := generator()
	cache[key] = newVal
	return newVal
}

// extractXmlValue extrae un valor del XML por ruta (para use en createFuncMap).
func extractXmlValue(path, xmlStr string) string {
	if xmlStr == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	tagName := parts[len(parts)-1]
	pattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([^<]*)</%s>`, regexp.QuoteMeta(tagName), regexp.QuoteMeta(tagName))
	re := regexp.MustCompile(pattern)
	xmlToSearch := xmlStr
	if len(parts) > 1 {
		parentTag := parts[len(parts)-2]
		parentPattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([\s\S]*?)</%s>`, regexp.QuoteMeta(parentTag), regexp.QuoteMeta(parentTag))
		parentRe := regexp.MustCompile(parentPattern)
		matches := parentRe.FindStringSubmatch(xmlStr)
		if len(matches) > 1 {
			xmlToSearch = matches[1]
		}
	}
	matches := re.FindStringSubmatch(xmlToSearch)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// jsonValue extrae un valor del JSON por ruta.
func jsonValue(path string, data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	var current interface{} = data
	if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
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
			if idx, err := strconv.Atoi(part); err == nil && idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return ""
			}
		default:
			return ""
		}
	}
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
}

// endToEndId obtiene EndToEndId del JSON o genera uno.
func endToEndId(data map[string]interface{}, cache map[string]interface{}) string {
	if data != nil {
		if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
			if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
				if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
					if endToEndId, ok := pmtInf0["EndToEndId"].(string); ok && endToEndId != "" {
						return endToEndId
					}
				}
			}
		}
	}
	key := "randNumericString_8"
	randomPart := getCached(cache, key, func() interface{} {
		digits := "0123456789"
		result := make([]byte, 8)
		for i := range result {
			result[i] = digits[rand.Intn(len(digits))]
		}
		return string(result)
	}).(string)
	return "0114" + time.Now().Format("20060102150405") + randomPart
}

// txId obtiene TxId del JSON o genera uno de 30 dígitos.
func txId(data map[string]interface{}, cache map[string]interface{}) string {
	if data != nil {
		if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
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
					key := "randNumericString_12"
					randomPart := getCached(cache, key, func() interface{} {
						digits := "0123456789"
						result := make([]byte, 12)
						for i := range result {
							result[i] = digits[rand.Intn(len(digits))]
						}
						return string(result)
					}).(string)
					return bankCodePadded + time.Now().Format("20060102150405") + randomPart
				}
			}
		}
	}
	key := "randNumericString_12"
	randomPart := getCached(cache, key, func() interface{} {
		digits := "0123456789"
		result := make([]byte, 12)
		for i := range result {
			result[i] = digits[rand.Intn(len(digits))]
		}
		return string(result)
	}).(string)
	return "0001" + time.Now().Format("20060102150405") + randomPart
}

// orgnlTxId mismo formato que txId.
func orgnlTxId(data map[string]interface{}, cache map[string]interface{}) string {
	return txId(data, cache)
}

// orgnlEndToEndId obtiene OrgnlEndToEndId del JSON o genera uno.
func orgnlEndToEndId(data map[string]interface{}, cache map[string]interface{}) string {
	if data != nil {
		if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
			if pmtInf, ok := cstmr["PmtInf"].([]interface{}); ok && len(pmtInf) > 0 {
				if pmtInf0, ok := pmtInf[0].(map[string]interface{}); ok {
					if endToEndId, ok := pmtInf0["EndToEndId"].(string); ok && endToEndId != "" {
						return endToEndId
					}
				}
			}
		}
	}
	key := "randNumericString_8"
	randomPart := getCached(cache, key, func() interface{} {
		digits := "0123456789"
		result := make([]byte, 8)
		for i := range result {
			result[i] = digits[rand.Intn(len(digits))]
		}
		return string(result)
	}).(string)
	return "0001" + time.Now().Format("20060102150405") + randomPart
}

// orgnlMsgId obtiene OrgnlMsgId del JSON o genera uno.
func orgnlMsgId(data map[string]interface{}, cache map[string]interface{}) string {
	if data != nil {
		if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
			if grpHdr, ok := cstmr["GrpHdr"].(map[string]interface{}); ok {
				if msgId, ok := grpHdr["MsgId"].(string); ok && msgId != "" {
					return msgId
				}
			}
		}
	}
	bankCode := "0172"
	if data != nil {
		if cstmr, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
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
	key := "randNumericString_8"
	randomPart := getCached(cache, key, func() interface{} {
		digits := "0123456789"
		result := make([]byte, 8)
		for i := range result {
			result[i] = digits[rand.Intn(len(digits))]
		}
		return string(result)
	}).(string)
	return "0172" + "01" + time.Now().Format("20060102150405") + randomPart
}

// clrSysRef construye ClrSysRef: VES + bankCode (6) + TEST + TxId (20).
func clrSysRef(data map[string]interface{}, cache map[string]interface{}, xmlBody string, bankCode string) string {
	txId := ""
	if data != nil {
		if pmtInf, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
			if pmtInfArr, ok := pmtInf["PmtInf"].([]interface{}); ok && len(pmtInfArr) > 0 {
				if pmtInf0, ok := pmtInfArr[0].(map[string]interface{}); ok {
					if txIdVal, ok := pmtInf0["TxId"].(string); ok && txIdVal != "" {
						txId = txIdVal
					}
				}
			}
		}
	}
	if txId == "" && xmlBody != "" {
		pmtIdPattern := regexp.MustCompile(`<PmtId(?:\s[^>]*)?>([\s\S]*?)</PmtId>`)
		pmtIdMatches := pmtIdPattern.FindStringSubmatch(xmlBody)
		if len(pmtIdMatches) > 1 {
			txIdPattern := regexp.MustCompile(`<TxId(?:\s[^>]*)?>([^<]*)</TxId>`)
			txIdMatches := txIdPattern.FindStringSubmatch(pmtIdMatches[1])
			if len(txIdMatches) > 1 {
				txId = strings.TrimSpace(txIdMatches[1])
			}
		}
	}
	if txId == "" {
		codePrefix := bankCode
		if len(bankCode) > 4 {
			codePrefix = bankCode[:4]
		} else if len(bankCode) < 4 {
			codePrefix = strings.Repeat("0", 4-len(bankCode)) + bankCode
		}
		dateTimeStr := time.Now().Format("20060102150405")
		digits := "0123456789"
		randomPart := make([]byte, 12)
		for i := range randomPart {
			randomPart[i] = digits[rand.Intn(len(digits))]
		}
		txId = codePrefix + dateTimeStr + string(randomPart)
	}
	bankCodePadded := bankCode
	if len(bankCode) < 6 {
		bankCodePadded = strings.Repeat("0", 6-len(bankCode)) + bankCode
	} else if len(bankCode) > 6 {
		bankCodePadded = bankCode[len(bankCode)-6:]
	}
	txIdPadded := txId
	if len(txId) == 0 {
		dateTimeStr := time.Now().Format("20060102150405")
		digits := "0123456789"
		randomPart := make([]byte, 6)
		for i := range randomPart {
			randomPart[i] = digits[rand.Intn(len(digits))]
		}
		txIdPadded = dateTimeStr + string(randomPart)
	} else {
		if len(txId) < 20 {
			txIdPadded = strings.Repeat("0", 20-len(txId)) + txId
		} else {
			txIdPadded = txId[len(txId)-20:]
		}
	}
	if len(txIdPadded) != 20 {
		if len(txIdPadded) < 20 {
			txIdPadded = strings.Repeat("0", 20-len(txIdPadded)) + txIdPadded
		} else {
			txIdPadded = txIdPadded[len(txIdPadded)-20:]
		}
	}
	clrSysRef := "VES" + bankCodePadded + "TEST" + txIdPadded
	if len(clrSysRef) > 33 {
		clrSysRef = clrSysRef[:33]
	}
	return clrSysRef
}

// createFuncMap construye el mapa de funciones para templates.
func (h *Handler) createFuncMap(c *gin.Context, data map[string]interface{}, cache map[string]interface{}, xmlBody string) template.FuncMap {
	return template.FuncMap{
		"toJson": func(v interface{}) string {
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(jsonBytes)
		},
		"default": func(value, defaultValue interface{}) interface{} {
			if value == nil {
				return defaultValue
			}
			if str, ok := value.(string); ok && str == "" {
				return defaultValue
			}
			return value
		},
		"printf": fmt.Sprintf,
		"now":    func() time.Time { return time.Now() },
		"query":  func(key string) string { return c.Query(key) },
		"uuid":   func() string { return uuid.New().String() },
		"randInt": func(min, max int) int {
			key := fmt.Sprintf("randInt_%d_%d", min, max)
			return getCached(cache, key, func() interface{} {
				return rand.Intn(max-min) + min
			}).(int)
		},
		"randNumericString": func(length int) string {
			key := fmt.Sprintf("randNumericString_%d", length)
			return getCached(cache, key, func() interface{} {
				digits := "0123456789"
				result := make([]byte, length)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				return string(result)
			}).(string)
		},
		"randName": func() string {
			return getCached(cache, "randName", func() interface{} {
				firstNames := []string{"Kathryn", "Rebecca", "John", "Maria", "Carlos", "Ana", "Luis", "Patricia", "Roberto", "Laura", "David", "Sofia", "Michael", "Isabella", "James", "Emily", "William", "Olivia", "Richard", "Emma"}
				lastNames := []string{"Schmitt", "Anderson", "Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez", "Hernandez", "Lopez", "Gonzalez", "Wilson", "Anderson", "Thomas", "Taylor", "Moore"}
				return firstNames[rand.Intn(len(firstNames))] + " " + lastNames[rand.Intn(len(lastNames))]
			}).(string)
		},
		"randVenezuelanID": func() string {
			return getCached(cache, "randVenezuelanID", func() interface{} {
				letters := "VEJGPC"
				letter := string(letters[rand.Intn(len(letters))])
				digits := "0123456789"
				result := make([]byte, 8)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				return letter + string(result)
			}).(string)
		},
		"randAccount": func(bankCode string, length int) string {
			key := fmt.Sprintf("randAccount_%s_%d", bankCode, length)
			return getCached(cache, key, func() interface{} {
				digits := "0123456789"
				result := make([]byte, length)
				for i := range result {
					result[i] = digits[rand.Intn(len(digits))]
				}
				return bankCode + string(result)
			}).(string)
		},
		"randMessage": func() string {
			return getCached(cache, "randMessage", func() interface{} {
				messages := []string{"PRUEBA ENVIO", "TRANSFERENCIA", "PAGO SERVICIO", "ABONO CUENTA", "DEBITO AUTOMATICO", "CREDITO AUTOMATICO", "TRANSACCION PRUEBA", "OPERACION TEST"}
				return messages[rand.Intn(len(messages))]
			}).(string)
		},
		"randString": func(length int) string {
			chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
			result := make([]byte, length)
			for i := range result {
				result[i] = chars[rand.Intn(len(chars))]
			}
			return string(result)
		},
		"randChoice": func(choices ...string) string {
			if len(choices) == 0 {
				return ""
			}
			return choices[rand.Intn(len(choices))]
		},
		"randFloat": func(min, max float64) float64 {
			return min + rand.Float64()*(max-min)
		},
		"invalidUTF8": func(args ...string) string {
			utf8Type := c.Query("utf8_type")
			if utf8Type != "" {
				return invalid.GetInvalidUTF8ByTypeName(utf8Type)
			}
			if len(args) > 0 && args[0] != "" {
				return invalid.GetInvalidUTF8ByTypeName(args[0])
			}
			return invalid.GenerateValidUTF8()
		},
		"jsonValue": func(path string) string {
			return jsonValue(path, data)
		},
		"jsonValueOr": func(path string, defaultValue string) string {
			v := jsonValue(path, data)
			if v == "" {
				return defaultValue
			}
			return v
		},
		"xmlValue": func(path string) string {
			return extractXmlValue(path, xmlBody)
		},
		"endToEndId": func() string {
			return endToEndId(data, cache)
		},
		"txId": func() string {
			return txId(data, cache)
		},
		"orgnlTxId": func() string {
			return orgnlTxId(data, cache)
		},
		"orgnlEndToEndId": func() string {
			return orgnlEndToEndId(data, cache)
		},
		"orgnlMsgId": func() string {
			return orgnlMsgId(data, cache)
		},
		"clrSysRef": func(bankCode string) string {
			return clrSysRef(data, cache, xmlBody, bankCode)
		},
		"requestBody": func() string {
			if b, ok := c.Get("requestBody"); ok {
				if s, ok := b.(string); ok {
					return s
				}
			}
			return ""
		},
	}
}

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
	if c.Writer.Status() == 0 {
		return 200
	}
	return c.Writer.Status()
}

// getActualResponseBody obtiene el response body real que se envió al cliente

func (h *Handler) getActualResponseBody(c *gin.Context, location models.Location) string {
	actualStatusCode := h.getActualStatusCode(c)

	if location.ChaosInjection != nil && actualStatusCode != location.StatusCode {
		if location.ChaosInjection.Error.Response != "" {
			return location.ChaosInjection.Error.Response
		}
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
