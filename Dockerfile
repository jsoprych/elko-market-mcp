# ── Builder ───────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Cache module downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /elko ./cmd/elko

# ── Final image ───────────────────────────────────────────────────────────────
FROM scratch

# CA certificates for HTTPS to external APIs.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary.
COPY --from=builder /elko /elko

# Data volume for SQLite persistence.
VOLUME ["/data"]

# REST API port.
EXPOSE 8080

ENTRYPOINT ["/elko"]
CMD ["serve", "--db", "/data/elko.db", "--port", "8080"]
