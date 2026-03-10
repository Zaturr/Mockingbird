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

// --- VARIABLES GLOBALES ---

var (
	randomCache      = make(map[string]map[string]interface{})
	randomCacheMutex sync.RWMutex
)

// --- ESTRUCTURA PRINCIPAL ---

type Handler struct {
	chaosEngine  *chaos.Engine
	schemas      map[string]*jsonschema.Schema
	Logger       *scribe.Scribe
	BatchManager *database.BatchManager
}

func NewHandler(logger *scribe.Scribe, batchManager *database.BatchManager) *Handler {
	return &Handler{
		chaosEngine:  chaos.NewEngine(),
		schemas:      make(map[string]*jsonschema.Schema),
		Logger:       logger,
		BatchManager: batchManager,
	}
}

// --- MÉTODOS DE REGISTRO Y VALIDACIÓN ---

func (h *Handler) RegisterLocation(location models.Location) error {
	h.Logger.Info().Str("path", location.Path).Str("method", location.Method).Msg("Registering location")
	if location.Schema != "" {
		schema, err := h.compileSchema(location.Schema)
		if err != nil {
			return fmt.Errorf("error compiling schema for path %s: %w", location.Path, err)
		}
		h.schemas[location.Path+":"+location.Method] = schema
	}
	return nil
}

func (h *Handler) compileSchema(schemaStr string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	var schemaData interface{}
	if err := json.Unmarshal([]byte(schemaStr), &schemaData); err != nil {
		return nil, err
	}
	if err := compiler.AddResource("schema.json", schemaData); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}

// --- CORE: MANEJO DE REQUESTS ---

func (h *Handler) HandleRequest(c *gin.Context, location models.Location) {
	start := time.Now()
	requestPath := location.Path
	requestMethod := c.Request.Method

	prom.HandlerActiveRequests.WithLabelValues(requestMethod, requestPath).Inc()
	defer prom.HandlerActiveRequests.WithLabelValues(requestMethod, requestPath).Dec()

	ctx := scribe.WithCtx(c.Request.Context())
	h.Logger.DebugCtx(ctx).Str("method", requestMethod).Str("path", requestPath).Msg("Handling request")

	// 1. Chaos Injection
	if location.ChaosInjection != nil && h.chaosEngine.ApplyChaos(c.Writer, location.ChaosInjection) {
		h.insertTransactionToDB(c, location)
		h.logMetrics(requestPath, requestMethod, c.Writer.Status(), start, "chaos_aborted")
		return
	}

	// 2. Schema Validation
	if schema, ok := h.schemas[requestPath+":"+requestMethod]; ok {
		if err := h.validateRequestBody(c, schema); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			h.insertTransactionToDB(c, location)
			h.logMetrics(requestPath, requestMethod, 400, start, "schema_validation_failed")
			return
		}
	}

	// 3. Headers & Async Calls
	if location.Headers != nil {
		for k, v := range *location.Headers {
			c.Header(k, v)
		}
	}

	if location.Async != nil {
		for _, v := range location.Async {
			go h.handleAsyncCall(&v, c)
			prom.HandlerAsyncCallsTotal.WithLabelValues(requestPath, requestMethod, v.Url).Inc()
		}
	}

	// 4. Response Body & Template Processing
	c.Status(location.StatusCode)
	if location.Response != "" {
		if location.Headers == nil || (*location.Headers)["Content-Type"] == "" {
			c.Header("Content-Type", "application/json")
		}

		responseBody, err := h.processResponseTemplate(c, string(location.Response))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Template error"})
			h.insertTransactionToDB(c, location)
			h.logMetrics(requestPath, requestMethod, 500, start, "template_error")
			return
		}
		c.String(location.StatusCode, responseBody)
	}

	h.insertTransactionToDB(c, location)
	h.logMetrics(requestPath, requestMethod, location.StatusCode, start, "")
}

// --- PROCESAMIENTO DE TEMPLATES Y DATOS ---

