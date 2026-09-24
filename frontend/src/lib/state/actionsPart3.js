import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';

export function attachActionsPart3(app) {
  app.submitManualCode = async function() {
    if (!app.oauthManualCode.trim()) return;
    app.oauthError = '';
    app.oauthMessage = '';
    app.isSubmittingManualCode = true;
    try {
      const res = await fetch('/api/gdrive/oauth/manual-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ code: app.oauthManualCode.trim() })
      });
      const data = await res.json();
      if (res.ok && data.success) {
        app.oauthConnected = true;
        app.oauthEmail = data.email || '';
        app.oauthManualCode = '';
        app.oauthMessage = `Connected successfully as ${data.email || 'Google User'}!`;
        await app.fetchOAuthStatus();
        setTimeout(() => { app.oauthMessage = ''; }, 5000);
      } else {
        app.oauthError = data.error || 'Failed to verify authorization code';
      }
    } catch (e) {
      app.oauthError = 'Network error: ' + e.message;
    } finally {
      app.isSubmittingManualCode = false;
    }
  }

  app.importRcloneToken = async function() {
    if (!app.rcloneTokenInput.trim()) return;
    app.oauthError = '';
    app.oauthMessage = '';
    app.isImportingRcloneToken = true;
    try {
      const res = await fetch('/api/gdrive/oauth/import-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: app.rcloneTokenInput.trim() })
      });
      const data = await res.json();
      if (res.ok && data.success) {
        app.oauthConnected = true;
        app.oauthEmail = data.email || '';
        app.rcloneTokenInput = '';
        app.oauthMessage = `Rclone token imported successfully! Connected as ${data.email || 'Google User'}.`;
        await app.fetchOAuthStatus();
        setTimeout(() => { app.oauthMessage = ''; }, 5000);
      } else {
        app.oauthError = data.error || 'Failed to import Rclone token';
      }
    } catch (e) {
      app.oauthError = 'Network error: ' + e.message;
    } finally {
      app.isImportingRcloneToken = false;
    }
  }

  app.disconnectOAuth = async function() {
    app.openConfirm('Disconnect Google Account', 'Disconnect Google Drive OAuth account? Automated quota bypass will no longer be active.', async () => {
      try {
        await fetch('/api/gdrive/oauth/disconnect', { method: 'POST' });
        app.oauthConnected = false;
        app.oauthEmail = '';
        app.oauthMessage = 'Google account disconnected.';
        setTimeout(() => { app.oauthMessage = ''; }, 3000);
      } catch (e) {
        console.error('Failed to disconnect OAuth:', e);
      }
    }, 'Disconnect', 'danger');
  }

  app.cleanupTempFolder = async function() {
    app.isCleaningTempFolder = true;
    app.oauthMessage = '';
    app.oauthError = '';
    try {
      const res = await fetch('/api/gdrive/oauth/cleanup-temp', { method: 'POST' });
      const data = await res.json();
      if (res.ok) {
        app.oauthMessage = `Cleaned up ${data.deleted_count || 0} temporary file(s) from ggdl_temp in your Google Drive.`;
        setTimeout(() => { app.oauthMessage = ''; }, 5000);
      } else {
        app.oauthError = data.error || 'Failed to clean temp folder';
      }
    } catch (e) {
      app.oauthError = 'Network error: ' + e.message;
    } finally {
      app.isCleaningTempFolder = false;
    }
  }

  app.checkAuthStatus = async function() {
    try {
      const res = await fetch('/api/auth/status');
      if (res.ok) {
        const data = await res.json();
        app.authEnabled = !!data.auth_enabled;
        app.authEnabledToggle = !!data.auth_enabled;
        app.isAuthenticated = !!data.authenticated;
        if (data.username) {
          app.currentAuthUser = data.username;
          app.changeNewUsername = data.username;
        }
      } else {
        app.isAuthenticated = false;
      }
    } catch (e) {
      console.warn('Failed to check auth status', e);
    } finally {
      app.authChecked = true;
      if (!app.authEnabled || app.isAuthenticated) {
        app.startAppServices();
      }
    }
  }

  app.submitLogin = async function() {
    app.loginError = '';
    if (!app.loginUsername.trim() || !app.loginPassword) {
      app.loginError = 'Please enter both username and password';
      return;
    }

    app.isLoggingIn = true;
    try {
      const res = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username: app.loginUsername.trim(),
          password: app.loginPassword
        })
      });

      if (res.ok) {
        const data = await res.json();
        app.isAuthenticated = true;
        app.currentAuthUser = data.username || app.loginUsername.trim();
        app.loginPassword = '';
        app.loginError = '';
        app.startAppServices();
      } else {
        const errData = await res.json().catch(() => ({ error: 'Login failed' }));
        app.loginError = errData.error || 'Invalid username or password';
      }
    } catch (e) {
      app.loginError = 'Connection error: ' + e.message;
    } finally {
      app.isLoggingIn = false;
    }
  }

  app.logoutWebUI = function() {
    app.openConfirm('Log Out Web UI', 'Are you sure you want to log out of the GDrive Downloader Web UI?', async () => {
      try {
        await fetch('/api/auth/logout', { method: 'POST' });
      } catch (e) {
        console.warn(e);
      }
      app.isAuthenticated = false;
      if (app.eventSource) {
        app.eventSource.close();
        app.eventSource = null;
      }
      if (app.pollInterval) {
        clearInterval(app.pollInterval);
        app.pollInterval = null;
      }
    }, 'Log Out', 'danger');
  }

  app.updateSecurityCredentials = async function() {
    app.securityError = '';
    app.securityMessage = '';

    if (app.changeNewPassword && app.changeNewPassword !== app.changeConfirmPassword) {
      app.securityError = 'New password and confirmation do not match.';
      return;
    }

    if (app.changeNewPassword && app.changeNewPassword.length < 4) {
      app.securityError = 'New password must be at least 4 characters long.';
      return;
    }

    app.isUpdatingSecurity = true;
    try {
      const res = await fetch('/api/auth/password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          old_password: app.changeOldPassword,
          new_username: app.changeNewUsername.trim(),
          new_password: app.changeNewPassword
        })
      });

      if (res.ok) {
        app.securityMessage = 'Credentials updated successfully!';
        app.changeOldPassword = '';
        app.changeNewPassword = '';
        app.changeConfirmPassword = '';
        if (app.changeNewUsername.trim()) {
          app.currentAuthUser = app.changeNewUsername.trim();
        }
      } else {
        const err = await res.json().catch(() => ({ error: 'Failed to update credentials' }));
        app.securityError = err.error || 'Failed to update credentials';
      }
    } catch (e) {
      app.securityError = 'Error: ' + e.message;
    } finally {
      app.isUpdatingSecurity = false;
    }
  }
}
