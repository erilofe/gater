FROM golang:alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o gater ./cmd/gater

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/gater .
COPY config/ config/
EXPOSE 8080
CMD ["./gater"]
