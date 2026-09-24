<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

    <!-- Left Category Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">Categories</div>
      <nav class="category-list">
        <button
          class="cat-item"
          class:active={app.activeFilter === 'all'}
          onclick={() => app.activeFilter = 'all'}
        >
          <span class="cat-label">All Downloads</span>
          <span class="cat-badge">{app.hideCompleted && app.activeFilter === 'all' ? `${app.filteredDownloads.length}/${app.counts.all}` : app.counts.all}</span>
        </button>

        <button
          class="cat-item"
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
          class="cat-item"
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
          class="cat-item"
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
          class="cat-item"
          class:active={app.activeFilter === 'completed'}
          onclick={() => app.activeFilter = 'completed'}
        >
          <span class="cat-label">
            <span class="status-indicator completed-dot"></span>
            Completed
          </span>
          <span class="cat-badge">{app.counts.completed}</span>
        </button>

        <button
          class="cat-item"
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
          class="cat-item"
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
          class="cat-item"
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
