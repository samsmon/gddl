<script>
  import { app } from '../state/appState.svelte.js';
</script>

<div class="settings-pane-header">
  <div class="settings-pane-title">
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
    </svg>
    <span>Anti-Throttle &amp; Cloudflare WARP</span>
  </div>
  {#if app.warpStatus.proxy_active}
    <span class="concurrency-badge badge-optimal">Proxy Active ({app.warpStatus.rotation_count})</span>
  {:else if app.autoWarpEnabled}
    <span class="concurrency-badge badge-balanced">Watchdog Armed (&lt; {app.autoWarpMinSpeedMB} MB/s)</span>
  {:else}
    <span class="concurrency-badge badge-safe">Direct</span>
  {/if}
</div>

<div class="form-group">
  <span class="form-hint" style="margin-bottom: 0.5rem; line-height: 1.45;">
    Automatically detects CDN IP bandwidth throttling (e.g. speed dropping to 50 KB/s), enables Cloudflare WARP Local SOCKS5 Proxy (or custom proxy pool), rotates WireGuard keys when throttled, and seamlessly reconnects active chunk streams at the exact byte offset.
  </span>

  <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-bottom: 0.75rem;">
    <input type="checkbox" bind:checked={app.autoWarpEnabled} style="width: auto;" />
    <span style="font-weight: 500;">Enable Automatic Speed Watchdog &amp; Proxy Rotation</span>
  </label>

  <div style="background: var(--table-row-alt); padding: 14px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: 12px;">
    <div style="display: grid; grid-template-columns: 210px 1fr; gap: 10px; align-items: center;">
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

    <div style="display: flex; align-items: center; justify-content: space-between; padding-top: 10px; border-top: 1px solid var(--border-subtle); font-size: 11px; color: var(--text-muted); flex-wrap: wrap; gap: 8px;">
      <span>
        Current Route: <strong style="color: var(--text-main);">{app.warpStatus.proxy_active ? (app.warpStatus.active_proxy || `socks5://127.0.0.1:${app.warpProxyPort}`) : 'Direct Connection'}</strong>
        {#if app.warpStatus.last_trigger_reason}
          &bull; Last Event: <em>{app.warpStatus.last_trigger_reason}</em>
        {/if}
      </span>
      <div style="display: flex; gap: 6px;">
        <button type="button" class="btn-secondary" style="padding: 4px 10px; font-size: 11px;" onclick={app.toggleWarpProxyMode} disabled={app.isRotatingWarp}>
          {app.warpStatus.proxy_active ? 'Disable Proxy' : 'Enable Proxy Now'}
        </button>
        <button type="button" class="btn-primary" style="padding: 4px 12px; font-size: 11px;" onclick={app.rotateWarpIP} disabled={app.isRotatingWarp}>
          {app.isRotatingWarp ? 'Rotating IP...' : 'Rotate IP Now'}
        </button>
      </div>
    </div>
  </div>
</div>
