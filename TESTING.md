# 🧪 **Guía de Pruebas - Servidor AST**

## 🚀 **Configuración Actual**

### **Puertos de Servicios:**
- **Jsonplaceholder**: Puerto 8080
- **Sypago**: Puerto 8081

## 📡 **Endpoints Disponibles para Pruebas**

### **1. Jsonplaceholder (Puerto 8080)**

#### **POST Endpoints:**
```bash
# Crear un post
curl -X POST http://localhost:8080/api/jsonplaceholder/posts \
  -H "Content-Type: application/json" \
  -d '{"title": "Test Post", "body": "Test content"}'

# Crear un usuario
curl -X POST http://localhost:8080/api/jsonplaceholder/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Test User", "email": "test@example.com"}'

# Crear un comentario
curl -X POST http://localhost:8080/api/jsonplaceholder/comments \
  -H "Content-Type: application/json" \
  -d '{"postId": 1, "body": "Test comment"}'
```

#### **GET Endpoints:**
```bash
# Health check
curl http://localhost:8080/api/jsonplaceholder/health

# Obtener post por ID
curl http://localhost:8080/api/jsonplaceholder/posts/1

# Endpoint de prueba de chaos engineering
curl http://localhost:8080/api/jsonplaceholder/chaos-test
```

### **2. Sypago (Puerto 8081)**

#### **POST Endpoints:**
```bash
# Procesar pago
curl -X POST http://localhost:8081/api/sypago/payments \
  -H "Content-Type: application/json" \
  -d '{"amount": 100.00, "currency": "USD"}'

# Crear transacción
curl -X POST http://localhost:8081/api/sypago/transactions \
  -H "Content-Type: application/json" \
  -d '{"paymentId": 1, "amount": 100.00}'
```

#### **GET Endpoints:**
```bash
# Health check
curl http://localhost:8081/api/sypago/health
```

## 🔍 **Características a Verificar**

### **1. Respuestas Personalizadas por Servicio**
- Cada servicio responde con su nombre en la respuesta
- Headers personalizados configurados
- Códigos de estado según configuración

### **2. Chaos Engineering (Estructura)**
- Endpoint `/chaos-test` tiene configuración de chaos
- 30% probabilidad de latencia (100ms - 1000ms)
- 10% probabilidad de abort con código 503
- 80% probabilidad de respuesta 200, 20% de 500

### **3. Múltiples Servidores Independientes**
- Jsonplaceholder en puerto 8080
- Sypago en puerto 8081
- Cada uno con su propia configuración AST

## 🧪 **Pasos para Probar**

### **1. Iniciar el Servidor**
```bash
./server.exe
```

### **2. Verificar que Ambos Servicios Estén Funcionando**
```bash
# Verificar Jsonplaceholder
curl http://localhost:8080/api/jsonplaceholder/health

# Verificar Sypago
curl http://localhost:8081/api/sypago/health
```

### **3. Probar Endpoints POST**
```bash
# Probar creación en Jsonplaceholder
curl -X POST http://localhost:8080/api/jsonplaceholder/posts \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'
```

### **4. Probar Endpoints GET**
```bash
# Probar health checks
curl http://localhost:8080/api/jsonplaceholder/health
curl http://localhost:8081/api/sypago/health
```

## 📊 **Respuestas Esperadas**

### **Jsonplaceholder Health:**
```json
{
  "status": "Jsonplaceholder service is healthy",
  "port": 8080
}
```

### **Sypago Health:**
```json
{
  "status": "Sypago service is healthy",
  "port": 8081
}
```

### **Posts Creados:**
```json
{
  "message": "Post created successfully",
  "service": "jsonplaceholder"
}
```

## ⚠️ **Notas Importantes**

1. **Chaos Engineering**: Por ahora solo está la estructura, no la funcionalidad
2. **Puertos**: Asegúrate de que los puertos 8080 y 8081 estén disponibles
3. **Variables de Entorno**: Puedes modificar los puertos en el archivo `.env`
4. **Logs**: El servidor mostrará en qué puerto se inicia cada servicio

## 🔧 **Solución de Problemas**

### **Puerto en Uso:**
```bash
# Cambiar puerto en .env
JSONPLACEHOLDER_PORT=9090
SYPAGO_PORT=9091
```

### **Verificar Servicios:**
```bash
# Ver qué puertos están en uso
netstat -an | findstr :8080
netstat -an | findstr :8081
```
