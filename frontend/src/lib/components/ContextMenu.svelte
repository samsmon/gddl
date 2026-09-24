<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Desktop Right-Click Context Menu (qBittorrent / IDM style) -->
  {#if app.showContextMenu}
    <div
      class="context-menu"
      role="menu"
      tabindex="-1"
      style="top: {app.contextMenuY}px; left: {app.contextMenuX}px;"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.key === 'Escape' && app.closeContextMenu()}
    >
      {#if app.contextMenuItem ? (app.contextMenuItem.status === 'downloading' || app.contextMenuItem.status === 'queued' || app.contextMenuItem.status === 'compressing') : app.selectedItems.some(d => d.status === 'downloading' || d.status === 'queued' || d.status === 'compressing')}
        <button
          class="context-item"
          disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
          onclick={() => { app.pauseSelected(); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <rect x="6" y="4" width="4" height="16"></rect>
            <rect x="14" y="4" width="4" height="16"></rect>
          </svg>
          <span>Pause{app.selectedIds.length > 1 ? ` (${app.selectedIds.length})` : ''}</span>
          <span class="context-key">Space</span>
        </button>
      {:else}
        <button
          class="context-item"
          disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
          onclick={() => { app.startSelected(); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <polygon points="5 3 19 12 5 21 5 3"></polygon>
          </svg>
          <span>Resume / Start{app.selectedIds.length > 1 ? ` (${app.selectedIds.length})` : ''}</span>
          <span class="context-key">Space</span>
        </button>

        <button
          class="context-item"
          disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
          onclick={() => { app.restartSelected(); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
          </svg>
          <span>Restart from Beginning</span>
        </button>
      {/if}

      {#if app.contextMenuItem?.status === 'missing'}
        <button
          class="context-item"
          onclick={() => { app.restartDownload(app.contextMenuItem.id); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
          </svg>
          <span>Re-download Missing File</span>
        </button>
        <button
          class="context-item"
          onclick={() => { app.checkFileStatus(app.contextMenuItem.id); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <span>Check File Existence on Disk</span>
        </button>
      {:else if app.contextMenuItem?.status === 'completed'}
        <button
          class="context-item"
          onclick={() => { app.checkFileStatus(app.contextMenuItem.id); app.closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <span>Verify File on Disk</span>
        </button>
      {/if}

      {#if app.contextMenuItem?.is_folder}
        <div class="context-divider"></div>
        <button
          class="context-item"
          onclick={() => { app.toggleExpandFolder(app.contextMenuItem.id); app.closeContextMenu(); }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            {#if app.expandedFolderIds.includes(app.contextMenuItem.id)}
              <polyline points="6 9 12 15 18 9"></polyline>
            {:else}
              <polyline points="9 18 15 12 9 6"></polyline>
            {/if}
          </svg>
          <span>{app.expandedFolderIds.includes(app.contextMenuItem.id) ? 'Collapse Files' : 'Expand Files'}</span>
        </button>
      {/if}

      <div class="context-divider"></div>

      {#if app.contextMenuItem || app.selectedItem}
        <button
          class="context-item"
          disabled={(app.contextMenuItem ? app.contextMenuItem.status : app.selectedItem?.status) === 'moving'}
          onclick={() => {
            const targetId = app.contextMenuItem ? app.contextMenuItem.id : (app.selectedItem ? app.selectedItem.id : null);
            if (targetId) app.openPickerFor('item', targetId);
            app.closeContextMenu();
          }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
          </svg>
          <span>Change Save Location / Move...</span>
        </button>
        <div class="context-divider"></div>
      {/if}

      <!-- Delete -->
      <button
        class="context-item text-danger"
        disabled={app.selectedIds.length === 0 || app.selectedItems.some(d => d.status === 'moving')}
        onclick={() => { app.openDeleteModal('selected', null, false); app.closeContextMenu(); }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="3 6 5 6 21 6"></polyline>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
        </svg>
        <span>Delete...</span>
        <span class="context-key">Del</span>
      </button>

      <div class="context-divider"></div>

      <button
        class="context-item"
        onclick={() => { app.openLogsModal(); app.closeContextMenu(); }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="4 17 10 11 4 5"></polyline>
          <line x1="12" y1="19" x2="20" y2="19"></line>
        </svg>
        <span>View Logs...</span>
      </button>

      <button
        class="context-item"
        onclick={() => { app.selectedIds = app.filteredDownloads.map(d => d.id); app.closeContextMenu(); }}
      >
        <span>Select All</span>
        <span class="context-key">Ctrl+A</span>
      </button>

      <button
        class="context-item"
        disabled={app.selectedIds.length === 0}
        onclick={() => { app.selectedIds = []; app.lastClickedId = null; app.closeContextMenu(); }}
      >
        <span>Deselect All</span>
        <span class="context-key">Esc</span>
      </button>
    </div>
  {/if}

