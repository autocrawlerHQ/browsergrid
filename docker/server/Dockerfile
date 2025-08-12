FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY browsergrid/go.mod browsergrid/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY browsergrid/ ./

# Build the API server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api ./cmd/api

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests, netcat for database waiting, and Atlas CLI
RUN apk --no-cache add ca-certificates netcat-openbsd curl && \
    curl -sSf https://atlasgo.sh | sh -s -- -b /usr/local/bin

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/api .

# Copy migrations directory
COPY browsergrid/migrations /migrations

# Copy entrypoint script
COPY docker/server/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose the API port
EXPOSE 8765

# Run the API server
ENTRYPOINT ["/entrypoint.sh"] 