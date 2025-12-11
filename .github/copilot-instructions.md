# Torarr - Copilot Instructions

## Project Overview

Torarr is a lightweight Tor proxy container with comprehensive health monitoring, designed as a sidecar for the \*arr stack (Sonarr, Radarr, etc.). Built with Go for minimal footprint (~25MB) and fast startup times.

## Architecture

- **Language**: Go 1.25 (zero external dependencies for HTTP routing)
- **Container**: Alpine 3.23-based (~25MB total)
- **Health Server**: HTTP endpoints on port 8085
- **Tor Control**: Direct socket communication on port 9051
- **SOCKS Proxy**: Port 9050
- **Dependencies**: Only Prometheus client for metrics

## Code Style Guidelines

- Use stdlib `net/http` - no external routing libraries
- Keep dependencies minimal (only Prometheus for metrics)
- Prefer direct socket communication over libraries
- Error handling: explicit returns, no panics in production code
- Configuration: environment variables with sensible defaults
- Logging: structured, leveled using `log/slog` (INFO, WARN, ERROR)
- **Formatting**: All Go code must be formatted with `gofmt -s` for simplification
- **Testing**: Write unit tests for all parsing logic and pure functions

## Key Components

1. **Tor Control Client** (`internal/tor/control.go`): Raw socket implementation of Tor control protocol
   - Parses bootstrap phase, circuit status, and traffic stats
   - Thread-safe with mutex protection
   - Automatic reconnection on errors

2. **Health Handlers** (`internal/health/handlers.go`): HTTP endpoints with different health check levels
   - `/ping` - Instant liveness (no dependencies)
   - `/health` - Readiness check (Tor control + bootstrap)
   - `/ready` - External connectivity via SOCKS proxy
   - `/status` - Detailed Tor state JSON
   - `/renew` - Send NEWNYM signal to Tor
   - `/metrics` - Prometheus metrics

3. **External Verification** (`internal/health/external.go`): Multi-fallback external IP verification
   - Supports multiple endpoints (TorProject, Dan.me.uk, IPInfo)
   - Retry logic with exponential backoff
   - SOCKS proxy support for Tor egress verification

4. **Metrics** (`internal/health/metrics.go`): Prometheus instrumentation
   - HTTP request duration and counts
   - Tor bootstrap progress and circuit status
   - Traffic statistics (bytes read/written)
   - External check success rates

5. **Config** (`internal/config/config.go`): Centralized environment variable handling
   - Type-safe configuration with defaults
   - Endpoint parsing with validation
   - Integer parsing with fallback to defaults

## Health Check Strategy

- `/ping`: Instant liveness check (<1ms, no Tor dependencies) - for container liveness probes
- `/health`: Fast readiness check (<50ms, Tor control port + bootstrap status ≥100%) - for container readiness probes
- `/ready`: External connectivity verification (1-15s, checks Tor egress via SOCKS) - one-off use only
- `/status`: Detailed Tor state JSON (<50ms, full status details)
- `/metrics`: Prometheus metrics (<10ms, for monitoring)

## Container Design

- **Multi-stage build**: 
  - Build stage: `golang:1.25.5-alpine3.23`
  - Runtime stage: `alpine:3.23`
- **Single entrypoint script** (`scripts/entrypoint.sh`): Manages both Tor daemon and health server
- **Graceful shutdown**: Signal handling for clean termination
- **Persistent volume**: `/var/lib/tor` for consensus cache (faster restarts)
- **Security**: Runs as non-root `tor` user (UID 1000)
- **Multi-arch**: Supports `linux/amd64` and `linux/arm64`

## Testing Approach

- **Unit tests** (`*_test.go`): 
  - Configuration parsing and defaults (100% coverage for config package)
  - Tor control protocol parsing (bootstrap, circuit, traffic)
  - External checker response parsing (TorProject, IPInfo, etc.)
  - HTTP handler behavior (status codes, JSON responses)
- **Testing patterns**:
  - Use table-driven tests for multiple scenarios
  - Mock external dependencies (avoid Prometheus metric registration in tests)
  - Use `httptest` for HTTP handler testing
  - Test with standard library functions (avoid custom parsing)
- **CI/CD**: All tests run with race detection (`-race`) and coverage reporting
- **No integration tests requiring live Tor**: Unit tests are self-contained

## Build and Test Commands

```bash
# Run tests with coverage
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Format code
gofmt -s -w .

# Build binary
go build -o healthserver ./cmd/healthserver

# Build Docker image
docker build -t torarr:dev .

# Run linter (in CI)
golangci-lint run
```

## CI/CD Workflows

Three GitHub Actions workflows provide comprehensive quality gates:

1. **CI** (`.github/workflows/ci.yml`) - Runs on pull requests
   - Unit tests with race detection and coverage
   - Linting with golangci-lint
   - CodeQL security analysis
   - Trivy container vulnerability scanning
   - Docker multi-arch build (after all checks pass)

2. **Release** (`.github/workflows/release.yml`) - Runs on version tags
   - Same quality checks as CI
   - Docker multi-arch build and push to GHCR
   - GitHub Release creation with auto-generated notes
   - Semantic versioning support (v1.2.3)

3. **Security** (`.github/workflows/security.yml`) - Runs weekly on schedule
   - CodeQL analysis for Go code vulnerabilities
   - Trivy scanning for container vulnerabilities
   - Results uploaded to GitHub Security tab
   - Enables proactive vulnerability detection between releases

## Environment Variables

All configurable via environment variables with defaults:

| Variable | Default | Description |
|----------|---------|-------------|
| `TOR_CONTROL_ADDRESS` | `127.0.0.1:9051` | Tor control port address |
| `TOR_CONTROL_PASSWORD` | (empty) | Tor control authentication password |
| `HEALTH_PORT` | `8085` | Health server HTTP port |
| `HEALTH_EXTERNAL_TIMEOUT` | `15` | Timeout for external checks (seconds) |
| `HEALTH_EXTERNAL_ENDPOINTS` | TorProject API | Comma-separated external check URLs |
| `LOG_LEVEL` | `INFO` | Log level (INFO, WARN, ERROR) |
| `TZ` | `UTC` | Container timezone |

See `internal/config/config.go` for implementation details.

## Project Structure

```
torarr/
├── cmd/
│   └── healthserver/        # Main application entry point
├── internal/
│   ├── config/             # Configuration management
│   ├── health/             # Health check handlers and metrics
│   └── tor/                # Tor control protocol client
├── pkg/
│   └── version/            # Version information
├── scripts/
│   └── entrypoint.sh       # Container entrypoint
├── .github/
│   └── workflows/          # CI/CD pipelines
├── Dockerfile              # Multi-stage container build
├── torrc                   # Tor daemon configuration
└── go.mod                  # Go module definition
```

## Key Design Decisions

1. **Minimal dependencies**: Only Prometheus client library, rest is stdlib
2. **Direct Tor control**: Raw socket implementation instead of library for minimal footprint
3. **Parallel workflows**: CI jobs run in parallel for fast feedback
4. **Security-first**: CodeQL + Trivy scanning in CI, Release, and on schedule
5. **Container-optimized**: Multi-stage build, non-root user, health checks built-in
6. **Production-ready**: Graceful shutdown, comprehensive logging, Prometheus metrics
