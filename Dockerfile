# RequestBite Slingshot Proxy - Dockerfile
# Multi-stage build for minimal production image

# Stage 1: Build
FROM golang:1.21-alpine AS builder

# Accept build arguments for versioning
ARG VERSION
ARG BUILD_TIME
ARG GIT_COMMIT

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build using Go directly with version info (bypassing Makefile's git commands)
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w \
    -X 'main.Version=${VERSION:-dev}' \
    -X 'main.BuildTime=${BUILD_TIME:-unknown}' \
    -X 'main.GitCommit=${GIT_COMMIT:-unknown}'" \
    -trimpath \
    -o build/rb-proxy \
    ./cmd/requestbite-proxy

# Stage 2: Runtime
FROM alpine:latest

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user for security
RUN addgroup -g 1000 proxy && \
  adduser -D -u 1000 -G proxy proxy

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /build/build/rb-proxy .

# Copy LICENSE and README if they exist
COPY --from=builder /build/LICENSE* /build/README* ./

# Change ownership to non-root user
RUN chown -R proxy:proxy /app

# Switch to non-root user
USER proxy

# Expose default port
EXPOSE 7331

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:7331/health || exit 1

# Run with default options
ENTRYPOINT ["/app/rb-proxy"]

# Default command (can be overridden)
CMD []