func (h *Handler) processResponseTemplate(c *gin.Context, responseTemplate string) (string, error) {
	if !strings.Contains(responseTemplate, "{{") {
		return responseTemplate, nil
	}

	// Extracción limpia de datos y caché
	data, xmlBody, txID := h.AnalistData(c)
	cache := h.getOrCreateCache(txID)

	// Inyectar en contexto para uso en otras funciones si es necesario
	c.Set("transactionID", txID)
	c.Set("randomValues", cache)

	funcMap := h.createFuncMap(c, data, cache, xmlBody)

	tmpl, err := template.New("response").Funcs(funcMap).Parse(responseTemplate)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err := tmpl.Execute(&tpl, data); err != nil {
		return "", err
	}

	return tpl.String(), nil
}

// AnalistData prepara el entorno de datos del request
func (h *Handler) AnalistData(c *gin.Context) (map[string]interface{}, string, string) {
	data := make(map[string]interface{})
	var xmlStr string

	if c.Request.Body != nil {
		body, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		if len(body) > 0 {
			if err := json.Unmarshal(body, &data); err != nil {
				xmlStr = string(body)
			}
		}
	}

	// Query params
	q := make(map[string]string)
	for k, v := range c.Request.URL.Query() {
		if len(v) > 0 {
			q[k] = v[0]
		}
	}
	data["Query"] = q

	// Transaction ID
	txID := c.GetHeader("X-Transaction-ID")
	if txID == "" {
		txID = uuid.New().String()
		c.Header("X-Transaction-ID", txID)
	}

	return data, xmlStr, txID
}

// --- LIBRERÍA DE FUNCIONES PARA TEMPLATES ---

func (h *Handler) createFuncMap(c *gin.Context, data map[string]interface{}, cache map[string]interface{}, xml string) template.FuncMap {
	return template.FuncMap{
		"now":    func() time.Time { return time.Now() },
		"uuid":   func() string { return uuid.New().String() },
		"printf": fmt.Sprintf,
		"query":  func(key string) string { return c.Query(key) },

		// Aleatorios con Caché
		"randInt": func(min, max int) int {
			key := fmt.Sprintf("int_%d_%d", min, max)
			return getCached(cache, key, func() interface{} {
				return rand.Intn(max-min) + min
			}).(int)
		},
		"randName": func() string {
			return getCached(cache, "name", func() interface{} {
				names := []string{"John Anderson", "Maria Garcia", "Rebecca Schmitt"}
				return names[rand.Intn(len(names))]
			}).(string)
		},
		"randVenezuelanID": func() string {
			return getCached(cache, "v_id", func() interface{} {
				return fmt.Sprintf("V%d", rand.Intn(99999999))
			}).(string)
		},

		// Extracción Lógica
		"jsonValue": func(path string) string {
			return ExtractJsonValue(path, data)
		},
		"xmlValue": func(path string) string {
			return h.extractXmlValue(path, xml)
		},

		// Identificadores de Negocio
		"txId": func() string {
			return getCached(cache, "txId", func() interface{} {
				bank := ExtractJsonValue("PmtInf.0.DbtrAgt", data)
				if bank == "" {
					bank = "0001"
				}
				return fmt.Sprintf("%s%s%012d", bank[:4], time.Now().Format("20060102150405"), rand.Int63n(1e12))
			}).(string)
		},
		"endToEndId": func() string {
			return getCached(cache, "e2eId", func() interface{} {
				val := ExtractJsonValue("PmtInf.0.EndToEndId", data)
				if val != "" {
					return val
				}
				return fmt.Sprintf("E2E%s%d", time.Now().Format("150405"), rand.Intn(9999))
			}).(string)
		},
		"invalidUTF8": func(args ...string) string {
			t := c.DefaultQuery("utf8_type", "random")
			return invalid.GetInvalidUTF8ByTypeName(t)
		},
	}
}

// --- HELPERS DE SOPORTE ---

func ExtractJsonValue(path string, data map[string]interface{}) string {
	var current interface{} = data
	// Salto automático a nodo raíz común
	if root, ok := data["CstmrCdtTrfInitn"].(map[string]interface{}); ok {
		current = root
	}

	for _, part := range strings.Split(path, ".") {
		switch v := current.(type) {
		case map[string]interface{}:
			current = v[part]
		case []interface{}:
			idx, _ := strconv.Atoi(part)
			if idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return ""
			}
		default:
			return ""
		}
	}
	return fmt.Sprintf("%v", current)
}

