# Dockerfile delta (inspect + change)

Agent checklist:
1) Remove hardcoded GOARCH (e.g., GOARCH=amd64).
   Use BuildKit args:
   - ARG TARGETOS TARGETARCH
   - GOOS=$TARGETOS GOARCH=$TARGETARCH
   Prefer:
   - FROM --platform=$BUILDPLATFORM golang:... AS builder

2) Confirm whether dashboard assets must exist on disk at runtime.
   - If yes: COPY the web assets into final image (scratch won't have them otherwise).
   - If assets are embedded via Go embed: no copy.

3) Keep HTTPS working in scratch:
   - copy CA certs into final image.

4) Keep ENTRYPOINT/CMD aligned with current runtime behavior.
