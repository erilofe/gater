# Gater

An API Gateway written in Go.

## Project Structure

- **`cmd/gater`**: Application entry point (`main.go`).
- **`internal/app`**: Private application business logic.
- **`pkg/utils`**: Public utility libraries.

## Requirements

- Go 1.25.1 or higher.
- Docker & Docker Compose (for containerized development)

## How to Run

### Local

```bash
go run cmd/gater/main.go
```

### Docker (Production / Stable)

To start the application in a production-like environment (optimized build, no file watching):

```bash
docker compose up --build
```

The application will be available at `http://localhost:8080`.

### Development (Hot Reloading)

To start the application with **Air** for live reloading:

```bash
docker compose -f docker-compose.dev.yml up --build
```

When running in development mode:

1. Keep the logs open (`docker compose -f docker-compose.dev.yml logs -f gater`).
2. Make changes to the code.
3. Air will automatically rebuild and restart the application.

### Verifying Connection

You can verify the connection to the mock services:

- **User Service**: `curl http://localhost:8080/users/test` -> Should return `{"service":"user", "status":"ok"}`
- **Post Service**: `curl http://localhost:8080/posts/test` -> Should return `{"service":"post", "status":"ok"}`

## How to Build

```bash
go build -o bin/gater.exe cmd/gater/main.go
```

## Testing

The project includes both unit tests and integration tests.

**Unit Tests (Local):**

You can run the unit tests locally (which use mock servers) using the standard Go command:

```bash
go test ./...
```

**Integration Tests (Docker):**

To run the full suite, including integration tests that require dependent services:

```bash
docker compose -f docker-compose.test.yml -p gater-test up --build --abort-on-container-exit && \
docker compose -f docker-compose.test.yml -p gater-test down
```
