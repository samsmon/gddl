<script>
  import { onMount, onDestroy } from 'svelte';
  import FolderPicker from './lib/FolderPicker.svelte';

  // State (Svelte 5 runes)
  let downloads = $state([]);
  let activeFilter = $state('all'); // all, downloading, queued, paused, completed, corrupted, failed
  let searchQuery = $state('');
  let selectedIds = $state([]);
  let lastClickedId = $state(null);
  let defaultFolder = $state('C:\\Users\\Sam\\Downloads');
  let maxConcurrency = $state(2);
  let backendConnected = $state(false);
  let currentTheme = $state('dark');
  let hasLogin = $state(false);

  // Web UI Authentication state
  let authChecked = $state(false);
  let authEnabled = $state(true);
  let isAuthenticated = $state(false);
  let currentAuthUser = $state('admin');
  let loginUsername = $state('admin');
  let loginPassword = $state('');
  let loginError = $state('');
  let isLoggingIn = $state(false);

  // Security settings state
  let authEnabledToggle = $state(true);
  let changeOldPassword = $state('');
  let changeNewUsername = $state('admin');
  let changeNewPassword = $state('');
  let changeConfirmPassword = $state('');
  let isUpdatingSecurity = $state(false);
  let securityMessage = $state('');
  let securityError = $state('');

  // Sorting state
  let sortColumn = $state('added'); // name, size, done, prog, status, speed, eta, path, added
  let sortDirection = $state('desc'); // 'asc' or 'desc'

  // Resizable columns state
  let colWidths = $state({
    name: 280,
    size: 90,
    done: 90,
    prog: 160,
    status: 110,
    speed: 95,
    eta: 80,
    path: 200,
    added: 135
  });

  // Desktop Menu Bar state
  let openMenu = $state(null); // 'file', 'edit', 'view', 'tools', 'help', null

  // Help & Info Modals state
  let showDocModal = $state(false);
  let showUpdateModal = $state(false);
  let showAboutModal = $state(false);
  let updateChecking = $state(false);
  let updateStatus = $state('');

  // Logs state
  let showLogsModal = $state(false);
  let logs = $state([]);
  let logFilter = $state('ALL'); // ALL, ERROR, WARN, INFO, SUCCESS
  let logSearch = $state('');
  let isFetchingLogs = $state(false);
  let logsAutoRefresh = $state(true);
  let copiedLogs = $state(false);

  // Modals state
  let showAddModal = $state(false);
  let showSettingsModal = $state(false);
  let showLoginModal = $state(false);
  let showFolderPicker = $state(false);
  let folderPickerTarget = $state('add'); // 'add' or 'settings'
  let showDeleteModal = $state(false);
  let deleteModalTarget = $state('selected'); // 'selected', 'single', 'completed'
  let deleteModalSingleId = $state(null);
  let deleteModalWithFile = $state(false);

  // Conflict / Duplicate Resolution state
  let showConflictModal = $state(false);
  let conflictList = $state([]);
  let conflictResolutions = $state({});

  // Expandable folder downloads state
  let expandedFolderIds = $state([]);

  // Desktop Context Menu state
  let showContextMenu = $state(false);
  let contextMenuX = $state(0);
  let contextMenuY = $state(0);
  let contextMenuItem = $state(null);

  // Inputs
  let addLinksInput = $state('');
  let addTargetFolder = $state('C:\\Users\\Sam\\Downloads');
  let isAdding = $state(false);
  let loginCookieInput = $state('');
  let isSavingLogin = $state(false);

  // Folder detection state
  let detectedFolder = $state(null);
  let folderZipMode = $state('zip'); // 'zip' = Compress to ZIP, 'folder' = Subfolder
  let isResolvingFolder = $state(false);
  let resolveError = $state('');
  let resolveTimer = null;

  // Event stream and polling
  let eventSource = null;
  let pollInterval = null;

  // Derived filtered & sorted items
  let filteredDownloads = $derived.by(() => {
    let list = downloads.filter(item => {
      // Filter by status
      if (activeFilter === 'downloading' && item.status !== 'downloading' && item.status !== 'compressing') return false;
      if (activeFilter === 'queued' && item.status !== 'queued') return false;
      if (activeFilter === 'paused' && item.status !== 'paused') return false;
      if (activeFilter === 'completed' && item.status !== 'completed') return false;
      if (activeFilter === 'missing' && item.status !== 'missing') return false;
      if (activeFilter === 'corrupted' && item.status !== 'corrupted') return false;
      if (activeFilter === 'failed' && item.status !== 'failed' && item.status !== 'cancelled') return false;

      // Filter by search query
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const name = (item.filename || item.url || '').toLowerCase();
        return name.includes(q);
      }
      return true;
    });

    // Sorting
    list.sort((a, b) => {
      let valA, valB;
      switch (sortColumn) {
        case 'name':
          valA = (a.filename || a.url || '').toLowerCase();
          valB = (b.filename || b.url || '').toLowerCase();
          break;
        case 'size':
          valA = a.total_bytes || 0;
          valB = b.total_bytes || 0;
          break;
        case 'done':
          valA = a.downloaded_bytes || 0;
          valB = b.downloaded_bytes || 0;
          break;
        case 'prog':
          valA = a.percentage || (a.status === 'completed' ? 100 : 0);
          valB = b.percentage || (b.status === 'completed' ? 100 : 0);
          break;
        case 'status':
          valA = a.status;
          valB = b.status;
          break;
        case 'speed':
          valA = a.status === 'downloading' ? (a.speed || 0) : 0;
          valB = b.status === 'downloading' ? (b.speed || 0) : 0;
          break;
        case 'eta':
          valA = a.status === 'downloading' ? (a.eta_seconds || 999999) : 999999;
          valB = b.status === 'downloading' ? (b.eta_seconds || 999999) : 999999;
          break;
        case 'path':
          valA = (a.target_folder || '').toLowerCase();
          valB = (b.target_folder || '').toLowerCase();
          break;
        case 'added':
        default:
          valA = new Date(a.last_try_at || a.created_at).getTime() || 0;
          valB = new Date(b.last_try_at || b.created_at).getTime() || 0;
          break;
      }

      if (valA < valB) return sortDirection === 'asc' ? -1 : 1;
      if (valA > valB) return sortDirection === 'asc' ? 1 : -1;
      return 0;
    });

    return list;
  });

  // Selected items derived objects (supports single and multi-selection)
  let selectedId = $derived.by(() => selectedIds.length > 0 ? selectedIds[selectedIds.length - 1] : null);
  let selectedItems = $derived.by(() => downloads.filter(d => selectedIds.includes(d.id)));
  let selectedItem = $derived.by(() => {
    if (selectedIds.length === 0) return null;
    return downloads.find(d => d.id === selectedIds[selectedIds.length - 1]) || null;
  });

  // Counts and stats
  let counts = $derived.by(() => ({
    all: downloads.length,
    downloading: downloads.filter(d => d.status === 'downloading' || d.status === 'compressing').length,
    queued: downloads.filter(d => d.status === 'queued').length,
    paused: downloads.filter(d => d.status === 'paused').length,
    completed: downloads.filter(d => d.status === 'completed').length,
    missing: downloads.filter(d => d.status === 'missing').length,
    corrupted: downloads.filter(d => d.status === 'corrupted').length,
    failed: downloads.filter(d => d.status === 'failed' || d.status === 'cancelled').length,
    totalSpeed: downloads
      .filter(d => d.status === 'downloading')
      .reduce((sum, d) => sum + (d.speed || 0), 0)
  }));

  // Logs derived state
  let errorLogsCount = $derived(logs.filter(l => l.level === 'ERROR').length);

  let filteredLogs = $derived.by(() => {
    return logs.filter(l => {
      if (logFilter !== 'ALL' && l.level !== logFilter) return false;
      if (!logSearch.trim()) return true;
      const q = logSearch.toLowerCase();
      return (
        (l.message && l.message.toLowerCase().includes(q)) ||
        (l.category && l.category.toLowerCase().includes(q)) ||
        (l.details && l.details.toLowerCase().includes(q)) ||
        (l.level && l.level.toLowerCase().includes(q))
      );
    });
  });

  function toggleSort(column) {
    if (sortColumn === column) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortColumn = column;
      sortDirection = (column === 'size' || column === 'done' || column === 'prog' || column === 'speed' || column === 'added') ? 'desc' : 'asc';
    }
  }

  function toggleTheme() {
    currentTheme = currentTheme === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', currentTheme);
    localStorage.setItem('gdrive_theme', currentTheme);
  }

  function startResize(col, event) {
    event.preventDefault();
    const startX = event.clientX;
    const startW = colWidths[col];

    function onMouseMove(e) {
      const diff = e.clientX - startX;
      const newW = Math.max(30, startW + diff);
      colWidths[col] = newW;
    }

    function onMouseUp() {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
      try {
        localStorage.setItem('gdrive_col_widths', JSON.stringify(colWidths));
      } catch (err) {}
    }

    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  function autoSizeColumn(col) {
    // Auto-measure column based on content
    let maxLen = 8;
    for (const item of filteredDownloads) {
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
    colWidths[col] = Math.round(calculated);
    try {
      localStorage.setItem('gdrive_col_widths', JSON.stringify(colWidths));
    } catch (e) {}
  }

  function getItemSize(item) {
    if (!item) return '--';
    if (item.total_bytes > 0) return formatBytes(item.total_bytes);
    if (item.is_folder) {
      if (item.folder_files && item.folder_files.length > 0) {
        let sum = 0;
        let known = 0;
        for (const f of item.folder_files) {
          if (f.size && f.size > 0) {
            sum += f.size;
            known++;
          }
        }
        if (sum > 0) {
          return known === item.folder_files.length ? formatBytes(sum) : `~${formatBytes(sum)}`;
        }
      }
      return item.downloaded_bytes > 0 ? `~${formatBytes(item.downloaded_bytes)}` : '--';
    }
    return formatBytes(item.total_bytes);
  }

  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatSpeed(bytesPerSec) {
    if (!bytesPerSec || bytesPerSec === 0) return '0 KB/s';
    return formatBytes(bytesPerSec) + '/s';
  }

  function formatTime(seconds) {
    if (!seconds || seconds <= 0 || !isFinite(seconds)) return '∞';
    if (seconds < 60) return `${Math.ceil(seconds)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.ceil(seconds % 60);
    return `${mins}m ${secs}s`;
  }

  function formatDateTime(dateStr) {
    if (!dateStr) return '--';
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '--';
    const day = String(d.getDate()).padStart(2, '0');
    const mon = String(d.getMonth() + 1).padStart(2, '0');
    const yr = d.getFullYear();
    const hr = String(d.getHours()).padStart(2, '0');
    const min = String(d.getMinutes()).padStart(2, '0');
    return `${day}/${mon}/${yr} ${hr}:${min}`;
  }

  async function loadConfig() {
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        const cfg = await res.json();
        if (cfg.download_folder) {
          defaultFolder = cfg.download_folder;
          addTargetFolder = cfg.download_folder;
        }
        if (cfg.max_concurrency) maxConcurrency = cfg.max_concurrency;
        hasLogin = !!cfg.has_login;
        if (typeof cfg.auth_enabled === 'boolean') {
          authEnabled = cfg.auth_enabled;
          authEnabledToggle = cfg.auth_enabled;
        }
        if (cfg.username) {
          currentAuthUser = cfg.username;
          changeNewUsername = cfg.username;
        }
      }
    } catch (e) {
      console.warn('Config fetch error', e);
    }
  }

  async function saveConfig() {
    try {
      await fetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          download_folder: defaultFolder,
          max_concurrency: parseInt(maxConcurrency, 10) || 2
        })
      });
      showSettingsModal = false;
    } catch (e) {
      alert('Error saving settings: ' + e.message);
    }
  }

  async function saveGoogleLogin() {
    if (!loginCookieInput.trim()) return;
    isSavingLogin = true;
    try {
      const res = await fetch('/api/gdrive/cookie', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          cookie: loginCookieInput.trim()
        })
      });
      if (res.ok) {
        hasLogin = true;
        loginCookieInput = '';
        showLoginModal = false;
        alert('Google Session Cookie saved! Restricted GDrive links can now be downloaded.');
      }
    } catch (e) {
      alert('Failed to save Google session: ' + e.message);
    } finally {
      isSavingLogin = false;
    }
  }

  async function logoutGoogle() {
    try {
      await fetch('/api/gdrive/logout', { method: 'POST' });
      hasLogin = false;
      showLoginModal = false;
      alert('Google Drive session cookie cleared.');
    } catch (e) {
      alert('Error: ' + e.message);
    }
  }

  async function checkAuthStatus() {
    try {
      const res = await fetch('/api/auth/status');
      if (res.ok) {
        const data = await res.json();
        authEnabled = !!data.auth_enabled;
        authEnabledToggle = !!data.auth_enabled;
        isAuthenticated = !!data.authenticated;
        if (data.username) {
          currentAuthUser = data.username;
          changeNewUsername = data.username;
        }
      } else {
        isAuthenticated = false;
      }
    } catch (e) {
      console.warn('Failed to check auth status', e);
    } finally {
      authChecked = true;
      if (!authEnabled || isAuthenticated) {
        startAppServices();
      }
    }
  }

  async function submitLogin() {
    loginError = '';
    if (!loginUsername.trim() || !loginPassword) {
      loginError = 'Please enter both username and password';
      return;
    }

    isLoggingIn = true;
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: loginUsername.trim(),
          password: loginPassword
        })
      });

      if (res.ok) {
        const data = await res.json();
        isAuthenticated = true;
        currentAuthUser = data.username || loginUsername.trim();
        loginPassword = '';
        loginError = '';
        startAppServices();
      } else {
        const errData = await res.json().catch(() => ({ error: 'Login failed' }));
        loginError = errData.error || 'Invalid username or password';
      }
    } catch (e) {
      loginError = 'Connection error: ' + e.message;
    } finally {
      isLoggingIn = false;
    }
  }

  async function logoutWebUI() {
    if (!confirm('Log out of GDrive Web UI?')) return;
    try {
      await fetch('/api/auth/logout', { method: 'POST' });
    } catch (e) {
      console.warn(e);
    }
    isAuthenticated = false;
    if (eventSource) {
      eventSource.close();
      eventSource = null;
    }
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  async function updateSecurityCredentials() {
    securityError = '';
    securityMessage = '';

    if (changeNewPassword && changeNewPassword !== changeConfirmPassword) {
      securityError = 'New password and confirmation do not match.';
      return;
    }

    if (changeNewPassword && changeNewPassword.length < 4) {
      securityError = 'New password must be at least 4 characters long.';
      return;
    }

    isUpdatingSecurity = true;
    try {
      const res = await fetch('/api/auth/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          old_password: changeOldPassword,
          new_username: changeNewUsername.trim(),
          new_password: changeNewPassword
        })
      });

      if (res.ok) {
        securityMessage = 'Credentials updated successfully!';
        changeOldPassword = '';
        changeNewPassword = '';
        changeConfirmPassword = '';
        if (changeNewUsername.trim()) {
          currentAuthUser = changeNewUsername.trim();
        }
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to update credentials' }));
        securityError = err.error || 'Failed to update credentials';
      }
    } catch (e) {
      securityError = 'Error: ' + e.message;
    } finally {
      isUpdatingSecurity = false;
    }
  }

  async function toggleAuthRequirement() {
    try {
      const res = await fetch('/api/auth/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: authEnabledToggle })
      });
      if (res.ok) {
        authEnabled = authEnabledToggle;
        securityMessage = `Web UI Authentication ${authEnabled ? 'Enabled' : 'Disabled'}.`;
      }
    } catch (e) {
      securityError = 'Failed to toggle auth: ' + e.message;
    }
  }

  function startAppServices() {
    loadConfig();
    fetchDownloads();
    fetchLogs();
    setupSSE();
    if (!pollInterval) {
      pollInterval = setInterval(() => {
        fetchDownloads();
        if (showLogsModal) {
          fetchLogs();
        }
      }, 3000);
    }
  }

  async function fetchLogs() {
    if (authEnabled && !isAuthenticated) return;
    isFetchingLogs = true;
    try {
      const res = await fetch('/api/logs?limit=300');
      if (res.ok) {
        logs = await res.json();
      }
    } catch (e) {
      console.error('Failed to fetch logs:', e);
    } finally {
      isFetchingLogs = false;
    }
  }

  async function clearLogs() {
    try {
      await fetch('/api/logs', { method: 'DELETE' });
      logs = [];
    } catch (e) {
      console.error('Failed to clear logs:', e);
    }
  }

  function openLogsModal(filter = 'ALL') {
    logFilter = filter;
    showLogsModal = true;
    fetchLogs();
  }

  function copyLogsToClipboard() {
    if (logs.length === 0) return;
    const text = logs.map(l => `[${l.timestamp}] [${l.level}] [${l.category}] ${l.message}${l.details ? ' | ' + l.details : ''}`).join('\n');
    navigator.clipboard.writeText(text).then(() => {
      copiedLogs = true;
      setTimeout(() => copiedLogs = false, 2000);
    }).catch(() => {});
  }

  async function fetchDownloads() {
    if (authEnabled && !isAuthenticated) return;
    try {
      const res = await fetch('/api/downloads');
      if (res.status === 401) {
        isAuthenticated = false;
        backendConnected = false;
        return;
      }
      if (res.ok) {
        downloads = await res.json();
        backendConnected = true;
      }
    } catch (e) {
      backendConnected = false;
    }
  }

  function setupSSE() {
    if (authEnabled && !isAuthenticated) return;
    if (eventSource) eventSource.close();
    try {
      eventSource = new EventSource('/api/events');
      eventSource.onopen = () => { backendConnected = true; };
      eventSource.onmessage = (event) => {
        try {
          downloads = JSON.parse(event.data);
          backendConnected = true;
        } catch (err) {
          console.error(err);
        }
      };
      eventSource.onerror = () => {
        backendConnected = false;
        eventSource.close();
        if (isAuthenticated || !authEnabled) {
          setTimeout(setupSSE, 3000);
        }
      };
    } catch (e) {
      console.error(e);
    }
  }

  function handleLinksInput() {
    clearTimeout(resolveTimer);
    detectedFolder = null;
    resolveError = '';

    const lines = addLinksInput.split(/[\n,]+/).map(s => s.trim()).filter(Boolean);
    const folderLink = lines.find(l => 
      l.includes('/drive/folders/') || 
      l.includes('/drive/u/') || 
      l.includes('folderview?id=') ||
      (l.includes('drive.google.com/open?id=') && l.includes('usp=drive_link'))
    );

    if (folderLink) {
      resolveTimer = setTimeout(() => {
        resolveFolderInfo(folderLink);
      }, 400);
    }
  }

  async function resolveFolderInfo(url) {
    isResolvingFolder = true;
    resolveError = '';
    try {
      const res = await fetch('/api/folders/resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url })
      });
      if (res.ok) {
        const data = await res.json();
        if (data.is_folder) {
          detectedFolder = data;
        }
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to inspect folder' }));
        resolveError = err.error || 'Failed to inspect folder';
      }
    } catch (e) {
      resolveError = e.message;
    } finally {
      isResolvingFolder = false;
    }
  }

  async function submitAddDownloads() {
    if (!addLinksInput.trim()) return;

    const raw = addLinksInput
      .split(/[\n,]+/)
      .map(l => l.trim())
      .filter(l => l.length > 0);

    if (raw.length === 0) return;

    isAdding = true;
    try {
      // 1. Fast conflict precheck
      const checkRes = await fetch('/api/downloads/precheck', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          links: raw,
          target_folder: addTargetFolder || defaultFolder,
          zip_mode: folderZipMode === 'zip'
        })
      });

      if (checkRes.ok) {
        const checkData = await checkRes.json();
        if (checkData.has_conflicts && checkData.conflicts && checkData.conflicts.length > 0) {
          conflictList = checkData.conflicts;
          const initialResolutions = {};
          for (const c of checkData.conflicts) {
            initialResolutions[c.file_id || c.url] = c.suggested_action || 'rename';
          }
          conflictResolutions = initialResolutions;
          showConflictModal = true;
          isAdding = false;
          return;
        }
      }

      // 2. If no conflicts detected, download immediately
      await executeAddDownloads({});
    } catch (e) {
      alert('Error: ' + e.message);
      isAdding = false;
    }
  }

  async function executeAddDownloads(resolutions = {}) {
    if (!addLinksInput.trim()) return;

    const raw = addLinksInput
      .split(/[\n,]+/)
      .map(l => l.trim())
      .filter(l => l.length > 0);

    if (raw.length === 0) return;

    isAdding = true;
    try {
      const res = await fetch('/api/downloads', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          links: raw,
          target_folder: addTargetFolder || defaultFolder,
          zip_mode: folderZipMode === 'zip',
          conflict_resolutions: resolutions
        })
      });

      if (res.ok) {
        addLinksInput = '';
        detectedFolder = null;
        resolveError = '';
        folderZipMode = 'zip';
        showConflictModal = false;
        showAddModal = false;
        fetchDownloads();
      } else {
        const err = await res.json();
        alert(err.error || 'Failed to add links');
      }
    } catch (e) {
      alert('Error: ' + e.message);
    } finally {
      isAdding = false;
    }
  }

  function setAllConflictActions(action) {
    const updated = { ...conflictResolutions };
    for (const c of conflictList) {
      updated[c.file_id || c.url] = action;
    }
    conflictResolutions = updated;
  }

  // Row selection handler supporting Click, Ctrl+Click / Cmd+Click, and Shift+Click
  function handleRowClick(item, e) {
    if (!item) return;

    // Real-time file presence check when row is selected/clicked
    if (item.status === 'completed' || item.status === 'missing') {
      checkFileStatus(item.id);
    }

    if (e.shiftKey) {
      // Shift+Click: Range selection between lastClickedId and item.id
      if (lastClickedId) {
        const fromIdx = filteredDownloads.findIndex(d => d.id === lastClickedId);
        const toIdx = filteredDownloads.findIndex(d => d.id === item.id);
        if (fromIdx !== -1 && toIdx !== -1) {
          const start = Math.min(fromIdx, toIdx);
          const end = Math.max(fromIdx, toIdx);
          const rangeIds = filteredDownloads.slice(start, end + 1).map(d => d.id);

          if (e.ctrlKey || e.metaKey) {
            const combined = new Set([...selectedIds, ...rangeIds]);
            selectedIds = Array.from(combined);
          } else {
            selectedIds = rangeIds;
          }
          return;
        }
      }
      // If no pivot, select clicked item
      selectedIds = [item.id];
      lastClickedId = item.id;
    } else if (e.ctrlKey || e.metaKey) {
      // Ctrl+Click / Cmd+Click: Toggle selection
      if (selectedIds.includes(item.id)) {
        selectedIds = selectedIds.filter(id => id !== item.id);
      } else {
        selectedIds = [...selectedIds, item.id];
      }
      lastClickedId = item.id;
    } else {
      // Standard Click: select only this item
      selectedIds = [item.id];
      lastClickedId = item.id;
    }
  }

  // Expand / Collapse folder details
  function toggleExpandFolder(id, event) {
    if (event) {
      event.preventDefault();
      event.stopPropagation();
    }
    if (expandedFolderIds.includes(id)) {
      expandedFolderIds = expandedFolderIds.filter(x => x !== id);
    } else {
      expandedFolderIds = [...expandedFolderIds, id];
    }
  }

  // Right-click Desktop Context Menu
  function handleContextMenu(item, e) {
    e.preventDefault();
    e.stopPropagation();
    if (!item) return;

    if (!selectedIds.includes(item.id)) {
      selectedIds = [item.id];
      lastClickedId = item.id;
    }
    contextMenuItem = item;

    const menuW = 220;
    const menuH = 320;
    let x = e.clientX;
    let y = e.clientY;
    if (x + menuW > window.innerWidth) x = Math.max(10, window.innerWidth - menuW - 10);
    if (y + menuH > window.innerHeight) y = Math.max(10, window.innerHeight - menuH - 10);

    contextMenuX = x;
    contextMenuY = y;
    showContextMenu = true;
  }

  function closeContextMenu() {
    showContextMenu = false;
  }

  // Desktop Menu Bar Controls
  function toggleMenu(menuName, e) {
    if (e) e.stopPropagation();
    openMenu = openMenu === menuName ? null : menuName;
  }

  function handleMenuHover(menuName) {
    if (openMenu !== null && openMenu !== menuName) {
      openMenu = menuName;
    }
  }

  function closeMenus() {
    openMenu = null;
  }

  // Real-time File Existence Detection
  async function checkFileStatus(id) {
    if (!id) return;
    try {
      const res = await fetch(`/api/downloads/${id}/check`, { method: 'POST' });
      if (res.ok) {
        fetchDownloads();
      }
    } catch (e) {
      console.warn('File check failed', e);
    }
  }

  async function checkAllFiles() {
    try {
      await fetch('/api/downloads/check-all', { method: 'POST' });
      fetchDownloads();
    } catch (e) {
      console.warn('Check all failed', e);
    }
  }

  // Global Batch Controls
  async function startAll() {
    const toStart = downloads.filter(d => d.status === 'paused' || d.status === 'failed' || d.status === 'cancelled' || d.status === 'missing');
    for (const item of toStart) {
      fetch(`/api/downloads/${item.id}/start`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(fetchDownloads, 250);
  }

  async function pauseAll() {
    const toPause = downloads.filter(d => d.status === 'downloading' || d.status === 'queued' || d.status === 'compressing');
    for (const item of toPause) {
      fetch(`/api/downloads/${item.id}/pause`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(fetchDownloads, 250);
  }

  function invertSelection() {
    const allFilteredIds = filteredDownloads.map(d => d.id);
    selectedIds = allFilteredIds.filter(id => !selectedIds.includes(id));
  }

  function expandAllFolders() {
    expandedFolderIds = downloads.filter(d => d.is_folder).map(d => d.id);
  }

  function collapseAllFolders() {
    expandedFolderIds = [];
  }

  function resetColumnWidths() {
    colWidths = {
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

  function checkForUpdates() {
    showUpdateModal = true;
    updateChecking = true;
    updateStatus = 'Connecting to release server...';
    setTimeout(() => {
      updateChecking = false;
      updateStatus = 'You are running the latest version (v1.2.0 Desktop Persistent Edition).';
    }, 700);
  }

  // Unified 2-Option Delete Modal ("apply di button delete dan segala delete")
  function openDeleteModal(mode = 'selected', id = null, defaultWithFile = false) {
    deleteModalTarget = mode;
    deleteModalSingleId = id;
    deleteModalWithFile = defaultWithFile;
    showDeleteModal = true;
    showContextMenu = false;
  }

  async function confirmDeleteModal() {
    showDeleteModal = false;
    const withFile = deleteModalWithFile;
    if (deleteModalTarget === 'completed') {
      await executeClearCompleted(withFile);
    } else if (deleteModalTarget === 'single' && deleteModalSingleId) {
      await executeDeleteSingle(deleteModalSingleId, withFile);
    } else {
      await executeDeleteSelected(withFile);
    }
  }

  async function executeDeleteSelected(deleteFile = false) {
    if (selectedIds.length === 0) return;
    const idsToDelete = [...selectedIds];
    selectedIds = [];
    lastClickedId = null;
    for (const id of idsToDelete) {
      try {
        await fetch(`/api/downloads/${id}?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'DELETE' });
      } catch (e) {
        console.error(e);
      }
    }
    fetchDownloads();
  }

  async function executeDeleteSingle(id, deleteFile = false) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'DELETE' });
      selectedIds = selectedIds.filter(x => x !== id);
      fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  async function executeClearCompleted(deleteFile = false) {
    try {
      await fetch(`/api/downloads/clear?delete_file=${deleteFile ? 'true' : 'false'}`, { method: 'POST' });
      selectedIds = selectedIds.filter(id => {
        const item = downloads.find(d => d.id === id);
        return item && item.status !== 'completed' && item.status !== 'corrupted' && item.status !== 'cancelled';
      });
      fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  // Multi-Selection Batch Actions
  async function startSelected() {
    if (selectedIds.length === 0) return;
    for (const id of selectedIds) {
      fetch(`/api/downloads/${id}/start`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(fetchDownloads, 200);
  }

  async function pauseSelected() {
    if (selectedIds.length === 0) return;
    for (const id of selectedIds) {
      fetch(`/api/downloads/${id}/pause`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(fetchDownloads, 200);
  }

  async function restartSelected() {
    if (selectedIds.length === 0) return;
    for (const id of selectedIds) {
      fetch(`/api/downloads/${id}/restart`, { method: 'POST' }).catch(console.error);
    }
    setTimeout(fetchDownloads, 200);
  }

  // Direct delete helper (used by direct buttons or shortcuts)
  function deleteSelected(deleteFile = false) {
    openDeleteModal('selected', null, deleteFile);
  }

  // Task Control Functions (Single item fallbacks)
  async function startDownload(id) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}/start`, { method: 'POST' });
      fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  async function pauseDownload(id) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}/pause`, { method: 'POST' });
      fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  async function restartDownload(id) {
    if (!id) return;
    try {
      await fetch(`/api/downloads/${id}/restart`, { method: 'POST' });
      fetchDownloads();
    } catch (e) {
      console.error(e);
    }
  }

  function deleteDownload(id, deleteFile = false) {
    openDeleteModal('single', id, deleteFile);
  }

  function clearCompleted() {
    openDeleteModal('completed', null, false);
  }

  function openPickerFor(target) {
    folderPickerTarget = target;
    showFolderPicker = true;
  }

  function handleFolderSelected(path) {
    if (folderPickerTarget === 'add') {
      addTargetFolder = path;
    } else if (folderPickerTarget === 'settings') {
      defaultFolder = path;
    }
    showFolderPicker = false;
  }

  // Keyboard Shortcuts (Ctrl+A select all, Escape unselect, Delete to delete with 2 options, Space to pause/resume)
  function handleKeyDown(e) {
    if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

    if (showDeleteModal) {
      if (e.key === 'Escape') {
        showDeleteModal = false;
        return;
      }
      if (e.key === 'Enter') {
        confirmDeleteModal();
        return;
      }
      return;
    }

    if (e.key === 'Escape') {
      if (openMenu) {
        closeMenus();
        return;
      }
      if (showContextMenu) {
        closeContextMenu();
        return;
      }
      if (showDocModal) {
        showDocModal = false;
        return;
      }
      if (showUpdateModal) {
        showUpdateModal = false;
        return;
      }
      if (showAboutModal) {
        showAboutModal = false;
        return;
      }
      if (showLogsModal) {
        showLogsModal = false;
        return;
      }
      selectedIds = [];
      lastClickedId = null;
      return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'n') {
      e.preventDefault();
      showAddModal = true;
      return;
    }

    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
      e.preventDefault();
      selectedIds = filteredDownloads.map(d => d.id);
      if (filteredDownloads.length > 0) {
        lastClickedId = filteredDownloads[0].id;
      }
      return;
    }

    if (e.key === 'Delete' && selectedIds.length > 0) {
      e.preventDefault();
      openDeleteModal('selected', null, e.shiftKey);
    } else if (e.key === ' ' && selectedIds.length > 0) {
      e.preventDefault();
      const hasActive = selectedItems.some(d => d.status === 'downloading' || d.status === 'queued');
      if (hasActive) {
        pauseSelected();
      } else {
        startSelected();
      }
    }
  }

  onMount(() => {
    // Theme setup
    const savedTheme = localStorage.getItem('gdrive_theme') || 'dark';
    currentTheme = savedTheme;
    document.documentElement.setAttribute('data-theme', savedTheme);

    // Column widths setup
    try {
      const savedWidths = localStorage.getItem('gdrive_col_widths');
      if (savedWidths) {
        Object.assign(colWidths, JSON.parse(savedWidths));
        if (!colWidths.prog || colWidths.prog < 150) {
          colWidths.prog = 160;
        }
      }
    } catch (e) {}

    checkAuthStatus();

    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('click', closeContextMenu);
    window.addEventListener('click', closeMenus);
  });

  onDestroy(() => {
    if (eventSource) eventSource.close();
    if (pollInterval) clearInterval(pollInterval);
    window.removeEventListener('keydown', handleKeyDown);
    window.removeEventListener('click', closeContextMenu);
    window.removeEventListener('click', closeMenus);
  });
</script>

{#if !authChecked}
  <div class="qb-login-overlay">
    <div style="color: var(--text-muted); font-size: 13px; display: flex; align-items: center; gap: 8px;">
      <span>Connecting to GDrive Client...</span>
    </div>
  </div>
{:else if authEnabled && !isAuthenticated}
  <div class="qb-login-overlay">
    <div class="qb-login-card">
      <div class="qb-login-header">
        <div class="qb-login-title-row">
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="var(--accent-blue)" stroke-width="2">
            <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
            <path d="M12 12v9"></path>
            <path d="m8 17 4 4 4-4"></path>
          </svg>
          <div class="qb-login-title-group">
            <span class="qb-login-title">GDrive Downloader</span>
            <span class="qb-login-subtitle">Web UI Access • Authentication Required</span>
          </div>
        </div>
        <button class="tb-btn" onclick={toggleTheme} title="Toggle Dark/Light Theme" style="padding: 4px 8px;">
          {#if currentTheme === 'dark'}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>
          {:else}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12.3 2a10 10 0 0 0-1.9 19.8 10 10 0 0 0 11.4-11.4A10 10 0 0 0 12.3 2z"/></svg>
          {/if}
        </button>
      </div>

      <form class="qb-login-form" onsubmit={(e) => { e.preventDefault(); submitLogin(); }}>
        {#if loginError}
          <div class="qb-login-error">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="8" x2="12" y2="12"></line>
              <line x1="12" y1="16" x2="12.01" y2="16"></line>
            </svg>
            <span>{loginError}</span>
          </div>
        {/if}

        <div class="qb-form-row">
          <label for="qb-user">Username:</label>
          <input
            id="qb-user"
            type="text"
            bind:value={loginUsername}
            placeholder="admin"
            required
            autocomplete="username"
          />
        </div>

        <div class="qb-form-row">
          <label for="qb-pass">Password:</label>
          <input
            id="qb-pass"
            type="password"
            bind:value={loginPassword}
            placeholder="••••••••"
            required
            autocomplete="current-password"
          />
        </div>

        <div class="qb-login-footer">
          <span class="qb-login-hint">Default: admin / adminadmin</span>
          <button type="submit" class="btn btn-primary qb-login-btn" disabled={isLoggingIn}>
            {isLoggingIn ? 'Logging in...' : 'Log In'}
          </button>
        </div>
      </form>
    </div>
  </div>
{:else}
<div class="desktop-app">
  <!-- Top Native Desktop Menu Bar (File, Edit, View, Tools, Help) -->
  <nav class="app-menubar" onclick={(e) => e.stopPropagation()}>
    <!-- FILE MENU -->
    <div class="menubar-item" class:active={openMenu === 'file'}>
      <button class="menubar-btn" onclick={(e) => toggleMenu('file', e)} onmouseenter={() => handleMenuHover('file')}>
        File
      </button>
      {#if openMenu === 'file'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { showAddModal = true; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/></svg></span>
            <span class="dropdown-text">Add Links...</span>
            <span class="dropdown-shortcut">Ctrl+N</span>
          </button>
          <button class="dropdown-item" onclick={() => { clearCompleted(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19.36 10.04l-7.4-7.4c-.78-.78-2.05-.78-2.83 0L2.71 9.06c-.78.78-.78 2.05 0 2.83l7.4 7.4c.78.78 2.05.78 2.83 0l6.42-6.42-6.42-6.42 1.41-1.41 7.41 7.41c.39.39.39 1.02 0 1.41l-2.4 2.4-1.41-1.41 1.69-1.69zM4.12 10.47l6.42-6.42 6.42 6.42-6.42 6.42-6.42-6.42z"/></svg></span>
            <span class="dropdown-text">Clear Completed</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { startAll(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M8 5v14l11-7z"/></svg></span>
            <span class="dropdown-text">Resume All</span>
          </button>
          <button class="dropdown-item" onclick={() => { pauseAll(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z"/></svg></span>
            <span class="dropdown-text">Pause All</span>
          </button>
          <button class="dropdown-item" onclick={() => { checkAllFiles(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/></svg></span>
            <span class="dropdown-text">Check All Files on Disk</span>
          </button>
          {#if authEnabled}
            <div class="menu-divider"></div>
            <button class="dropdown-item text-danger" onclick={() => { logoutWebUI(); closeMenus(); }}>
              <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M17 7l-1.41 1.41L18.17 11H8v2h10.17l-2.58 2.58L17 17l5-5zM4 5h8V3H4c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h8v-2H4V5z"/></svg></span>
              <span class="dropdown-text">Log Out Web UI</span>
            </button>
          {/if}
        </div>
      {/if}
    </div>

    <!-- EDIT MENU -->
    <div class="menubar-item" class:active={openMenu === 'edit'}>
      <button class="menubar-btn" onclick={(e) => toggleMenu('edit', e)} onmouseenter={() => handleMenuHover('edit')}>
        Edit
      </button>
      {#if openMenu === 'edit'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { selectedIds = filteredDownloads.map(d => d.id); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 3H5c-1.11 0-2 .9-2 2v14c0 1.1.89 2 2 2h14c1.11 0 2-.9 2-2V5c0-1.1-.89-2-2-2zm-9 14l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg></span>
            <span class="dropdown-text">Select All</span>
            <span class="dropdown-shortcut">Ctrl+A</span>
          </button>
          <button class="dropdown-item" disabled={selectedIds.length === 0} onclick={() => { selectedIds = []; lastClickedId = null; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 5v14H5V5h14m0-2H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z"/></svg></span>
            <span class="dropdown-text">Deselect All</span>
            <span class="dropdown-shortcut">Esc</span>
          </button>
          <button class="dropdown-item" onclick={() => { invertSelection(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M6.99 11L3 15l3.99 4v-3H14v-2H6.99v-3zM21 9l-3.99-4v3H10v2h7.01v3L21 9z"/></svg></span>
            <span class="dropdown-text">Invert Selection</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item text-danger" disabled={selectedIds.length === 0} onclick={() => { openDeleteModal('selected', null, false); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"/></svg></span>
            <span class="dropdown-text">Delete...</span>
            <span class="dropdown-shortcut">Del</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- VIEW MENU -->
    <div class="menubar-item" class:active={openMenu === 'view'}>
      <button class="menubar-btn" onclick={(e) => toggleMenu('view', e)} onmouseenter={() => handleMenuHover('view')}>
        View
      </button>
      {#if openMenu === 'view'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { toggleTheme(); closeMenus(); }}>
            <span class="dropdown-icon">
              {#if currentTheme === 'dark'}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>
              {:else}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12.3 2a10 10 0 0 0-1.9 19.8 10 10 0 0 0 11.4-11.4A10 10 0 0 0 12.3 2z"/></svg>
              {/if}
            </span>
            <span class="dropdown-text">Switch to {currentTheme === 'dark' ? 'Light Theme' : 'Dark Theme'}</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { expandAllFolders(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M20 6h-8l-2-2H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm0 12H4V8h16v10z"/></svg></span>
            <span class="dropdown-text">Expand All Folder Archives</span>
          </button>
          <button class="dropdown-item" onclick={() => { collapseAllFolders(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg></span>
            <span class="dropdown-text">Collapse All Folder Archives</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { resetColumnWidths(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M21 6H3c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm0 10H3V8h2v4h2V8h2v4h2V8h2v4h2V8h2v4h2V8h3v8z"/></svg></span>
            <span class="dropdown-text">Reset Column Widths</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { activeFilter = 'all'; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 3H4.99c-1.11 0-1.98.89-1.98 2L3 19c0 1.1.88 2 1.99 2H19c1.1 0 2-.9 2-2V5c0-1.11-.9-2-2-2zm0 12h-4c0 1.66-1.35 3-3 3s-3-1.34-3-3H4.99V5H19v10z"/></svg></span>
            <span class="dropdown-text">All Downloads ({counts.all})</span>
          </button>
          <button class="dropdown-item" onclick={() => { activeFilter = 'downloading'; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M7 2v11h3v9l7-12h-4l4-8z"/></svg></span>
            <span class="dropdown-text">Downloading ({counts.downloading})</span>
          </button>
          <button class="dropdown-item" onclick={() => { activeFilter = 'completed'; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/></svg></span>
            <span class="dropdown-text">Completed ({counts.completed})</span>
          </button>
          <button class="dropdown-item" onclick={() => { activeFilter = 'missing'; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M1 21h22L12 2 1 21zm12-3h-2v-2h2v2zm0-4h-2v-4h2v4z"/></svg></span>
            <span class="dropdown-text">Missing / Moved ({counts.missing})</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- TOOLS MENU -->
    <div class="menubar-item" class:active={openMenu === 'tools'}>
      <button class="menubar-btn" onclick={(e) => toggleMenu('tools', e)} onmouseenter={() => handleMenuHover('tools')}>
        Tools
      </button>
      {#if openMenu === 'tools'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { showLoginModal = true; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M21 11c0 5.52-4.48 10-10 10S1 16.52 1 11 5.48 1 11 1s10 4.48 10 10zm-3.5-3.5c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm-5-3c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm-4 7c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5zm7 4c-.83 0-1.5.67-1.5 1.5s.67 1.5 1.5 1.5 1.5-.67 1.5-1.5-.67-1.5-1.5-1.5z"/></svg></span>
            <span class="dropdown-text">Google Session Cookie...</span>
          </button>
          <button class="dropdown-item" onclick={() => { showSettingsModal = true; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/></svg></span>
            <span class="dropdown-text">Options & Preferences...</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { checkAllFiles(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 10.99h7c-.53 4.12-3.28 7.79-7 8.94V12H5V6.3l7-3.11v8.8z"/></svg></span>
            <span class="dropdown-text">Verify File Integrity & Disk Check</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { openLogsModal(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-5 14H7v-2h7v2zm3-4H7v-2h10v2zm0-4H7V7h10v2z"/></svg></span>
            <span class="dropdown-text">Execution & Error Logs...</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- HELP MENU -->
    <div class="menubar-item" class:active={openMenu === 'help'}>
      <button class="menubar-btn" onclick={(e) => toggleMenu('help', e)} onmouseenter={() => handleMenuHover('help')}>
        Help
      </button>
      {#if openMenu === 'help'}
        <div class="menubar-dropdown" role="menu">
          <button class="dropdown-item" onclick={() => { showDocModal = true; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M18 2H6c-1.1 0-2 .9-2 2v16c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zM6 4h5v8l-2.5-1.5L6 12V4z"/></svg></span>
            <span class="dropdown-text">Documentation & Guide</span>
          </button>
          <a class="dropdown-item" href="https://github.com" target="_blank" rel="noopener noreferrer" onclick={closeMenus}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/></svg></span>
            <span class="dropdown-text">Project GitHub Repository ↗</span>
          </a>
          <button class="dropdown-item" onclick={() => { checkForUpdates(); closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46C19.54 15.03 20 13.57 20 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74C4.46 8.97 4 10.43 4 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"/></svg></span>
            <span class="dropdown-text">Check for Updates...</span>
          </button>
          <div class="menu-divider"></div>
          <button class="dropdown-item" onclick={() => { showAboutModal = true; closeMenus(); }}>
            <span class="dropdown-icon"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/></svg></span>
            <span class="dropdown-text">About GDrive Downloader</span>
          </button>
        </div>
      {/if}
    </div>
  </nav>

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
      <button class="tb-btn tb-btn-primary" onclick={() => showAddModal = true} title="Add new Google Drive download links">
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
        disabled={selectedIds.length === 0}
        onclick={startSelected}
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
        disabled={selectedIds.length === 0 || !selectedItems.some(d => d.status === 'downloading' || d.status === 'queued')}
        onclick={pauseSelected}
        title="Pause selected download(s)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <rect x="6" y="4" width="4" height="16"></rect>
          <rect x="14" y="4" width="4" height="16"></rect>
        </svg>
        <span>Pause</span>
      </button>

      <!-- Restart (IDM) -->
      <button
        class="tb-btn"
        disabled={selectedIds.length === 0}
        onclick={restartSelected}
        title="Restart selected download(s) from beginning"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
        </svg>
        <span>Restart</span>
      </button>

      <!-- Delete -->
      <button
        class="tb-btn"
        disabled={selectedIds.length === 0}
        onclick={() => openDeleteModal('selected', null, false)}
        title="Delete selected item(s) (Del)"
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polyline points="3 6 5 6 21 6"></polyline>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          <line x1="10" y1="11" x2="10" y2="17"></line>
          <line x1="14" y1="11" x2="14" y2="17"></line>
        </svg>
        <span>Delete{selectedIds.length > 1 ? ` (${selectedIds.length})` : ''}</span>
      </button>

      <!-- Clear Completed -->
      <button class="tb-btn" onclick={clearCompleted} title="Delete all completed/cancelled tasks from list">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
          <polyline points="22 4 12 14.01 9 11.01"></polyline>
        </svg>
        <span>Delete Completed</span>
      </button>

      <div class="tb-separator"></div>

      <!-- Google Cookie Session (For downloading restricted files) -->
      <button class="tb-btn" onclick={() => showLoginModal = true} title="Google Account Session Cookie (for restricted GDrive files)">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
          <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
        </svg>
        <span>{hasLogin ? 'Google Active' : 'Google Cookie'}</span>
        {#if hasLogin}
          <span class="auth-dot"></span>
        {/if}
      </button>

      <!-- Options -->
      <button class="tb-btn" onclick={() => showSettingsModal = true} title="Preferences, Web UI Security & Target Folder">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3"></circle>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
        </svg>
        <span>Options</span>
      </button>

      <!-- Logs -->
      <button class="tb-btn" onclick={() => openLogsModal()} title="View Execution & Error Logs">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="4 17 10 11 4 5"></polyline>
          <line x1="12" y1="19" x2="20" y2="19"></line>
        </svg>
        <span>Logs</span>
        {#if errorLogsCount > 0}
          <span class="log-badge-error">{errorLogsCount}</span>
        {/if}
      </button>

      <!-- Theme Switcher -->
      <button class="tb-btn" onclick={toggleTheme} title="Toggle Light / Dark Theme">
        {#if currentTheme === 'dark'}
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="5"></circle>
            <line x1="12" y1="1" x2="12" y2="3"></line>
            <line x1="12" y1="21" x2="12" y2="23"></line>
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line>
            <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line>
            <line x1="1" y1="12" x2="3" y2="12"></line>
            <line x1="21" y1="12" x2="23" y2="12"></line>
            <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line>
            <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line>
          </svg>
          <span>Light</span>
        {:else}
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path>
          </svg>
          <span>Dark</span>
        {/if}
      </button>

      {#if authEnabled}
        <div class="tb-separator"></div>

        <div class="user-pill" title="Current Web UI User">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
            <circle cx="12" cy="7" r="4"></circle>
          </svg>
          <span>{currentAuthUser}</span>
        </div>

        <button class="tb-btn" onclick={logoutWebUI} title="Log out of Web UI">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"></path>
            <polyline points="16 17 21 12 16 7"></polyline>
            <line x1="21" y1="12" x2="9" y2="12"></line>
          </svg>
          <span>Logout</span>
        </button>
      {/if}
    </div>

    <!-- Search filter -->
    <div class="toolbar-search">
      <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--text-dim)" stroke-width="2.5">
        <circle cx="11" cy="11" r="8"></circle>
        <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
      </svg>
      <input
        type="text"
        placeholder="Filter list..."
        bind:value={searchQuery}
      />
      {#if searchQuery}
        <button class="clear-search-btn" aria-label="Clear Search" onclick={() => searchQuery = ''}>
          <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      {/if}
    </div>
  </header>

  <!-- Main Body: Sidebar + Resizable & Sortable Data Grid Table -->
  <div class="workspace">
    <!-- Left Category Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">Categories</div>
      <nav class="category-list">
        <button
          class="cat-item"
          class:active={activeFilter === 'all'}
          onclick={() => activeFilter = 'all'}
        >
          <span class="cat-label">All Downloads</span>
          <span class="cat-badge">{counts.all}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'downloading'}
          onclick={() => activeFilter = 'downloading'}
        >
          <span class="cat-label">
            <span class="status-indicator active-dot"></span>
            Downloading
          </span>
          <span class="cat-badge">{counts.downloading}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'queued'}
          onclick={() => activeFilter = 'queued'}
        >
          <span class="cat-label">
            <span class="status-indicator queued-dot"></span>
            Queued
          </span>
          <span class="cat-badge">{counts.queued}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'paused'}
          onclick={() => activeFilter = 'paused'}
        >
          <span class="cat-label">
            <span class="status-indicator paused-dot"></span>
            Paused
          </span>
          <span class="cat-badge">{counts.paused}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'completed'}
          onclick={() => activeFilter = 'completed'}
        >
          <span class="cat-label">
            <span class="status-indicator completed-dot"></span>
            Completed
          </span>
          <span class="cat-badge">{counts.completed}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'missing'}
          onclick={() => activeFilter = 'missing'}
        >
          <span class="cat-label">
            <span class="status-indicator missing-dot"></span>
            Missing / Moved
          </span>
          <span class="cat-badge" style="color: var(--accent-amber); font-weight: bold;">{counts.missing}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'corrupted'}
          onclick={() => activeFilter = 'corrupted'}
        >
          <span class="cat-label">
            <span class="status-indicator corrupt-dot"></span>
            Corrupted
          </span>
          <span class="cat-badge" style="color: var(--accent-red); font-weight: bold;">{counts.corrupted}</span>
        </button>

        <button
          class="cat-item"
          class:active={activeFilter === 'failed'}
          onclick={() => activeFilter = 'failed'}
        >
          <span class="cat-label">
            <span class="status-indicator failed-dot"></span>
            Error / Cancelled
          </span>
          <span class="cat-badge">{counts.failed}</span>
        </button>
      </nav>

      <!-- Quick Folder Info / Jellyfin Picker Trigger -->
      <div class="sidebar-folder-box">
        <div class="folder-box-header">
          <span>DEFAULT SAVE PATH</span>
          <button class="btn-browse-mini" onclick={() => openPickerFor('settings')}>Browse</button>
        </div>
        <div class="folder-box-path" title={defaultFolder}>
          {defaultFolder}
        </div>
      </div>
    </aside>

    <!-- Main Content: Data Grid Table with Click-to-Sort & Resizable Columns -->
    <main class="content-pane">
      <div class="table-container">
        <table class="torrent-table">
          <colgroup>
            <col style="width: {colWidths.name}px;" />
            <col style="width: {colWidths.size}px;" />
            <col style="width: {colWidths.done}px;" />
            <col style="width: {colWidths.prog}px;" />
            <col style="width: {colWidths.status}px;" />
            <col style="width: {colWidths.speed}px;" />
            <col style="width: {colWidths.eta}px;" />
            <col style="width: {colWidths.path}px;" />
            <col style="width: {colWidths.added}px;" />
          </colgroup>
          <thead>
            <tr>
              <!-- Name Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.name}px; min-width: {colWidths.name}px; max-width: {colWidths.name}px;"
                onclick={() => toggleSort('name')}
              >
                <span>Name</span>
                {#if sortColumn === 'name'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('name', e)} ondblclick={() => autoSizeColumn('name')}></span>
              </th>

              <!-- Size Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.size}px; min-width: {colWidths.size}px; max-width: {colWidths.size}px;"
                onclick={() => toggleSort('size')}
              >
                <span>Size</span>
                {#if sortColumn === 'size'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('size', e)} ondblclick={() => autoSizeColumn('size')}></span>
              </th>

              <!-- Done Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.done}px; min-width: {colWidths.done}px; max-width: {colWidths.done}px;"
                onclick={() => toggleSort('done')}
              >
                <span>Done</span>
                {#if sortColumn === 'done'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('done', e)} ondblclick={() => autoSizeColumn('done')}></span>
              </th>

              <!-- Progress Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.prog}px; min-width: {colWidths.prog}px; max-width: {colWidths.prog}px;"
                onclick={() => toggleSort('prog')}
              >
                <span>Progress</span>
                {#if sortColumn === 'prog'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('prog', e)} ondblclick={() => autoSizeColumn('prog')}></span>
              </th>

              <!-- Status Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.status}px; min-width: {colWidths.status}px; max-width: {colWidths.status}px;"
                onclick={() => toggleSort('status')}
              >
                <span>Status</span>
                {#if sortColumn === 'status'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('status', e)} ondblclick={() => autoSizeColumn('status')}></span>
              </th>

              <!-- Down Speed Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.speed}px; min-width: {colWidths.speed}px; max-width: {colWidths.speed}px;"
                onclick={() => toggleSort('speed')}
              >
                <span>Down Speed</span>
                {#if sortColumn === 'speed'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('speed', e)} ondblclick={() => autoSizeColumn('speed')}></span>
              </th>

              <!-- ETA Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.eta}px; min-width: {colWidths.eta}px; max-width: {colWidths.eta}px;"
                onclick={() => toggleSort('eta')}
              >
                <span>ETA</span>
                {#if sortColumn === 'eta'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('eta', e)} ondblclick={() => autoSizeColumn('eta')}></span>
              </th>

              <!-- Save Path Column -->
              <th
                class="th-sortable"
                style="width: {colWidths.path}px; min-width: {colWidths.path}px; max-width: {colWidths.path}px;"
                onclick={() => toggleSort('path')}
              >
                <span>Save Path</span>
                {#if sortColumn === 'path'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('path', e)} ondblclick={() => autoSizeColumn('path')}></span>
              </th>

              <!-- Added / Last Try Date Column (IDM style) -->
              <th
                class="th-sortable"
                style="width: {colWidths.added}px; min-width: {colWidths.added}px; max-width: {colWidths.added}px;"
                onclick={() => toggleSort('added')}
              >
                <span>Added / Last Try</span>
                {#if sortColumn === 'added'}
                  <span class="sort-indicator">{sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => startResize('added', e)} ondblclick={() => autoSizeColumn('added')}></span>
              </th>
            </tr>
          </thead>
          <tbody>
            {#if filteredDownloads.length === 0}
              <tr class="empty-row" onclick={() => { selectedIds = []; lastClickedId = null; }}>
                <td colspan="9">
                  <div class="table-empty">
                    <span>No transfers in this view. Click <strong>Add Links</strong> to start downloading.</span>
                  </div>
                </td>
              </tr>
            {:else}
              {#each filteredDownloads as item (item.id)}
                <tr
                  class="torrent-row"
                  class:selected={selectedIds.includes(item.id)}
                  class:row-corrupt={item.status === 'corrupted'}
                  onclick={(e) => handleRowClick(item, e)}
                  oncontextmenu={(e) => handleContextMenu(item, e)}
                >
                  <td class="col-name" title={item.filename || item.url}>
                    <div class="name-cell">
                      {#if item.is_folder}
                        <button
                          type="button"
                          class="folder-expand-toggle"
                          class:expanded={expandedFolderIds.includes(item.id)}
                          onclick={(e) => toggleExpandFolder(item.id, e)}
                          title={expandedFolderIds.includes(item.id) ? "Collapse folder files" : "Expand to view files"}
                        >
                          <span class="folder-arrow">{expandedFolderIds.includes(item.id) ? '▼' : '▶'}</span>
                          {#if item.total_files}
                            <span class="folder-file-badge">{item.total_files}</span>
                          {/if}
                        </button>
                      {/if}
                      <span class="file-text">{item.filename || 'Resolving name...'}</span>
                    </div>
                  </td>
                  <td class="col-size font-mono">{getItemSize(item)}</td>
                  <td class="col-done font-mono">{formatBytes(item.downloaded_bytes)}</td>
                  <td class="col-prog" style="width: {colWidths.prog}px; max-width: {colWidths.prog}px;">
                    <div class="progress-cell" title={item.status === 'compressing' ? `Compressing ZIP: ${(item.compression_progress || 0).toFixed(1)}%` : (item.is_folder && item.total_files ? `${(item.percentage || 0).toFixed(1)}% (${item.completed_files || 0} of ${item.total_files} files)` : `${(item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0)).toFixed(1)}%`)}>
                      <div class="native-progress-track">
                        <div
                          class="native-progress-fill"
                          class:prog-done={item.status === 'completed'}
                          class:prog-corrupt={item.status === 'corrupted'}
                          class:prog-paused={item.status === 'paused'}
                          class:prog-error={item.status === 'failed'}
                          class:prog-compressing={item.status === 'compressing'}
                          style="width: {item.status === 'compressing' ? (item.compression_progress || 0) : (item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0))}%"
                        ></div>
                      </div>
                      <span class="prog-label font-mono">
                        {#if item.status === 'compressing'}
                          {colWidths.prog >= 130 ? `${(item.compression_progress || item.percentage || 0).toFixed(0)}% (ZIP)` : `${(item.compression_progress || item.percentage || 0).toFixed(0)}%`}
                        {:else if item.is_folder && item.total_files}
                          {#if colWidths.prog >= 150}
                            {(item.percentage || 0).toFixed(0)}% ({item.completed_files || 0}/{item.total_files})
                          {:else if colWidths.prog >= 115}
                            {(item.percentage || 0).toFixed(0)}% [{item.completed_files || 0}/{item.total_files}]
                          {:else}
                            {(item.percentage || 0).toFixed(0)}%
                          {/if}
                        {:else}
                          {(item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0)).toFixed(colWidths.prog >= 110 ? 1 : 0)}%
                        {/if}
                      </span>
                    </div>
                  </td>
                  <td class="col-status">
                    {#if item.status === 'corrupted'}
                      <span class="status-tag status-corrupted" title={item.error}>
                        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                          <line x1="12" y1="9" x2="12" y2="13"></line>
                          <line x1="12" y1="17" x2="12.01" y2="17"></line>
                        </svg>CORRUPTED</span>
                    {:else if item.status === 'missing'}
                      <span class="status-tag status-missing" title={item.error || 'File might be deleted or moved'}>
                        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                          <circle cx="12" cy="12" r="10"></circle>
                          <line x1="12" y1="8" x2="12" y2="12"></line>
                          <line x1="12" y1="16" x2="12.01" y2="16"></line>
                        </svg>MISSING / MOVED</span>
                    {:else if item.status === 'compressing'}
                      <span class="status-tag status-compressing" title="Compressing into ZIP archive">
                        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                          <polyline points="21 8 21 21 3 21 3 8"></polyline>
                          <rect x="1" y="3" width="22" height="5"></rect>
                          <line x1="10" y1="12" x2="14" y2="12"></line>
                        </svg>COMPRESSING</span>
                    {:else}
                      <span class="status-tag status-{item.status}">{item.status}</span>
                    {/if}
                  </td>
                  <td class="col-speed font-mono">
                    {item.status === 'downloading' ? formatSpeed(item.speed) : '--'}
                  </td>
                  <td class="col-eta font-mono">
                    {item.status === 'downloading' ? formatTime(item.eta_seconds) : '--'}
                  </td>
                  <td class="col-path" title={item.target_folder}>
                    {item.target_folder}
                  </td>
                  <td class="col-added font-mono" title="Added: {formatDateTime(item.created_at)} | Last Try: {formatDateTime(item.last_try_at)}">
                    {formatDateTime(item.last_try_at || item.created_at)}
                  </td>
                </tr>

                {#if item.is_folder && expandedFolderIds.includes(item.id) && item.folder_files && item.folder_files.length > 0}
                  {#each item.folder_files as child, ci (child.id || ci)}
                    <tr class="child-file-row">
                      <td class="col-name" title={child.filename}>
                        <div class="child-name-cell">
                          <span class="tree-line">└─</span>
                          <span class="child-filename">{child.filename}</span>
                        </div>
                      </td>
                      <td class="col-size font-mono">{child.size ? formatBytes(child.size) : '--'}</td>
                      <td class="col-done font-mono">{child.status === 'completed' && child.size ? formatBytes(child.size) : '--'}</td>
                      <td class="col-prog" style="width: {colWidths.prog}px; max-width: {colWidths.prog}px;">
                        <div class="child-prog-wrap" title="{child.status === 'completed' ? '100% Completed' : (child.status === 'downloading' ? 'Active downloading' : 'Pending')}">
                          <div class="native-progress-track">
                            <div
                              class="native-progress-fill"
                              class:prog-done={child.status === 'completed'}
                              class:prog-active={child.status === 'downloading'}
                              style="width: {child.status === 'completed' ? 100 : (child.status === 'downloading' ? 65 : 0)}%"
                            ></div>
                          </div>
                          <span class="child-prog-text font-mono">
                            {child.status === 'completed' ? '100%' : (child.status === 'downloading' ? (colWidths.prog >= 120 ? 'Active' : '...') : '0%')}
                          </span>
                        </div>
                      </td>
                      <td class="col-status">
                        <span class="child-status-pill child-status-{child.status}">
                          {child.status.toUpperCase()}
                        </span>
                      </td>
                      <td class="col-speed font-mono">--</td>
                      <td class="col-eta font-mono">--</td>
                      <td class="col-path font-mono">
                        <span class="in-archive-label">↳ Inside Archive</span>
                      </td>
                      <td class="col-added font-mono">--</td>
                    </tr>
                  {/each}
                {/if}
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      <!-- Bottom Details Pane (qBittorrent style) -->
      {#if selectedIds.length > 1}
        <div class="detail-pane">
          <div class="detail-header">
            <span class="detail-title">
              <strong>Batch Selection:</strong> {selectedIds.length} items selected
            </span>
            <div class="detail-actions">
              <button class="btn-mini btn-action" onclick={startSelected}>
                Start / Resume ({selectedIds.length})
              </button>
              <button class="btn-mini btn-secondary" onclick={pauseSelected}>
                Pause ({selectedIds.length})
              </button>
              <button class="btn-mini btn-secondary" onclick={restartSelected}>
                Restart ({selectedIds.length})
              </button>
              <button class="btn-mini btn-danger" onclick={() => openDeleteModal('selected', null, false)} title="Delete selected items">
                Delete ({selectedIds.length})...
              </button>
            </div>
          </div>

          <div class="detail-grid">
            <div class="detail-col">
              <div>
                <span class="prop-label">Selected Count:</span>
                <span class="prop-val font-mono">{selectedIds.length} transfers</span>
              </div>
              <div>
                <span class="prop-label">Total Size:</span>
                <span class="prop-val font-mono">{formatBytes(selectedItems.reduce((acc, d) => acc + (d.total_bytes || 0), 0))}</span>
              </div>
              <div>
                <span class="prop-label">Downloaded:</span>
                <span class="prop-val font-mono">{formatBytes(selectedItems.reduce((acc, d) => acc + (d.downloaded_bytes || 0), 0))}</span>
              </div>
            </div>
            <div class="detail-col">
              <div>
                <span class="prop-label">Combined Speed:</span>
                <span class="prop-val font-mono">{formatSpeed(selectedItems.reduce((acc, d) => acc + (d.speed || 0), 0))}</span>
              </div>
              <div>
                <span class="prop-label">Status Summary:</span>
                <span class="prop-val font-mono">
                  {selectedItems.filter(d => d.status === 'downloading' || d.status === 'compressing').length} downloading,
                  {selectedItems.filter(d => d.status === 'completed').length} completed,
                  {selectedItems.filter(d => d.status === 'paused').length} paused
                </span>
              </div>
            </div>
          </div>
        </div>
      {:else if selectedItem}
        <div class="detail-pane">
          <div class="detail-header">
            <span class="detail-title">
              <strong>Transfer Details:</strong> {selectedItem.filename}
            </span>
            <div class="detail-actions">
              {#if selectedItem.status === 'missing'}
                <button class="btn-mini btn-action" onclick={() => restartDownload(selectedItem.id)} title="Re-download the missing file">
                  Re-download
                </button>
                <button class="btn-mini btn-secondary" onclick={() => checkFileStatus(selectedItem.id)} title="Verify if file was moved back or restored">
                  Re-check
                </button>
              {:else if selectedItem.status === 'downloading' || selectedItem.status === 'queued'}
                <button class="btn-mini btn-secondary" onclick={() => pauseDownload(selectedItem.id)}>
                  Pause
                </button>
              {:else if selectedItem.status === 'paused' || selectedItem.status === 'failed' || selectedItem.status === 'cancelled'}
                <button class="btn-mini btn-action" onclick={() => startDownload(selectedItem.id)}>
                  Start / Resume
                </button>
              {/if}

              <button class="btn-mini btn-secondary" onclick={() => restartDownload(selectedItem.id)}>
                Restart
              </button>

              <button class="btn-mini btn-danger" onclick={() => openDeleteModal('single', selectedItem.id, false)} title="Delete this download">
                Delete...
              </button>
            </div>
          </div>

          <div class="detail-grid">
            <div class="detail-col">
              <div>
                <span class="prop-label">Status:</span>
                <span class="prop-val" class:val-corrupt={selectedItem.status === 'corrupted'} class:val-missing={selectedItem.status === 'missing'}>
                  {selectedItem.status.toUpperCase()}
                </span>
              </div>
              <div><span class="prop-label">Downloaded:</span> <span class="prop-val font-mono">{formatBytes(selectedItem.downloaded_bytes)} / {formatBytes(selectedItem.total_bytes)}</span></div>
              <div><span class="prop-label">Speed:</span> <span class="prop-val font-mono">{formatSpeed(selectedItem.speed)}</span></div>
              {#if selectedItem.is_folder}
                <div>
                  <span class="prop-label">Folder Progress:</span>
                  <span class="prop-val font-mono">{selectedItem.completed_files || 0} / {selectedItem.total_files || 0} files</span>
                </div>
              {/if}
            </div>
            <div class="detail-col">
              <div><span class="prop-label">ETA:</span> <span class="prop-val font-mono">{formatTime(selectedItem.eta_seconds)}</span></div>
              <div><span class="prop-label">Save Path:</span> <span class="prop-val">{selectedItem.target_folder}</span></div>
              <div><span class="prop-label">Added / Last Try:</span> <span class="prop-val font-mono">{formatDateTime(selectedItem.created_at)} / {formatDateTime(selectedItem.last_try_at)}</span></div>
              {#if selectedItem.current_file}
                <div>
                  <span class="prop-label">{selectedItem.status === 'compressing' ? 'Compressing:' : 'Active File:'}</span>
                  <span class="prop-val url-truncate font-mono" title={selectedItem.current_file}>{selectedItem.current_file}</span>
                </div>
              {/if}
            </div>
          </div>

          {#if selectedItem.status === 'missing'}
            <div class="detail-missing">
              <div class="detail-missing-header">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0; margin-top: 2px;">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                <div>
                  <strong>File Missing or Moved from Disk:</strong>
                  <div class="error-desc">{selectedItem.error || `The file or folder could not be found at "${selectedItem.target_folder}". It might have been deleted, moved, or renamed.`}</div>
                </div>
              </div>
              <div class="detail-missing-actions">
                <button class="btn-mini btn-action" onclick={() => restartDownload(selectedItem.id)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="vertical-align: middle; margin-right: 4px;">
                    <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
                  </svg>Re-download to Save Path
                </button>
                <button class="btn-mini btn-secondary" onclick={() => checkFileStatus(selectedItem.id)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="vertical-align: middle; margin-right: 4px;">
                    <circle cx="11" cy="11" r="8"></circle>
                    <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                  </svg>Check File Again
                </button>
                <button class="btn-mini btn-secondary" onclick={() => openDeleteModal('single', selectedItem.id, false)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                  </svg>Remove from List
                </button>
              </div>
            </div>
          {:else if selectedItem.status === 'corrupted'}
            <div class="detail-corrupt">
              <div style="display: flex; align-items: center; gap: 6px; margin-bottom: 4px;">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                <strong>Integrity Verification Alert:</strong>
              </div>
              <div class="error-desc">{selectedItem.error}</div>
              <button class="btn-mini btn-secondary" onclick={() => openLogsModal('ERROR')} style="margin-top: 6px;">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                  <polyline points="4 17 10 11 4 5"></polyline>
                  <line x1="12" y1="19" x2="20" y2="19"></line>
                </svg>View Logs
              </button>
            </div>
          {:else if selectedItem.error}
            <div class="detail-error">
              <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 8px;">
                <div>
                  <strong>Error:</strong> {selectedItem.error}
                </div>
                <button class="btn-mini btn-secondary" onclick={() => openLogsModal('ERROR')} style="white-space: nowrap;">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                    <polyline points="4 17 10 11 4 5"></polyline>
                    <line x1="12" y1="19" x2="20" y2="19"></line>
                  </svg>View Logs
                </button>
              </div>
            </div>
          {/if}
        </div>
      {/if}
    </main>
  </div>

  <!-- Bottom Desktop Status Bar -->
  <footer class="statusbar">
    <div class="sb-section">
      <span class="status-dot" class:connected={backendConnected}></span>
      <span>{backendConnected ? 'Online (Go 1.27 :8080)' : 'Disconnected'}</span>
      {#if hasLogin}
        <span class="sb-badge-auth">Google Session Active</span>
      {/if}
    </div>

    <div class="sb-section">
      <span>Total: <strong>{counts.all}</strong></span>
      {#if selectedIds.length > 0}
        <span class="sb-divider">|</span>
        <span>Selected: <strong style="color: var(--accent-blue)">{selectedIds.length}</strong></span>
      {/if}
      <span class="sb-divider">|</span>
      <span>Active: <strong style="color: var(--accent-blue)">{counts.downloading}</strong></span>
      <span class="sb-divider">|</span>
      <span>Paused: <strong style="color: var(--accent-amber)">{counts.paused}</strong></span>
      <span class="sb-divider">|</span>
      <span>Done: <strong style="color: var(--accent-green)">{counts.completed}</strong></span>
      {#if counts.corrupted > 0}
        <span class="sb-divider">|</span>
        <span>Corrupt: <strong style="color: var(--accent-red)">{counts.corrupted}</strong></span>
      {/if}
    </div>

    <div class="sb-section font-mono">
      <span>DL: <strong style="color: var(--accent-blue)">{formatSpeed(counts.totalSpeed)}</strong></span>
    </div>
  </footer>

  <!-- Desktop Right-Click Context Menu (qBittorrent / IDM style) -->
  {#if showContextMenu}
    <div
      class="context-menu"
      role="menu"
      tabindex="-1"
      style="top: {contextMenuY}px; left: {contextMenuX}px;"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.key === 'Escape' && closeContextMenu()}
    >
      {#if contextMenuItem ? (contextMenuItem.status === 'downloading' || contextMenuItem.status === 'queued' || contextMenuItem.status === 'compressing') : selectedItems.some(d => d.status === 'downloading' || d.status === 'queued' || d.status === 'compressing')}
        <button
          class="context-item"
          disabled={selectedIds.length === 0}
          onclick={() => { pauseSelected(); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <rect x="6" y="4" width="4" height="16"></rect>
            <rect x="14" y="4" width="4" height="16"></rect>
          </svg>
          <span>Pause{selectedIds.length > 1 ? ` (${selectedIds.length})` : ''}</span>
          <span class="context-key">Space</span>
        </button>
      {:else}
        <button
          class="context-item"
          disabled={selectedIds.length === 0}
          onclick={() => { startSelected(); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <polygon points="5 3 19 12 5 21 5 3"></polygon>
          </svg>
          <span>Resume / Start{selectedIds.length > 1 ? ` (${selectedIds.length})` : ''}</span>
          <span class="context-key">Space</span>
        </button>

        <button
          class="context-item"
          disabled={selectedIds.length === 0}
          onclick={() => { restartSelected(); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
          </svg>
          <span>Restart from Beginning</span>
        </button>
      {/if}

      {#if contextMenuItem?.status === 'missing'}
        <button
          class="context-item"
          onclick={() => { restartDownload(contextMenuItem.id); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
          </svg>
          <span>Re-download Missing File</span>
        </button>
        <button
          class="context-item"
          onclick={() => { checkFileStatus(contextMenuItem.id); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <span>Check File Existence on Disk</span>
        </button>
      {:else if contextMenuItem?.status === 'completed'}
        <button
          class="context-item"
          onclick={() => { checkFileStatus(contextMenuItem.id); closeContextMenu(); }}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
            <circle cx="11" cy="11" r="8"></circle>
            <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
          </svg>
          <span>Verify File on Disk</span>
        </button>
      {/if}

      {#if contextMenuItem?.is_folder}
        <div class="context-divider"></div>
        <button
          class="context-item"
          onclick={() => { toggleExpandFolder(contextMenuItem.id); closeContextMenu(); }}
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            {#if expandedFolderIds.includes(contextMenuItem.id)}
              <polyline points="6 9 12 15 18 9"></polyline>
            {:else}
              <polyline points="9 18 15 12 9 6"></polyline>
            {/if}
          </svg>
          <span>{expandedFolderIds.includes(contextMenuItem.id) ? 'Collapse Files' : 'Expand Files'}</span>
        </button>
      {/if}

      <div class="context-divider"></div>

      <!-- Delete -->
      <button
        class="context-item text-danger"
        disabled={selectedIds.length === 0}
        onclick={() => { openDeleteModal('selected', null, false); closeContextMenu(); }}
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
        onclick={() => { openLogsModal(); closeContextMenu(); }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="4 17 10 11 4 5"></polyline>
          <line x1="12" y1="19" x2="20" y2="19"></line>
        </svg>
        <span>View Logs...</span>
      </button>

      <button
        class="context-item"
        onclick={() => { selectedIds = filteredDownloads.map(d => d.id); closeContextMenu(); }}
      >
        <span>Select All</span>
        <span class="context-key">Ctrl+A</span>
      </button>

      <button
        class="context-item"
        disabled={selectedIds.length === 0}
        onclick={() => { selectedIds = []; lastClickedId = null; closeContextMenu(); }}
      >
        <span>Deselect All</span>
        <span class="context-key">Esc</span>
      </button>
    </div>
  {/if}

  <!-- Modal: Confirm Delete with 2 Options (List vs Disk) -->
  {#if showDeleteModal}
    <div class="modal-overlay" role="presentation" onclick={() => showDeleteModal = false}>
      <div class="modal-window delete-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header delete-modal-header">
          <div class="delete-header-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <polyline points="3 6 5 6 21 6"></polyline>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
            </svg>
            <span>Confirm Deletion</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => showDeleteModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <p class="delete-prompt-desc">
            {#if deleteModalTarget === 'completed'}
              Are you sure you want to remove all <strong>completed & finished</strong> downloads?
            {:else if deleteModalTarget === 'selected'}
              Are you sure you want to remove <strong>{selectedIds.length}</strong> selected transfer{selectedIds.length > 1 ? 's' : ''}?
            {:else}
              Are you sure you want to remove <strong>{downloads.find(d => d.id === deleteModalSingleId)?.filename || 'this item'}</strong>?
            {/if}
          </p>

          <div class="delete-radio-group">
            <label class="delete-radio-card" class:active={!deleteModalWithFile}>
              <input type="radio" name="deleteOptionGroup" value={false} bind:group={deleteModalWithFile} />
              <div class="delete-radio-text">
                <span class="delete-radio-title">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                    <line x1="8" y1="6" x2="21" y2="6"></line>
                    <line x1="8" y1="12" x2="21" y2="12"></line>
                    <line x1="8" y1="18" x2="21" y2="18"></line>
                    <line x1="3" y1="6" x2="3.01" y2="6"></line>
                    <line x1="3" y1="12" x2="3.01" y2="12"></line>
                    <line x1="3" y1="18" x2="3.01" y2="18"></line>
                  </svg>Delete from List only
                </span>
                <span class="delete-radio-sub">Removes the task from the download manager. Downloaded files and archives remain safe on your hard disk.</span>
              </div>
            </label>

            <label class="delete-radio-card danger" class:active={deleteModalWithFile}>
              <input type="radio" name="deleteOptionGroup" value={true} bind:group={deleteModalWithFile} />
              <div class="delete-radio-text">
                <span class="delete-radio-title">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                    <polyline points="3 6 5 6 21 6"></polyline>
                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                  </svg>Delete from List AND remove file(s) from disk
                </span>
                <span class="delete-radio-sub">Permanently removes the downloaded .zip, files, and temporary staging folders from your storage.</span>
              </div>
            </label>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => showDeleteModal = false}>Cancel</button>
          <button class="btn btn-danger" onclick={confirmDeleteModal}>
            {deleteModalWithFile ? 'Permanently Delete' : 'Remove from List'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: Add Downloads -->
  {#if showAddModal}
    <div class="modal-overlay" role="presentation" onclick={() => showAddModal = false} onkeydown={(e) => e.key === 'Escape' && (showAddModal = false)}>
      <div class="modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>Add Google Drive Links</span>
          <button class="modal-close" aria-label="Close" onclick={() => showAddModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <label class="form-group">
            <span class="form-title">Enter Google Drive URLs (one per line):</span>
            <textarea
              bind:value={addLinksInput}
              oninput={handleLinksInput}
              rows="5"
              placeholder="https://drive.google.com/file/d/1A2B3C.../view&#10;https://drive.google.com/drive/folders/1sSPph9ml0...&#10;https://drive.usercontent.google.com/download?id=13g1RiKb..."
            ></textarea>
          </label>

          {#if isResolvingFolder}
            <div class="folder-preview-loading">
              <div class="mini-spinner"></div>
              <span>Inspecting Google Drive folder contents...</span>
            </div>
          {:else if detectedFolder}
            <div class="folder-preview-card">
              <div class="folder-preview-header">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; flex-shrink: 0;">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <div class="folder-preview-info">
                  <div class="folder-preview-title" title={detectedFolder.title}>{detectedFolder.title}</div>
                  <div class="folder-preview-subtitle">{detectedFolder.files_count} files detected</div>
                </div>
              </div>
            </div>
          {:else if resolveError}
            <div class="folder-resolve-error">
              <span style="display: flex; align-items: center; gap: 6px;">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                {resolveError}
              </span>
            </div>
          {/if}

          <!-- Folder Download Format (Always prominent, defaults to ZIP) -->
          <div class="form-group" style="margin-top: 0.75rem;">
            <span class="form-title">Folder Download Mode:</span>
            <div class="folder-mode-options">
              <label class="radio-option">
                <input type="radio" name="folderMode" value="zip" bind:group={folderZipMode} />
                <div class="radio-text">
                  <span class="radio-label-title">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                      <polyline points="21 8 21 21 3 21 3 8"></polyline>
                      <rect x="1" y="3" width="22" height="5"></rect>
                      <line x1="10" y1="12" x2="14" y2="12"></line>
                    </svg>Compress to ZIP Archive (.zip) [Default]
                  </span>
                  <span class="radio-label-desc">Downloads files to local staging and compresses locally into a single verified ZIP file with live progress.</span>
                </div>
              </label>

              <label class="radio-option">
                <input type="radio" name="folderMode" value="folder" bind:group={folderZipMode} />
                <div class="radio-text">
                  <span class="radio-label-title">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                    </svg>Download as Subfolder
                  </span>
                  <span class="radio-label-desc">Downloads each file individually into a subfolder named after the folder.</span>
                </div>
              </label>
            </div>
          </div>

          <!-- Jellyfin-style Folder Picker Input Trigger -->
          <div class="form-group" style="margin-top: 0.75rem;">
            <span class="form-title">Save Path:</span>
            <div class="path-picker-row">
              <input type="text" bind:value={addTargetFolder} />
              <button class="btn-browse" onclick={() => openPickerFor('add')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <span>Browse...</span>
              </button>
            </div>
            <span class="form-hint">Files will be saved into this directory.</span>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => showAddModal = false}>Cancel</button>
          <button class="btn btn-primary" disabled={isAdding || !addLinksInput.trim()} onclick={submitAddDownloads}>
            {isAdding ? 'Adding...' : 'Start Download'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: Conflict / Duplicate Resolution -->
  {#if showConflictModal}
    <div class="modal-overlay" role="presentation" onclick={() => showConflictModal = false} onkeydown={(e) => e.key === 'Escape' && (showConflictModal = false)}>
      <div class="modal-window modal-conflict" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="8" x2="12" y2="12"></line>
              <line x1="12" y1="16" x2="12.01" y2="16"></line>
            </svg>
            <span>Existing File or Duplicate Transfer Detected ({conflictList.length})</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => showConflictModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body conflict-modal-body">
          <p class="conflict-intro">
            One or more links match an existing transfer or a file with the same name already exists in your destination folder. Choose how to handle each item:
          </p>

          {#if conflictList.length > 1}
            <div class="conflict-quick-bar">
              <span class="quick-bar-label">Apply to all:</span>
              <div class="quick-bar-actions">
                <button type="button" class="btn-mini btn-action" onclick={() => setAllConflictActions('rename')}>
                  Keep Both (Rename with Suffix)
                </button>
                <button type="button" class="btn-mini btn-action" onclick={() => setAllConflictActions('overwrite')}>
                  Overwrite Existing
                </button>
                <button type="button" class="btn-mini btn-action" onclick={() => setAllConflictActions('monitor')}>
                  Re-monitor & Verify Integrity
                </button>
              </div>
            </div>
          {/if}

          <div class="conflict-cards-list">
            {#each conflictList as c (c.file_id || c.url)}
              <div class="conflict-card">
                <div class="conflict-card-header">
                  <div class="conflict-header-title">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0;">
                      {#if c.is_folder}
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                      {:else}
                        <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path>
                        <polyline points="13 2 13 9 20 9"></polyline>
                      {/if}
                    </svg>
                    <span class="conflict-filename" title={c.expected_filename || c.title}>{c.expected_filename || c.title}</span>
                  </div>

                  <div class="conflict-badges">
                    {#if c.exists_in_list}
                      <span class="conflict-badge badge-in-list" title="Item is present in current download history">
                        IN LIST: {(c.existing_status || 'UNKNOWN').toUpperCase()}
                      </span>
                    {/if}
                    {#if c.exists_on_disk}
                      <span class="conflict-badge badge-on-disk" title="File was detected in local download folder">
                        ON DISK: {formatBytes(c.disk_size)}
                      </span>
                    {/if}
                  </div>
                </div>

                <div class="conflict-card-path font-mono" title={c.target_path}>
                  {c.target_path}
                </div>

                <div class="conflict-options">
                  <!-- Option 1: Rename (2) -->
                  <label class="conflict-option" class:selected={conflictResolutions[c.file_id || c.url] === 'rename'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="rename"
                      bind:group={conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Keep Both</strong> (Save as copy with suffix <code>(2)</code>)
                      </span>
                      <span class="conflict-option-desc">
                        Downloads as a separate file without altering the existing file or list item.
                      </span>
                    </div>
                  </label>

                  <!-- Option 2: Overwrite -->
                  <label class="conflict-option" class:selected={conflictResolutions[c.file_id || c.url] === 'overwrite'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="overwrite"
                      bind:group={conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Overwrite & Replace</strong>
                      </span>
                      <span class="conflict-option-desc">
                        Replaces the existing file on disk and re-downloads the file from 0%.
                      </span>
                    </div>
                  </label>

                  <!-- Option 3: Re-monitor & Verify -->
                  <label class="conflict-option" class:selected={conflictResolutions[c.file_id || c.url] === 'monitor'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="monitor"
                      bind:group={conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Re-monitor & Verify Integrity</strong> (Resume if incomplete)
                      </span>
                      <span class="conflict-option-desc">
                        {#if c.exists_on_disk}
                          Inspects the {formatBytes(c.disk_size)} file on disk. If complete, marks completed. If corrupted or partial, verifies and resumes.
                        {:else}
                          Re-checks file status and resumes download without starting over.
                        {/if}
                      </span>
                    </div>
                  </label>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => showConflictModal = false}>Cancel</button>
          <button class="btn btn-primary" disabled={isAdding} onclick={() => executeAddDownloads(conflictResolutions)}>
            {isAdding ? 'Processing...' : 'Confirm & Proceed'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: One-Time Login (Google Cookie Session) -->
  {#if showLoginModal}
    <div class="modal-overlay" role="presentation" onclick={() => showLoginModal = false} onkeydown={(e) => e.key === 'Escape' && (showLoginModal = false)}>
      <div class="modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>One-Time Google Login (Cookie Session)</span>
          <button class="modal-close" aria-label="Close" onclick={() => showLoginModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="auth-status-banner">
            <span class="status-indicator" class:active-dot={hasLogin} class:failed-dot={!hasLogin}></span>
            <span>Status: <strong>{hasLogin ? 'Logged in with Google Session Cookie' : 'Guest / Anonymous (Public files only)'}</strong></span>
          </div>

          <div class="form-group" style="margin-top: 1rem;">
            <span class="form-title">Paste Google Drive Cookie:</span>
            <textarea
              bind:value={loginCookieInput}
              rows="4"
              placeholder="Paste your Cookie header string here (e.g. SID=...; HSID=...; SSID=...; SAPISID=...; APISID=...)"
            ></textarea>
            <span class="form-hint" style="display: flex; align-items: flex-start; gap: 6px; margin-top: 6px;">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0; margin-top: 2px;">
                <circle cx="12" cy="12" r="10"></circle>
                <line x1="12" y1="16" x2="12" y2="12"></line>
                <line x1="12" y1="8" x2="12.01" y2="8"></line>
              </svg>
              <span><strong>How to get your Cookie:</strong> Open Google Drive in Chrome/Edge &gt; Press F12 (DevTools) &gt; Open the <em>Network</em> or <em>Application &gt; Cookies</em> tab &gt; Copy the Cookie header string. Stored securely only on your local machine.</span>
            </span>
          </div>
        </div>

        <div class="modal-footer">
          {#if hasLogin}
            <button class="btn btn-danger" onclick={logoutGoogle}>Clear / Logout</button>
          {/if}
          <button class="btn btn-secondary" onclick={() => showLoginModal = false}>Cancel</button>
          <button class="btn btn-primary" disabled={isSavingLogin || !loginCookieInput.trim()} onclick={saveGoogleLogin}>
            {isSavingLogin ? 'Saving...' : 'Save & Login'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: Options / Settings -->
  {#if showSettingsModal}
    <div class="modal-overlay" role="presentation" onclick={() => showSettingsModal = false} onkeydown={(e) => e.key === 'Escape' && (showSettingsModal = false)}>
      <div class="modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>Options & Preferences</span>
          <button class="modal-close" aria-label="Close" onclick={() => showSettingsModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <span class="form-title">Default Destination Folder:</span>
            <div class="path-picker-row">
              <input type="text" bind:value={defaultFolder} />
              <button class="btn-browse" onclick={() => openPickerFor('settings')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <span>Browse...</span>
              </button>
            </div>
            <span class="form-hint">Supports local drives (C:\, D:\) and mapped Homelab network shares (e.g. Z:\Music).</span>
          </div>

          <div class="form-group" style="margin-top: 1rem;">
            <span class="form-title">Maximum Concurrent Downloads:</span>
            <input type="number" min="1" max="5" bind:value={maxConcurrency} style="width: 100px;" />
            <span class="form-hint">Recommended: 2 to 3 workers.</span>
          </div>

          <!-- Divider -->
          <div style="border-top: 1px solid var(--border-color); margin: 1.25rem 0 1rem 0;"></div>

          <!-- Web UI Security Section (qBittorrent server style) -->
          <div class="form-group">
            <span class="form-title" style="font-weight: 600; color: var(--accent-blue);">Web UI Security (Server Access)</span>
            <span class="form-hint" style="margin-bottom: 0.75rem;">Protect your server deployment with login authentication, just like qBittorrent WebUI.</span>

            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-bottom: 0.75rem;">
              <input type="checkbox" bind:checked={authEnabledToggle} onchange={toggleAuthRequirement} style="width: auto;" />
              <span style="font-weight: 500;">Require username and password to access Web UI</span>
            </label>

            {#if authEnabledToggle}
              <div style="background: var(--table-row-alt); padding: 12px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: 8px;">
                <span style="font-weight: 600; font-size: 12px;">Change Admin Credentials:</span>

                {#if securityError}
                  <div style="color: var(--accent-red); font-size: 11px; background: rgba(248,81,73,0.1); padding: 4px 8px; border-radius: 4px;">{securityError}</div>
                {/if}
                {#if securityMessage}
                  <div style="color: var(--accent-green); font-size: 11px; background: rgba(46,160,67,0.1); padding: 4px 8px; border-radius: 4px;">{securityMessage}</div>
                {/if}

                <div style="display: grid; grid-template-columns: 130px 1fr; gap: 8px; align-items: center;">
                  <span style="font-size: 12px; color: var(--text-muted);">Username:</span>
                  <input type="text" bind:value={changeNewUsername} placeholder="admin" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">Current Password:</span>
                  <input type="password" bind:value={changeOldPassword} placeholder="••••••••" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">New Password:</span>
                  <input type="password" bind:value={changeNewPassword} placeholder="••••••••" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">Confirm Password:</span>
                  <input type="password" bind:value={changeConfirmPassword} placeholder="••••••••" style="padding: 4px 8px;" />
                </div>

                <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
                  <button class="btn btn-secondary" onclick={updateSecurityCredentials} disabled={isUpdatingSecurity} style="font-size: 11px; padding: 4px 12px;">
                    {isUpdatingSecurity ? 'Updating...' : 'Update Password'}
                  </button>
                </div>
              </div>
            {/if}
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => showSettingsModal = false}>Cancel</button>
          <button class="btn btn-primary" onclick={saveConfig}>Save Options</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Jellyfin Interactive Folder Picker Modal -->
  {#if showFolderPicker}
    <FolderPicker
      initialPath={folderPickerTarget === 'add' ? addTargetFolder : defaultFolder}
      onSelect={handleFolderSelected}
      onClose={() => showFolderPicker = false}
    />
  {/if}

  <!-- Modal: Documentation & Quick Guide -->
  {#if showDocModal}
    <div class="modal-overlay" role="presentation" onclick={() => showDocModal = false} onkeydown={(e) => e.key === 'Escape' && (showDocModal = false)}>
      <div class="modal-window doc-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path>
              <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path>
            </svg>
            <span style="font-weight: 700;">Google Drive Client - Documentation & User Guide</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => showDocModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body doc-modal-body">
          <section class="doc-section">
            <h4>Getting Started & Downloading</h4>
            <p>Paste any Google Drive link into <strong>Add Links</strong> (or press <kbd>Ctrl+N</kbd>). Multiple links can be added at once by separating them with newlines.</p>
            <ul>
              <li><strong>Single File URLs:</strong> <code>https://drive.google.com/file/d/&lt;FILE_ID&gt;/view</code></li>
              <li><strong>Direct Download URLs:</strong> <code>https://drive.usercontent.google.com/download?id=&lt;FILE_ID&gt;&amp;export=download</code></li>
              <li><strong>Folder URLs:</strong> <code>https://drive.google.com/drive/folders/&lt;FOLDER_ID&gt;</code></li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Folder Download Modes</h4>
            <ul>
              <li><strong>Compress to ZIP Archive (.zip) [Recommended]:</strong> Downloads all files inside the folder and compresses them locally into a verified <code>.zip</code> archive. You can monitor live Turn-by-turn and compression progress.</li>
              <li><strong>Download as Subfolder:</strong> Creates a local folder named after the Drive directory and queues each file individually into the transfer list.</li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Real-Time File Sync & Missing File Detection</h4>
            <p>If a downloaded file is moved, renamed, or deleted from your local disk:</p>
            <ul>
              <li>Clicking or checking the row in the table instantly detects that the file is missing and marks it as <span class="status-tag status-missing">MISSING / MOVED</span>.</li>
              <li>You can click <strong>Re-download</strong> from the Transfer Details pane or context menu to immediately re-fetch the file.</li>
              <li>All download history is safely preserved in <code>downloads.json</code> and survives server reboots, git pulls, and software updates.</li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Downloading Restricted / Quota-Exceeded Files</h4>
            <p>If a file requires Google account login or has exceeded its public quota, open <strong>Tools &gt; Google Session Cookie</strong> and paste your session cookie from Chrome/Edge Developer Tools (<kbd>F12</kbd> &gt; Network/Application &gt; Cookies).</p>
          </section>

          <section class="doc-section">
            <h4>Keyboard Shortcuts Cheat Sheet</h4>
            <div class="shortcut-table">
              <div><kbd>Ctrl+N</kbd> <span>Open Add Links dialog</span></div>
              <div><kbd>Ctrl+A</kbd> <span>Select all transfers in current view</span></div>
              <div><kbd>Esc</kbd> <span>Deselect all / Close open menus & modals</span></div>
              <div><kbd>Space</kbd> <span>Start or pause selected transfer(s)</span></div>
              <div><kbd>Del</kbd> <span>Delete selected transfer(s) from list only</span></div>
              <div><kbd>Shift+Del</kbd> <span>Delete transfer(s) from list AND remove file from disk</span></div>
            </div>
          </section>
        </div>

        <div class="modal-footer">
          <button class="btn btn-primary" onclick={() => showDocModal = false}>Got it</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: Check for Updates -->
  {#if showUpdateModal}
    <div class="modal-overlay" role="presentation" onclick={() => showUpdateModal = false} onkeydown={(e) => e.key === 'Escape' && (showUpdateModal = false)}>
      <div class="modal-window" style="max-width: 440px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
            </svg>
            <span style="font-weight: 700;">Check for Updates</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => showUpdateModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body" style="text-align: center; padding: 1.5rem 1rem;">
          {#if updateChecking}
            <div class="mini-spinner" style="margin: 0 auto 12px auto; width: 24px; height: 24px;"></div>
            <div style="font-size: 0.9rem; color: var(--text-muted);">{updateStatus}</div>
          {:else}
            <div style="margin: 0 auto 10px auto; width: 44px; height: 44px; display: flex; align-items: center; justify-content: center; border-radius: 50%; background: var(--table-row-alt); border: 1px solid var(--border-color);">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                <polyline points="20 6 9 17 4 12"></polyline>
              </svg>
            </div>
            <h3 style="margin: 0 0 6px 0; font-size: 1.05rem; color: var(--text-main);">You're Up to Date!</h3>
            <p style="font-size: 0.82rem; color: var(--text-muted); margin: 0 0 12px 0;">
              Google Drive Client <strong>v1.2.0 (Persistent Desktop Edition)</strong> is currently the latest version.
            </p>
            <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 10px; font-size: 0.78rem; text-align: left; line-height: 1.5;">
              <strong>What's New in v1.2.0:</strong>
              <ul style="margin: 6px 0 0 16px; padding: 0; color: var(--text-muted);">
                <li>Desktop application menu bar (File, Edit, View, Tools, Help)</li>
                <li>Persistent real-time download history (downloads.json)</li>
                <li>Real-time file presence detection (Missing / Moved / Deleted)</li>
                <li>One-click re-download for missing files</li>
              </ul>
            </div>
          {/if}
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => showUpdateModal = false}>Close</button>
          <a class="btn btn-primary" href="https://github.com" target="_blank" rel="noopener noreferrer" style="text-decoration: none;">
            View Releases
          </a>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: About GDrive Downloader -->
  {#if showAboutModal}
    <div class="modal-overlay" role="presentation" onclick={() => showAboutModal = false} onkeydown={(e) => e.key === 'Escape' && (showAboutModal = false)}>
      <div class="modal-window" style="max-width: 460px;" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="16" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
            <span style="font-weight: 700;">About GDrive Downloader</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => showAboutModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body" style="text-align: center; padding: 1.5rem 1rem;">
          <div style="width: 52px; height: 52px; border-radius: 12px; background: rgba(56, 139, 253, 0.12); border: 1px solid rgba(56, 139, 253, 0.3); display: flex; align-items: center; justify-content: center; margin: 0 auto 12px auto;">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
              <path d="M12 12v9"></path>
              <path d="m8 17 4 4 4-4"></path>
            </svg>
          </div>
          <h3 style="margin: 0 0 4px 0; font-size: 1.15rem; color: var(--text-main);">Google Drive Downloader</h3>
          <div style="font-size: 0.8rem; color: var(--text-main); font-weight: 600; margin-bottom: 12px;">v1.2.0 • Persistent Desktop Edition</div>
          <p style="font-size: 0.82rem; color: var(--text-muted); line-height: 1.5; margin: 0 0 14px 0;">
            A high-performance Google Drive downloader featuring multi-worker queuing, ZIP compression, chunked streaming, file integrity verification, and qBittorrent-style server authentication.
          </p>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; font-size: 0.75rem; text-align: left; background: var(--table-row-alt); padding: 10px 14px; border-radius: 6px; border: 1px solid var(--border-subtle); margin-bottom: 12px;">
            <div><span style="color: var(--text-dim);">Backend:</span> <strong style="color: var(--text-main);">Go 1.27 Concurrency</strong></div>
            <div><span style="color: var(--text-dim);">Frontend:</span> <strong style="color: var(--text-main);">Svelte 5 Runes + Vite</strong></div>
            <div><span style="color: var(--text-dim);">Storage:</span> <strong style="color: var(--text-main);">Realtime downloads.json</strong></div>
            <div><span style="color: var(--text-dim);">License:</span> <strong style="color: var(--text-main);">MIT Open Source</strong></div>
          </div>
        </div>

        <div class="modal-footer">
          <a class="btn btn-secondary" href="https://github.com" target="_blank" rel="noopener noreferrer" style="text-decoration: none;">
            GitHub Repository
          </a>
          <button class="btn btn-primary" onclick={() => showAboutModal = false}>OK</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Modal: System & Execution Logs -->
  {#if showLogsModal}
    <div class="modal-overlay" role="presentation" onclick={() => showLogsModal = false} onkeydown={(e) => e.key === 'Escape' && (showLogsModal = false)}>
      <div class="modal-window logs-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
            <span style="font-weight: 700;">System Execution & Error Logs</span>
            {#if errorLogsCount > 0}
              <span class="log-badge-error">{errorLogsCount} error{errorLogsCount > 1 ? 's' : ''}</span>
            {/if}
          </div>
          <div class="logs-header-actions">
            <button class="btn-mini btn-secondary" onclick={copyLogsToClipboard} title="Copy all logs to clipboard">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px; vertical-align: middle;">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
              </svg>
              {copiedLogs ? 'Copied!' : 'Copy'}
            </button>
            <button class="btn-mini btn-secondary" onclick={clearLogs} title="Clear log buffer">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px; vertical-align: middle;">
                <polyline points="3 6 5 6 21 6"></polyline>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
              </svg>
              Clear
            </button>
            <button class="btn-mini btn-action" onclick={fetchLogs} disabled={isFetchingLogs} title="Refresh logs">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="margin-right: 4px; vertical-align: middle;">
                <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
              </svg>
              {isFetchingLogs ? 'Refreshing...' : 'Refresh'}
            </button>
            <button class="modal-close" aria-label="Close" onclick={() => showLogsModal = false}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>

        <div class="logs-filter-bar">
          <div class="logs-filter-chips">
            <button
              class="log-filter-chip"
              class:active={logFilter === 'ALL'}
              onclick={() => logFilter = 'ALL'}
            >
              All ({logs.length})
            </button>
            <button
              class="log-filter-chip chip-error"
              class:active={logFilter === 'ERROR'}
              onclick={() => logFilter = 'ERROR'}
            >
              Errors ({logs.filter(l => l.level === 'ERROR').length})
            </button>
            <button
              class="log-filter-chip chip-warn"
              class:active={logFilter === 'WARN'}
              onclick={() => logFilter = 'WARN'}
            >
              Warnings ({logs.filter(l => l.level === 'WARN').length})
            </button>
            <button
              class="log-filter-chip chip-info"
              class:active={logFilter === 'INFO'}
              onclick={() => logFilter = 'INFO'}
            >
              Info ({logs.filter(l => l.level === 'INFO').length})
            </button>
            <button
              class="log-filter-chip chip-success"
              class:active={logFilter === 'SUCCESS'}
              onclick={() => logFilter = 'SUCCESS'}
            >
              Success ({logs.filter(l => l.level === 'SUCCESS').length})
            </button>
          </div>

          <input
            type="text"
            class="logs-search-input"
            placeholder="Filter logs by keyword..."
            bind:value={logSearch}
          />
        </div>

        <div class="modal-body logs-modal-body">
          {#if filteredLogs.length === 0}
            <div class="log-empty">
              <span>No logs found matching filter criteria.</span>
            </div>
          {:else}
            <div class="logs-container">
              {#each filteredLogs as log (log.id)}
                <div class="log-row log-row-{log.level}">
                  <span class="log-time">{log.timestamp}</span>
                  <span class="log-level-pill log-level-{log.level}">{log.level}</span>
                  <span class="log-category">[{log.category}]</span>
                  <div class="log-msg-col">
                    <span class="log-msg">{log.message}</span>
                    {#if log.details}
                      <div class="log-details">{log.details}</div>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <div class="modal-footer" style="justify-content: space-between;">
          <span style="font-size: 11px; color: var(--text-dim);">
            Displaying {filteredLogs.length} of {logs.length} entries • In-memory buffer: last 500 actions
          </span>
          <button class="btn btn-primary" onclick={() => showLogsModal = false}>Close</button>
        </div>
      </div>
    </div>
  {/if}
</div>
{/if}

<style>
  .desktop-app {
    display: flex;
    flex-direction: column;
    width: 100vw;
    height: 100vh;
    background: var(--app-bg);
  }

  /* Toolbar */
  .toolbar {
    height: 42px;
    background: var(--toolbar-bg);
    border-bottom: 1px solid var(--border-color);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
    display: flex;
    align-items: center;
    padding: 0 0.75rem;
    gap: 0.4rem;
    flex-shrink: 0;
    z-index: 20;
  }
  .toolbar-brand {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    margin-right: 0.4rem;
    padding-right: 0.6rem;
    border-right: 1px solid var(--border-color);
  }
  .app-title {
    font-weight: 700;
    font-size: 0.88rem;
    color: var(--text-main);
    letter-spacing: 0.02em;
  }
  .toolbar-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
  }
  .tb-btn {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    background: var(--tb-btn-bg);
    border: 1px solid var(--tb-btn-border);
    color: var(--text-main);
    padding: 0.3rem 0.6rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 500;
    position: relative;
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
    transition: background 0.12s, border-color 0.12s, color 0.12s;
  }
  .tb-btn:hover:not(:disabled) {
    background: var(--tb-btn-hover);
    border-color: var(--tb-btn-hover-border);
  }
  .tb-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
    background: transparent;
    border-color: transparent;
    box-shadow: none;
  }
  .tb-btn-primary {
    background: var(--accent-green);
    border-color: var(--accent-green);
    color: #fff;
  }
  .tb-btn-primary:hover {
    opacity: 0.9;
  }
  .tb-separator {
    width: 1px;
    height: 22px;
    background: var(--border-color);
    margin: 0 0.3rem;
  }
  .auth-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent-green);
    margin-left: 2px;
  }
  .toolbar-search {
    margin-left: auto;
    display: flex;
    align-items: center;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 0.15rem 0.5rem;
    gap: 0.35rem;
  }
  .toolbar-search input {
    border: none;
    background: transparent;
    padding: 0.15rem;
    font-size: 0.8rem;
    width: 150px;
  }
  .clear-search-btn {
    background: transparent;
    color: var(--text-muted);
    font-size: 0.75rem;
  }

  /* Workspace */
  .workspace {
    flex: 1;
    display: flex;
    overflow: hidden;
  }

  /* Left Sidebar */
  .sidebar {
    width: 190px;
    background: var(--sidebar-bg);
    border-right: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
  }
  .sidebar-header {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--text-dim);
    padding: 0.65rem 0.85rem 0.25rem;
    letter-spacing: 0.05em;
  }
  .category-list {
    display: flex;
    flex-direction: column;
    padding: 0.25rem 0.5rem;
    gap: 1px;
  }
  .cat-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.35rem 0.6rem;
    border-radius: 4px;
    background: transparent;
    color: var(--text-muted);
    font-size: 0.8rem;
    text-align: left;
  }
  .cat-item:hover {
    background: var(--table-row-hover);
    color: var(--text-main);
  }
  .cat-item.active {
    background: var(--accent-blue);
    color: #ffffff;
    font-weight: 600;
  }
  .cat-label {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }
  .cat-badge {
    font-size: 0.75rem;
    font-family: var(--font-mono);
    opacity: 0.85;
  }
  .status-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
  }
  .active-dot { background: var(--accent-blue); }
  .queued-dot { background: var(--accent-amber); }
  .paused-dot { background: #eab308; }
  .completed-dot { background: var(--accent-green); }
  .corrupt-dot { background: var(--accent-red); }
  .failed-dot { background: var(--accent-red); }

  .sidebar-folder-box {
    margin-top: auto;
    padding: 0.75rem;
    border-top: 1px solid var(--border-color);
    background: var(--sidebar-bg);
  }
  .folder-box-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.65rem;
    font-weight: 700;
    color: var(--text-dim);
    margin-bottom: 0.3rem;
  }
  .btn-browse-mini {
    background: transparent;
    color: var(--accent-blue);
    font-size: 0.7rem;
    padding: 0;
  }
  .btn-browse-mini:hover {
    text-decoration: underline;
  }
  .folder-box-path {
    font-size: 0.75rem;
    color: var(--text-muted);
    word-break: break-all;
    font-family: var(--font-mono);
  }

  /* Table Content Pane */
  .content-pane {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background: var(--table-bg);
  }
  .table-container {
    flex: 1;
    overflow: auto;
  }
  .torrent-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    text-align: left;
    white-space: nowrap;
    table-layout: fixed;
  }
  .torrent-table th {
    position: sticky;
    top: 0;
    background: var(--table-header-bg);
    color: var(--text-muted);
    font-weight: 600;
    padding: 0.4rem 0.6rem;
    border-bottom: 1px solid var(--border-color);
    border-right: 1px solid var(--border-subtle);
    font-size: 0.75rem;
    position: relative;
    user-select: none;
  }

  .th-sortable {
    cursor: pointer;
  }
  .th-sortable:hover {
    background: var(--table-row-hover);
    color: var(--text-main);
  }
  .sort-indicator {
    font-size: 0.65rem;
    margin-left: 0.3rem;
    color: var(--accent-blue);
  }

  /* Column Resizer Handle */
  .col-resizer {
    position: absolute;
    right: 0;
    top: 0;
    bottom: 0;
    width: 5px;
    cursor: col-resize;
    z-index: 10;
  }
  .col-resizer:hover {
    background: var(--resizer-hover);
  }

  .torrent-row {
    border-bottom: 1px solid var(--border-subtle);
    cursor: pointer;
    user-select: none;
  }
  .torrent-row:nth-child(even) {
    background: var(--table-row-alt);
  }
  .torrent-row:hover {
    background: var(--table-row-hover);
  }
  .torrent-row.selected {
    background: var(--table-row-selected) !important;
    color: var(--text-main);
  }
  .torrent-row.row-corrupt {
    background: rgba(239, 68, 68, 0.08);
  }
  .torrent-table td {
    padding: 0.35rem 0.6rem;
    border-right: 1px solid var(--border-subtle);
    overflow: hidden;
    text-overflow: ellipsis;
    box-sizing: border-box;
  }

  .col-size, .col-done { text-align: right; }
  .col-prog {
    overflow: hidden;
    box-sizing: border-box;
    padding: 0.35rem 0.5rem;
  }
  .col-status { text-align: center; }
  .col-speed { text-align: right; }
  .col-eta { text-align: right; }
  .col-path { color: var(--text-muted); }
  .col-added { color: var(--text-muted); font-size: 0.75rem; }

  .font-mono {
    font-family: var(--font-mono);
  }

  .name-cell {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    overflow: hidden;
  }
  .file-text {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Progress cell */
  .progress-cell {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    width: 100%;
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
    box-sizing: border-box;
  }
  .native-progress-track {
    flex: 1 1 auto;
    min-width: 20px;
    height: 8px;
    background: var(--border-subtle);
    border-radius: 2px;
    overflow: hidden;
    border: 1px solid var(--border-color);
  }
  .native-progress-fill {
    height: 100%;
    background: var(--accent-blue);
    transition: width 0.2s;
  }
  .native-progress-fill.prog-done { background: var(--accent-green); }
  .native-progress-fill.prog-corrupt { background: var(--accent-red); }
  .native-progress-fill.prog-paused { background: var(--accent-amber); }
  .native-progress-fill.prog-error { background: var(--accent-red); }
  .native-progress-fill.prog-compressing { background: #a855f7; }
  .prog-label {
    flex-shrink: 0;
    font-size: 0.72rem;
    text-align: right;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    line-height: 1;
  }

  /* Status tag */
  .status-tag {
    font-size: 0.7rem;
    font-weight: 700;
    text-transform: uppercase;
    padding: 0.15rem 0.45rem;
    border-radius: 3px;
  }
  .status-downloading { background: rgba(56, 139, 253, 0.18); color: var(--accent-blue); }
  .status-compressing { background: rgba(168, 85, 247, 0.22); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.4); }
  .status-queued { background: rgba(210, 153, 34, 0.18); color: var(--accent-amber); }
  .status-paused { background: rgba(234, 179, 8, 0.18); color: #eab308; }
  .status-completed { background: rgba(46, 160, 67, 0.18); color: var(--accent-green); }
  .status-missing { background: rgba(217, 119, 6, 0.22); color: #f59e0b; border: 1px solid rgba(245, 158, 11, 0.45); }
  .missing-dot { background: #f59e0b; }
  .val-missing { color: #f59e0b; font-weight: 700; }
  .status-corrupted { background: rgba(248, 81, 73, 0.25); color: var(--accent-red); border: 1px solid var(--accent-red); }
  .status-failed { background: rgba(248, 81, 73, 0.18); color: var(--accent-red); }
  .status-cancelled { background: var(--border-subtle); color: var(--text-dim); }

  /* Folder Preview in Add Modal */
  .folder-preview-card {
    margin-top: 0.75rem;
    padding: 0.75rem;
    background: var(--toolbar-bg);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .folder-preview-header {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .folder-preview-icon {
    font-size: 1.4rem;
  }
  .folder-preview-info {
    flex: 1;
    overflow: hidden;
  }
  .folder-preview-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .folder-preview-subtitle {
    font-size: 0.75rem;
    color: var(--accent-blue);
    font-weight: 500;
  }
  .folder-mode-options {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    margin-top: 0.2rem;
  }
  .radio-option {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    padding: 0.4rem 0.5rem;
    border-radius: 4px;
    background: var(--input-bg);
    border: 1px solid var(--border-subtle);
    cursor: pointer;
    user-select: none;
  }
  .radio-option:hover {
    border-color: var(--accent-blue);
  }
  .radio-option input[type="radio"] {
    margin-top: 0.2rem;
    cursor: pointer;
  }
  .radio-text {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .radio-label-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .radio-label-desc {
    font-size: 0.72rem;
    color: var(--text-muted);
    line-height: 1.25;
  }
  .folder-preview-loading {
    margin-top: 0.75rem;
    padding: 0.6rem 0.75rem;
    background: var(--toolbar-bg);
    border: 1px dashed var(--border-color);
    border-radius: 4px;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--text-muted);
    font-size: 0.75rem;
  }
  .folder-resolve-error {
    margin-top: 0.75rem;
    padding: 0.5rem 0.75rem;
    background: rgba(248, 81, 73, 0.1);
    border: 1px solid rgba(248, 81, 73, 0.3);
    border-radius: 4px;
    color: var(--accent-red);
    font-size: 0.75rem;
  }

  /* Empty table */
  .table-empty {
    padding: 3rem;
    text-align: center;
    color: var(--text-dim);
  }

  /* Detail Pane */
  .detail-pane {
    height: 145px;
    background: var(--toolbar-bg);
    border-top: 1px solid var(--border-color);
    padding: 0.6rem 0.85rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    flex-shrink: 0;
  }
  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 0.35rem;
  }
  .detail-title {
    font-size: 0.8rem;
    color: var(--text-main);
  }
  .detail-actions {
    display: flex;
    gap: 0.35rem;
  }
  .detail-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .prop-label {
    color: var(--text-dim);
    margin-right: 0.35rem;
  }
  .prop-val {
    color: var(--text-main);
    font-weight: 500;
  }
  .val-corrupt {
    color: var(--accent-red);
    font-weight: 700;
  }
  .url-truncate {
    display: inline-block;
    max-width: 320px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: bottom;
  }
  .detail-corrupt {
    font-size: 0.75rem;
    color: var(--accent-red);
    background: rgba(248, 81, 73, 0.12);
    padding: 0.35rem 0.6rem;
    border-radius: 4px;
    border: 1px solid var(--accent-red);
  }
  .detail-corrupt .error-desc {
    margin-top: 0.2rem;
    font-family: var(--font-mono);
  }
  .detail-error {
    font-size: 0.75rem;
    color: var(--accent-red);
    background: rgba(248, 81, 73, 0.1);
    padding: 0.25rem 0.5rem;
    border-radius: 3px;
    border: 1px solid rgba(248, 81, 73, 0.3);
  }
  .btn-mini {
    padding: 0.2rem 0.5rem;
    font-size: 0.75rem;
    border-radius: 3px;
    border: 1px solid var(--border-color);
  }
  .btn-action { background: var(--accent-green); color: #fff; border: none; }
  .btn-danger { background: var(--accent-red); color: #fff; border: none; }

  /* Statusbar */
  .statusbar {
    height: 24px;
    background: var(--statusbar-bg);
    border-top: 1px solid var(--border-color);
    display: flex;
    align-items: center;
    padding: 0 0.75rem;
    font-size: 0.75rem;
    color: var(--text-muted);
    gap: 1rem;
    flex-shrink: 0;
  }
  .sb-section {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .sb-divider {
    color: var(--border-color);
  }
  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--accent-red);
  }
  .status-dot.connected {
    background: var(--accent-green);
  }
  .sb-badge-auth {
    font-size: 0.65rem;
    background: rgba(46, 160, 67, 0.15);
    color: var(--accent-green);
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
    font-weight: 600;
  }

  /* Modals */
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
  }
  .modal-window {
    width: 90%;
    max-width: 520px;
    background: var(--modal-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 15px 35px rgba(0,0,0,0.4);
    display: flex;
    flex-direction: column;
    color: var(--text-main);
  }
  .modal-conflict {
    max-width: 660px !important;
    width: 92% !important;
    max-height: 85vh;
  }
  .conflict-modal-body {
    overflow-y: auto;
    max-height: calc(85vh - 110px);
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.85rem;
  }
  .conflict-intro {
    font-size: 0.78rem;
    color: var(--text-muted);
    margin: 0;
    line-height: 1.4;
  }
  .conflict-quick-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: var(--toolbar-bg);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 0.4rem 0.6rem;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .quick-bar-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-dim);
  }
  .quick-bar-actions {
    display: flex;
    gap: 0.35rem;
    flex-wrap: wrap;
  }
  .conflict-cards-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }
  .conflict-card {
    background: var(--toolbar-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    padding: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  .conflict-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }
  .conflict-header-title {
    display: flex;
    align-items: center;
    gap: 6px;
    overflow: hidden;
    flex: 1;
    min-width: 0;
  }
  .conflict-filename {
    font-weight: 600;
    font-size: 0.82rem;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .conflict-badges {
    display: flex;
    gap: 0.35rem;
    flex-shrink: 0;
  }
  .conflict-badge {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0.15rem 0.45rem;
    border-radius: 3px;
    letter-spacing: 0.3px;
  }
  .badge-in-list {
    background: rgba(56, 139, 253, 0.18);
    color: var(--accent-blue);
    border: 1px solid rgba(56, 139, 253, 0.35);
  }
  .badge-on-disk {
    background: rgba(46, 160, 67, 0.18);
    color: var(--accent-green);
    border: 1px solid rgba(46, 160, 67, 0.35);
  }
  .conflict-card-path {
    font-size: 0.72rem;
    color: var(--text-muted);
    word-break: break-all;
  }
  .conflict-options {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    margin-top: 0.2rem;
  }
  .conflict-option {
    display: flex;
    align-items: flex-start;
    gap: 0.6rem;
    padding: 0.45rem 0.6rem;
    background: var(--bg-main);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    cursor: pointer;
    transition: border-color 0.15s, background 0.15s;
  }
  .conflict-option:hover {
    border-color: var(--accent-blue);
  }
  .conflict-option.selected {
    border-color: var(--accent-blue);
    background: rgba(56, 139, 253, 0.08);
  }
  .conflict-option input[type="radio"] {
    margin-top: 3px;
    cursor: pointer;
  }
  .conflict-option-content {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .conflict-option-title {
    font-size: 0.78rem;
    color: var(--text-main);
  }
  .conflict-option-title code {
    background: var(--toolbar-bg);
    padding: 1px 4px;
    border-radius: 3px;
    font-size: 0.72rem;
    border: 1px solid var(--border-color);
  }
  .conflict-option-desc {
    font-size: 0.72rem;
    color: var(--text-muted);
    line-height: 1.3;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.6rem 0.85rem;
    background: var(--modal-header-bg);
    border-bottom: 1px solid var(--border-color);
    font-weight: 600;
    font-size: 0.85rem;
  }
  .modal-close {
    background: transparent;
    color: var(--text-muted);
    font-size: 0.85rem;
  }
  .modal-close:hover { color: var(--text-main); }
  .modal-body {
    padding: 0.85rem;
  }
  .auth-status-banner {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    background: var(--sidebar-bg);
    padding: 0.5rem 0.75rem;
    border-radius: 4px;
    border: 1px solid var(--border-color);
    font-size: 0.8rem;
  }
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .form-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .form-hint {
    font-size: 0.7rem;
    color: var(--text-dim);
    line-height: 1.4;
  }
  .path-picker-row {
    display: flex;
    gap: 0.4rem;
  }
  .path-picker-row input {
    flex: 1;
    padding: 0.35rem 0.6rem;
    font-family: var(--font-mono);
  }
  .btn-browse {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0.35rem 0.65rem;
    border-radius: 4px;
    font-size: 0.8rem;
  }
  .btn-browse:hover {
    background: var(--btn-hover);
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    padding: 0.6rem 0.85rem;
    background: var(--modal-header-bg);
    border-top: 1px solid var(--border-color);
  }
  .btn {
    padding: 0.35rem 0.85rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-weight: 500;
  }
  .btn-secondary {
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
  }
  .btn-secondary:hover { background: var(--btn-hover); }
  .btn-danger {
    background: var(--accent-red);
    color: #fff;
  }
  .btn-danger:hover { opacity: 0.9; }
  .btn-primary {
    background: var(--accent-green);
    color: #fff;
    font-weight: 600;
  }
  .btn-primary:hover:not(:disabled) { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.4; cursor: not-allowed; }

  /* Spinners */
  .mini-spinner {
    width: 13px;
    height: 13px;
    border: 2px solid rgba(56, 139, 253, 0.25);
    border-top-color: var(--accent-blue);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    flex-shrink: 0;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Expandable Folder Toggle */
  .folder-expand-toggle {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: var(--btn-bg);
    border: 1px solid var(--border-subtle);
    border-radius: 3px;
    padding: 1px 5px;
    margin-right: 4px;
    color: var(--text-main);
    cursor: pointer;
    font-size: 0.65rem;
    line-height: 1;
    transition: background 0.15s;
    user-select: none;
    flex-shrink: 0;
  }
  .folder-expand-toggle:hover {
    background: var(--btn-hover);
    border-color: var(--accent-blue);
  }
  .folder-arrow {
    font-size: 0.6rem;
    color: var(--accent-blue);
    display: inline-block;
  }
  .folder-file-badge {
    font-family: var(--font-mono);
    color: var(--text-muted);
    font-size: 0.65rem;
  }

  /* Child File Rows (Sub-items in folder) */
  .child-file-row {
    background: rgba(0, 0, 0, 0.15);
    border-bottom: 1px dashed var(--border-subtle);
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  :global([data-theme="light"]) .child-file-row {
    background: rgba(0, 0, 0, 0.025);
  }
  .child-file-row:hover {
    background: var(--table-row-hover);
    color: var(--text-main);
  }
  .child-name-cell {
    display: flex;
    align-items: center;
    gap: 6px;
    padding-left: 1.25rem;
  }
  .tree-line {
    color: var(--text-dim);
    font-family: var(--font-mono);
    font-size: 0.75rem;
  }
  .child-filename {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .child-prog-wrap {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    width: 100%;
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
    box-sizing: border-box;
  }
  .child-prog-text {
    flex-shrink: 0;
    font-size: 0.7rem;
    color: var(--text-muted);
    min-width: 28px;
    text-align: right;
    white-space: nowrap;
    overflow: hidden;
    line-height: 1;
  }
  .child-status-pill {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 3px;
    font-size: 0.65rem;
    font-weight: 600;
  }
  .child-status-completed {
    background: rgba(34, 197, 94, 0.15);
    color: var(--accent-green);
  }
  .child-status-downloading {
    background: rgba(56, 139, 253, 0.15);
    color: var(--accent-blue);
  }
  .child-status-queued {
    background: var(--border-subtle);
    color: var(--text-dim);
  }
  .child-status-failed {
    background: rgba(239, 68, 68, 0.15);
    color: var(--accent-red);
  }
  .in-archive-label {
    font-size: 0.7rem;
    color: var(--text-dim);
    font-style: italic;
  }

  /* Desktop Context Menu */
  .context-menu {
    position: fixed;
    z-index: 10000;
    min-width: 200px;
    background: var(--modal-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
    padding: 4px;
    display: flex;
    flex-direction: column;
    animation: menuFadeIn 0.1s ease-out;
  }
  @keyframes menuFadeIn {
    from { opacity: 0; transform: scale(0.96); }
    to { opacity: 1; transform: scale(1); }
  }
  .context-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 10px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-main);
    font-size: 0.78rem;
    text-align: left;
    cursor: pointer;
    user-select: none;
  }
  .context-item:hover:not(:disabled) {
    background: var(--accent-blue);
    color: #ffffff;
  }
  .context-item:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .context-item.text-danger:hover:not(:disabled) {
    background: var(--accent-red);
    color: #ffffff;
  }
  .context-key {
    margin-left: auto;
    font-size: 0.68rem;
    font-family: var(--font-mono);
    color: var(--text-dim);
    opacity: 0.8;
  }
  .context-item:hover:not(:disabled) .context-key {
    color: #ffffff;
  }
  .context-divider {
    height: 1px;
    background: var(--border-color);
    margin: 4px 0;
  }

  /* 2-Option Delete Modal Window */
  .delete-modal-window {
    max-width: 480px;
    width: 90%;
  }
  .delete-header-title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 700;
    color: var(--accent-red);
  }
  .delete-prompt-desc {
    font-size: 0.85rem;
    color: var(--text-main);
    line-height: 1.5;
    margin-bottom: 1rem;
  }
  .delete-radio-group {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .delete-radio-card {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 12px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    background: var(--card-bg);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .delete-radio-card:hover {
    border-color: var(--accent-blue);
    background: var(--btn-hover);
  }
  .delete-radio-card.active {
    border-color: var(--accent-blue);
    background: rgba(56, 139, 253, 0.08);
  }
  .delete-radio-card.danger.active {
    border-color: var(--accent-red);
    background: rgba(239, 68, 68, 0.08);
  }
  .delete-radio-card input {
    margin-top: 3px;
  }
  .delete-radio-text {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .delete-radio-title {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .delete-radio-sub {
    font-size: 0.72rem;
    color: var(--text-muted);
    line-height: 1.4;
  }

  /* Native Desktop Menu Bar */
  .app-menubar {
    display: flex;
    align-items: center;
    background: var(--menubar-bg);
    border-bottom: 1px solid var(--border-color);
    padding: 0 6px;
    height: 28px;
    flex-shrink: 0;
    user-select: none;
    font-size: 0.8rem;
    z-index: 50;
    position: relative;
  }
  .menubar-item {
    position: relative;
  }
  .menubar-btn {
    background: transparent;
    border: none;
    color: var(--text-main);
    padding: 3px 9px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.8rem;
    display: flex;
    align-items: center;
    gap: 4px;
    transition: background 0.12s;
  }
  .menubar-btn:hover,
  .menubar-item.active .menubar-btn {
    background: var(--btn-hover);
    color: var(--accent-blue);
  }
  .menubar-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    margin-top: 2px;
    min-width: 230px;
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
    padding: 4px;
    z-index: 200;
    animation: menuFadeIn 0.12s ease-out;
  }
  @keyframes menuFadeIn {
    from { opacity: 0; transform: translateY(-3px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .dropdown-item {
    display: flex;
    align-items: center;
    width: 100%;
    padding: 6px 10px;
    background: transparent;
    border: none;
    border-radius: 4px;
    color: var(--text-main);
    font-size: 0.78rem;
    text-align: left;
    cursor: pointer;
    text-decoration: none;
    gap: 8px;
    box-sizing: border-box;
  }
  .dropdown-item:hover:not(:disabled) {
    background: var(--accent-blue);
    color: #ffffff;
  }
  .dropdown-item:disabled {
    opacity: 0.35;
    cursor: not-allowed;
  }
  .dropdown-item.text-danger:hover:not(:disabled) {
    background: var(--accent-red);
    color: #ffffff;
  }
  .dropdown-icon {
    font-size: 0.85rem;
    width: 18px;
    text-align: center;
  }
  .dropdown-text {
    flex: 1;
  }
  .dropdown-shortcut {
    font-size: 0.68rem;
    font-family: var(--font-mono);
    color: var(--text-dim);
    margin-left: auto;
  }
  .dropdown-item:hover:not(:disabled) .dropdown-shortcut {
    color: #ffffff;
  }
  .menu-divider {
    height: 1px;
    background: var(--border-color);
    margin: 4px 0;
  }

  /* Detail Pane: Missing / Moved Banner */
  .detail-missing {
    margin-top: 0.75rem;
    padding: 0.75rem 1rem;
    background: rgba(245, 158, 11, 0.1);
    border: 1px solid rgba(245, 158, 11, 0.35);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
  }
  .detail-missing-header {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    color: #f59e0b;
  }
  .missing-desc {
    font-size: 0.8rem;
    color: var(--text-muted);
    margin: 3px 0 0 0;
    line-height: 1.4;
  }
  .detail-missing-actions {
    display: flex;
    gap: 8px;
    margin-left: 28px;
    flex-wrap: wrap;
  }

  /* Documentation Modal */
  .doc-modal-window {
    max-width: 680px;
    width: 92%;
    max-height: 85vh;
  }
  .doc-modal-body {
    overflow-y: auto;
    max-height: 65vh;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    font-size: 0.84rem;
    line-height: 1.55;
  }
  .doc-section h4 {
    margin: 0 0 0.4rem 0;
    color: var(--accent-blue);
    font-size: 0.92rem;
  }
  .doc-section p {
    margin: 0 0 0.5rem 0;
    color: var(--text-main);
  }
  .doc-section ul {
    margin: 0 0 0.5rem 1.2rem;
    padding: 0;
    color: var(--text-muted);
  }
  .doc-section li {
    margin-bottom: 0.25rem;
  }
  .doc-section kbd {
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    border-radius: 3px;
    padding: 2px 6px;
    font-size: 0.75rem;
    font-family: var(--font-mono);
    color: var(--text-main);
  }
  .shortcut-table {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 6px;
    background: var(--table-row-alt);
    padding: 10px;
    border-radius: 6px;
    border: 1px solid var(--border-subtle);
  }
  .shortcut-table div {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .shortcut-table span {
    color: var(--text-muted);
    font-size: 0.78rem;
  }

  /* Logs Badge & Modal */
  .log-badge-error {
    background: var(--accent-red);
    color: #fff;
    border-radius: 10px;
    padding: 0 5px;
    font-size: 10px;
    font-weight: 700;
    margin-left: 4px;
    line-height: 16px;
    display: inline-block;
  }
  .logs-modal-window {
    max-width: 960px;
    width: 95%;
    height: 80vh;
    display: flex;
    flex-direction: column;
  }
  .logs-header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .logs-filter-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: 8px 16px;
    background: var(--table-header-bg);
    border-bottom: 1px solid var(--border-subtle);
    flex-wrap: wrap;
  }
  .logs-filter-chips {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }
  .log-filter-chip {
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-muted);
    border-radius: 12px;
    padding: 2px 10px;
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .log-filter-chip:hover {
    color: var(--text-main);
    border-color: var(--accent-blue);
  }
  .log-filter-chip.active {
    background: var(--accent-blue);
    color: #fff;
    border-color: var(--accent-blue);
  }
  .log-filter-chip.chip-error.active {
    background: var(--accent-red);
    border-color: var(--accent-red);
  }
  .log-filter-chip.chip-warn.active {
    background: var(--accent-amber);
    border-color: var(--accent-amber);
  }
  .log-filter-chip.chip-success.active {
    background: var(--accent-green);
    border-color: var(--accent-green);
  }
  .logs-search-input {
    min-width: 220px;
    flex: 1;
    max-width: 320px;
    padding: 4px 10px;
    font-size: 11.5px;
    border-radius: 4px;
    background: var(--app-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
  }
  .logs-modal-body {
    flex: 1;
    overflow-y: auto;
    padding: 8px;
    background: #0d1117;
    color: #c9d1d9;
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .logs-container {
    display: flex;
    flex-direction: column;
    gap: 3px;
  }
  .log-row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 3px 6px;
    border-radius: 3px;
    line-height: 1.4;
    border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  }
  .log-row-ERROR {
    background: rgba(239, 68, 68, 0.1);
  }
  .log-row-WARN {
    background: rgba(245, 158, 11, 0.08);
  }
  .log-time {
    color: #8b949e;
    min-width: 125px;
    white-space: nowrap;
  }
  .log-level-pill {
    font-size: 9.5px;
    font-weight: 700;
    padding: 0 5px;
    border-radius: 3px;
    min-width: 52px;
    text-align: center;
    line-height: 16px;
    display: inline-block;
  }
  .log-level-ERROR {
    background: rgba(239, 68, 68, 0.25);
    color: #f87171;
  }
  .log-level-WARN {
    background: rgba(245, 158, 11, 0.25);
    color: #fbbf24;
  }
  .log-level-INFO {
    background: rgba(59, 130, 246, 0.25);
    color: #60a5fa;
  }
  .log-level-SUCCESS {
    background: rgba(34, 197, 94, 0.25);
    color: #4ade80;
  }
  .log-category {
    color: #a78bfa;
    font-weight: 600;
    min-width: 80px;
    white-space: nowrap;
  }
  .log-msg-col {
    flex: 1;
    word-break: break-word;
  }
  .log-msg {
    color: #f0f6fc;
  }
  .log-details {
    color: #8b949e;
    margin-top: 2px;
    font-size: 10px;
  }
  .log-empty {
    padding: 40px;
    text-align: center;
    color: #8b949e;
  }
  .child-status-paused {
    background: rgba(245, 158, 11, 0.15);
    color: var(--accent-amber);
  }
</style>
