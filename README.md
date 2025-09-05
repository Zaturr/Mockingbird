# Catalyst

Catalyst is a dynamic mock server system that allows you to configure multiple HTTP servers with custom endpoints, responses, and behaviors.

## Features

- **Dynamic Server Creation**: Create multiple HTTP servers with different ports from a single YAML configuration.
- **Flexible Routing**: Define custom endpoints with specific HTTP methods, responses, and status codes.
- **JSON Schema Validation**: Validate incoming requests against JSON schemas.
- **Chaos Injection**: Simulate network issues with configurable latency, aborts, and errors.
- **Async Callbacks**: Configure endpoints to make asynchronous HTTP calls to other services.
- **Custom Headers**: Set custom response headers for each endpoint.

## Installation

```bash
go get github.com/yourusername/catalyst
```

## Usage

### Configuration

Create a YAML configuration file that defines your servers and endpoints:

```yaml
http:
  servers:
    - listen: 8080
      logger: true
      location:
        - path: /api/hello
          method: GET
          response: '{"message": "Hello, World!"}'
          status_code: 200
          headers:
            Content-Type: application/json
        - path: /api/echo
          method: POST
          schema: |
            {
              "type": "object",
              "properties": {
                "message": { "type": "string" }
              },
              "required": ["message"]
            }
          response: '{"echo": "{{.message}}"}'
          status_code: 200
          headers:
            Content-Type: application/json
```

### Running the Server

```bash
catalyst -file config.yaml
```

Or load all configuration files from a directory:

```bash
catalyst -config ./configs
```

## Configuration Reference

### Server Configuration

| Field | Type | Description |
|-------|------|-------------|
| listen | int | The port to listen on |
| logger | bool | Enable/disable request logging |
| chaos_injection | object | Configuration for chaos injection |
| location | array | Array of endpoint configurations |

### Location Configuration

| Field | Type | Description |
|-------|------|-------------|
| path | string | The endpoint path |
| method | string | The HTTP method (GET, POST, etc.) |
| schema | string | JSON schema for request validation |
| response | string | The response body |
| async | object | Configuration for async callbacks |
| headers | object | Response headers |
| status_code | int | The HTTP status code to return |
| chaos_injection | object | Configuration for chaos injection |

### Chaos Injection Configuration

| Field | Type | Description |
|-------|------|-------------|
| latency | object | Configuration for response latency |
| abort | object | Configuration for request abortion |
| error | object | Configuration for error responses |

## Project Structure

- `cmd/catalyst`: Main application entry point
- `internal/models`: Data models for configuration
- `internal/config`: Configuration loading and validation
- `internal/server`: Server creation and management
- `internal/handler`: Request handling and routing
- `internal/chaos`: Chaos injection implementation

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o catalyst ./cmd/catalyst
```

## License

MIT