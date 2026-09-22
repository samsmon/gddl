# Google Drive Downloader (Desktop Web Edition)

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://golang.org)
[![Svelte Version](https://img.shields.io/badge/Svelte-5%20(Runes)-FF3E00?style=flat&logo=svelte)](https://svelte.dev)
[![Vite](https://img.shields.io/badge/Vite-6.x-646CFF?style=flat&logo=vite)](https://vitejs.dev)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](https://www.docker.com)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS%20%7C%20Docker%20%7C%20NAS-green)](#server-deployment--external-storage-setup)

A high-performance, self-hosted desktop download manager for Google Drive with a native **qBittorrent & Internet Download Manager (IDM)** user interface. Built with a concurrent Go backend and a responsive Svelte 5 frontend. Features batch queuing, folder-to-ZIP compression, real-time file sync, SHA-256 integrity verification, Google session cookies for restricted files, and enterprise-grade Web UI authentication for headless server deployments.

---

## Quick View & Screenshots

### Active Transfers & Real-Time Monitoring
![Active Transfers & Real-Time Monitoring](docs/screenshots/transfer_list.png)
*Desktop-grade transfer grid featuring multi-select, sortable columns, real-time speed/ETA, expandable folder contents, and contextual action toolbar.*

### Clean Dashboard & Desktop Interface
![Clean Dashboard & Desktop Interface](docs/screenshots/empty_dashboard.png)
*Modern dark theme with native application menu bar (File, Edit, View, Tools, Help), category sidebar, and Jellyfin-style destination browser.*

---

## Core Features

- **Concurrent Multi-Worker Engine**: Configurable parallel workers (1 to 5) powered by lightweight Go goroutines.
- **Bypass Virus Scan Prompts**: Automatically extracts Google Drive confirmation tokens for large files (>100MB).
- **Smart Folder & ZIP Compression**: Paste any public or private Google Drive folder link. Choose between downloading as a subfolder or compressing all folder contents locally into a verified `.zip` archive with real-time compression progress.
- **Conflict & Duplicate Resolution**: Automatically checks for existing files on disk or duplicate tasks in the queue when pasting links. Provides 3 options: Keep both with auto-suffix `(2)`, Overwrite & Replace, or Re-monitor & Verify integrity with resume support.
- **Real-Time File Presence & Missing Sync**: If files are moved, renamed, or deleted from disk, the application immediately updates their status to `MISSING / MOVED` and provides a 1-click re-download button.
- **Google Cookie Session (Restricted Files)**: Download private files, shared team drives, or files hitting "Download quota exceeded" by pasting your session cookie into the secure Tools menu.
- **Server-Ready Web UI Authentication**: Built-in authentication (qBittorrent-style) protecting your server or VPS from unauthorized access. Supports environment variables (`AUTH_ENABLED`, `AUTH_USER`, `AUTH_PASS`).
- **External HDD & NAS Auto-Detection**: The Jellyfin-style storage browser automatically detects external hard drives and mount points (`/mnt`, `/media`, `/volume1-9`, `D:\`, `E:\`).
- **In-App System Logs**: Real-time diagnostic logger with error filtering, clipboard copying, and live refresh to troubleshoot rate limits or Google network timeouts.
- **Keyboard Shortcuts**: Fully operable via desktop keybindings (`Ctrl+N`, `Ctrl+A`, `Del`, `Shift+Del`, `Space`, `Esc`).

---

## Server Deployment & External Storage Setup

### Option 1: Docker Compose (Recommended for Servers & NAS)

Create a `docker-compose.yml` file on your server:

```yaml
version: '3.8'

services:
  gdrive-downloader:
    image: gdrive-downloader:latest
    build: .
    container_name: gdrive-downloader
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - HOST=0.0.0.0
      - DOWNLOAD_DIR=/downloads
      - CONFIG_DIR=/config
      - AUTH_ENABLED=true
      - AUTH_USER=admin
      - AUTH_PASS=adminadmin
      # Optional: Add extra paths for the storage browser
      # - EXTRA_DRIVES=/mnt/external_hdd,/mnt/backup
    volumes:
      # 1. State & Configuration (downloads.json & config.json survive reboots)
      - ./config:/config

      # 2. Main download directory
      - ./downloads:/downloads

      # 3. Mount External Hard Drives, USB drives, or NAS Shares:
      # Linux / Homelab:
      # - /mnt/storage:/mnt/storage:rw
      # - /media/usb:/media/usb:rw
      #
      # Synology NAS (DSM):
      # - /volume1/downloads:/volume1/downloads:rw
      #
      # Unraid / TrueNAS:
      # - /mnt/user/downloads:/mnt/user/downloads:rw
```

Start the service:
```bash
docker compose up -d
```
Access the dashboard at `http://<your-server-ip>:8080`.

---

### Option 2: Linux Systemd Service (VPS / Bare-Metal)

1. Compile the binary or copy `gdrive-downloader` to `/opt/gdrive-downloader/`.
2. Create the systemd service file `/etc/systemd/system/gdrive-downloader.service`:

```ini
[Unit]
Description=Google Drive Downloader Server Daemon
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gdrive-downloader
ExecStart=/opt/gdrive-downloader/gdrive-downloader -port 8080 -dir /mnt/storage/downloads -config /opt/gdrive-downloader/config
Restart=on-failure
RestartSec=5s

# Environment Variables
Environment=PORT=8080
Environment=HOST=0.0.0.0
Environment=AUTH_ENABLED=true
Environment=DOWNLOAD_DIR=/mnt/storage/downloads
Environment=CONFIG_DIR=/opt/gdrive-downloader/config

[Install]
WantedBy=multi-user.target
```

3. Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gdrive-downloader
sudo systemctl status gdrive-downloader
```

---

### Option 3: External Hard Drive Mounting Guide (Linux / Raspberry Pi)

To make external USB hard drives or external storage permanently accessible:

1. Identify the drive UUID:
   ```bash
   sudo blkid
   ```
2. Create a mount point:
   ```bash
   sudo mkdir -p /mnt/external_hdd
   ```
3. Add to `/etc/fstab` for auto-mounting on boot:
   ```text
   UUID=YOUR-UUID-HERE  /mnt/external_hdd  ext4  defaults,nofail  0  2
   ```
4. Mount the drive:
   ```bash
   sudo mount -a
   ```
5. Google Drive Downloader will automatically detect `/mnt/external_hdd` in the **Browse...** destination picker.

---

## Configuration Reference

You can configure the application via command-line arguments or environment variables:

| Flag | Environment Variable | Default | Description |
| :--- | :--- | :--- | :--- |
| `-port` | `PORT` | `8080` | Port to listen on. |
| `-host` | `HOST` | `0.0.0.0` | IP address to bind to (`0.0.0.0` allows LAN/WAN access). |
| `-dir` | `DOWNLOAD_DIR` | `~/Downloads` | Default folder where files and ZIP archives are stored. |
| `-config` | `CONFIG_DIR` | `.` | Directory where `config.json` and `downloads.json` are stored. |
| `-web` | `WEB_DIR` | auto-detected | Path to the frontend `dist` directory. |
| `-auth` | `AUTH_ENABLED` | `true` | Enable or disable Web UI login authentication (`true`/`false`). |
| `-user` | `AUTH_USER` | `admin` | Initial Web UI username for headless deployments. |
| `-pass` | `AUTH_PASS` | `adminadmin` | Initial Web UI password for headless deployments. |
| - | `EXTRA_DRIVES` | `""` | Comma-separated list of additional storage mount paths. |

---

## REST & Streaming API Documentation

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/events` | `GET` | **Server-Sent Events (SSE)** stream delivering live download queue state, speed, ETA, and progress. |
| `/api/downloads` | `GET` | Get all active, queued, and completed downloads as JSON. |
| `/api/downloads` | `POST` | Add one or multiple Google Drive URLs. Accepts `{ links: [], target_folder: "", zip_mode: true, conflict_resolutions: {} }`. |
| `/api/downloads/precheck` | `POST` | Fast conflict precheck against active queue and disk before adding. |
| `/api/downloads/{id}/start` | `POST` | Start / resume task. |
| `/api/downloads/{id}/pause` | `POST` | Pause running download. |
| `/api/downloads/{id}/restart`| `POST` | Restart download from the beginning. |
| `/api/downloads/{id}` | `DELETE` | Delete task from list. Supports optional disk deletion: `?delete_file=true`. |
| `/api/downloads/{id}/check` | `POST` | Check real-time file presence on disk. |
| `/api/downloads/check-all` | `POST` | Check disk presence for all files in list. |
| `/api/downloads/clear` | `POST` | Clear completed downloads from list. |
| `/api/folders/resolve` | `POST` | Inspect Google Drive folder contents before adding: `{ url: "..." }`. |
| `/api/fs/browse` | `GET` | Browse server filesystem folders and drives: `?path=/mnt`. |
| `/api/config` | `GET` / `POST` | Read or update settings (Target folder, Concurrency, Google Cookie). |
| `/api/auth/status` | `GET` | Check whether authentication is enabled and current login state. |
| `/api/auth/login` | `POST` | Authenticate into Web UI: `{ username: "", password: "" }`. |
| `/api/auth/logout` | `POST` | Terminate session and invalidate auth token. |
| `/api/auth/password` | `POST` | Change admin username and password. |
| `/api/auth/toggle` | `POST` | Enable or disable Web UI authentication. |
| `/api/gdrive/cookie` | `POST` | Save Google session cookie for quota bypass. |
| `/api/gdrive/logout` | `POST` | Clear saved Google session cookie. |
| `/api/logs` | `GET` / `DELETE` | Fetch or clear execution and error logs. |

---

## Desktop Keyboard Shortcuts

| Shortcut | Action |
| :--- | :--- |
| <kbd>Ctrl</kbd> + <kbd>N</kbd> | Open the **Add Links** modal |
| <kbd>Ctrl</kbd> + <kbd>A</kbd> | Select all items in current category view |
| <kbd>Esc</kbd> | Deselect all items / Close active modal or context menu |
| <kbd>Space</kbd> | Toggle Start / Pause on selected downloads |
| <kbd>Del</kbd> | Open delete prompt for selected downloads |
| <kbd>Shift</kbd> + <kbd>Del</kbd> | Open delete prompt defaulted to **Remove from disk** |
| <kbd>Ctrl</kbd> + Click | Toggle individual row selection |
| <kbd>Shift</kbd> + Click | Select continuous range of rows |
| Right-Click | Open desktop context menu with full controls |

---

## Local Development & Build from Source

### Prerequisites
- **Go 1.22+**: [golang.org/dl](https://golang.org/dl/)
- **Node.js 20+** & **npm**: [nodejs.org](https://nodejs.org/)

### 1. Build the Frontend
```bash
cd frontend
npm install
npm run build
```

### 2. Compile the Backend Binary
```bash
cd ../backend
go build -o gdrive-downloader.exe .
```

### 3. Run the Application
```bash
# On Windows:
.\gdrive-downloader.exe

# On Linux / macOS:
./gdrive-downloader
```

Open `http://localhost:8080` in your web browser.

---

## License
This project is open-source under the [MIT License](LICENSE).
