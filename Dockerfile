# Build stage
# Runs on the build host's own platform and cross-compiles for the target, so a
# multi-platform release does not need QEMU for the Go build and, more
# importantly, the arm64 image gets an arm64 binary. GOOS/GOARCH come from
# BuildKit's TARGETOS/TARGETARCH, which default to the host when building
# without --platform.
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum* ./

# Download dependencies (if go.sum exists)
RUN go mod download || true

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/

# Build-time variables with defaults for local builds (overridden by CI/CD)
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

# Target platform. BuildKit sets TARGETOS/TARGETARCH per platform in a
# multi-platform build (linux/amd64, linux/arm64). A builder that does not
# (the legacy builder) leaves them empty, and the build then targets the
# machine it runs on rather than silently defaulting to amd64.
ARG TARGETOS
ARG TARGETARCH

# Build static binary for the target platform
RUN set -eu; \
  arch="${TARGETARCH:-$(uname -m)}"; \
  case "$arch" in x86_64) arch=amd64 ;; aarch64) arch=arm64 ;; esac; \
  echo "building healthserver for ${TARGETOS:-linux}/${arch}"; \
  CGO_ENABLED=0 GOOS="${TARGETOS:-linux}" GOARCH="$arch" go build \
  -ldflags="-w -s -X github.com/eslutz/torarr/pkg/version.Version=${VERSION} -X github.com/eslutz/torarr/pkg/version.Commit=${COMMIT} -X github.com/eslutz/torarr/pkg/version.Date=${DATE}" \
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
RUN (id -u tor || adduser -D -H -u 1000 tor) && \
  mkdir -p /var/lib/tor /etc/tor && \
  chown -R tor:tor /var/lib/tor /etc/tor

# Copy binary from builder
COPY --from=builder /build/healthserver /usr/local/bin/healthserver
RUN chmod +x /usr/local/bin/healthserver

# Copy configuration files
COPY torrc /etc/tor/torrc
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set default environment variables
ENV TZ=UTC \
  HEALTH_PORT=9091 \
  HEALTH_EXTERNAL_TIMEOUT=15 \
  LOG_LEVEL=INFO

# Expose ports
EXPOSE 9050 9091

# Set working directory
WORKDIR /var/lib/tor

# Use tor user
USER tor

# Health check (shell form so HEALTH_PORT is honoured)
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD wget -qO- --timeout=5 "http://localhost:${HEALTH_PORT:-9091}/health" || exit 1

# Run entrypoint
ENTRYPOINT ["/entrypoint.sh"]
