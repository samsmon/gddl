import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart6(app) {
  app.executeAddDownloads = async function(resolutions = {}) {
    if (!app.addLinksInput.trim()) return;

    const raw = app.addLinksInput
      .split(/[\n,]+/)
      .map(l => l.trim())
      .filter(l => l.length > 0);

    if (raw.length === 0) return;

    app.isAdding = true;
    try {
      const res = await fetch('/api/downloads', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          links: raw,
          target_folder: app.addTargetFolder || app.defaultFolder,
          zip_mode: app.folderZipMode === 'zip',
          conflict_resolutions: resolutions
        })
      });

      if (res.ok) {
        app.addLinksInput = '';
        app.detectedFolder = null;
        app.resolveError = '';
        app.folderZipMode = 'zip';
        app.showConflictModal = false;
        app.showAddModal = false;
        app.fetchDownloads();
      } else {
        const err = await res.json();
        alert(err.error || 'Failed to add links');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      app.isAdding = false;
    }
  }

  app.setAllConflictActions = function(action) {
    const updated = { ...app.conflictResolutions };
    for (const c of app.conflictList) {
      updated[c.file_id || c.url] = action;
    }
    app.conflictResolutions = updated;
  }

  app.handleRowClick = function(item, e) {
    if (!item) return;

    // Real-time file presence check when row is selected/clicked
    if (item.status === 'completed' || item.status === 'missing') {
      app.checkFileStatus(item.id);
    }

    if (e.shiftKey) {
      // Shift+Click: Range selection between lastClickedId and item.id
      if (app.lastClickedId) {
        const fromIdx = app.filteredDownloads.findIndex(d => d.id === app.lastClickedId);
        const toIdx = app.filteredDownloads.findIndex(d => d.id === item.id);
        if (fromIdx !== -1 && toIdx !== -1) {
          const start = Math.min(fromIdx, toIdx);
          const end = Math.max(fromIdx, toIdx);
          const rangeIds = app.filteredDownloads.slice(start, end + 1).map(d => d.id);

          if (e.ctrlKey || e.metaKey) {
            const combined = new Set([...app.selectedIds, ...rangeIds]);
            app.selectedIds = Array.from(combined);
          } else {
            app.selectedIds = rangeIds;
          }
          return;
        }
      }
      // If no pivot, select clicked item
      app.selectedIds = [item.id];
      app.lastClickedId = item.id;
    } else if (e.ctrlKey || e.metaKey) {
      // Ctrl+Click / Cmd+Click: Toggle selection
      if (app.selectedIds.includes(item.id)) {
        app.selectedIds = app.selectedIds.filter(id => id !== item.id);
      } else {
        app.selectedIds = [...app.selectedIds, item.id];
      }
      app.lastClickedId = item.id;
    } else {
      // Standard Click: select only this item
      app.selectedIds = [item.id];
      app.lastClickedId = item.id;
    }
  }

  app.toggleExpandFolder = function(id, event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    if (app.expandedFolderIds.includes(id)) {
      app.expandedFolderIds = app.expandedFolderIds.filter(x => x !== id);
    } else {
      app.expandedFolderIds = [...app.expandedFolderIds, id];
    }
  }

  app.handleContextMenu = function(item, e) {
    e.preventDefault();
    e.stopPropagation();
    if (!item) return;

    if (!app.selectedIds.includes(item.id)) {
      app.selectedIds = [item.id];
      app.lastClickedId = item.id;
    }
    app.contextMenuItem = item;

    const menuW = 220;
    const menuH = 320;
    let x = e.clientX;
    let y = e.clientY;
    if (x + menuW > window.innerWidth) x = Math.max(10, window.innerWidth - menuW - 10);
    if (y + menuH > window.innerHeight) y = Math.max(10, window.innerHeight - menuH - 10);

    app.contextMenuX = x;
    app.contextMenuY = y;
    app.showContextMenu = true;
  }

  app.closeContextMenu = function() {
    app.showContextMenu = false;
  }

  app.toggleMenu = function(menuName, e) {
    if (e) e.stopPropagation();
    app.openMenu = app.openMenu === menuName ? null : menuName;
  }

  app.handleMenuHover = function(menuName) {
    if (app.openMenu !== null && app.openMenu !== menuName) {
      app.openMenu = menuName;
    }
  }

  app.closeMenus = function() {
    app.openMenu = null;
  }

  app.checkFileStatus = async function(id) {
    if (!id) return;
    try {
      const res = await fetch(`/api/downloads/${id}/check`, { method: 'POST' });
      if (res.ok) {
        app.fetchDownloads();
      }
    } catch (e) {
      console.warn('File check failed', e);
    }
  }

  app.checkAllFiles = async function() {
    try {
      await fetch('/api/downloads/check-all', { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.warn('Check all failed', e);
    }
  }

  app.startAll = async function() {
    try {
      await fetch('/api/downloads/resume-all', { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.error('Resume all failed', e);
    }
  }

  app.pauseAll = async function() {
    try {
      await fetch('/api/downloads/pause-all', { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.error('Pause all failed', e);
    }
  }

  app.invertSelection = function() {
    const allFilteredIds = app.filteredDownloads.map(d => d.id);
    app.selectedIds = allFilteredIds.filter(id => !app.selectedIds.includes(id));
  }

  app.expandAllFolders = function() {
    app.expandedFolderIds = app.downloads.filter(d => d.is_folder).map(d => d.id);
  }

  app.collapseAllFolders = function() {
    app.expandedFolderIds = [];
  }

  app.resetColumnWidths = function() {
    app.colWidths = {
      name: 280,
      size: 90,
      done: 90,
      prog: 160,
      status: 110,
      speed: 95,
      eta: 80,
      path: 200,
      added: 135
    };
    try {
      localStorage.removeItem('gdrive_col_widths');
    } catch (e) {}
  }

  app.checkForUpdates = async function(silent = false) {
    if (!silent) {
      app.showUpdateModal = true;
    }
    app.updateChecking = true;
    app.updateStatus = 'Checking GitHub repository for latest commits...';
    app.updateError = '';

    try {
      const res = await fetch('/api/system/check-update');
      if (res.ok) {
        const data = await res.json();
        app.updateInfo = data;
        app.hasUpdate = !!data.update_available;
        app.updateStatus = data.update_available ? 'New update available!' : 'You are on the latest version.';
      } else {
        const ghRes = await fetch('https://raw.githubusercontent.com/samsmon/gddl/main/version.json');
        if (ghRes.ok) {
          const remote = await ghRes.json();
          const curCommit = app.updateInfo?.current_commit || '1e382c5';
          const updateAvail = remote.commit && remote.commit !== curCommit;
          app.updateInfo = {
            update_available: updateAvail,
            current_version: '1.3.0',
            current_commit: curCommit,
            latest_version: remote.version || '1.3.0',
            latest_commit: remote.commit,
            latest_title: remote.title || remote.message,
            latest_message: remote.message,
            changelog: remote.changelog || [],
            pull_command: 'git pull origin main',
            rebuild_command: 'npm run build --prefix frontend && go build -o gddl.exe ./backend'
          };
          app.hasUpdate = updateAvail;
        } else {
          throw new Error('Unable to connect to GitHub release server');
        }
      }
    } catch (e) {
      app.updateError = 'Failed to check updates: ' + e.message;
    } finally {
      app.updateChecking = false;
    }
  }

  app.copyUpdateCommand = function() {
    const cmd = `git pull origin main\ncd frontend && npm run build\ncd ../backend && go build -o ../gddl.exe .`;
    if (navigator.clipboard && window.isSecureContext && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(cmd).then(() => {
        app.copiedUpdateCmd = true;
        setTimeout(() => { app.copiedUpdateCmd = false; }, 2500);
      }).catch(() => {
        app.fallbackCopy(cmd);
        app.copiedUpdateCmd = true;
        setTimeout(() => { app.copiedUpdateCmd = false; }, 2500);
      });
    } else {
      app.fallbackCopy(cmd);
      app.copiedUpdateCmd = true;
      setTimeout(() => { app.copiedUpdateCmd = false; }, 2500);
    }
  }
}

