# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=0 for static binary (better for Alpine)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o homeautomation \
    ./cmd/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user for security
RUN addgroup -g 1000 homeautomation && \
    adduser -D -u 1000 -G homeautomation homeautomation

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/homeautomation .

# Copy example env file for reference
COPY --from=builder /build/.env.example .

# Change ownership
RUN chown -R homeautomation:homeautomation /app

# Switch to non-root user
USER homeautomation

# The application doesn't expose ports by default (it's a WebSocket client)
# But we could add health check or metrics endpoints in the future
# EXPOSE 8080

# Run the application
CMD ["./homeautomation"]
