# Build stage
FROM golang:1.25.6-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG BUILD_TIME=unknown

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files and download dependencies (cached layer)
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build binary with version information and optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}'" \
    -a -installsuffix cgo \
    -o gater ./cmd/gater

# Runtime stage
FROM alpine:3.21

# OCI labels for metadata
LABEL org.opencontainers.image.title="Gater"
LABEL org.opencontainers.image.description="API Gateway written in Go with service discovery"
LABEL org.opencontainers.image.authors="Pietro Agazzi <pietro@example.com>"
LABEL org.opencontainers.image.source="https://github.com/pietroagazzi/gater"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.version="${VERSION}"

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 gater && \
    adduser -D -u 1000 -G gater gater

# Set working directory
WORKDIR /home/gater

# Copy binary and config from builder
COPY --from=builder --chown=gater:gater /app/gater .
COPY --from=builder --chown=gater:gater /app/config/ ./config/

# Switch to non-root user
USER gater

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./gater"]
