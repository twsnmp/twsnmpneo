# syntax=docker/dockerfile:1

ARG NODE_IMAGE=mirror.gcr.io/library/node:22-alpine
ARG GO_IMAGE=mirror.gcr.io/library/golang:alpine
ARG BASE_IMAGE=mirror.gcr.io/library/alpine:3.20

# Stage 1: Build Frontend SPA (Runs natively on host builder platform)
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS frontend-builder
WORKDIR /app/frontend

RUN corepack enable && corepack prepare pnpm@latest --activate

COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts

COPY frontend/ ./
RUN pnpm run build

# Stage 2: Build Backend Go binary (Runs natively using Go's fast cross-compiler)
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS backend-builder
WORKDIR /app/backend

RUN apk add --no-cache git ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# Copy built static assets into backend/web/dist for go:embed
COPY --from=frontend-builder /app/backend/web/dist ./web/dist

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=v0.1.0
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -tags wgpu24 \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /twsnmpneo ./cmd/twsnmpneo

# Stage 3: Minimal Runtime
FROM ${BASE_IMAGE}
RUN apk add --no-cache ca-certificates tzdata gcompat libc6-compat

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
