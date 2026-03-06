# ── Builder ───────────────────────────────────────────────────────────────────
# TARGETOS/TARGETARCH are injected by buildx for multi-arch builds.
# Defaults allow plain `docker build` (no buildx) to work on amd64.
FROM golang:1.25-alpine AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /build

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /elko ./cmd/elko

# ── Final image ───────────────────────────────────────────────────────────────
FROM scratch

# CA certificates for HTTPS to external APIs.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary.
COPY --from=builder /elko /elko

# Web dashboard assets (served from /web at runtime; binary uses "./web"
# which resolves to /web when the process working directory is /).
COPY --from=builder /build/web /web

# Data volume for SQLite persistence.
VOLUME ["/data"]

# REST API port.
EXPOSE 8080

ENTRYPOINT ["/elko"]
CMD ["serve", "--db", "/data/elko.db", "--port", "8080"]
