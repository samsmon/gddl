<script>
  import { app } from '../state/appState.svelte.js';
</script>

<!-- Modal: Check for Updates -->
{#if app.showUpdateModal}
  <div class="modal-overlay" role="presentation" onclick={() => app.showUpdateModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showUpdateModal = false)}>
    <div class="modal-window" style="max-width: 520px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
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

      <div class="modal-body" style="padding: 18px 20px; display: flex; flex-direction: column; gap: 14px;">
        {#if app.updateChecking}
          <div style="text-align: center; padding: 2rem 1rem;">
            <div class="mini-spinner" style="margin: 0 auto 12px auto; width: 28px; height: 28px;"></div>
            <div style="font-size: 0.9rem; color: var(--text-muted);">{app.updateStatus}</div>
          </div>
        {:else if app.updateError}
          <div style="background: rgba(248,81,73,0.1); border: 1px solid rgba(248,81,73,0.3); padding: 12px; border-radius: 6px; color: var(--accent-red); font-size: 12px;">
            <strong>Check Failed:</strong> {app.updateError}
          </div>
          <div style="text-align: center;">
            <button class="btn btn-secondary" onclick={() => app.checkForUpdates()}>Try Again</button>
          </div>
        {:else if app.updateInfo?.update_available}
          <!-- Update Available View -->
          <div style="display: flex; align-items: center; gap: 12px; background: rgba(56, 139, 253, 0.1); border: 1px solid rgba(56, 139, 253, 0.3); padding: 12px 14px; border-radius: 6px;">
            <div style="width: 38px; height: 38px; border-radius: 50%; background: var(--accent-blue); display: flex; align-items: center; justify-content: center; flex-shrink: 0; color: #fff;">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <line x1="12" y1="5" x2="12" y2="19"></line>
                <polyline points="19 12 12 19 5 12"></polyline>
              </svg>
            </div>
            <div>
              <div style="font-weight: 700; font-size: 13px; color: var(--text-main);">New Update Available!</div>
              <div style="font-size: 11.5px; color: var(--accent-blue); font-family: var(--font-mono);">
                v{app.updateInfo.latest_version} (Commit: {app.updateInfo.latest_commit})
              </div>
            </div>
          </div>

          <!-- Commit Message / Title -->
          <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 10px 12px;">
            <span style="font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-dim); font-weight: 700; display: block; margin-bottom: 4px;">Update Summary</span>
            <div style="font-size: 12px; font-weight: 600; color: var(--text-main); font-family: var(--font-mono); line-height: 1.4;">
              {app.updateInfo.latest_title || app.updateInfo.latest_message}
            </div>
          </div>

          <!-- Changelog -->
          {#if app.updateInfo.changelog && app.updateInfo.changelog.length > 0}
            <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 10px 12px;">
              <span style="font-size: 10.5px; text-transform: uppercase; letter-spacing: 0.05em; color: var(--text-dim); font-weight: 700; display: block; margin-bottom: 4px;">What's Changed</span>
              <ul style="margin: 4px 0 0 16px; padding: 0; font-size: 11.5px; color: var(--text-muted); line-height: 1.45;">
                {#each app.updateInfo.changelog as change}
                  <li>{change}</li>
                {/each}
              </ul>
            </div>
          {/if}

          <!-- Git Pull & Rebuild Instruction Card -->
          <div style="background: var(--input-bg); border: 1px solid var(--border-color); border-radius: 6px; padding: 12px; display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span style="font-weight: 600; font-size: 11.5px; color: var(--accent-green);">Pull &amp; Rebuild Instructions:</span>
              <button class="btn btn-secondary" onclick={app.copyUpdateCommand} style="font-size: 10.5px; padding: 3px 8px;">
                {app.copiedUpdateCmd ? '✓ Copied!' : 'Copy Commands'}
              </button>
            </div>
            <pre style="margin: 0; background: rgba(0,0,0,0.35); border: 1px solid var(--border-subtle); border-radius: 4px; padding: 8px 10px; font-family: var(--font-mono); font-size: 11px; color: #38bdf8; overflow-x: auto; line-height: 1.4;">git pull origin main
cd frontend &amp;&amp; npm run build
cd ../backend &amp;&amp; go build -o ../gddl.exe .</pre>
            <span style="font-size: 10px; color: var(--text-dim);">Run the commands above in terminal to update and restart your server.</span>
          </div>

        {:else}
          <!-- Up to Date View -->
          <div style="text-align: center; padding: 1rem 0;">
            <div style="margin: 0 auto 10px auto; width: 44px; height: 44px; display: flex; align-items: center; justify-content: center; border-radius: 50%; background: rgba(46, 160, 67, 0.15); border: 1px solid rgba(46, 160, 67, 0.35); color: var(--accent-green);">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </div>
            <h3 style="margin: 0 0 4px 0; font-size: 1.05rem; color: var(--text-main);">You're Up to Date!</h3>
            <p style="font-size: 0.82rem; color: var(--text-muted); margin: 0 0 14px 0;">
              Google Drive Client <strong>v{app.updateInfo?.current_version || '1.3.0'}</strong> (Commit: <code>{app.updateInfo?.current_commit || '1e382c5'}</code>) is the latest version.
            </p>

            {#if app.updateInfo?.changelog && app.updateInfo.changelog.length > 0}
              <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 10px 14px; text-align: left;">
                <span style="font-weight: 600; font-size: 11px; color: var(--text-dim); text-transform: uppercase; letter-spacing: 0.04em;">Active Release Features:</span>
                <ul style="margin: 6px 0 0 16px; padding: 0; color: var(--text-muted); font-size: 11.5px; line-height: 1.45;">
                  {#each app.updateInfo.changelog as log}
                    <li>{log}</li>
                  {/each}
                </ul>
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => app.showUpdateModal = false}>Close</button>
        <button class="btn btn-secondary" onclick={() => app.checkForUpdates()} disabled={app.updateChecking}>
          {app.updateChecking ? 'Checking...' : 'Check Again'}
        </button>
        <a class="btn btn-primary" href="https://github.com/samsmon/gddl/commits/main" target="_blank" rel="noopener noreferrer" style="text-decoration: none;">
          View GitHub Commits
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
        <div style="font-size: 0.8rem; color: var(--text-main); font-weight: 600; margin-bottom: 12px;">v1.3.0 • Persistent Desktop Edition</div>
        <p style="font-size: 0.82rem; color: var(--text-muted); line-height: 1.5; margin: 0 0 14px 0;">
          A high-performance Google Drive and Discord CDN downloader featuring multi-chunk parallel streaming, automated quota bypass, and qBittorrent-style server management.
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
