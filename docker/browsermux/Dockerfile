FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY ./docker/browsermux/go.mod ./docker/browsermux/go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY ./docker/browsermux/cmd/ ./cmd/
COPY ./docker/browsermux/internal/ ./internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /browsermux ./cmd/browsermux/main.go

# Create a minimal runtime image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /browsermux .

# Set environment variables
ENV PORT=8080
ENV BROWSER_URL=ws://browser:9222/devtools/browser
ENV MAX_MESSAGE_SIZE=1048576
ENV CONNECTION_TIMEOUT_SECONDS=10

# Expose the service port
EXPOSE 8080

# Run the application
CMD ["./browsermux"]
