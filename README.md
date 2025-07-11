# Mock Harbor - HTTP Response Mocking Server

Mock Harbor is a flexible HTTP mock server that reads configuration files to simulate API responses. It allows you to mock multiple services simultaneously on different ports, making it ideal for testing applications that interact with various APIs.

## Features

- Mock multiple services simultaneously on different ports
- Configure response status codes, headers, and JSON bodies
- Match requests based on path, method, and request body
- Organize mock configurations by service and use case
- Configurable response delays to simulate network latency
- Hot reloading of configuration files without server restart

## Configuration Structure

The configuration follows this directory structure:

```
configs/
├── config.yaml              # Global configuration
├── serviceA/                # Service-specific directory
│   ├── config.yaml          # Service configuration (port)
│   └── usecases/
│       └── happypath/
│           └── all.json     # Request/response configurations
└── serviceB/
    ├── config.yaml
    └── usecases/
        ├── happypath/
        │   └── all.json
        └── error/
            └── all.json
```

### Global Configuration (config.yaml)

```yaml
services:
  - name: serviceA
    usecase: happypath
  - name: serviceB
    usecase: error
```

### Service Configuration (serviceA/config.yaml)

```yaml
port: 8081
name: serviceA

# Optional delay configuration
delay:
  enabled: true    # Whether to enable delay for this service
  fixed: 1000      # Fixed delay in milliseconds (1 second)
  # OR use random delay range
  # min: 500       # Minimum delay in milliseconds
  # max: 2000      # Maximum delay in milliseconds
```

### Mock Configurations (serviceA/usecases/happypath/all.json)

```json
[
  {
    "request": {
      "path": "/api/users",
      "method": "GET"
    },
    "response": {
      "body": {
        "users": [
          {"id": 1, "name": "John Doe"},
          {"id": 2, "name": "Jane Smith"}
        ]
      },
      "statusCode": 200,
      "headers": {
        "Content-Type": "application/json"
      }
    }
  }
]
```

## Usage

Start the server with:

```bash
go run cmd/server/main.go
```

Or specify a custom config directory:

```bash
go run cmd/server/main.go -config-dir /path/to/configs
```

### Command Line Flags

Mock Harbor supports the following command line flags:

```bash
-config-dir string    Directory containing configuration files (default "configs")
-no-hot-reload       Disable hot reloading of configuration files
-verbose             Enable verbose logging
```

#### Hot Reloading

By default, Mock Harbor watches for changes in your configuration files and automatically reloads them without requiring a server restart. This makes development and testing much faster.

If a service configuration (including delay settings) or mock response is changed while the server is running, it will be automatically detected and applied. If you need to disable this feature, use the `-no-hot-reload` flag.

## Example

1. Configure your mock responses in the config files
2. Start the Mock Harbor server
3. Send requests to the mock server endpoints
4. Receive the configured mock responses

For example, with the above configuration, sending a GET request to `http://localhost:8081/api/users` will return the configured JSON response with a 200 status code.

## Development

### Building from Source

```bash
go build -o mock-harbor cmd/server/main.go
```

### Running Tests

```bash
go test ./...
```
