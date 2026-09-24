import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart8(app) {
  app.handleKeyDown = function(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    if (app.showDeleteModal) {
      if (e.key === 'Escape') {
        app.showDeleteModal = false;
        return;
      }
      if (e.key === 'Enter') {
        app.confirmDeleteModal();
        return;
      }
      return;
    }

    if (e.key === 'Escape') {
      if (app.openMenu) {
        app.closeMenus();
        return;
      }
      if (app.showContextMenu) {
        app.closeContextMenu();
        return;
      }
      if (app.showDocModal) {
        app.showDocModal = false;
        return;
      }
      if (app.showUpdateModal) {
        app.showUpdateModal = false;
        return;
      }
      if (app.showAboutModal) {
        app.showAboutModal = false;
        return;
      }
      if (app.showLogsModal) {
        app.showLogsModal = false;
        return;
      }
      if (app.showDiscordExportModal) {
        app.showDiscordExportModal = false;
        return;
      }
      if (app.showDiscordRefreshModal) {
        app.showDiscordRefreshModal = false;
        return;
      }
      app.selectedIds = [];
      app.lastClickedId = null;
      return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'n') {
      e.preventDefault();
      app.openAddModal();
      return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
      e.preventDefault();
      app.selectedIds = app.filteredDownloads.map(d => d.id);
      if (app.filteredDownloads.length > 0) {
        app.lastClickedId = app.filteredDownloads[0].id;
      }
      return;
    }

    if (e.key === 'Delete' && app.selectedIds.length > 0 && !app.selectedItems.some(d => d.status === 'moving')) {
      e.preventDefault();
      app.openDeleteModal('selected', null, e.shiftKey);
    } else if (e.key === ' ' && app.selectedIds.length > 0 && !app.selectedItems.some(d => d.status === 'moving')) {
      e.preventDefault();
      const hasActive = app.selectedItems.some(d => d.status === 'downloading' || d.status === 'queued');
      if (hasActive) {
        app.pauseSelected();
      } else {
        app.startSelected();
      }
    }
  }
}
