# Docker Deployment

![Docker build and deployment](img/docker-build.svg)

elko ships as a published multi-arch Docker image (amd64 + arm64) on GHCR and Docker Hub, plus a multi-stage Dockerfile for building from source.

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Pull the Published Image](#pull-the-published-image)
3. [Build the Image](#build-the-image)
4. [Configuration](#configuration)
5. [Persistent Cache](#persistent-cache)
6. [docker-compose Reference](#docker-compose-reference)
7. [Dockerfile Details](#dockerfile-details)
8. [Health Check](#health-check)
9. [Running Without Compose](#running-without-compose)

---

## Quick Start

### Pull and run the published image (fastest)

```bash
# GHCR
docker pull ghcr.io/jsoprych/elko-market-mcp:latest

# Docker Hub mirror
docker pull jsoprych/elko-market-mcp:latest

# Run
docker run -d --name elko \
  -p 8080:8080 \
  -e SEC_USER_AGENT="MyApp me@example.com" \
  ghcr.io/jsoprych/elko-market-mcp:latest

# Open the dashboard
open http://localhost:8080
curl localhost:8080/health
```

### With docker-compose.run.yml (published image + persistent cache)

```bash
export SEC_USER_AGENT="MyApp me@example.com"
docker compose -f docker-compose.run.yml up -d
open http://localhost:8080
```

### Build from source

```bash
# 1. Set your SEC contact (required for EDGAR tools)
export SEC_USER_AGENT="MyApp me@example.com"

# 2. Build and start
docker compose up

# 3. Open the dashboard
open http://localhost:8080
```

> **No `docker compose`?** Install the plugin: `sudo apt install docker-compose-plugin`
> or use the plain `docker` one-liners in [Running Without Compose](#running-without-compose) below.

---

## Pull the Published Image

Images are published to two registries on every `v*.*.*` tag, for `linux/amd64` and `linux/arm64`:

| Registry | Image |
|----------|-------|
| GHCR | `ghcr.io/jsoprych/elko-market-mcp:latest` |
| Docker Hub | `jsoprych/elko-market-mcp:latest` |

Pin a specific version:

```bash
docker pull ghcr.io/jsoprych/elko-market-mcp:v0.1.1
```

---

## Build the Image

### Using docker-compose (recommended)

```bash
docker compose build
docker compose up
```

### Manual build

```bash
docker build -t elko-market-mcp .
```

### Build args

The Dockerfile uses a multi-stage build:
- **Stage 1 (builder):** `golang:1.25-alpine` — compiles the binary with CGO disabled
- **Stage 2 (runtime):** `scratch` — minimal image containing only the binary and CA certificates

Binary is compiled with:
```
CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /elko ./cmd/elko
```

`TARGETOS` and `TARGETARCH` default to `linux/amd64` for plain `docker build`; BuildKit sets them automatically for multi-arch builds.

The `-s -w` flags strip debug info and DWARF tables for a smaller binary.

---

## Configuration

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `SEC_USER_AGENT` | For EDGAR tools | `"AppName contact@email.com"` — per [SEC policy](https://www.sec.gov/developer) |

Set in `docker-compose.yml`:

```yaml
environment:
  SEC_USER_AGENT: "MyApp me@example.com"
```

Or pass at runtime:

```bash
SEC_USER_AGENT="MyApp me@example.com" docker compose up
```

### Port

Default: `8080`. Change in `docker-compose.yml`:

```yaml
ports:
  - "9090:8080"   # host:container
```

### Source Filtering

To restrict which data sources are enabled, override the `command` in `docker-compose.yml`:

```yaml
command: ["serve", "--port", "8080", "--sources", "yahoo,edgar"]
```

---

## Persistent Cache

By default, the container uses in-memory cache only — responses are lost when the container restarts.

Enable SQLite persistence by mounting a volume:

```yaml
volumes:
  - elko-data:/data

# The compose file already does this. The cache file lives at /data/cache.db
# which maps to the elko-data named volume.
```

The cache file is passed to the binary via `--db /data/cache.db` (already configured in `docker-compose.yml`).

**Custom host path:**

```yaml
volumes:
  - /home/user/.elko:/data
```

This mounts the host directory `/home/user/.elko` as `/data` in the container.

---

## docker-compose Reference

```yaml
services:
  elko:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - elko-data:/data
    environment:
      SEC_USER_AGENT: "MyApp me@example.com"
    command: ["serve", "--port", "8080", "--db", "/data/cache.db"]
    healthcheck:
      test: ["/elko", "catalogue", "--sources", "yahoo"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s
    restart: unless-stopped

volumes:
  elko-data:
```

### Override options

```bash
# Different port
docker compose run --rm -p 9000:8080 elko serve --port 8080

# Only Yahoo Finance
docker compose run --rm -p 8080:8080 elko serve --port 8080 --sources yahoo

# No cache persistence
docker compose run --rm -p 8080:8080 elko serve --port 8080

# Run the CLI
docker compose run --rm elko call yahoo_quote symbol=AAPL
```

---

## Dockerfile Details

```dockerfile
# Stage 1: Build
FROM golang:1.25-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /elko ./cmd/elko

# Stage 2: Runtime
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /elko /elko
COPY --from=builder /build/web /web
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/elko"]
CMD ["serve", "--db", "/data/elko.db", "--port", "8080"]
```

**Key points:**
- `scratch` base = zero OS footprint, no shell, no package manager
- CA certificates copied from builder for HTTPS calls to external APIs
- `CGO_ENABLED=0` enables static linking (required for scratch base)
- `COPY /build/web /web` — dashboard assets served from disk, not embedded
- `ARG TARGETOS/TARGETARCH` — BuildKit sets these for multi-arch; defaults to `linux/amd64`
- Final image size: ~11 MB (binary + web assets + CA certs)

---

## Health Check

The compose file includes a healthcheck that verifies the binary can load and query tools:

```yaml
healthcheck:
  test: ["/elko", "catalogue", "--sources", "yahoo"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 5s
```

This runs `elko catalogue --sources yahoo` every 30 seconds. If it fails 3 times, the container is marked unhealthy.

Check container health:

```bash
docker compose ps
docker inspect elko-market-mcp-elko-1 | jq '.[0].State.Health'
```

---

## Running Without Compose

Use these when `docker compose` is not available (e.g. older Docker installs, CI runners, minimal environments).

```bash
# 1. Build
docker build -t elko-market-mcp .

# 2a. Run — in-memory cache (ephemeral, simplest)
docker run -d --name elko \
  -p 8080:8080 \
  -e SEC_USER_AGENT="MyApp me@example.com" \
  elko-market-mcp

# 2b. Run — persistent SQLite cache (survives restarts)
docker run -d --name elko \
  -p 8080:8080 \
  -e SEC_USER_AGENT="MyApp me@example.com" \
  -v "$HOME/.elko:/data" \
  elko-market-mcp serve --port 8080 --db /data/cache.db

# 3. Verify
curl localhost:8080/health
# → {"status":"ok","tools":10,"version":"0.1.0"}

curl localhost:8080/v1/sources
# → {"sources":["bls","edgar","fdic","treasury","worldbank","yahoo"]}

curl -s -XPOST localhost:8080/v1/call/yahoo_quote \
  -H 'Content-Type: application/json' \
  -d '{"symbol":"AAPL"}'

# 4. CLI one-shot (no server)
docker run --rm \
  -e SEC_USER_AGENT="MyApp me@example.com" \
  elko-market-mcp call yahoo_quote symbol=NVDA

# 5. MCP mode (pipe stdio — for remote/containerised MCP setups)
docker run -i --rm \
  -e SEC_USER_AGENT="MyApp me@example.com" \
  elko-market-mcp mcp

# 6. Stop / remove
docker stop elko && docker rm elko
```

---

## Troubleshooting

### "certificate signed by unknown authority"

CA certificates are embedded from the builder stage. If you see this error in a custom base image, ensure `/etc/ssl/certs/ca-certificates.crt` is present.

### SEC EDGAR returns 403

Set `SEC_USER_AGENT` to a valid `"AppName contact@email.com"` string. The SEC blocks requests without proper identification.

### Container exits immediately

Check logs: `docker compose logs elko`. Common causes:
- Port already in use — change the host port in `ports:`
- Invalid `--sources` flag — use lowercase: `yahoo`, `edgar`, etc.

### Data not persisting between restarts

Ensure the volume is mounted and `--db` points to the volume path (`/data/cache.db`).
