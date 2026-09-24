<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Bottom Desktop Status Bar -->
  <footer class="statusbar">
    <div class="sb-section">
      <span class="status-dot" class:connected={app.backendConnected}></span>
      <span>{app.backendConnected ? 'Online (Go 1.27 :8080)' : 'Disconnected'}</span>
      {#if app.hasLogin}
        <span class="sb-badge-auth">Google Session Active</span>
      {/if}
      {#if app.bulkAddingStatus}
        <span class="sb-divider">|</span>
        <span style="color: var(--accent-blue); display: inline-flex; align-items: center; gap: 4px;">
          <span class="status-indicator active-dot"></span>
          {app.bulkAddingStatus}
        </span>
      {/if}
    </div>

    <div class="sb-section">
      <span>Total: <strong>{app.counts.all}</strong></span>
      {#if app.selectedIds.length > 0}
        <span class="sb-divider">|</span>
        <span>Selected: <strong style="color: var(--accent-blue)">{app.selectedIds.length}</strong></span>
      {/if}
      <span class="sb-divider">|</span>
      <span>Active: <strong style="color: var(--accent-blue)">{app.counts.downloading}</strong></span>
      <span class="sb-divider">|</span>
      <span>Paused: <strong style="color: var(--accent-amber)">{app.counts.paused}</strong></span>
      <span class="sb-divider">|</span>
      <span>Done: <strong style="color: var(--accent-green)">{app.counts.completed}</strong></span>
      {#if app.counts.corrupted > 0}
        <span class="sb-divider">|</span>
        <span>Corrupt: <strong style="color: var(--accent-red)">{app.counts.corrupted}</strong></span>
      {/if}
    </div>

    <div class="sb-section font-mono">
      <span>DL: <strong style="color: var(--accent-blue)">{formatSpeed(app.counts.totalSpeed)}</strong></span>
    </div>
  </footer>
