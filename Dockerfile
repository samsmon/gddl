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

# Stage 3: Minimal Production Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata mailcap
WORKDIR /app

# Copy binary and frontend assets
COPY --from=backend-builder /app/gdrive-downloader /app/gdrive-downloader
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Default directories for persistence and downloads
RUN mkdir -p /downloads /config
VOLUME ["/downloads", "/config"]

# Environment variables
ENV PORT=8080 \
    HOST=0.0.0.0 \
    DOWNLOAD_DIR=/downloads \
    CONFIG_DIR=/config \
    WEB_DIR=/app/frontend/dist \
    AUTH_ENABLED=true

EXPOSE 8080

ENTRYPOINT ["/app/gdrive-downloader"]
