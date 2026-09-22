# AI Agent Fast-Track Guide & Architecture Context

This document is dedicated to AI coding assistants and autonomous agents working on this codebase. It provides immediate structural orientation without requiring full-codebase scanning.

## Architecture Overview

1. **Backend (`/backend`)**:
   - Pure Go HTTP service using standard library `net/http` with Go 1.22+ routing patterns (`mux.HandleFunc("METHOD /path", ...)`).
   - Core packages:
     - `pkg/server`: REST API endpoints, Server-Sent Events (SSE) broadcaster, auth & CORS middleware, filesystem browser.
     - `pkg/queue`: Download queue state machine (`Manager`), worker pool, disk persistence (`downloads.json`), pre-check and conflict resolver (`PrecheckDownloads`, `AddWithResolutions`).
     - `pkg/gdrive`: Google Drive streaming downloader, token extraction for >100MB confirmation prompts, HTTP Range resume support, folder crawler, and local ZIP packaging.
     - `pkg/auth`: Session token manager, password hasher, and configuration storage (`config.json`).
     - `pkg/logger`: Thread-safe runtime circular buffer for system and error logs.

2. **Frontend (`/frontend`)**:
   - Built with Svelte 5 (using `$state`, `$derived`, `$props` runes) and Vite.
   - Single Page Application styled with desktop-grade CSS variables (light and dark mode).
   - Strictly uses monochrome Google vector SVG icons with `currentColor`. No unicode emojis in the UI.

3. **Storage & State Persistence**:
   - State files (`downloads.json` and `config.json`) are stored in `CONFIG_DIR` (defaults to current directory).
   - Real-time disk sync verifies file presence on disk. If files are moved or deleted, status transitions to `missing`.
   - Conflict resolution pre-check detects existing files or queue items before adding new links.

## Key Source Files

- Backend main entry: `backend/main.go`
- API routing & FS browse: `backend/pkg/server/server.go`
- Queue & conflict engine: `backend/pkg/queue/manager.go`
- Downloader & resume engine: `backend/pkg/gdrive/downloader.go`
- Google Drive folder parser: `backend/pkg/gdrive/folder.go`
- File integrity validator: `backend/pkg/gdrive/integrity.go`
- Auth & config manager: `backend/pkg/auth/auth.go`
- Main Svelte desktop UI: `frontend/src/App.svelte`
- Storage picker component: `frontend/src/lib/FolderPicker.svelte`

## Developer & Agent Guidelines

- **Zero Emojis**: Do not add unicode emojis in code, UI text, labels, or documentation. Use clean typography and monochrome vector SVGs.
- **Cross-Platform Safety**: Always use `filepath.Join`, `filepath.Clean`, and cross-platform path handling for Windows, Linux, macOS, and Docker.
- **Build Verification**:
  - Frontend: `cd frontend && npm run build`
  - Backend: `cd backend && go build -o gdrive-downloader.exe .`
