# Gater

An API Gateway written in Go.

## Project Structure

- **`cmd/gater`**: Application entry point (`main.go`).
- **`internal/app`**: Private application business logic.
- **`pkg/utils`**: Public utility libraries.

## Requirements

- Go 1.25.1 or higher.

## How to Run

```bash
go run cmd/gater/main.go
```

## How to Build

```bash
go build -o bin/gater.exe cmd/gater/main.go
```