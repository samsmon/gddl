<script>
  import { app } from '../state/appState.svelte.js';
</script>

<div class="settings-pane-header">
  <div class="settings-pane-title">
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
    </svg>
    <span>General &amp; Downloads</span>
  </div>
</div>

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

<div class="form-group" style="margin-top: 0.5rem;">
  <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
    <span class="form-title">Maximum Concurrent Downloads:</span>
    {#if app.maxConcurrency <= 2}
      <span class="concurrency-badge badge-safe">Safe &amp; Stable</span>
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
<div class="form-group" style="margin-top: 0.5rem;">
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
