FROM golang:1.23-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY browsergrid/go.mod browsergrid/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY browsergrid/ ./

# Build the worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests and netcat for database waiting
RUN apk --no-cache add ca-certificates netcat-openbsd

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/worker .

# Copy entrypoint script
COPY docker/worker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose any ports if needed (none required for worker)

# Run the worker
ENTRYPOINT ["/entrypoint.sh"] 