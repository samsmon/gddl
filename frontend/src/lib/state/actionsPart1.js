import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart1(app) {
  app.toggleHideCompleted = function() {
    app.hideCompleted = !app.hideCompleted;
    try {
      localStorage.setItem('gddl_hide_completed', String(app.hideCompleted));
    } catch (_) {}
  }

  app.handleTableScroll = function(e) {
    app.scrollTop = e.currentTarget.scrollTop;
    app.viewportHeight = e.currentTarget.clientHeight || 600;
  }

  app.toggleSort = function(column) {
    if (app.sortColumn === column) {
      app.sortDirection = app.sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      app.sortColumn = column;
      app.sortDirection = (column === 'size' || column === 'done' || column === 'prog' || column === 'speed' || column === 'added') ? 'desc' : 'asc';
    }
  }

  app.toggleTheme = function() {
    app.currentTheme = app.currentTheme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', app.currentTheme);
    localStorage.setItem('gdrive_theme', app.currentTheme);
  }

  app.startResize = function(col, event) {
    event.preventDefault();
    const startX = event.clientX;
    const startW = app.colWidths[col];

    function onMouseMove(e) {
      const diff = e.clientX - startX;
      const newW = Math.max(30, startW + diff);
      app.colWidths[col] = newW;
    }

    function onMouseUp() {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
      try {
        localStorage.setItem('gdrive_col_widths', JSON.stringify(app.colWidths));
      } catch (err) {}
    }

    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  app.autoSizeColumn = function(col) {
    // Auto-measure column based on content
    let maxLen = 8;
    for (const item of app.filteredDownloads) {
      let text = '';
      if (col === 'name') text = item.filename || item.url || '';
      else if (col === 'path') text = item.target_folder || '';
      else if (col === 'size') text = getItemSize(item);
      else if (col === 'done') text = formatBytes(item.downloaded_bytes);
      else if (col === 'speed') text = formatSpeed(item.speed);
      else if (col === 'status') text = item.status || '';
      else if (col === 'added') text = formatDateTime(item.last_try_at || item.created_at);
      else if (col === 'prog') {
        if (item.status === 'compressing') text = '100% (ZIP)';
        else if (item.is_folder && item.total_files) text = `100% (${item.total_files}/${item.total_files})`;
        else text = '100.0%';
      }

      if (text.length > maxLen) maxLen = text.length;
    }

    let calculated = col === 'prog' ? Math.max(160, maxLen * 8.5 + 65) : Math.min(650, Math.max(50, maxLen * 8.5 + 40));
    app.colWidths[col] = Math.round(calculated);
    try {
      localStorage.setItem('gdrive_col_widths', JSON.stringify(app.colWidths));
    } catch (e) {}
  }

  app.fetchWarpStatus = async function() {
    if (app.authEnabled && !app.isAuthenticated) return;
    try {
      const res = await fetch('/api/warp/status');
      if (res.ok) {
        app.warpStatus = await res.json();
      }
    } catch (e) {}
  }

  app.rotateWarpIP = async function() {
    if (app.isRotatingWarp || app.warpStatus.is_rotating) return;
    app.isRotatingWarp = true;
    try {
      const res = await fetch('/api/warp/rotate', { method: 'POST' });
      if (res.ok) {
        const data = await res.json();
        if (data.status) app.warpStatus = data.status;
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to rotate IP' }));
        alert('WARP / Proxy Error: ' + (err.error || 'Failed to rotate'));
      }
    } catch (e) {
      alert('Failed to rotate WARP IP: ' + e.message);
    } finally {
      app.isRotatingWarp = false;
      app.fetchWarpStatus();
    }
  }

  app.toggleWarpProxyMode = async function() {
    app.isRotatingWarp = true;
    const targetState = !app.warpStatus.proxy_active;
    try {
      const res = await fetch('/api/warp/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ proxy_active: targetState })
      });
      if (res.ok) {
        app.warpStatus = await res.json();
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to toggle WARP' }));
        alert('WARP Error: ' + (err.error || 'Failed'));
      }
    } catch (e) {
      alert('WARP Error: ' + e.message);
    } finally {
      app.isRotatingWarp = false;
      app.fetchWarpStatus();
    }
  }

  app.loadConfig = async function() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const cfg = await res.json();
        if (cfg.download_folder) {
          app.defaultFolder = cfg.download_folder;
          app.addTargetFolder = cfg.download_folder;
        }
        if (cfg.max_concurrency) app.maxConcurrency = cfg.max_concurrency;
        if (cfg.chunks_per_download) app.chunksPerDownload = cfg.chunks_per_download;
        if (typeof cfg.auto_warp_enabled === 'boolean') app.autoWarpEnabled = cfg.auto_warp_enabled;
        if (cfg.auto_warp_min_speed_mb) app.autoWarpMinSpeedMB = cfg.auto_warp_min_speed_mb;
        if (cfg.warp_proxy_port) app.warpProxyPort = cfg.warp_proxy_port;
        if (typeof cfg.custom_proxy_url === 'string') app.customProxyURL = cfg.custom_proxy_url;
        app.hasLogin = !!cfg.has_login;
        if (typeof cfg.auth_enabled === 'boolean') {
          app.authEnabled = cfg.auth_enabled;
          app.authEnabledToggle = cfg.auth_enabled;
        }
        if (cfg.username) {
          app.currentAuthUser = cfg.username;
          app.changeNewUsername = cfg.username;
        }
      }
    } catch (e) {
      console.warn('Config fetch error', e);
    }
  }

  app.saveConfig = async function() {
    try {
      await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          download_folder: app.defaultFolder,
          max_concurrency: parseInt(app.maxConcurrency, 10) || 2,
          chunks_per_download: parseInt(app.chunksPerDownload, 10) || 4,
          auto_warp_enabled: !!app.autoWarpEnabled,
          auto_warp_min_speed_mb: parseFloat(app.autoWarpMinSpeedMB) || 5.0,
          warp_proxy_port: parseInt(app.warpProxyPort, 10) || 40000,
          custom_proxy_url: app.customProxyURL.trim()
        })
      });
      app.addTargetFolder = app.defaultFolder;
      app.fetchWarpStatus();
      app.showSettingsModal = false;
    } catch (e) {
      alert('Error saving settings: ' + e.message);
    }
  }

  app.saveGoogleLogin = async function() {
    if (!app.loginCookieInput.trim()) return;
    app.isSavingLogin = true;
    try {
      const res = await fetch('/api/gdrive/cookie', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          cookie: app.loginCookieInput.trim()
        })
      });
      if (res.ok) {
        app.hasLogin = true;
        app.loginCookieInput = '';
        app.showLoginModal = false;
        alert('Google Session Cookie saved! Restricted GDrive links can now be downloaded.');
      }
    } catch (e) {
      alert('Failed to save Google session: ' + e.message);
    } finally {
      app.isSavingLogin = false;
    }
  }

  app.openConfirm = function(title, message, onConfirm, confirmText = 'Confirm', confirmType = 'danger') {
    app.confirmDialog = {
      show: true,
      title,
      message,
      confirmText,
      confirmType,
      onConfirm
    };
  }
}
