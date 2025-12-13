# Torarr AI Coding Guidelines

## Project Overview

Torarr is a production-ready Tor SOCKS proxy container with a Go health/metrics sidecar designed for the \*arr stack (Sonarr, Radarr, Prowlarr). The architecture consists of two processes running in a single container:

1. **Tor daemon** - Main process providing SOCKS5 proxy on port 9050
2. **Health server** (Go) - HTTP sidecar on port 8085 providing health checks, circuit renewal, and Prometheus metrics

**Key Design Philosophy**: The entrypoint script generates/hashes the Tor control password, configures torrc dynamically, then starts the health server in background and Tor as the main process.

## Architecture & Data Flow

```
User Request → Tor (SOCKS :9050) → Internet
                 ↕ control port :9051
          Health Server (HTTP :8085) ← Health Checks / Metrics Scraping
```

- **Health server communicates with Tor** via control port protocol (see `internal/tor/control.go`)
- **External readiness checks** route through SOCKS proxy to verify Tor egress (`internal/health/external.go`)
- **Metrics collection** queries Tor via control port for bootstrap, circuit, and traffic stats

## Code Organization

```
cmd/healthserver/          # Main entry point for health server
internal/
  config/                  # Environment-based configuration (no files)
  health/
    handlers.go            # HTTP endpoint handlers
    external.go            # External Tor egress verification via SOCKS
    metrics.go             # Prometheus metrics setup and observation
  tor/
    control.go             # Tor control port protocol client
pkg/version/               # Build-time version injection
```

## Critical Patterns

### Configuration Loading (`internal/config/config.go`)

- **All config via environment variables** - No config files, defaults in code
- Use `getEnv()` helper with defaults, `getEnvAsInt()` for numeric values
- Comma-separated string parsing for `HEALTH_EXTERNAL_ENDPOINTS`
- Example: `TOR_CONTROL_ADDRESS` defaults to `127.0.0.1:9051`

### Tor Control Protocol (`internal/tor/control.go`)

- Custom protocol client, not using external libraries
- **Thread-safe** with `sync.Mutex` protecting connection state
- Command format: `"GETINFO key\r\n"`, responses start with status codes (250 = success)
- Parse multi-line responses with `650+` prefix, terminated by `650 OK`
- Extract numeric values from `key=value` pairs (bootstrap, circuits, traffic)

### Testing Strategy

- **Table-driven tests** for parsers (see `control_test.go`, `external_test.go`)
- **Test all error paths** - invalid formats, missing fields, type conversion failures
- **Mock-free unit tests** where possible - test pure functions with sample data
- **HTTP handler tests** use `httptest.ResponseRecorder` (see `handlers_test.go`)

### Error Handling

- Wrap errors with context: `fmt.Errorf("operation failed: %w", err)`
- HTTP handlers: log errors with `slog.Error()`, return structured JSON with status codes
- Graceful degradation: metrics set to 0 on Tor unavailability

### Logging

- **Structured JSON logging** via `log/slog` (default handler set in `main.go`)
- Use `slog.Info/Warn/Error` with key-value pairs: `slog.Error("msg", "key", value)`
- Log level controlled by `LOG_LEVEL` env var (INFO default)

### Metrics (`internal/health/metrics.go`)

- Use `prometheus/client_golang` with `promauto` for automatic registration
- Namespace all metrics with `torarr_` prefix
- Version info exposed via `torarr_info` gauge with build labels (see `pkg/version/version.go`)
- Observe patterns: increment counters, set gauges, time histograms in middleware

## Build & Release

### Version Injection

Build args set version info at compile time:

```bash
go build -ldflags="-X github.com/eslutz/torarr/pkg/version.Version=${VERSION} ..."
```

Access via `version.Version`, `version.Commit`, `version.Date`

### Release Process

1. Update `VERSION` file in root (e.g., `0.2.0`)
2. Merge to `main` branch
3. CI reads `VERSION`, creates git tag (e.g., `v0.2.0`), builds multi-arch image, publishes to ghcr.io
4. **No manual tagging** - fully automated via `.github/workflows/release.yml`

### Local Development

```bash
# Run tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Build binary
go build -o healthserver ./cmd/healthserver

# Build Docker image
docker build -t torarr:local .
```

## HTTP Endpoints Semantics

- **`/ping`** - Simple liveness (always 200 if process alive)
- **`/health`** - Tor bootstrap readiness (200 only if bootstrap=100%)
- **`/ready`** - Full egress verification through SOCKS proxy (makes external HTTP calls)
- **`/status`** - Diagnostics snapshot (JSON with all Tor info)
- **`/metrics`** - Prometheus scrape target (OpenMetrics format)
- **`POST /renew`** - Circuit renewal via `SIGNAL NEWNYM`

**Kubernetes probes**: Use `/ping` for liveness, `/health` for readiness (or `/ready` if external verification needed)

## Common Tasks

### Adding a New Config Variable

1. Add to `Config` struct in `internal/config/config.go`
2. Load in `Load()` function with `getEnv()` or `getEnvAsInt()`
3. Document in README.md configuration table
4. Add test case in `config_test.go`

### Adding a New Metric

1. Define in `internal/health/metrics.go` with `promauto.NewCounter/Gauge/Histogram`
2. Observe in appropriate handler or `observeTorStatus()`
3. Document in README.md metrics table
4. Consider adding to Grafana dashboard JSON

### Adding a New Endpoint

1. Add handler method to `Handler` in `internal/health/handlers.go`
2. Register in `SetupRoutes()` with `instrument()` middleware
3. Add tests in `handlers_test.go`
4. Document in README.md endpoints table

## Dependencies

- **Minimal external deps**: Only `prometheus/client_golang` for metrics
- **No Tor library** - custom control protocol implementation
- **Alpine base** for runtime image (Tor from apk)
- Go 1.25+ required

## Security Considerations

- Tor control password auto-generated if unset (in `entrypoint.sh`)
- Container runs as non-root `tor` user (UID 1000)
- SOCKS proxy should bind to localhost or private network only
- Control port not exposed outside container
