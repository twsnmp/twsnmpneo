# syntax=docker/dockerfile:1

# Stage 1: Build Frontend SPA
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend

RUN corepack enable && corepack prepare pnpm@latest --activate

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm run build

# Stage 2: Build Backend Go binary
FROM golang:1.27-alpine AS backend-builder
WORKDIR /app/backend

RUN apk add --no-cache git ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# Copy built static assets into backend/web/dist for go:embed
COPY --from=frontend-builder /app/frontend/dist ./web/dist

ARG VERSION=v0.1.0
ARG COMMIT=none
ARG DATE=unknown

ENV GOEXPERIMENT=simd

RUN CGO_ENABLED=0 go build \
    -tags wgpu24 \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /twsnmpneo ./cmd/twsnmpneo

# Stage 3: Minimal Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

# Create data directory for bbolt db, parquet columnar logs, and private PKI
RUN mkdir -p /data && chmod 755 /data
VOLUME [ "/data" ]

COPY --from=backend-builder /twsnmpneo /usr/local/bin/twsnmpneo

# Exposed ports:
# 8080: Web UI & REST API & MCP Server
# 8082/tcp: Optional PKI OCSP/SCEP/CRL HTTP services (enable in PKI settings)
# 8083/tcp: Optional ACME HTTPS service (enable in PKI settings)
# 8443/tcp: Optional ACME HTTPS service when configured with --acme-url
# 514/udp, 514/tcp: Syslog
# 162/udp: SNMP Trap
# 2055/udp: NetFlow
EXPOSE 8080/tcp 8082/tcp 8083/tcp 8443/tcp 514/udp 514/tcp 162/udp 2055/udp

ENTRYPOINT [ "/usr/local/bin/twsnmpneo" ]
CMD [ "--datadir", "/data" ]
