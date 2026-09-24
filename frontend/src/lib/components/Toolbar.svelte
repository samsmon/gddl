<script>
  import { app } from '../state/appState.svelte.js';
</script>

  <!-- Top Native Toolbar (IDM / qBittorrent style) -->
  <header class="toolbar">
    <div class="toolbar-brand">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--accent-blue)" stroke-width="2">
        <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
        <path d="M12 12v9"></path>
        <path d="m8 17 4 4 4-4"></path>
      </svg>
      <span class="app-title">GDrive Client</span>
    </div>

    <div class="toolbar-actions">
      <!-- Add Links -->
      <button class="tb-btn tb-btn-primary" onclick={app.openAddModal} title="Add new Google Drive download links">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <line x1="12" y1="5" x2="12" y2="19"></line>
          <line x1="5" y1="12" x2="19" y2="12"></line>
        </svg>
        <span>Add Links</span>
      </button>

      <div class="tb-separator"></div>

      <!-- Start / Resume (IDM) -->
      <button
        class="tb-btn"
        disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
        onclick={app.startSelected}
        title="Resume / Start selected download(s)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polygon points="5 3 19 12 5 21 5 3"></polygon>
        </svg>
        <span>Start</span>
      </button>

      <!-- Pause (IDM) -->
      <button
        class="tb-btn"
        disabled={app.selectedIds.length === 0 || !app.selectedItems.some(d => d.status === 'downloading' || d.status === 'queued') || app.selectedItems.some(d => d.status === 'moving')}
        onclick={app.pauseSelected}
        title="Pause selected download(s)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <rect x="6" y="4" width="4" height="16"></rect>
          <rect x="14" y="4" width="4" height="16"></rect>
        </svg>
        <span>Pause</span>
      </button>

      <!-- Stop All (IDM style) -->
      <button
        class="tb-btn tb-btn-stop"
        disabled={!app.canStopAll}
        onclick={app.pauseAll}
        title="Stop all active and queued downloads"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <rect x="5" y="5" width="14" height="14" rx="2"></rect>
        </svg>
        <span>Stop All</span>
      </button>

      <!-- Resume All / Start Queue (IDM style) -->
      <button
        class="tb-btn tb-btn-resume"
        disabled={!app.canResumeAll}
        onclick={app.startAll}
        title="Resume all downloads / Start Queue (IDM style)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <path d="M5 3.868v16.264a1 1 0 0 0 1.54.841l12.44-8.132a1 1 0 0 0 0-1.682L6.54 3.027A1 1 0 0 0 5 3.868z"/>
        </svg>
        <span>Resume All</span>
      </button>

      <!-- Delete -->
      <button
        class="tb-btn"
        disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
        onclick={() => app.openDeleteModal('selected', null, false)}
        title="Delete selected item(s) (Del)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="3 6 5 6 21 6"></polyline>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          <line x1="10" y1="11" x2="10" y2="17"></line>
          <line x1="14" y1="11" x2="14" y2="17"></line>
        </svg>
        <span>Delete{app.selectedIds.length > 1 ? ` (${app.selectedIds.length})` : ''}</span>
      </button>

      <div class="tb-separator"></div>

      <!-- Cloudflare WARP / Anti-Throttle Status & Quick Rotate -->
      <button
        class="tb-btn {app.warpStatus.proxy_active ? 'active-auth' : ''}"
        onclick={app.toggleWarpProxyMode}
        disabled={app.isRotatingWarp}
        title={app.warpStatus.proxy_active
          ? `WARP Proxy Active (${app.warpStatus.active_proxy || '127.0.0.1:' + app.warpStatus.proxy_port}) | Rotations: ${app.warpStatus.rotation_count} | Click to switch back to Direct IP`
          : `Anti-Throttle Watchdog (${app.autoWarpEnabled ? 'Auto < ' + app.autoWarpMinSpeedMB + ' MB/s' : 'Manual'}) | Click to enable WARP Proxy immediately`}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
        </svg>
        <span>{app.warpStatus.proxy_active ? `WARP #${app.warpStatus.rotation_count || 1}` : (app.autoWarpEnabled ? 'WARP: Auto' : 'WARP: Off')}</span>
        {#if app.warpStatus.proxy_active}
          <span class="auth-dot"></span>
        {/if}
      </button>

      <button
        class="tb-btn"
        onclick={app.rotateWarpIP}
        disabled={app.isRotatingWarp}
        title="Force rotate Cloudflare WARP tunnel keys / Proxy IP and immediately reconnect throttled download streams"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="23 4 23 10 17 10"></polyline>
          <polyline points="1 20 1 14 7 14"></polyline>
          <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
        <span>{app.isRotatingWarp ? 'Rotating...' : 'Rotate IP'}</span>
      </button>

      <!-- Options -->
      <button class="tb-btn" onclick={() => app.showSettingsModal = true} title="Preferences, Web UI Security & Target Folder">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3"></circle>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
        </svg>
        <span>Options</span>
      </button>

      {#if app.authEnabled}
        <div class="tb-separator"></div>

        <div class="user-pill" title="Current Web UI User">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
            <circle cx="12" cy="7" r="4"></circle>
          </svg>
          <span>{app.currentAuthUser}</span>
        </div>

        <button class="tb-btn" onclick={app.logoutWebUI} title="Log out of Web UI">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
            <polyline points="16 17 21 12 16 7"></polyline>
            <line x1="21" y1="12" x2="9" y2="12"></line>
          </svg>
          <span>Logout</span>
        </button>
      {/if}
    </div>

    <!-- Hide Completed Toggle Button -->
    <button
      type="button"
      class="tb-btn toggle-hide-btn"
      class:active={app.hideCompleted}
      onclick={app.toggleHideCompleted}
      title={app.hideCompleted ? 'Completed transfers hidden. Click to show all.' : 'Hide completed transfers to focus on active queue.'}
    >
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        {#if app.hideCompleted}
          <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
          <line x1="1" y1="1" x2="23" y2="23"></line>
        {:else}
          <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
          <circle cx="12" cy="3" r="3"></circle>
        {/if}
      </svg>
      <span>{app.hideCompleted ? 'Completed Hidden' : 'Hide Completed'}</span>
      {#if app.hideCompleted && app.counts.completed > 0}
        <span class="tb-badge-count">{app.counts.completed}</span>
      {/if}
    </button>

    <!-- Search filter -->
    <div class="toolbar-search">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--text-dim)" stroke-width="2.5">
        <circle cx="11" cy="11" r="8"></circle>
        <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
      </svg>
      <input
        type="text"
        placeholder="Filter list..."
        bind:value={app.searchQuery}
      />
      {#if app.searchQuery}
        <button class="clear-search-btn" aria-label="Clear Search" onclick={() => app.searchQuery = ''}>
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      {/if}
    </div>
  </header>
