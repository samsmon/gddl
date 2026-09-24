<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
  import SecuritySettingsSection from './SecuritySettingsSection.svelte';
</script>

  <!-- Modal: Options / Settings -->
  {#if app.showSettingsModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showSettingsModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showSettingsModal = false)}>
      <div class="modal-window" style="max-width: 620px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>Options & Preferences</span>
          <button class="modal-close" aria-label="Close" onclick={() => app.showSettingsModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <span class="form-title">Default Destination Folder:</span>
            <div class="path-picker-row">
              <input type="text" bind:value={app.defaultFolder} />
              <button class="btn-browse" onclick={() => app.openPickerFor('settings')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <span>Browse...</span>
              </button>
            </div>
            <span class="form-hint">Supports local drives (C:\, D:\) and mapped Homelab network shares (e.g. Z:\Music).</span>
          </div>

          <div class="form-group" style="margin-top: 1rem;">
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
              <span class="form-title">Maximum Concurrent Downloads:</span>
              {#if app.maxConcurrency <= 2}
                <span class="concurrency-badge badge-safe">Safe & Stable</span>
              {:else if app.maxConcurrency === 3}
                <span class="concurrency-badge badge-optimal">Recommended</span>
              {:else if app.maxConcurrency === 4}
                <span class="concurrency-badge badge-fast">High Speed / Caution</span>
              {:else}
                <span class="concurrency-badge badge-aggressive">Aggressive / High Risk</span>
              {/if}
            </div>

            <div style="display: flex; align-items: center; gap: 10px; margin-top: 6px;">
              <input type="number" min="1" max="5" bind:value={app.maxConcurrency} style="width: 80px;" />
              <span class="form-hint" style="margin: 0;">Workers active simultaneously (1 to 5).</span>
            </div>

            <!-- Dynamic Risk & Behavior Hint Card -->
            <div class="concurrency-hint-card {app.maxConcurrency <= 2 ? 'hint-safe' : app.maxConcurrency === 3 ? 'hint-optimal' : app.maxConcurrency === 4 ? 'hint-warn' : 'hint-danger'}">
              {#if app.maxConcurrency <= 2}
                <div class="hint-header">Safe Profile (1 - 2 Workers)</div>
                <p>Minimal risk of IP throttling. Ideal for continuous background downloads and standard public Google Drive links without cookies.</p>
              {:else if app.maxConcurrency === 3}
                <div class="hint-header">Optimal Balance (3 Workers - Default)</div>
                <p>Optimal balance between speed and reliability. Safely handles simultaneous Discord CDN and Google Drive streams with near-zero rate limiting.</p>
              {:else if app.maxConcurrency === 4}
                <div class="hint-header">High Concurrency Caution (4 Workers)</div>
                <p><strong>Discord CDN:</strong> Fully safe and protected by internal anti-429 jitter.<br/>
                <strong>Google Drive:</strong> Unauthenticated public downloads (>100MB) can trigger temporary IP quotas (Download quota exceeded / HTTP 403). Adding a Google session cookie in Settings is recommended to avoid rate limits.</p>
              {:else}
                <div class="hint-header">Aggressive Concurrency (5 Workers)</div>
                <p>High probability of Google Drive IP-level connection throttling or temporary 24-hour file lockouts unless logged in with valid cookies. Discord CDN may experience connection queueing.</p>
              {/if}
            </div>
          </div>

          <!-- Multi-Chunk Parallel Segmented Downloader Section -->
          <div class="form-group" style="margin-top: 1rem;">
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
              <span class="form-title">Parallel Chunks per Download (IDM-Style):</span>
              {#if app.chunksPerDownload <= 1}
                <span class="concurrency-badge badge-safe">Single Stream (10 MB/s Cap)</span>
              {:else if app.chunksPerDownload === 2}
                <span class="concurrency-badge badge-optimal">Dual Stream (~20 MB/s)</span>
              {:else if app.chunksPerDownload === 4}
                <span class="concurrency-badge badge-safe">Recommended (4 Chunks)</span>
              {:else if app.chunksPerDownload === 8}
                <span class="concurrency-badge badge-fast">High Speed (8 Chunks)</span>
              {:else}
                <span class="concurrency-badge badge-aggressive">Extreme (16 Chunks)</span>
              {/if}
            </div>

            <div style="display: flex; align-items: center; gap: 10px; margin-top: 6px;">
              <select
                bind:value={app.chunksPerDownload}
                style="padding: 5px 8px; border-radius: 4px; background: var(--input-bg); color: var(--text-main); border: 1px solid var(--border-color); font-size: 12px; width: 260px;"
              >
                <option value={1}>1 Chunk (Single Stream - 10 MB/s Cap)</option>
                <option value={2}>2 Chunks (Dual Stream - Up to ~20 MB/s)</option>
                <option value={4}>4 Chunks (Recommended - Fast &amp; Balanced)</option>
                <option value={8}>8 Chunks (High Speed - Up to ~80 MB/s)</option>
                <option value={16}>16 Chunks (Extreme - Maximum Bandwidth)</option>
              </select>
              <span class="form-hint" style="margin: 0;">Parallel HTTP Range connections per file (&gt;10 MB).</span>
            </div>

            <div class="concurrency-hint-card hint-optimal" style="margin-top: 8px;">
              <div class="hint-header">Google Drive 10 MB/s Speed Cap Bypass</div>
              <p>Google Drive throttles individual TCP streams to ~10 MB/s. Dividing files larger than 10 MB into multiple parallel byte range segments bypasses this single-stream bottleneck (just like Internet Download Manager) without triggering Google quota restrictions.</p>
            </div>
          </div>

          <!-- Divider -->
          <div style="border-top: 1px solid var(--border-color); margin: 1.25rem 0 1rem 0;"></div>

          <!-- Anti-Throttle & Cloudflare WARP Auto-Bypass Section -->
          <div class="form-group">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="form-title" style="font-weight: 600; color: var(--accent-blue);">Anti-Throttle &amp; Cloudflare WARP Auto-Bypass</span>
              {#if app.warpStatus.proxy_active}
                <span class="concurrency-badge badge-optimal">Proxy Active (Rotations: {app.warpStatus.rotation_count})</span>
              {:else if app.autoWarpEnabled}
                <span class="concurrency-badge badge-balanced">Watchdog Armed (&lt; {app.autoWarpMinSpeedMB} MB/s)</span>
              {:else}
                <span class="concurrency-badge badge-safe">Direct Connection</span>
              {/if}
            </div>
            <span class="form-hint" style="margin-bottom: 0.75rem;">
              Automatically detects CDN IP bandwidth throttling (e.g. speed dropping to 50 KB/s), enables Cloudflare WARP Local SOCKS5 Proxy (or custom proxy pool), rotates WireGuard keys when a proxy IP is also throttled, and seamlessly reconnects active chunk streams at the exact byte offset.
            </span>

            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-bottom: 0.75rem;">
              <input type="checkbox" bind:checked={app.autoWarpEnabled} style="width: auto;" />
              <span style="font-weight: 500;">Enable Automatic Speed Watchdog &amp; Proxy Rotation</span>
            </label>

            <div style="background: var(--table-row-alt); padding: 12px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: 10px;">
              <div style="display: grid; grid-template-columns: 210px 1fr; gap: 8px; align-items: center;">
                <span style="font-size: 12px; color: var(--text-muted);">Min Speed Trigger (MB/s):</span>
                <div style="display: flex; align-items: center; gap: 8px;">
                  <input type="number" step="0.5" min="0.5" max="100" bind:value={app.autoWarpMinSpeedMB} style="padding: 4px 8px; width: 90px;" />
                  <span class="form-hint" style="margin: 0;">Triggers bypass/rotate if total speed stays below this for 7s.</span>
                </div>

                <span style="font-size: 12px; color: var(--text-muted);">WARP Local SOCKS5 Port:</span>
                <div style="display: flex; align-items: center; gap: 8px;">
                  <input type="number" min="1024" max="65535" bind:value={app.warpProxyPort} style="padding: 4px 8px; width: 90px;" />
                  <span class="form-hint" style="margin: 0;">{app.warpStatus.installed ? `warp-cli detected (${app.warpStatus.warp_state || 'Ready'})` : 'warp-cli not detected (uses Custom Proxy if set)'}</span>
                </div>

                <span style="font-size: 12px; color: var(--text-muted);">Custom Proxy Pool (Optional):</span>
                <input
                  type="text"
                  bind:value={app.customProxyURL}
                  placeholder="socks5://127.0.0.1:40000, http://user:pass@ip:port (comma-separated)"
                  style="padding: 4px 8px; font-family: monospace; font-size: 11px;"
                />
              </div>

              <div style="display: flex; align-items: center; justify-content: space-between; padding-top: 6px; border-top: 1px solid var(--border-subtle); font-size: 11px; color: var(--text-muted);">
                <span>
                  Current Route: <strong style="color: var(--text-main);">{app.warpStatus.proxy_active ? (app.warpStatus.active_proxy || `socks5://127.0.0.1:${app.warpProxyPort}`) : 'Direct Connection'}</strong>
                  {#if app.warpStatus.last_trigger_reason}
                    &bull; Last Event: <em>{app.warpStatus.last_trigger_reason}</em>
                  {/if}
                </span>
                <div style="display: flex; gap: 6px;">
                  <button type="button" class="btn-secondary" style="padding: 3px 10px; font-size: 11px;" onclick={app.toggleWarpProxyMode} disabled={app.isRotatingWarp}>
                    {app.warpStatus.proxy_active ? 'Disable Proxy' : 'Enable Proxy Now'}
                  </button>
                  <button type="button" class="btn-primary" style="padding: 3px 10px; font-size: 11px;" onclick={app.rotateWarpIP} disabled={app.isRotatingWarp}>
                    {app.isRotatingWarp ? 'Rotating IP...' : 'Rotate IP Now'}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <!-- Divider -->
          <div style="border-top: 1px solid var(--border-color); margin: 1.25rem 0 1rem 0;"></div>

          <SecuritySettingsSection />
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => app.showSettingsModal = false}>Cancel</button>
          <button class="btn btn-primary" onclick={app.saveConfig}>Save Options</button>
        </div>
      </div>
    </div>
  {/if}
