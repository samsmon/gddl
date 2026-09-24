<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

{#if !app.authChecked}
  <div class="qb-login-overlay">
    <div style="color: var(--text-muted); font-size: 13px; display: flex; align-items: center; gap: 8px;">
      <span>Connecting to GDrive Client...</span>
    </div>
  </div>
{:else if app.authEnabled && !app.isAuthenticated}
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
        <button class="tb-btn" onclick={app.toggleTheme} title="Toggle Dark/Light Theme" style="padding: 4px 8px;">
          {#if app.currentTheme === 'dark'}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12 7c-2.76 0-5 2.24-5 5s2.24 5 5 5 5-2.24 5-5-2.24-5-5-5zM2 13h2c.55 0 1-.45 1-1s-.45-1-1-1H2c-.55 0-1 .45-1 1s.45 1 1 1zm18 0h2c.55 0 1-.45 1-1s-.45-1-1-1h-2c-.55 0-1 .45-1 1s.45 1 1 1zM11 2v2c0 .55.45 1 1 1s1-.45 1-1V2c0-.55-.45-1-1-1s-1 .45-1 1zm0 18v2c0 .55.45 1 1 1s1-.45 1-1v-2c0-.55-.45-1-1-1s-1 .45-1 1zM5.99 4.58c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41L5.99 4.58zm12.37 12.37c-.39-.39-1.03-.39-1.41 0s-.39 1.03 0 1.41l1.06 1.06c.39.39 1.03.39 1.41 0s.39-1.03 0-1.41l-1.06-1.06zm1.06-10.96c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06zM7.05 18.36c.39-.39.39-1.03 0-1.41s-1.03-.39-1.41 0l-1.06 1.06c-.39.39-.39 1.03 0 1.41s1.03.39 1.41 0l1.06-1.06z"/></svg>
          {:else}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M12.3 2a10 10 0 0 0-1.9 19.8 10 10 0 0 0 11.4-11.4A10 10 0 0 0 12.3 2z"/></svg>
          {/if}
        </button>
      </div>

      <form class="qb-login-form" onsubmit={(e) => { e.preventDefault(); app.submitLogin(); }}>
        {#if app.loginError}
          <div class="qb-login-error">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="8" x2="12" y2="12"></line>
              <line x1="12" y1="16" x2="12.01" y2="16"></line>
            </svg>
            <span>{app.loginError}</span>
          </div>
        {/if}

        <div class="qb-form-row">
          <label for="qb-user">Username:</label>
          <input
            id="qb-user"
            type="text"
            bind:value={app.loginUsername}
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
            bind:value={app.loginPassword}
            placeholder="••••••••"
            required
            autocomplete="current-password"
          />
        </div>

        <div class="qb-login-footer">
          <span class="qb-login-hint">Default: admin / adminadmin</span>
          <button type="submit" class="btn btn-primary qb-login-btn" disabled={app.isLoggingIn}>
            {app.isLoggingIn ? 'Logging in...' : 'Log In'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
