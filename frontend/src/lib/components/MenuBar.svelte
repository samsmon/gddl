<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Top Native Desktop Menu Bar (File, Edit, View, Tools, Help) -->
  <nav class="app-menubar" onclick={(e) => e.stopPropagation()}>
    <!-- FILE MENU -->
    <div class="menubar-item" class:active={app.openMenu === 'file'}>
      <button class="menubar-btn" onclick={(e) => app.toggleMenu('file', e)} onmouseenter={() => app.handleMenuHover('file')}>
        File
      </button>
      {#if app.openMenu === 'file'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { app.openAddModal(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/></svg></span>
            <span class="dropdown-text">Add Links...</span>
            <span class="dropdown-shortcut">Ctrl+N</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.clearCompleted(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19.36 10.04l-7.4-7.4c-.78-.78-2.05-.78-2.83 0L2.71 9.06c-.78.78-.78 2.05 0 2.83l7.4 7.4c.78.78 2.05.78 2.83 0l6.42-6.42-6.42-6.42 1.41-1.41 7.41 7.41c.39.39.39 1.02 0 1.41l-2.4 2.4-1.41-1.41 1.69-1.69zM4.12 10.47l6.42-6.42 6.42 6.42-6.42 6.42-6.42-6.42z"/></svg></span>
            <span class="dropdown-text">Clear Completed</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.startAll(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg></span>
            <span class="dropdown-text">Resume All (Start Queue)</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.pauseAll(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><rect x="5" y="5" width="14" height="14" rx="2"/></svg></span>
            <span class="dropdown-text">Stop All</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.checkAllFiles(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg></span>
            <span class="dropdown-text">Check All Files on Disk</span>
          </button>
          {#if app.authEnabled}
            <div class="menu-divider"></div>
            <button class="dropdown-item text-danger" onclick={() => { app.logoutWebUI(); app.closeMenus(); }}>
              <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"/></svg></span>
              <span class="dropdown-text">Log Out Web UI</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>

    <!-- EDIT MENU -->
    <div class="menubar-item" class:active={app.openMenu === 'edit'}>
      <button class="menubar-btn" onclick={(e) => app.toggleMenu('edit', e)} onmouseenter={() => app.handleMenuHover('edit')}>
        Edit
      </button>
      {#if app.openMenu === 'edit'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { app.selectedIds = app.filteredDownloads.map(d => d.id); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.11 0 2-.9 2-2V5c0-1.1-.89-2-2-2zm-9 14l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg></span>
            <span class="dropdown-text">Select All</span>
            <span class="dropdown-shortcut">Ctrl+A</span>
          </button>
          <button class="dropdown-item" disabled={app.selectedIds.length === 0} onclick={() => { app.selectedIds = []; app.lastClickedId = null; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 5v14H5V5h14m0-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z"/></svg></span>
            <span class="dropdown-text">Deselect All</span>
            <span class="dropdown-shortcut">Esc</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.invertSelection(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M6.99 11L3 15l3.99 4v-3H14v-2H6.99v-3zM21 9l-3.99-4v3H10v2h7.01v3L21 9z"/></svg></span>
            <span class="dropdown-text">Invert Selection</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item text-danger" disabled={app.selectedIds.length === 0} onclick={() => { app.openDeleteModal('selected', null, false); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg></span>
            <span class="dropdown-text">Delete...</span>
            <span class="dropdown-shortcut">Del</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- VIEW MENU -->
    <div class="menubar-item" class:active={app.openMenu === 'view'}>
      <button class="menubar-btn" onclick={(e) => app.toggleMenu('view', e)} onmouseenter={() => app.handleMenuHover('view')}>
        View
      </button>
      {#if app.openMenu === 'view'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { app.toggleTheme(); app.closeMenus(); }}>
            <span class="dropdown-icon">
              {#if app.currentTheme === 'dark'}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>
              {:else}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12.3 2a10 10 0 0 0-1.9 19.8 10 10 0 0 0 11.4-11.4A10 10 0 0 0 12.3 2z"/></svg>
              {/if}
            </span>
            <span class="dropdown-text">Switch to {app.currentTheme === 'dark' ? 'Light Theme' : 'Dark Theme'}</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.expandAllFolders(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M20 6h-8l-2-2H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm0 12H4V8h16v10z"/></svg></span>
            <span class="dropdown-text">Expand All Folder Archives</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.collapseAllFolders(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg></span>
            <span class="dropdown-text">Collapse All Folder Archives</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.resetColumnWidths(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M21 6H3c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm0 10H3V8h2v4h2V8h2v4h2V8h2v4h2V8h2v4h2V8h3v8z"/></svg></span>
            <span class="dropdown-text">Reset Column Widths</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.activeFilter = 'all'; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path></svg></span>
            <span class="dropdown-text">View: All Downloads</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.activeFilter = 'unfinished'; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg></span>
            <span class="dropdown-text">View: Unfinished Downloads</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.activeFilter = 'finished'; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"></polyline></svg></span>
            <span class="dropdown-text">View: Finished Downloads</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- TOOLS MENU -->
    <div class="menubar-item" class:active={app.openMenu === 'tools'}>
      <button class="menubar-btn" onclick={(e) => app.toggleMenu('tools', e)} onmouseenter={() => app.handleMenuHover('tools')}>
        Tools
      </button>
      {#if app.openMenu === 'tools'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { app.showLoginModal = true; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M21 11c0 5.52-4.48 10-10 10S1 16.52 1 11 5.48 1 11 1s10 4.48 10 10zm-3.5-3.5c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm-5-3c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm-4 7c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm7 4c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5z"/></svg></span>
            <span class="dropdown-text">Google Session Cookie...</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.showSettingsModal = true; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/></svg></span>
            <span class="dropdown-text">Options & Preferences...</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.checkAllFiles(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 10.99h7c-.53 4.12-3.28 7.79-7 8.94V12H5V6.3l7-3.11v8.8z"/></svg></span>
            <span class="dropdown-text">Verify File Integrity & Disk Check</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.openDiscordExportModal(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z"/></svg></span>
            <span class="dropdown-text">Export Discord Downloads (AI Agent)...</span>
          </button>
          <button class="dropdown-item" onclick={() => { app.openDiscordRefreshModal(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg></span>
            <span class="dropdown-text">Refresh Discord Links...</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.openLogsModal(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-5 14H7v-2h7v2zm3-4H7v-2h10v2zm0-4H7V7h10v2z"/></svg></span>
            <span class="dropdown-text">Execution & Error Logs...</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- HELP MENU -->
    <div class="menubar-item" class:active={app.openMenu === 'help'}>
      <button class="menubar-btn" onclick={(e) => app.toggleMenu('help', e)} onmouseenter={() => app.handleMenuHover('help')}>
        Help
      </button>
      {#if app.openMenu === 'help'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { app.showDocModal = true; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 4h5v8l-2.5-1.5L6 12V4z"/></svg></span>
            <span class="dropdown-text">Documentation & Guide</span>
          </button>
          <a class="dropdown-item" href="https://github.com/samsmon/gddl" target="_blank" rel="noopener noreferrer" onclick={app.closeMenus}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg></span>
            <span class="dropdown-text">Project GitHub Repository ↗</span>
          </a>
          <button class="dropdown-item" onclick={() => { app.checkForUpdates(); app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg></span>
            <span class="dropdown-text">Check for Updates...</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { app.showAboutModal = true; app.closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg></span>
            <span class="dropdown-text">About GDrive Downloader</span>
          </button>
        </div>
      {/if}
    </div>
  </nav>

