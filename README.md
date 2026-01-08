# Torarr

[![Workflow Status](https://github.com/eslutz/torarr/actions/workflows/release.yml/badge.svg)](https://github.com/eslutz/torarr/actions/workflows/release.yml)
[![Security Check](https://github.com/eslutz/torarr/actions/workflows/security.yml/badge.svg)](https://github.com/eslutz/torarr/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/eslutz/torarr)](https://goreportcard.com/report/github.com/eslutz/torarr)
[![License](https://img.shields.io/github/license/eslutz/torarr)](LICENSE)
[![Release](https://img.shields.io/github/v/release/eslutz/torarr?color=007ec6)](https://github.com/eslutz/torarr/releases/latest)

A lightweight, production-ready Tor SOCKS proxy container with a Go health/metrics sidecar. Designed for the *arr stack (Sonarr, Radarr, Prowlarr, etc.).

## Features

- **Tor SOCKS Proxy**: Exposes SOCKS5 on port 9050
- **Health & Readiness**: Kubernetes-compatible endpoints for liveness/readiness
- **Tor Egress Verification**: Optional external checks via `/ready`
- **Circuit Renewal**: `POST /renew` sends `NEWNYM` to request a new circuit
- **Prometheus Metrics**: `/metrics` endpoint + included Grafana dashboard
- **Multi-Architecture**: Supports `linux/amd64` and `linux/arm64`
- **Non-Root Runtime**: Runs as a dedicated `tor` user in the container

## Quick Start

### Docker Compose

See [docs/docker-compose.example.yml](docs/docker-compose.example.yml).

### Docker CLI

```bash
docker run -d \
  --name torarr \
  -p 127.0.0.1:9050:9050 \
  -p 127.0.0.1:9091:9091 \
  -e TZ=America/New_York \
  -v tor-data:/var/lib/tor \
  --restart unless-stopped \
  ghcr.io/eslutz/torarr:latest
```

## Configuration

All application configuration is done via environment variables. Below is a quick reference of key settings. For a complete configuration template with all options, see [docs/.env.example](docs/.env.example).

### General Settings

| Variable | Default | Description |
| --- | --- | --- |
| `TZ` | `UTC` | Container timezone |
| `LOG_LEVEL` | `INFO` | Logging level (DEBUG, INFO, WARN, ERROR) |

### Health Server Settings

| Variable | Default | Description |
| --- | --- | --- |
| `HEALTH_PORT` | `9091` | HTTP server port for health/metrics |
| `HEALTH_EXTERNAL_TIMEOUT` | `15` | Timeout (seconds) for external Tor egress checks |

### Tor Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `TOR_CONTROL_ADDRESS` | `127.0.0.1:9051` | Tor control port address |
| `TOR_CONTROL_PASSWORD` | *(auto-generated)* | Tor control password; auto-generated if unset |
| `TOR_EXIT_NODES` | *(none)* | Exit node selector (e.g. `{us},{ca}`) |

### Webhook Notifications

| Variable | Default | Description |
| --- | --- | --- |
| `WEBHOOK_URL` | *(none)* | Webhook endpoint URL (Discord, Slack, etc.) |
| `WEBHOOK_TEMPLATE` | `discord` | Webhook format: `discord`, `slack`, `gotify`, `json` |
| `WEBHOOK_EVENTS` | `circuit_renewed,bootstrap_failed,health_changed` | Events to notify on (comma-separated) |

> **📝 Full Configuration:** See [docs/.env.example](docs/.env.example) for all available options with detailed comments and examples.

## Architecture

```txt
┌────────────────────────────────────────────────────────────┐
│                         Torarr                             │
├────────────────────────────────────────────────────────────┤
│  ┌──────────────┐      control port      ┌───────────────┐ │
│  │     Tor      │◄──────────────────────►│  Healthserver │ │
│  │ SOCKS :9050  │        :9051           │  HTTP :9091   │ │
│  └──────┬───────┘                        └───────────────┘ │
│         │                                                  │
│         ▼                                                  │
│   /var/lib/tor  (mount as volume for faster restarts)      │
└────────────────────────────────────────────────────────────┘
```

**How it works:**

1. The entrypoint hashes `TOR_CONTROL_PASSWORD` (or generates one) and updates `/etc/tor/torrc`
2. Tor runs as the main process and exposes SOCKS5 on `:9050`
3. The Go health server queries Tor via the control port and exposes HTTP endpoints on `:${HEALTH_PORT}`
4. `/ready` verifies Tor egress by calling external endpoints through the SOCKS proxy

## Tor Configuration

Tor uses the `torrc` file in the repository root (copied into the image at `/etc/tor/torrc`). The entrypoint modifies it at startup (control password hashing, optional exit nodes).

If you want to customize Tor settings, mount your own `torrc` **as writable** (the entrypoint needs to update `HashedControlPassword`).

## HTTP Endpoints

| Endpoint | Purpose | Response |
| --- | --- | --- |
| `GET /ping` | Liveness probe | `200 OK` if running |
| `GET /health` | Tor bootstrap readiness | `200 OK` when bootstrap is 100% |
| `GET /ready` | Tor egress verification | `200 OK` if external check succeeds and `IsTor=true` |
| `GET /status` | Diagnostics | JSON status snapshot |
| `GET /metrics` | Prometheus metrics | OpenMetrics/Prometheus format |
| `POST /renew` | Request a new circuit | `200 OK` if `NEWNYM` was sent |

### Endpoint Usage

- **/ping**: Liveness probe (restart container if it fails)
- **/health**: Readiness probe for Tor bootstrap
- **/ready**: Readiness probe when you need confirmed Tor egress (makes outbound requests)
- **/status**: Manual debugging/monitoring snapshot
- **/metrics**: Prometheus scraping target

## Prometheus Metrics

| Metric | Type | Description |
| --- | --- | --- |
| `torarr_info` | Gauge | Build information (version, commit, date, go_version) |
| `torarr_http_requests_total` | Counter | Total HTTP requests (labels: path, method, code) |
| `torarr_http_request_duration_seconds` | Histogram | HTTP request durations (labels: path, method, code) |
| `torarr_tor_bootstrap_percent` | Gauge | Tor bootstrap percent |
| `torarr_tor_circuit_established` | Gauge | Circuit established (1/0) |
| `torarr_tor_ready` | Gauge | Readiness derived from circuit state (1/0) |
| `torarr_tor_bytes_read` | Gauge | Bytes read (Tor traffic stats) |
| `torarr_tor_bytes_written` | Gauge | Bytes written (Tor traffic stats) |
| `torarr_external_check_total` | Counter | External check attempts (labels: endpoint, success, is_tor) |
| `torarr_webhook_requests_total` | Counter | Webhook notification attempts (labels: event, status) |
| `torarr_webhook_duration_seconds` | Histogram | Webhook notification duration (labels: event) |

## Grafana Dashboard

Import [docs/torarr-grafana-dashboard.json](docs/torarr-grafana-dashboard.json) into Grafana.

## Webhook Notifications

Torarr can send webhook notifications for various events. Configure webhooks using the environment variables listed in the Configuration section.

### Supported Templates

- **Discord**: Rich embeds with color-coded events
- **Slack**: Attachments with formatted fields
- **Gotify**: Priority-based notifications
- **JSON**: Plain JSON payloads for custom integrations

### Events

| Event | Description |
| --- | --- |
| `circuit_renewed` | Triggered when `POST /renew` successfully sends NEWNYM |
| `bootstrap_failed` | Tor bootstrap is below 100%; fired on **every** `/health` check while unhealthy (can be very frequent) |
| `health_changed` | Health status changed (state transition only) |

> **Note:** `bootstrap_failed` is evaluated on each health check. In environments like Kubernetes where probes may run every few seconds, this can generate many webhook calls during bootstrap or outages. Consider:
>
> - Using `health_changed` for primary alerting on state transitions
> - Enabling `bootstrap_failed` only when per-check visibility is required
> - Implementing rate limiting / deduplication on the webhook receiver to avoid notification spam

### Example: Discord Webhook

```bash
docker run -d \
  --name torarr \
  -p 127.0.0.1:9050:9050 \
  -p 127.0.0.1:9091:9091 \
  -e WEBHOOK_URL=https://discord.com/api/webhooks/YOUR_WEBHOOK_ID \
  -e WEBHOOK_TEMPLATE=discord \
  -e WEBHOOK_EVENTS=circuit_renewed,bootstrap_failed \
  ghcr.io/eslutz/torarr:latest
```

### Example: Slack Webhook

```bash
-e WEBHOOK_URL=https://hooks.slack.com/services/YOUR_WEBHOOK_PATH \
-e WEBHOOK_TEMPLATE=slack
```

## Releases

Releases are driven by the `VERSION` file:

1. CI runs on PRs and pushes to `main`
2. After CI succeeds on `main`, the release workflow reads `VERSION`
3. If the corresponding tag (e.g. `v0.1.0`) doesn't exist, it creates the tag, pushes a multi-arch image, and creates a GitHub Release

To cut a new release: update `VERSION` and merge to `main`.

## Building from Source

```bash
git clone https://github.com/eslutz/torarr.git
cd torarr

# Build binary
go build -o healthserver ./cmd/healthserver

# Build Docker image
docker build -t torarr:local .
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and lint
5. Submit a pull request

## Security

- Don't expose the SOCKS proxy publicly; bind it to localhost or a private network.
- The Tor control password is generated at startup if unset; treat container logs as sensitive if you rely on auto-generation.

Report vulnerabilities via GitHub Security Advisories:
<https://github.com/eslutz/torarr/security/advisories/new>

## License

This project is licensed under the MIT License - see [LICENSE](LICENSE).

## Acknowledgments

- [Tor Project](https://www.torproject.org/) - Anonymous communication network
- [Prometheus](https://prometheus.io/) - Monitoring system and time series database

## Related Projects

- [Forwardarr](https://github.com/eslutz/forwardarr) - SPort update container for Gluetun to qBittorrent port syncing that updates the listening port on change
