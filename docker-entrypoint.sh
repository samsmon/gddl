#!/bin/sh
set -e

# Start Cloudflare WARP daemon in background if installed
if command -v warp-svc >/dev/null 2>&1; then
    echo "[Entrypoint] Starting Cloudflare WARP daemon (warp-svc)..."
    # Ensure directory exists for socket & config
    mkdir -p /var/lib/cloudflare-warp /run/cloudflare-warp
    warp-svc >/var/log/warp-svc.log 2>&1 &
    WARP_PID=$!

    # Wait up to 5 seconds for warp-svc to create its local socket
    for i in $(seq 1 10); do
        if warp-cli --accept-tos status >/dev/null 2>&1 || [ -S /run/cloudflare-warp/warp_service ]; then
            echo "[Entrypoint] Cloudflare WARP daemon is ready."
            break
        fi
        sleep 0.5
    done

    # Attempt initial automatic registration if not registered
    echo "[Entrypoint] Initializing WARP client registration..."
    warp-cli --accept-tos registration new >/dev/null 2>&1 || warp-cli --accept-tos register >/dev/null 2>&1 || true
    warp-cli --accept-tos mode proxy >/dev/null 2>&1 || true
fi

# Graceful shutdown handler
cleanup() {
    echo "[Entrypoint] Received termination signal, shutting down..."
    if [ -n "$WARP_PID" ]; then
        kill "$WARP_PID" 2>/dev/null || true
    fi
    exit 0
}
trap cleanup TERM INT


# Exec main application
exec /app/gdrive-downloader "$@"
