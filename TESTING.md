# Testing del Sistema Mockingbird

## 🚀 Iniciar el Servidor

```bash
# Compilar el proyecto
go build

# Ejecutar el servidor
go run main.go
```

## 📋 Servicios Disponibles

### 1. JSONPlaceholder (Puerto 8080)
- **GET** `/health` - Health check
- **GET** `/api/posts/:id` - Obtener post por ID
- **GET** `/api/GET/4` - Chaos test endpoint
- **POST** `/api/POST/0` - Crear post
- **POST** `/api/POST/1` - Crear usuario
- **POST** `/api/POST/2` - Crear comentario
- **PUT** `/api/PUT/0` - Actualizar post
- **DELETE** `/api/DELETE/0` - Eliminar post

### 2. Sypago (Puerto 8081)
- **GET** `/health` - Health check
- **POST** `/api/POST/0` - Procesar pago
- **POST** `/api/POST/1` - Crear transacción
- **POST** `/api/v1/transaction/otp` - Enviar OTP para transacción
- **PUT** `/api/PUT/0` - Actualizar pago
- **DELETE** `/api/DELETE/0` - Cancelar transacción

### 3. Users (Puerto 8082) - Nuevo Servicio
- **GET** `/health` - Health check
- **GET** `/api/GET/0` - Obtener perfil de usuario
- **POST** `/api/POST/0` - Registrar usuario
- **PUT** `/api/PUT/0` - Actualizar perfil
- **DELETE** `/api/DELETE/0` - Eliminar cuenta

## 🧪 Probar Endpoints

### Con curl:

```bash
# Health check de JSONPlaceholder
curl http://localhost:8080/health

# Crear un post
curl -X POST http://localhost:8080/api/POST/0

# Obtener perfil de usuario
curl http://localhost:8082/api/GET/0

# Procesar pago
curl -X POST http://localhost:8081/api/POST/0
```

### Con Postman:

1. **JSONPlaceholder Service**
   - Base URL: `http://localhost:8080`
   - Endpoints: Ver lista arriba

2. **Sypago Service**
   - Base URL: `http://localhost:8081`
   - Endpoints: Ver lista arriba

3. **Users Service**
   - Base URL: `http://localhost:8082`
   - Endpoints: Ver lista arriba

## 🎯 Testing de Chaos Engineering

El endpoint de chaos test en JSONPlaceholder incluye:
- **Latencia**: 100ms en 30% de requests
- **Abort**: Error 503 en 10% de requests
- **Error**: Error 500 en 5% de requests

```bash
# Probar chaos test (puede fallar aleatoriamente)
curl http://localhost:8080/api/GET/4
```

## 🔧 Agregar Nuevos Endpoints

### 1. Editar `infraestructura/config/endpoints.go`
### 2. Agregar nuevo endpoint en la función correspondiente
### 3. Reiniciar el servidor

Ejemplo de nuevo endpoint:

```go
{
    Method:     "GET",
    Response:   map[string]interface{}{"message": "Nuevo endpoint", "service": "jsonplaceholder"},
    Headers:    &handler.Headers{"Content-Type": "application/json"},
    StatusCode: "200",
},
```

## 📊 Monitoreo

### Logs del Servidor:
```bash
# Ver logs en tiempo real
go run main.go 2>&1 | tee server.log
```

### Verificar Puertos:
```bash
# Verificar que los servicios estén corriendo
netstat -an | grep :808
# o en Windows:
netstat -an | findstr :808
```

## 🚨 Troubleshooting

### Puerto ya en uso:
```bash
# En Windows
netstat -ano | findstr :8080
taskkill /PID <PID> /F

# En Linux/Mac
lsof -i :8080
kill -9 <PID>
```

### Error de compilación:
```bash
# Limpiar y reinstalar dependencias
go clean -modcache
go mod tidy
go build
```

## 📝 Notas Importantes

- Todos los servicios se inician automáticamente
- Los cambios en la configuración requieren reiniciar el servidor
- Cada servicio tiene su propio puerto y configuración
- El sistema es completamente centralizado en `infraestructura/config/endpoints.go`