func getCached(cache map[string]interface{}, key string, generator func() interface{}) interface{} {
	if val, exists := cache[key]; exists {
		return val
	}
	newVal := generator()
	cache[key] = newVal
	return newVal
}

func (h *Handler) getOrCreateCache(id string) map[string]interface{} {
	randomCacheMutex.Lock()
	defer randomCacheMutex.Unlock()
	if _, exists := randomCache[id]; !exists {
		randomCache[id] = make(map[string]interface{})
		go func(txID string) {
			time.Sleep(5 * time.Minute)
			randomCacheMutex.Lock()
			delete(randomCache, txID)
			randomCacheMutex.Unlock()
		}(id)
	}
	return randomCache[id]
}

func (h *Handler) logMetrics(path, method string, status int, start time.Time, errType string) {
	s := strconv.Itoa(status)
	prom.HandlerResquestTotal.WithLabelValues(path, method, s).Inc()
	prom.HandlerRequestDuration.WithLabelValues(path, method, s).Observe(time.Since(start).Seconds())
	if errType != "" {
		prom.HandlerErrorsTotal.WithLabelValues(path, method, errType).Inc()
	}
}

// --- FUNCIONES ORIGINALES RESTANTES (Stubs/Referencia) ---

func (h *Handler) validateRequestBody(c *gin.Context, schema *jsonschema.Schema) error {
	body, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	var data interface{}
	json.Unmarshal(body, &data)
	return schema.Validate(data)
}

func (h *Handler) insertTransactionToDB(c *gin.Context, location models.Location) {
	if h.BatchManager == nil {
		h.Logger.Warn().Msg("BatchManager is nil, skipping database insertion")
		return
	}

	if !h.BatchManager.IsRunning() {
		h.Logger.Warn().Msg("BatchManager is not running, skipping database insertion")
		return
	}

	requestHeaders, _ := json.Marshal(c.Request.Header)
	requestBody := h.getRequestBody(c)
	responseHeaders, _ := json.Marshal(c.Writer.Header())
	responseBody := h.getActualResponseBody(c, location)
	actualStatusCode := h.getActualStatusCode(c)

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

func (h *Handler) getRequestBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.Logger.Error().AnErr("error", err).Msg("Error reading request body for database")
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
	return string(body)
}

func (h *Handler) getActualStatusCode(c *gin.Context) int {
	if c.Writer.Status() == 0 {
		return 200
	}
	return c.Writer.Status()
}

func (h *Handler) getActualResponseBody(c *gin.Context, location models.Location) string {
	actualStatusCode := h.getActualStatusCode(c)
	if location.ChaosInjection != nil && actualStatusCode != location.StatusCode {
		if location.ChaosInjection.Error.Response != "" {
			return location.ChaosInjection.Error.Response
		}
		return ""
	}
	if location.Response != "" {
		responseBody, err := h.processResponseTemplate(c, string(location.Response))
		if err != nil {
			return string(location.Response)
		}
		return responseBody
	}
	return ""
}

func (h *Handler) handleAsyncCall(async *models.Async, c *gin.Context) {
	// Implementación de Async existente simplificada para usar el nuevo procesador
	client := &http.Client{Timeout: 5 * time.Second}
	processedBody, _ := h.processResponseTemplate(c, async.Body)
	req, _ := http.NewRequest(async.Method, async.Url, strings.NewReader(processedBody))
	client.Do(req)
}

func (h *Handler) extractXmlValue(path string, xml string) string {
	if xml == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	tagName := parts[len(parts)-1]

	pattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([^<]*)</%s>`, regexp.QuoteMeta(tagName), regexp.QuoteMeta(tagName))
	re := regexp.MustCompile(pattern)

	xmlToSearch := xml
	if len(parts) > 1 {
		parentTag := parts[len(parts)-2]
		parentPattern := fmt.Sprintf(`<%s(?:\s[^>]*)?>([\s\S]*?)</%s>`, regexp.QuoteMeta(parentTag), regexp.QuoteMeta(parentTag))
		parentRe := regexp.MustCompile(parentPattern)
		matches := parentRe.FindStringSubmatch(xml)
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
