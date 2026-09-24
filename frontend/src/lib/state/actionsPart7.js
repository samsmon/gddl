import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart7(app) {
  app.openDeleteModal = function(mode = 'selected', id = null, defaultWithFile = false) {
    app.deleteModalTarget = mode;
    app.deleteModalSingleId = id;
    app.deleteModalWithFile = defaultWithFile;
    app.showDeleteModal = true;
    app.showContextMenu = false;
  }

  app.confirmDeleteModal = async function() {
    app.showDeleteModal = false;
    const withFile = app.deleteModalWithFile;
    if (app.deleteModalTarget === 'completed') {
      await app.executeClearCompleted(withFile);
    } else if (app.deleteModalTarget === 'single' && app.deleteModalSingleId) {
      await app.executeDeleteSingle(app.deleteModalSingleId, withFile);
    } else {
      await app.executeDeleteSelected(withFile);
    }
  }

  app.executeDeleteSelected = async function(deleteFile = false) {
    if (app.selectedIds.length === 0) return;
    const idsToDelete = [...app.selectedIds];
    app.selectedIds = [];
    app.lastClickedId = null;
    for (const id of idsToDelete) {
      try {
        await fetch(`/api/downloads/${id}?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'DELETE' });
      } catch (e) {
        console.error(e);
      }
    }
    app.fetchDownloads();
  }

  app.executeDeleteSingle = async function(id, deleteFile = false) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'DELETE' });
      app.selectedIds = app.selectedIds.filter(x => x !== id);
      app.fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  app.executeClearCompleted = async function(deleteFile = false) {
    try {
      await fetch(`/api/downloads/clear?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'POST' });
      app.selectedIds = app.selectedIds.filter(id => {
        const item = app.downloads.find(d => d.id === id);
        return item && item.status !== 'completed' && item.status !== 'corrupted' && item.status !== 'cancelled';
      });
      app.fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  app.startSelected = async function() {
    if (app.selectedIds.length === 0) return;
    const toStart = app.downloads.filter(d => app.selectedIds.includes(d.id) && (d.status === 'paused' || d.status === 'failed' || d.status === 'cancelled'));
    for (const item of toStart) {
      fetch(`/api/downloads/${item.id}/start`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(app.fetchDownloads, 200);
  }

  app.pauseSelected = async function() {
    if (app.selectedIds.length === 0) return;
    const toPause = app.downloads.filter(d => app.selectedIds.includes(d.id) && (d.status === 'downloading' || d.status === 'queued' || d.status === 'compressing'));
    for (const item of toPause) {
      fetch(`/api/downloads/${item.id}/pause`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(app.fetchDownloads, 200);
  }

  app.restartSelected = async function() {
    if (app.selectedIds.length === 0) return;
    for (const id of app.selectedIds) {
      fetch(`/api/downloads/${id}/restart`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(app.fetchDownloads, 200);
  }

  app.deleteSelected = function(deleteFile = false) {
    app.openDeleteModal('selected', null, deleteFile);
  }

  app.startDownload = async function(id) {
    if (!id) return;
    const item = app.downloads.find(d => d.id === id);
    if (item && item.status === 'completed') return;
    try {
      await fetch(`/api/downloads/${id}/start`, { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  app.pauseDownload = async function(id) {
    if (!id) return;
    const item = app.downloads.find(d => d.id === id);
    if (item && (item.status === 'completed' || item.status === 'paused')) return;
    try {
      await fetch(`/api/downloads/${id}/pause`, { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  app.restartDownload = async function(id) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}/restart`, { method: 'POST' });
      app.fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  app.deleteDownload = function(id, deleteFile = false) {
    app.openDeleteModal('single', id, deleteFile);
  }

  app.clearCompleted = function() {
    app.openDeleteModal('completed', null, false);
  }

  app.openAddModal = function() {
    app.addTargetFolder = app.defaultFolder;
    app.showAddModal = true;
  }

  app.openPickerFor = function(target, itemId = null) {
    app.folderPickerTarget = target;
    app.relocateItemTargetId = itemId;
    app.showFolderPicker = true;
  }

  app.handleFolderSelected = async function(path) {
    if (app.folderPickerTarget === 'add') {
      app.addTargetFolder = path;
    } else if (app.folderPickerTarget === 'settings') {
      app.defaultFolder = path;
      app.addTargetFolder = path;
      await app.saveConfig();
    } else if (app.folderPickerTarget === 'item' && app.relocateItemTargetId) {
      await app.setItemFolder(app.relocateItemTargetId, path);
    }
    app.showFolderPicker = false;
  }

  app.setItemFolder = async function(id, newPath) {
    try {
      const res = await fetch(`/api/downloads/${id}/target-folder`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ target_folder: newPath })
      });
      if (res.ok) {
        app.fetchDownloads();
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to update folder' }));
        alert(err.error || 'Failed to update save path');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      app.relocateItemTargetId = null;
    }
  }
}
