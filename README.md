# Gater

An API Gateway written in Go.

## Project Structure

- **`cmd/gater`**: Application entry point (`main.go`).
- **`internal/app`**: Private application business logic.
- **`pkg/utils`**: Public utility libraries.

## Requirements

- Go 1.25.1 or higher.
- Docker & Docker Compose (for containerized development)

## Development Setup (Git Hooks)

This project uses [pre-commit](https://pre-commit.com/) to manage git hooks for code quality (linting, formatting, tests).

1. Install `pre-commit`:
   ```bash
   pip install pre-commit
   ```
2. Install the hooks:
   ```bash
   pre-commit install
   ```

Now, `golangci-lint`, `go fmt`, and tests will run automatically on every commit.

## How to Run

### Local

```bash
go run cmd/gater/main.go
```

### Docker (Production / Stable)

To start the application in a production-like environment (optimized build, no file watching):

```bash
docker compose up --build -d
```

The gateway will be available at `http://localhost:8080` and the Consul UI at `http://localhost:8500`. Persistent data lives in `consul_data/`; delete its contents between runs if you need a clean cluster snapshot.

### Development (Hot Reloading)

To start the application with **Air** for live reloading:

- Start the dev stack (foreground build + hot reload):
    ```bash
    docker compose -f docker-compose.dev.yml up --build
    ```
- Edit files under `cmd/` or `internal/`; Air rebuilds and reloads automatically.

The development compose file mounts `consul_config/` and `consul_data/`, so updates to `consul_config/services.json` are picked up automatically after a Consul restart.

### Verifying Connection

You can verify the connection to the mock services:

- **User Service**: `curl http://localhost:8080/users/test` -> Should return `{"service":"user", "status":"ok"}`

- **Post Service**: `curl http://localhost:8080/posts/test` -> Should return `{"service":"post", "status":"ok"}`

To inspect the Consul catalog directly:

```bash
docker compose -f docker-compose.dev.yml exec consul consul catalog services
```

If you change `consul_config/services.json`, reload the service definitions with:

```bash
docker compose -f docker-compose.dev.yml restart consul
```

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

**Consul (ACL bootstrap & access)**

1. Generate a management token inside the running agent:
   ```bash
   docker compose -f docker-compose.dev.yml exec -it consul consul acl bootstrap
   ```
2. Copy the `SecretID` from the output and store it securely.
3. Authenticate with the token:
   - **CLI**: `docker compose -f docker-compose.dev.yml exec consul consul login -token <SecretID>`
   - **UI**: open `http://localhost:8500`, click **Log In**, and paste the `SecretID`.

Example bootstrap output:

```
AccessorID:       xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
SecretID:         xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
Description:      Bootstrap Token (Global Management)
Local:            false
Create Time:      2026-01-04 19:18:15.825222154 +0000 UTC
Policies:
   00000000-0000-0000-0000-000000000001 - global-management
```

Treat the `SecretID` as sensitive; rotate it if it is ever exposed.
