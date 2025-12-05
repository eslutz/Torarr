# Build stage
FROM golang:1.25.5-alpine3.23 AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies (if go.sum exists)
RUN go mod download || true

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o healthserver \
  ./cmd/healthserver

# Runtime stage
FROM alpine:3.23

# Install Tor and ca-certificates
RUN apk add --no-cache \
  tor \
  ca-certificates \
  tzdata

# Create tor user and directories
RUN adduser -D -H -u 1000 tor && \
  mkdir -p /var/lib/tor /etc/tor && \
  chown -R tor:tor /var/lib/tor /etc/tor

# Copy binary from builder
COPY --from=builder /build/healthserver /usr/local/bin/healthserver
RUN chmod +x /usr/local/bin/healthserver

# Copy configuration files
COPY torrc /etc/tor/torrc
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set default environment variables
ENV TZ=UTC \
  HEALTH_PORT=8085 \
  HEALTH_FULL_TIMEOUT=15 \
  LOG_LEVEL=INFO

# Expose ports
EXPOSE 9050 8085

# Set working directory
WORKDIR /var/lib/tor

# Use tor user
USER tor

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD wget -qO- --timeout=5 http://localhost:8085/health || exit 1

# Run entrypoint
ENTRYPOINT ["/entrypoint.sh"]
