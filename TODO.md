# GDDL - Project Roadmap & Completed Milestones

## ✅ Completed Milestones

### 1. ⚡ IDM-Style Multi-Chunk / Parallel Segmented Downloader
- **Implemented:** Parallel HTTP Range segmented downloading for files >10 MB using RFC 7233 byte ranges and zero-merging `WriteAt`.
- **Capabilities:**
  - Bypasses Google Drive's 10 MB/s single-stream bottleneck safely and effectively.
  - Fully integrated in both direct downloads (`downloader.go`) and quota bypass clones (`bypass.go`).
  - Configurable chunk count in Settings Modal (1, 2, 4, 8, 16 streams, default 4).
  - Pre-allocates destination files with `Truncate` to minimize disk fragmentation and allow parallel chunk writes.
  - Comprehensive unit test with mock HTTP Range server (`chunked_test.go`).

### 2. 🔄 Rclone Token Direct Import ("Login with Rclone" / Acefile Style)
- **Implemented:** Dedicated direct token import for Google Drive OAuth.
- **Workflow:**
  - User runs `rclone authorize "drive"` on any terminal.
  - User pastes the resulting JSON token (or snippet from `rclone.conf`).
  - GDDL validates and parses the token, auto-resolves account email via Google UserInfo API, and saves it into `config.json`.
  - Enables instant `ggdl_temp` quota bypass without requiring users to configure their own Google Cloud Project or Test Users.
  - Supported via `POST /api/gdrive/oauth/import-token` and frontend Account Modal Option 1.

### 3. 🌐 Full English Internationalization
- **Implemented:** Standardized 100% of user-facing UI labels, modals, hints, guide accordions, status banners, and code comments to clean, professional English across `frontend/src/App.svelte` and backend services.
- **Documentation:** Added clear instructions and setup tips in the in-app OAuth modal, including how switching Google Cloud publishing status from **Testing** to **In production (Publish App)** allows any Google account to authenticate without being added as a Test User.

### 4. 🛡️ Download Reliability & Error Logging
- Replaced 60-second HTTP client body timeout with indefinite streaming client (`Timeout: 0`) in `bypass.go`, eliminating `context deadline exceeded` errors on long downloads.
- Added comprehensive logger calls (`logger.Errorf`) for single-file download failures in `queue/manager.go` to ensure all errors appear in real-time UI logs.

---

## 📋 Backlog / Next Ideas
- [ ] Multi-thread speed graphs / chunk visualization widget in detail drawer.
- [ ] Auto-refresh token health indicator in status bar.
- [ ] Optional proxy / SOCKS5 support for segmented streams.

---

*Last Updated: 2026-09-24*
