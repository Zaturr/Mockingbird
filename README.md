# Mockingbird - Servidor con Estructura AST

Servidor mock HTTP con configuración basada en AST (Abstract Syntax Tree) y características de chaos engineering.

## 🏗️ **Nueva Estructura Implementada**

### **Archivos Principales:**
- **`network/handler/ast.go`** - Estructura AST completa
- **`network/handler/ast_handler.go`** - Handler que interpreta la configuración AST
- **`cmd/server/server/server.go`** - Servidor actualizado para usar AST

## 🚀 **Características de la Estructura AST**

### **1. Configuración de Servidores**
- Múltiples servidores en diferentes puertos
- Configuración de logging por servidor
- Chaos injection a nivel de servidor

### **2. Configuración de Locations (Endpoints)**
- Métodos HTTP (GET, POST, PUT, DELETE, PATCH)
- Headers de respuesta configurables
- Códigos de estado con probabilidades
- Cuerpos de respuesta personalizables

### **3. Chaos Engineering (Estructura)**
- **Latency Injection**: Inyección de latencia con probabilidad
- **Abort Injection**: Abortar requests con códigos específicos
- **Error Injection**: Inyectar errores HTTP

### **4. Requests Asíncronos**
- Configuración de URLs de callback
- Timeouts configurables
- Lógica de reintentos
- Headers personalizables

## 📝 **Configuración Actual**

La configuración de los servicios está implementada directamente en el código Go:

```go
// Ejemplo de configuración AST en Go
config := &handler.AST{
    Servers: []handler.Server{
        {
            Port: 8080,
            Locations: []handler.Location{
                {
                    Path: "/api/jsonplaceholder/posts",
                    Method: "POST",
                    Response: map[string]interface{}{"message": "Success"},
                    Headers: map[string]string{"Content-Type": "application/json"},
                    StatusCode: []handler.StatusCodeConfig{{Code: 201, Probability: 1.0}},
                },
            },
        },
    },
}
```

## ⚠️ **TODOs Pendientes**

### **Chaos Engineering (Funcionalidad)**
- Implementar inyección de latencia funcional
- Implementar inyección de abort funcional  
- Implementar inyección de error funcional
- Implementar manejo de requests asíncronos
- Implementar lógica de reintentos
- Implementar manejo de timeouts

### **Parser Externo**
- Integrar con el repositorio de GitHub del parser
- Reemplazar configuración hardcodeada por parsing dinámico
- **La configuración será leída desde archivos externos en el futuro**

## 🔧 **Cómo Usar**

1. **Compilar:**
   ```bash
   go build -o server.exe .
   ```

2. **Ejecutar:**
   ```bash
   ./server.exe
   ```

3. **Configurar puertos** en `env.example` o `.env`

## 📡 **Endpoints Disponibles**

Basados en la configuración AST actual:
- `POST /api/jsonplaceholder/posts` - Crear posts
- `POST /api/jsonplaceholder/users` - Crear usuarios
- `POST /api/jsonplaceholder/comments` - Crear comentarios
- `GET /api/jsonplaceholder/health` - Health check
- `GET /api/jsonplaceholder/chaos-test` - Endpoint con chaos engineering
- `POST /api/sypago/payments` - Procesar pagos
- `POST /api/sypago/transactions` - Crear transacciones
- `GET /api/sypago/health` - Health check de Sypago

## 🎯 **Próximos Pasos**

1. **Integrar parser externo** para configuración dinámica
2. **Reemplazar configuración hardcodeada** por parsing de archivos externos
3. **Implementar funcionalidad** de chaos engineering
4. **Agregar más endpoints** según configuración AST
5. **Implementar logging y monitoreo**

## 📋 **Notas Importantes**

- **La configuración actual** está hardcodeada en el servidor Go
- **El parser externo** permitirá leer configuración desde archivos
- **Por ahora** los endpoints están configurados directamente en el código Go
- **La estructura AST** está lista para recibir configuración externa
