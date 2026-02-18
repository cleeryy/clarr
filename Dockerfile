# ─── Stage 1: Builder ───────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Dépendances d'abord (cache Docker optimal)
COPY go.mod go.sum ./
RUN go mod download

# Code source
COPY . .

# Build statique (pas de dépendances système en runtime)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always)" \
    -o clarr ./cmd/clarr/main.go

# ─── Stage 2: Runtime ───────────────────────────────────────────────
FROM scratch

# Certificats SSL pour les appels HTTP vers les APIs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binaire uniquement
COPY --from=builder /app/clarr /clarr

EXPOSE 8090

ENTRYPOINT ["/clarr"]
