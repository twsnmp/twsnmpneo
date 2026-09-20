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
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/backend

RUN apk add --no-cache git ca-certificates tzdata

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# Copy built static assets into backend/web/dist for go:embed
COPY --from=frontend-builder /app/frontend/dist ./web/dist

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

RUN CGO_ENABLED=0 go build \
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
# 514/udp, 514/tcp: Syslog
# 162/udp: SNMP Trap
# 2055/udp: NetFlow
EXPOSE 8080/tcp 514/udp 514/tcp 162/udp 2055/udp

ENTRYPOINT [ "/usr/local/bin/twsnmpneo" ]
CMD [ "--datadir", "/data" ]
