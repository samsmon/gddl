import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart4(app) {
  app.toggleAuthRequirement = async function() {
    try {
      const res = await fetch('/api/auth/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: app.authEnabledToggle })
      });
      if (res.ok) {
        app.authEnabled = app.authEnabledToggle;
        app.securityMessage = `Web UI Authentication ${app.authEnabled ? 'Enabled' : 'Disabled'}.`;
      }
    } catch (e) {
      app.securityError = 'Failed to toggle auth: ' + e.message;
    }
  }

  app.startAppServices = function() {
    app.loadConfig();
    app.fetchDownloads();
    app.fetchLogs();
    app.fetchGoogleCookies();
    app.fetchOAuthStatus();
    app.fetchWarpStatus();
    app.setupSSE();
    if (!app.pollInterval) {
      app.pollInterval = setInterval(() => {
        app.fetchDownloads();
        app.fetchWarpStatus();
        if (app.showLogsModal) {
          app.fetchLogs();
        }
      }, 3000);
    }
  }

  app.fetchLogs = async function() {
    if (app.authEnabled && !app.isAuthenticated) return;
    app.isFetchingLogs = true;
    try {
      const res = await fetch('/api/logs?limit=300');
      if (res.ok) {
        app.logs = await res.json();
      }
    } catch (e) {
      console.error('Failed to fetch logs:', e);
    } finally {
      app.isFetchingLogs = false;
    }
  }

  app.clearLogs = async function() {
    try {
      await fetch('/api/logs', { method: 'DELETE' });
      app.logs = [];
    } catch (e) {
      console.error('Failed to clear logs:', e);
    }
  }

  app.openLogsModal = function(filter = 'ALL') {
    app.logFilter = filter;
    app.showLogsModal = true;
    app.fetchLogs();
  }

  app.copyLogsToClipboard = function() {
    if (app.logs.length === 0) return;
    const text = app.logs.map(l => `[${l.timestamp}] [${l.level}] [${l.category}] ${l.message}${l.details ? ' | ' + l.details : ''}`).join('\n');
    
    // Modern Clipboard API with fallback for non-secure HTTP contexts
    if (navigator.clipboard && window.isSecureContext && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        app.copiedLogs = true;
        setTimeout(() => app.copiedLogs = false, 2000);
      }).catch(() => app.fallbackCopy(text));
    } else {
      app.fallbackCopy(text);
    }
  }

  app.fallbackCopy = function(text) {
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.left = '-9999px';
      textArea.style.top = '0';
      textArea.style.opacity = '0';
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      const success = document.execCommand('copy');
      document.body.removeChild(textArea);
      if (success) {
        app.copiedLogs = true;
        setTimeout(() => app.copiedLogs = false, 2000);
      }
    } catch (err) {
      console.error('Fallback copy failed', err);
    }
  }

  app.copyLogRow = function(log) {
    const text = `[${log.timestamp}] [${log.level}] [${log.category}] ${log.message}${log.details ? ' | ' + log.details : ''}`;
    if (navigator.clipboard && window.isSecureContext && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        app.copiedRowId = log.id;
        setTimeout(() => app.copiedRowId = null, 1500);
      }).catch(() => app.fallbackCopyRow(text, log.id));
    } else {
      app.fallbackCopyRow(text, log.id);
    }
  }

  app.fallbackCopyRow = function(text, id) {
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.left = '-9999px';
      textArea.style.top = '0';
      textArea.style.opacity = '0';
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      const success = document.execCommand('copy');
      document.body.removeChild(textArea);
      if (success) {
        app.copiedRowId = id;
        setTimeout(() => app.copiedRowId = null, 1500);
      }
    } catch (err) {
      console.error('Fallback copy failed', err);
    }
  }

  app.openDiscordExportModal = async function() {
    app.showDiscordExportModal = true;
    app.discordExportCopied = false;
    await app.fetchUnfinishedDiscord();
  }

  app.fetchUnfinishedDiscord = async function() {
    app.isFetchingDiscord = true;
    try {
      const res = await fetch('/api/discord/unfinished');
      if (res.ok) {
        app.unfinishedDiscordItems = (await res.json()) || [];
      } else {
        console.error('Failed to fetch unfinished discord items');
      }
    } catch (e) {
      console.error('Error fetching unfinished discord items:', e);
    } finally {
      app.isFetchingDiscord = false;
    }
  }

  app.copyDiscordExport = function() {
    if (!app.formattedDiscordExportText) return;
    if (navigator.clipboard && window.isSecureContext && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(app.formattedDiscordExportText).then(() => {
        app.discordExportCopied = true;
        setTimeout(() => app.discordExportCopied = false, 2000);
      }).catch(() => app.fallbackCopyDiscordExport(app.formattedDiscordExportText));
    } else {
      app.fallbackCopyDiscordExport(app.formattedDiscordExportText);
    }
  }

  app.fallbackCopyDiscordExport = function(text) {
    try {
      const textArea = document.createElement('textarea');
      textArea.value = text;
      textArea.style.position = 'fixed';
      textArea.style.left = '-9999px';
      textArea.style.top = '0';
      textArea.style.opacity = '0';
      document.body.appendChild(textArea);
      textArea.focus();
      textArea.select();
      const success = document.execCommand('copy');
      document.body.removeChild(textArea);
      if (success) {
        app.discordExportCopied = true;
        setTimeout(() => app.discordExportCopied = false, 2000);
      }
    } catch (err) {
      console.error('Fallback copy failed', err);
    }
  }

  app.downloadDiscordExport = function(format) {
    if (!app.formattedDiscordExportText) return;
    const isJson = format === 'json';
    const filename = isJson ? 'unfinished_discord_downloads.json' : 'unfinished_discord_urls.txt';
    const mime = isJson ? 'application/json' : 'text/plain';
    const blob = new Blob([app.formattedDiscordExportText], { type: mime });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }

  app.openDiscordRefreshModal = function() {
    app.discordRefreshResult = null;
    app.showDiscordRefreshModal = true;
  }
}
