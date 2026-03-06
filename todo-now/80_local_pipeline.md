# Run the pipeline locally (no Actions)

Login:
- docker login ghcr.io
- docker login (Docker Hub)

Build/push multi-arch:
```bash
docker buildx create --use --name elko-builder || true
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ghcr.io/jsoprych/elko-market-mcp:dev \
  -t jsoprych/elko-market-mcp:dev \
  --push .
```
