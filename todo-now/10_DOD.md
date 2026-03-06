# Definition of Done (DoD)

## User experience
- Users can pull/run a published image:
  - `ghcr.io/jsoprych/elko-market-mcp:latest`
  - (mirror) `jsoprych/elko-market-mcp:latest`
- Running it exposes:
  - Dashboard: http://localhost:8080
  - Health: http://localhost:8080/health
  - REST tool call endpoint: POST /v1/call/{tool}

## CI/CD publishing
- GitHub Actions builds on tag pushes matching: `v*.*.*`
- It publishes multi-arch images (linux/amd64, linux/arm64) to:
  - GHCR: `ghcr.io/jsoprych/elko-market-mcp:{vX.Y.Z,latest}`
  - Docker Hub: `jsoprych/elko-market-mcp:{vX.Y.Z,latest}`

## Repo hygiene (minimum)
- Add workflow file to `.github/workflows/docker-image.yml`
- Add/adjust Dockerfile so buildx multi-arch works (no hardcoded GOARCH=amd64)
- Ensure the final scratch image contains the web dashboard assets (if required by runtime)
- Compose: choose least-disruptive approach (see 50_compose_delta.md)
