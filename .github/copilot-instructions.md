# Torarr - Copilot Instructions

## Project Overview

Torarr is a lightweight Tor proxy container with comprehensive health monitoring, designed as a sidecar for the \*arr stack (Sonarr, Radarr, etc.).

## Architecture

- **Language**: Go (zero external dependencies for HTTP routing)
- **Container**: Alpine-based (~25MB total)
- **Health Server**: HTTP endpoints on port 8080
- **Tor Control**: Direct socket communication on port 9051
- **SOCKS Proxy**: Port 9050

## Code Style Guidelines

- Use stdlib `net/http` - no external routing libraries
- Keep dependencies minimal
- Prefer direct socket communication over libraries
- Error handling: explicit returns, no panics in production code
- Configuration: environment variables with sensible defaults
- Logging: structured, leveled (INFO, WARN, ERROR)

## Key Components

1. **Tor Control Client** (`internal/tor/control.go`): Raw socket implementation of Tor control protocol
2. **Health Handlers** (`internal/health/handlers.go`): HTTP endpoints with different health check levels
3. **External Verification** (`internal/health/external.go`): Multi-fallback external IP verification with caching
4. **Config** (`internal/config/config.go`): Centralized environment variable handling

## Health Check Strategy

- `/ping`: Instant liveness check (no Tor dependencies)
- `/health`: Fast readiness check (Tor control port + bootstrap status)
- `/health/external`: External connectivity verification (one-off use only)
- `/status`: Detailed Tor state JSON

## Container Design

- Multi-stage build (build in golang:1.23-alpine, run in alpine:3.20)
- Single entrypoint script manages both processes
- Graceful shutdown with signal handling
- Persistent volume for `/var/lib/tor` (consensus cache)

## Testing Approach

- Unit tests for control protocol parsing
- Integration tests assume Tor is running
- Health endpoint tests can mock external calls
- Docker builds tested in CI/CD

## Environment Variables

All configurable via environment variables with defaults. See `internal/config/config.go` for canonical list.
