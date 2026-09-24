# Stage 1: Build Frontend (Svelte 5 + Vite)
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Backend (Go)
FROM golang:1.23-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/gdrive-downloader main.go

# Stage 3: Production Runtime with Official Cloudflare WARP Client
# Using debian:bookworm-slim for official Cloudflare glibc package support
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install essential dependencies, official Cloudflare GPG key, and cloudflare-warp package
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    gnupg \
    tzdata \
    mailcap \
    procps \
    && curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor --output /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg \
    && echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ bookworm main" | tee /etc/apt/sources.list.d/cloudflare-client.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends cloudflare-warp \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binary and frontend assets
COPY --from=backend-builder /app/gdrive-downloader /app/gdrive-downloader
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist
COPY docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh /app/gdrive-downloader

# Default directories for persistence and downloads
RUN mkdir -p /downloads /config /var/lib/cloudflare-warp /run/cloudflare-warp
VOLUME ["/downloads", "/config"]

# Environment variables
ENV PORT=8080 \
    HOST=0.0.0.0 \
    DOWNLOAD_DIR=/downloads \
    CONFIG_DIR=/config \
    WEB_DIR=/app/frontend/dist \
    AUTH_ENABLED=true

EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
