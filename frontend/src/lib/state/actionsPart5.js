import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart5(app) {
  app.submitRefreshDiscordURLs = async function() {
    if (!app.discordRefreshInput.trim()) return;
    app.isRefreshingDiscord = true;
    app.discordRefreshResult = null;

    try {
      const res = await fetch('/api/discord/refresh-urls', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ raw: app.discordRefreshInput.trim() })
      });
      const data = await res.json();
      if (res.ok) {
        app.discordRefreshResult = {
          success: true,
          message: `Updated ${data.updated} item(s)! Downloads have resumed.`,
          updated: data.updated,
          not_found: data.not_found,
          total: data.total_urls
        };
        app.fetchDownloads();
      } else {
        app.discordRefreshResult = {
          success: false,
          message: data.error || 'Failed to update Discord URLs'
        };
      }
    } catch (err) {
      app.discordRefreshResult = {
        success: false,
        message: 'Network or server error while updating URLs: ' + err.message
      };
    } finally {
      app.isRefreshingDiscord = false;
    }
  }

  app.fetchDownloads = async function() {
    if (app.authEnabled && !app.isAuthenticated) return;
    try {
      const res = await fetch('/api/downloads');
      if (res.status === 401) {
        app.isAuthenticated = false;
        app.backendConnected = false;
        return;
      }
      if (res.ok) {
        app.downloads = await res.json();
        app.backendConnected = true;
      }
    } catch (e) {
      app.backendConnected = false;
    }
  }

  app.setupSSE = function() {
    if (app.authEnabled && !app.isAuthenticated) return;
    if (app.eventSource) app.eventSource.close();
    try {
      app.eventSource = new EventSource('/api/events');
      app.eventSource.onopen = () => { app.backendConnected = true; };
      app.eventSource.onmessage = (event) => {
        try {
          app.downloads = JSON.parse(event.data);
          app.backendConnected = true;
        } catch (err) {
          console.error(err);
        }
      };
      app.eventSource.onerror = () => {
        app.backendConnected = false;
        app.eventSource.close();
        if (app.isAuthenticated || !app.authEnabled) {
          setTimeout(app.setupSSE, 3000);
        }
      };
    } catch (e) {
      console.error(e);
    }
  }

  app.handleLinksInput = function() {
    clearTimeout(app.resolveTimer);
    app.detectedFolder = null;
    app.resolveError = '';

    const lines = app.addLinksInput.split(/[\n,]+/).map(s => s.trim()).filter(Boolean);
    const folderLink = lines.find(l => 
      l.includes('/drive/folders/') || 
      l.includes('/drive/u/') || 
      l.includes('folderview?id=') ||
      (l.includes('drive.google.com/open?id=') && l.includes('usp=drive_link'))
    );

    if (folderLink) {
      app.resolveTimer = setTimeout(() => {
        app.resolveFolderInfo(folderLink);
      }, 400);
    }
  }

  app.resolveFolderInfo = async function(url) {
    app.isResolvingFolder = true;
    app.resolveError = '';
    try {
      const res = await fetch('/api/folders/resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ url })
      });
      if (res.ok) {
        const data = await res.json();
        if (data.is_folder) {
          app.detectedFolder = data;
        }
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to inspect folder' }));
        app.resolveError = err.error || 'Failed to inspect folder';
      }
    } catch (e) {
      app.resolveError = e.message;
    } finally {
      app.isResolvingFolder = false;
    }
  }

  app.submitAddDownloads = async function() {
    if (!app.addLinksInput.trim()) return;

    const raw = app.addLinksInput
      .split(/[\n,]+/)
      .map(l => l.trim())
      .filter(l => l.length > 0);

    if (raw.length === 0) return;

    // For bulk links (multiple URLs), close modal instantly like IDM!
    if (raw.length > 1) {
      const linksToProcess = [...raw];
      const target = app.addTargetFolder || app.defaultFolder;
      const isZip = app.folderZipMode === 'zip';

      app.showAddModal = false;
      app.addLinksInput = '';
      app.detectedFolder = null;
      app.resolveError = '';
      app.folderZipMode = 'zip';

      app.bulkAddingStatus = `Enqueuing ${linksToProcess.length} links...`;
      try {
        const res = await fetch('/api/downloads', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            links: linksToProcess,
            target_folder: target,
            zip_mode: isZip,
            conflict_resolutions: {}
          })
        });
        if (res.ok) {
          app.bulkAddingStatus = `Added ${linksToProcess.length} links to queue.`;
          app.fetchDownloads();
          setTimeout(() => { app.bulkAddingStatus = ''; }, 3500);
        } else {
          const err = await res.json().catch(() => ({ error: 'Failed to enqueue links' }));
          app.bulkAddingStatus = 'Error: ' + (err.error || 'Failed');
          setTimeout(() => { app.bulkAddingStatus = ''; }, 5000);
        }
      } catch (e) {
        app.bulkAddingStatus = 'Error: ' + e.message;
        setTimeout(() => { app.bulkAddingStatus = ''; }, 5000);
      }
      return;
    }

    // Single link flow with fast precheck:
    app.isAdding = true;
    try {
      const checkRes = await fetch('/api/downloads/precheck', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          links: raw,
          target_folder: app.addTargetFolder || app.defaultFolder,
          zip_mode: app.folderZipMode === 'zip'
        })
      });

      if (checkRes.ok) {
        const checkData = await checkRes.json();
        if (checkData.has_conflicts && checkData.conflicts && checkData.conflicts.length > 0) {
          app.conflictList = checkData.conflicts;
          const initialResolutions = {};
          for (const c of checkData.conflicts) {
            initialResolutions[c.file_id || c.url] = c.suggested_action || 'rename';
          }
          app.conflictResolutions = initialResolutions;
          app.showConflictModal = true;
          app.isAdding = false;
          return;
        }
      }

      await app.executeAddDownloads({});
    } catch (e) {
      alert('Error: ' + e.message);
      app.isAdding = false;
    }
  }
}
