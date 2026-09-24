<script>
  import { app } from '../state/appState.svelte.js';
</script>

<!-- Left Category Sidebar (IDM Tree Style) -->
<aside class="sidebar">
  <div class="sidebar-header">Categories</div>
  <nav class="category-list">
    <!-- Top-level: All Downloads -->
    <button
      type="button"
      class="cat-item"
      class:active={app.activeFilter === 'all'}
      onclick={() => app.activeFilter = 'all'}
      title="Show all downloads"
    >
      <span class="cat-label">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
        </svg>
        All Downloads
      </span>
      <span class="cat-badge">{app.counts.all}</span>
    </button>

    <!-- Sub-level 1: Unfinished & Finished (IDM Core Filtering) -->
    <button
      type="button"
      class="cat-item cat-sub-item"
      class:active={app.activeFilter === 'unfinished'}
      onclick={() => app.activeFilter = 'unfinished'}
      title="Show unfinished downloads (downloading, queued, paused, error)"
    >
      <span class="cat-label">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--accent-amber)" stroke-width="2">
          <circle cx="12" cy="12" r="10"></circle>
          <polyline points="12 6 12 12 16 14"></polyline>
        </svg>
        Unfinished
      </span>
      <span class="cat-badge" style="color: {app.counts.unfinished > 0 ? 'var(--accent-amber)' : 'inherit'}; font-weight: {app.counts.unfinished > 0 ? '600' : 'normal'};">
        {app.counts.unfinished}
      </span>
    </button>

    <button
      type="button"
      class="cat-item cat-sub-item"
      class:active={app.activeFilter === 'finished' || app.activeFilter === 'completed'}
      onclick={() => app.activeFilter = 'finished'}
      title="Show completed downloads"
    >
      <span class="cat-label">
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--accent-green)" stroke-width="2.5">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
        Finished
      </span>
      <span class="cat-badge">{app.counts.finished}</span>
    </button>

    <!-- Section Header: By Service / Source -->
    <div class="sidebar-section-title">SOURCES</div>

    <button
      type="button"
      class="cat-item"
      class:active={app.activeFilter === 'gdrive'}
      onclick={() => app.activeFilter = 'gdrive'}
      title="Filter Google Drive downloads"
    >
      <span class="cat-label">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--accent-blue)" stroke-width="2">
          <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
          <path d="M12 12v9"></path>
          <path d="m8 17 4 4 4-4"></path>
        </svg>
        Google Drive
      </span>
      <span class="cat-badge">{app.counts.gdrive}</span>
    </button>

    <button
      type="button"
      class="cat-item"
      class:active={app.activeFilter === 'discord'}
      onclick={() => app.activeFilter = 'discord'}
      title="Filter Discord CDN downloads"
    >
      <span class="cat-label">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#5865F2" stroke-width="2">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path>
        </svg>
        Discord CDN
      </span>
      <span class="cat-badge">{app.counts.discord}</span>
    </button>

    <!-- Section Header: By Status -->
    <div class="sidebar-section-title">STATUS</div>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'downloading'}
      onclick={() => app.activeFilter = 'downloading'}
    >
      <span class="cat-label">
        <span class="status-indicator active-dot"></span>
        Downloading
      </span>
      <span class="cat-badge">{app.counts.downloading}</span>
    </button>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'queued'}
      onclick={() => app.activeFilter = 'queued'}
    >
      <span class="cat-label">
        <span class="status-indicator queued-dot"></span>
        Queued
      </span>
      <span class="cat-badge">{app.counts.queued}</span>
    </button>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'paused'}
      onclick={() => app.activeFilter = 'paused'}
    >
      <span class="cat-label">
        <span class="status-indicator paused-dot"></span>
        Paused
      </span>
      <span class="cat-badge">{app.counts.paused}</span>
    </button>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'missing'}
      onclick={() => app.activeFilter = 'missing'}
    >
      <span class="cat-label">
        <span class="status-indicator missing-dot"></span>
        Missing / Moved
      </span>
      <span class="cat-badge" style="color: var(--accent-amber); font-weight: bold;">{app.counts.missing}</span>
    </button>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'corrupted'}
      onclick={() => app.activeFilter = 'corrupted'}
    >
      <span class="cat-label">
        <span class="status-indicator corrupt-dot"></span>
        Corrupted
      </span>
      <span class="cat-badge" style="color: var(--accent-red); font-weight: bold;">{app.counts.corrupted}</span>
    </button>

    <button
      type="button"
      class="cat-item cat-status-item"
      class:active={app.activeFilter === 'failed'}
      onclick={() => app.activeFilter = 'failed'}
    >
      <span class="cat-label">
        <span class="status-indicator failed-dot"></span>
        Error / Cancelled
      </span>
      <span class="cat-badge">{app.counts.failed}</span>
    </button>
  </nav>

  <!-- Quick Folder Info / Jellyfin Picker Trigger -->
  <div class="sidebar-folder-box">
    <div class="folder-box-header">
      <span>DEFAULT SAVE PATH</span>
      <button class="btn-browse-mini" onclick={() => app.openPickerFor('settings')}>Browse</button>
    </div>
    <div class="folder-box-path" title={app.defaultFolder}>
      {app.defaultFolder}
    </div>
  </div>
</aside>
