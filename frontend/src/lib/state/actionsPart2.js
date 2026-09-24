import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart2(app) {
  app.handleConfirmAction = function() {
    const fn = app.confirmDialog.onConfirm;
    app.confirmDialog.show = false;
    if (fn) fn();
  }

  app.logoutGoogle = function() {
    app.openConfirm('Clear All Google Accounts', 'This will remove all stored Google session cookies and reset downloads to anonymous quota. Continue?', async () => {
      try {
        await fetch('/api/gdrive/logout', { method: 'POST' });
        app.hasLogin = false;
        await app.fetchGoogleCookies();
      } catch (e) {
        console.error('Error clearing cookies:', e);
      }
    }, 'Clear All Accounts', 'danger');
  }

  app.handleCookieInput = function(val) {
    const cleaned = parseCookieInput(val);
    if (cleaned && cleaned !== val) {
      app.newCookieValue = cleaned;
      app.wasCookieExtracted = true;
      setTimeout(() => { app.wasCookieExtracted = false; }, 4000);
    } else {
      app.newCookieValue = val;
    }
  }

  app.fetchGoogleCookies = async function() {
    try {
      const res = await fetch('/api/gdrive/cookies');
      if (res.ok) {
        app.cookieList = await res.json();
        app.hasLogin = app.cookieList.length > 0;
      }
    } catch (e) {
      console.error('Failed fetching cookies:', e);
    }
  }

  app.addCookieToPool = async function() {
    const rawVal = app.newCookieValue.trim();
    if (!rawVal) return;
    const cleaned = parseCookieInput(rawVal);
    app.isAddingCookie = true;
    app.cookieError = '';
    app.cookieMessage = '';
    try {
      const res = await fetch('/api/gdrive/cookies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          label: app.newCookieLabel.trim(),
          cookie: cleaned
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: 'Failed to add cookie' }));
        throw new Error(err.error || 'Failed to add cookie');
      }
      app.newCookieLabel = '';
      app.newCookieValue = '';
      app.wasCookieExtracted = false;
      app.cookieMessage = 'Account added to Cookie Pool successfully.';
      await app.fetchGoogleCookies();
      setTimeout(() => { app.cookieMessage = ''; }, 4000);
    } catch (e) {
      app.cookieError = e.message;
    } finally {
      app.isAddingCookie = false;
    }
  }

  app.removeCookie = function(id) {
    app.openConfirm('Remove Account from Pool', 'Remove this Google account session cookie from the pool?', async () => {
      try {
        const res = await fetch(`/api/gdrive/cookies/${id}`, { method: 'DELETE' });
        if (res.ok) {
          await app.fetchGoogleCookies();
        }
      } catch (e) {
        console.error(e);
      }
    }, 'Remove', 'danger');
  }

  app.resetCookieCooldown = async function(id) {
    try {
      const res = await fetch(`/api/gdrive/cookies/${id}/reset`, { method: 'POST' });
      if (res.ok) {
        await app.fetchGoogleCookies();
      }
    } catch (e) {
      console.error(e);
    }
  }

  app.resetAllCookieCooldowns = async function() {
    try {
      const res = await fetch('/api/gdrive/cookies/reset-all', { method: 'POST' });
      if (res.ok) {
        await app.fetchGoogleCookies();
        app.cookieMessage = 'All account cooldowns have been reset to Active.';
        setTimeout(() => { app.cookieMessage = ''; }, 3500);
      }
    } catch (e) {
      console.error(e);
    }
  }

  app.resetCooldownAndRetry = async function(itemId) {
    await app.resetAllCookieCooldowns();
    if (itemId) {
      app.startDownload(itemId);
    }
  }

  app.copyRedirectURIToClipboard = function(text) {
    if (!text) return;
    if (navigator.clipboard && window.isSecureContext && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => {
        app.copiedRedirectURI = true;
        setTimeout(() => app.copiedRedirectURI = false, 2000);
      }).catch(() => {
        app.fallbackCopy(text);
        app.copiedRedirectURI = true;
        setTimeout(() => app.copiedRedirectURI = false, 2000);
      });
    } else {
      app.fallbackCopy(text);
      app.copiedRedirectURI = true;
      setTimeout(() => app.copiedRedirectURI = false, 2000);
    }
  }

  app.fetchOAuthStatus = async function() {
    try {
      const res = await fetch('/api/gdrive/oauth/status');
      if (res.ok) {
        const data = await res.json();
        app.oauthConfigured = !!data.configured;
        app.oauthConnected = !!data.connected;
        if (data.client_id && !app.oauthClientID) {
          app.oauthClientID = data.client_id;
        }
        app.oauthEmail = data.email || '';
        app.oauthAutoBypass = data.auto_bypass ?? true;
        app.oauthRedirectURI = data.redirect_uri || '';
        app.oauthRedirectURIOverride = data.redirect_uri_override || '';
      }
    } catch (e) {
      console.warn('Failed to fetch OAuth status', e);
    }
  }

  app.saveOAuthConfig = async function() {
    app.oauthError = '';
    app.oauthMessage = '';
    app.isSavingOAuthConfig = true;
    try {
      const res = await fetch('/api/gdrive/oauth/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          client_id: app.oauthClientID.trim(),
          client_secret: app.oauthClientSecret.trim(),
          redirect_uri: app.oauthRedirectURIOverride.trim(),
          auto_bypass: app.oauthAutoBypass
        })
      });
      if (res.ok) {
        app.oauthMessage = 'Google OAuth credentials saved successfully!';
        app.oauthConfigured = !!app.oauthClientID.trim();
        await app.fetchOAuthStatus();
        setTimeout(() => { app.oauthMessage = ''; }, 4000);
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to save OAuth credentials' }));
        app.oauthError = err.error || 'Failed to save OAuth credentials';
      }
    } catch (e) {
      app.oauthError = 'Network error saving credentials: ' + e.message;
    } finally {
      app.isSavingOAuthConfig = false;
    }
  }

  app.startGoogleOAuth = async function() {
    app.oauthError = '';
    app.oauthMessage = '';
    try {
      if (app.oauthClientID.trim() || app.oauthClientSecret.trim() || app.oauthRedirectURIOverride.trim()) {
        await app.saveOAuthConfig();
      }

      const res = await fetch('/api/gdrive/oauth/auth-url');
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: 'Failed to generate authorization URL' }));
        throw new Error(err.error || 'Failed to generate authorization URL');
      }
      const data = await res.json();
      app.oauthAuthURL = data.auth_url;
      if (data.redirect_uri) {
        app.oauthRedirectURI = data.redirect_uri;
      }

      const width = 600;
      const height = 700;
      const left = window.screenX + (window.outerWidth - width) / 2;
      const top = window.screenY + (window.outerHeight - height) / 2;
      window.open(
        data.auth_url,
        'GoogleDriveAuth',
        `width=${width},height=${height},left=${left},top=${top},status=no,toolbar=no,menubar=no`
      );

      app.oauthMessage = 'Authorization window opened. Sign in and grant Google Drive permissions.';
    } catch (e) {
      app.oauthError = e.message;
    }
  }
}
