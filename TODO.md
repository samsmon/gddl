# GDDL - Project Roadmap & Next Tasks

## 📋 Backlog / Planned Features

### 1. 🔄 Rclone Token Direct Import ("Login with Rclone" / Acefile style)
- **Goal:** Allow users to connect Google Drive without creating their own Google Cloud Project, Client ID, or Client Secret.
- **Workflow:**
  - User runs `rclone authorize "drive"` on their local machine.
  - User copies the resulting JSON token:
    ```json
    {"access_token":"ya29...","token_type":"Bearer","refresh_token":"1//0g...","expiry":"..."}
    ```
  - User pastes it into GDDL in a dedicated "Import Rclone Token" input.
  - GDDL parses and saves the token to `config.json`, enabling automated `ggdl_temp` quota bypass immediately.
- **Components to update:**
  - `backend/pkg/server/server.go`: Endpoint `POST /api/gdrive/oauth/token-import`
  - `backend/pkg/gdrive/oauth.go`: Method `ImportRawToken(jsonStr string)`
  - `frontend/src/App.svelte`: Add "Login via Rclone Token" tab/box with instructions.

---

### 2. 🌐 Full Internationalization / Translate Indonesian Text to English
- **Goal:** Standardize all user-facing text and backend responses to professional, clean English.
- **Tasks:**
  - **Frontend UI (`frontend/src/App.svelte` & components):**
    - Account Modal: Translate OAuth tab & Cookie Pool tab labels, guides, and button tooltips.
    - Settings Modal: Translate download folder, concurrency, and theme options.
    - Menubar & Action Bars: Check for any remaining Indonesian tooltips or strings.
    - Status badges, error banners, and conflict resolution modals.
    - Help accordion: "Panduan 1 Menit" -> "1-Minute Quick Setup Guide".
  - **Backend Messages:**
    - Audit all error messages in `backend/pkg/server/server.go`, `downloader.go`, `oauth.go`, and `bypass.go`.
    - Ensure clean, consistent JSON error responses in English.

---

### 3. 📖 Documentation & Setup Tips
- Add a tip in the OAuth setup guide indicating that switching Publishing Status from **Testing** to **In Production (Publish App)** in Google Cloud Console allows *any* Google account to log in without being manually registered as a Test User.

---

*Last Updated: 2026-09-24*
