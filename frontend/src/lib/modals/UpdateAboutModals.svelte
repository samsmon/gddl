<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Check for Updates -->
  {#if app.showUpdateModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showUpdateModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showUpdateModal = false)}>
      <div class="modal-window" style="max-width: 440px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
            </svg>
            <span style="font-weight: 700;">Check for Updates</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showUpdateModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body" style="text-align: center; padding: 1.5rem 1rem;">
          {#if app.updateChecking}
            <div class="mini-spinner" style="margin: 0 auto 12px auto; width: 24px; height: 24px;"></div>
            <div style="font-size: 0.9rem; color: var(--text-muted);">{app.updateStatus}</div>
          {:else}
            <div style="margin: 0 auto 10px auto; width: 44px; height: 44px; display: flex; align-items: center; justify-content: center; border-radius: 50%; background: var(--table-row-alt); border: 1px solid var(--border-color);">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </div>
            <h3 style="margin: 0 0 6px 0; font-size: 1.05rem; color: var(--text-main);">You're Up to Date!</h3>
            <p style="font-size: 0.82rem; color: var(--text-muted); margin: 0 0 12px 0;">
              Google Drive Client <strong>v1.2.0 (Persistent Desktop Edition)</strong> is currently the latest version.
            </p>
            <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 10px; font-size: 0.78rem; text-align: left; line-height: 1.5;">
              <strong>What's New in v1.2.0:</strong>
              <ul style="margin: 6px 0 0 16px; padding: 0; color: var(--text-muted);">
                <li>Desktop application menu bar (File, Edit, View, Tools, Help)</li>
                <li>Persistent real-time download history (downloads.json)</li>
                <li>Real-time file presence detection (Missing / Moved / Deleted)</li>
                <li>One-click re-download for missing files</li>
              </ul>
            </div>
          {/if}
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => app.showUpdateModal = false}>Close</button>
          <a class="btn btn-primary" href="https://github.com/samsmon/gddl/releases" target="_blank" rel="noopener noreferrer" style="text-decoration: none;">
            View Releases
          </a>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: About GDrive Downloader -->
  {#if app.showAboutModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showAboutModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showAboutModal = false)}>
      <div class="modal-window" style="max-width: 460px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="16" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
            <span style="font-weight: 700;">About GDrive Downloader</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showAboutModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body" style="text-align: center; padding: 1.5rem 1rem;">
          <div style="width: 52px; height: 52px; border-radius: 12px; background: rgba(56, 139, 253, 0.12); border: 1px solid rgba(56, 139, 253, 0.3); display: flex; align-items: center; justify-content: center; margin: 0 auto 12px auto;">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
              <path d="M12 12v9"></path>
              <path d="m8 17 4 4 4-4"></path>
            </svg>
          </div>
          <h3 style="margin: 0 0 4px 0; font-size: 1.15rem; color: var(--text-main);">Google Drive Downloader</h3>
          <div style="font-size: 0.8rem; color: var(--text-main); font-weight: 600; margin-bottom: 12px;">v1.2.0 • Persistent Desktop Edition</div>
          <p style="font-size: 0.82rem; color: var(--text-muted); line-height: 1.5; margin: 0 0 14px 0;">
            A high-performance Google Drive downloader featuring multi-worker queuing, ZIP compression, chunked streaming, file integrity verification, and qBittorrent-style server authentication.
          </p>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; font-size: 0.75rem; text-align: left; background: var(--table-row-alt); padding: 10px 14px; border-radius: 6px; border: 1px solid var(--border-subtle); margin-bottom: 12px;">
            <div><span style="color: var(--text-dim);">Backend:</span> <strong style="color: var(--text-main);">Go 1.23 Concurrency</strong></div>
            <div><span style="color: var(--text-dim);">Frontend:</span> <strong style="color: var(--text-main);">Svelte 5 Runes + Vite</strong></div>
            <div><span style="color: var(--text-dim);">Storage:</span> <strong style="color: var(--text-main);">Realtime downloads.json</strong></div>
            <div><span style="color: var(--text-dim);">License:</span> <strong style="color: var(--text-main);">MIT Open Source</strong></div>
          </div>
        </div>

        <div class="modal-footer">
          <a class="btn btn-secondary" href="https://github.com/samsmon/gddl" target="_blank" rel="noopener noreferrer" style="text-decoration: none;">
            GitHub Repository
          </a>
          <button class="btn btn-primary" onclick={() => app.showAboutModal = false}>OK</button>
        </div>
      </div>
    </div>
  {/if}

<!-- In-App Confirmation Modal -->
{#if app.confirmDialog.show}
  <div
    class="modal-overlay"
    style="z-index: 1000;"
    role="presentation"
    onclick={() => app.confirmDialog.show = false}
    onkeydown={(e) => e.key === 'Escape' && (app.confirmDialog.show = false)}
  >
    <div
      class="modal-window confirm-modal-window"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      style="max-width: 440px;"
    >
      <div class="modal-header">
        <span>{app.confirmDialog.title || 'Confirm Action'}</span>
        <button class="modal-close" aria-label="Close" onclick={() => app.confirmDialog.show = false}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>
      <div class="modal-body" style="padding: 18px 20px;">
        <p style="margin: 0; font-size: 13px; line-height: 1.5; color: var(--text-main); white-space: pre-line;">{app.confirmDialog.message}</p>
      </div>
      <div class="modal-footer" style="display: flex; justify-content: flex-end; gap: 8px;">
        <button class="btn btn-secondary" onclick={() => app.confirmDialog.show = false}>Cancel</button>
        <button class="btn btn-{app.confirmDialog.confirmType || 'primary'}" onclick={app.handleConfirmAction}>
          {app.confirmDialog.confirmText || 'Confirm'}
        </button>
      </div>
    </div>
  </div>
{/if}
