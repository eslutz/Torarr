# Torarr - Custom Tor Proxy Container

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/eslutz/Torarr/build.yml)
![GitHub Release](https://img.shields.io/github/v/release/eslutz/Torarr)
![Docker Image Size](https://img.shields.io/docker/image-size/eslutz/Torarr/latest)
![GitHub License](https://img.shields.io/github/license/eslutz/Torarr)

A lightweight, health-monitored Tor proxy container designed as a sidecar for the *arr stack (Sonarr, Radarr, Prowlarr, etc.). Built with Go for minimal footprint (~25MB) and fast startup times.

## Features

- **Lightweight**: Alpine-based image (~25MB total)
- **Health Monitoring**: Multiple health check endpoints with different verification levels
- **Fast Restarts**: Persistent volume for Tor consensus cache
- **Multi-Architecture**: Supports `linux/amd64` and `linux/arm64`
- **Zero External Dependencies**: Pure Go stdlib for HTTP routing
- **Metrics Ready**: Prometheus `/metrics` endpoint with Grafana dashboard template
- **Production Ready**: Graceful shutdown, signal handling, and comprehensive logging

## Architecture

```text
┌─────────────────────────────────────────────────────────┐
│                    Torarr Container                     │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────┐         ┌──────────────────────────┐  │
│  │     Tor      │◄───────►│   Health Server (Go)     │  │
│  │              │  9051   │                          │  │
│  │  SOCKS:9050  │ Control │  HTTP:8085               │  │
│  │              │  Port   │  ├─ GET /ping            │  │
│  └──────────────┘         │  ├─ GET /health          │  │
│         │                 │  ├─ GET /health/external │  │
│         │                 │  └─ GET /status          │  │
│         ▼                 └──────────────────────────┘  │
│  ┌──────────────┐                                       │
│  │ /var/lib/tor │                                       │
│  │   (volume)   │                                       │
│  │  Consensus   │                                       │
│  │    Cache     │                                       │
│  └──────────────┘                                       │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### Using Docker Compose (Recommended)

```yaml
version: '3.8'

services:
  tor-proxy:
    image: ghcr.io/eslutz/torarr:latest
    container_name: tor-proxy
    environment:
      - TZ=America/New_York
    ports:
      - "127.0.0.1:9050:9050"  # SOCKS5 proxy
      - "127.0.0.1:8085:8085"  # Health endpoint (optional)
    volumes:
      - tor-data:/var/lib/tor
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "--timeout=5", "http://localhost:8085/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

volumes:
  tor-data:
```

### Using Docker CLI

```bash
docker run -d \
  --name tor-proxy \
  -p 127.0.0.1:9050:9050 \
  -p 127.0.0.1:8085:8085 \
  -e TZ=America/New_York \
  -v tor-data:/var/lib/tor \
  --restart unless-stopped \
  ghcr.io/eslutz/torarr:latest
```

## Health Endpoints

Torarr provides multiple health check endpoints with different verification levels:

| Endpoint | Purpose | Speed | External Deps | Use Case |
|----------|---------|-------|---------------|----------|
| `GET /ping` | Liveness check | <1ms | None | Container orchestrator liveness probe |
| `GET /health` | Readiness check | <50ms | None | Container orchestrator readiness probe |
| `GET /health/external` | External connection test | 1-15s | Tor network | Manual verification of external connectivity |
| `GET /status` | Full Tor status | <50ms | None | Monitoring/debugging |
| `GET /metrics` | Prometheus metrics | <10ms | None | Prometheus/Grafana scraping |

### Endpoint Details

#### `/ping` - Liveness Check

Returns immediately with `{"status":"OK"}`. Use for container liveness probes.

```bash
curl http://localhost:8085/ping
```

#### `/health` - Readiness Check

Checks Tor control port and bootstrap status. Returns ready when bootstrap ≥100%.

```bash
curl http://localhost:8085/health
```

**Response (Ready):**

```json
{
  "status": "READY"
}
```

**Response (Not Ready):**

```json
{
  "status": "NOT_READY",
  "error": "tor not ready"
}
```

#### `/health/external` - External Connection Test

Tests external connectivity through the Tor proxy using multiple endpoints with retry/fallback logic. This endpoint bypasses Tor readiness checks and directly verifies external connection.

**⚠️ Important**: Use this endpoint for one-off verification (e.g., manual testing). Do **not** use as a constant healthcheck, as it makes requests to external services on each call.

```bash
curl http://localhost:8085/health/external
```

**Response (Connected via Tor):**

```json
{
  "success": true,
  "is_tor": true,
  "ip": "185.220.101.1",
  "endpoint": "https://check.torproject.org/api/ip",
  "checked_at": "2024-01-15T12:00:00Z"
}
```

**Response (Not Connected via Tor):**

```json
{
  "success": false,
  "is_tor": false,
  "error": "all endpoints failed",
  "checked_at": "2024-01-15T12:00:00Z"
}
```

#### `/status` - Full Status

Returns detailed Tor status including version, bootstrap phase, circuits, and traffic.

```bash
curl http://localhost:8085/status
```

**Response:**

```json
{
  "status": "OK",
  "version": "0.4.8.10",
  "bootstrap_phase": 100,
  "circuit_established": true,
  "num_circuits": 3,
  "traffic": {
    "bytes_read": 1048576,
    "bytes_written": 524288
  }
}
```

#### `/metrics` - Prometheus Metrics

Exposes Prometheus-format metrics for HTTP requests, Tor readiness/traffic, and external check outcomes.

```bash
curl http://localhost:8085/metrics
```

Prometheus scrape example:

```yaml
scrape_configs:
  - job_name: torarr
    static_configs:
      - targets: ['tor-proxy:8085']
```

### Grafana Dashboard

Import `grafana/torarr-dashboard.json` into Grafana and select your Prometheus datasource. Panels include readiness, bootstrap progress, traffic, request rates, and external check results.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TZ` | `UTC` | Timezone for logs |
| `TOR_CONTROL_PASSWORD` | (auto-generated) | Password for Tor control port |
| `TOR_CONTROL_ADDRESS` | `127.0.0.1:9051` | Tor control port address |
| `HEALTH_PORT` | `8085` | Health server port |
| `HEALTH_FULL_TIMEOUT` | `15` | Timeout for `/health/external` in seconds |
| `HEALTH_EXTERNAL_ENDPOINTS` | (see below) | Comma-separated external check URLs |
| `LOG_LEVEL` | `INFO` | Log level (DEBUG, INFO, WARN, ERROR) |

**Default External Endpoints:**

- `https://check.torproject.org/api/ip`
- `https://check.dan.me.uk/`
- `https://ipinfo.io/json`

## Integration with *arr Stack

### Prowlarr Example

```yaml
version: '3.8'

services:
  tor-proxy:
    image: ghcr.io/eslutz/torarr:latest
    container_name: tor-proxy
    environment:
      - TZ=America/New_York
    ports:
      - "127.0.0.1:9050:9050"
    volumes:
      - tor-data:/var/lib/tor
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "--timeout=5", "http://localhost:8085/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s

  prowlarr:
    image: lscr.io/linuxserver/prowlarr:latest
    container_name: prowlarr
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=America/New_York
    volumes:
      - ./prowlarr:/config
    ports:
      - "9696:9696"
    restart: unless-stopped
    depends_on:
      tor-proxy:
        condition: service_healthy

volumes:
  tor-data:
```

**Configure Prowlarr to use Tor:**

1. Open Prowlarr web interface
2. Go to Settings → Indexers
3. Add/Edit indexer
4. Under "Proxy Settings":
   - Type: `SOCKS5`
   - Hostname: `tor-proxy` (or `127.0.0.1` if not using Docker network)
   - Port: `9050`

## Persistent Volume

**Important**: Mount `/var/lib/tor` as a persistent volume for:

- Faster restarts (cached Tor consensus)
- Reduced Tor network load
- Consistent guard relay selection

Without persistence, Tor must download fresh consensus on each restart (~30-60s delay).

## Troubleshooting

### Check Tor Bootstrap Status

```bash
curl http://localhost:8085/status
```

Look for `bootstrap_phase: 100` and `circuit_established: true`.

### View Container Logs

```bash
docker logs tor-proxy
```

### Test SOCKS Proxy Directly

```bash
curl --socks5-hostname localhost:9050 https://check.torproject.org/api/ip
```

Expected response: `{"IsTor":true,"IP":"..."}`

### Container Won't Start

1. Check if ports 9050/8085 are available:

   ```bash
   netstat -tuln | grep -E '9050|8085'
   ```

2. Verify volume permissions:

   ```bash
   docker volume inspect tor-data
   ```

3. Check for conflicting Tor instances:

   ```bash
   pgrep -a tor
   ```

### Tor Bootstrap Stuck

If `bootstrap_phase` is stuck below 100:

1. Check internet connectivity
2. Verify no firewall blocking Tor (ports 443, 9001, 9030)
3. Try deleting volume and restarting:

   ```bash
   docker-compose down
   docker volume rm tor-data
   docker-compose up -d
   ```

### External Verification Fails

If `/health/external` shows `is_tor: false`:

1. Check if SOCKS proxy is working (see above)
2. Verify external endpoints are accessible
3. Check logs for DNS resolution issues
4. Some external services may be temporarily down (check multiple endpoints)

## Building from Source

```bash
git clone https://github.com/eslutz/torarr.git
cd torarr
docker build -t torarr:local .
```

### Multi-Architecture Build

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t ghcr.io/eslutz/torarr:latest \
  --push .
```

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

### Automated Releases & Versioning

Releases are fully automated based on commit messages using Semantic Versioning:

- **fix:** triggers a Patch release (v1.0.0 -> v1.0.1)
- **feat:** triggers a Minor release (v1.0.0 -> v1.1.0)
- **BREAKING CHANGE:** triggers a Major release (v1.0.0 -> v2.0.0)

On every push to `main`, the system builds the Docker image, tags it, and creates a GitHub Release with a changelog.

## Security

- **Tor Control Password**: A random password is automatically generated on container startup and hashed for `torrc`. This ensures every instance is unique and secure.
- **Health Server**: Binds to all interfaces (0.0.0.0) to allow container orchestration health checks.
- **SOCKS Proxy**: Should be bound to localhost or a private network in production (default in docker-compose).
- **Logging**: No sensitive data (like passwords) is logged.

**Reporting Security Issues**: Please email <security@example.com> (do not open public issues)

## License

MIT License - see [LICENSE](LICENSE) for details

## Acknowledgments

- [Tor Project](https://www.torproject.org/) for the Tor software
- [Alpine Linux](https://alpinelinux.org/) for the minimal base image
- *arr community for inspiration

## Roadmap

- [x] Prometheus metrics endpoint
- [ ] Configurable circuit refresh intervals
- [ ] Bridge support for censored networks
- [ ] Bandwidth statistics dashboard
- [ ] Multi-proxy load balancing

---

**Note**: This is not affiliated with or endorsed by the Tor Project. Use responsibly and in accordance with local laws.
