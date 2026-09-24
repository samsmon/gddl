<script>
  import { app } from '../state/appState.svelte.js';
  import CookiePoolSection from './CookiePoolSection.svelte';
</script>

<div class="settings-pane-header">
  <div class="settings-pane-title">
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <path d="M12 2a10 10 0 1 0 10 10 4 4 0 0 1-5-5 4 4 0 0 1-5-5"></path>
      <path d="M8.5 8.5v.01"></path>
      <path d="M16 15.5v.01"></path>
      <path d="M12 12v.01"></path>
      <path d="M11 17v.01"></path>
      <path d="M7 13v.01"></path>
    </svg>
    <span>Google Cookies &amp; Account Quota</span>
  </div>
  {#if app.cookieList.length > 0}
    <span class="concurrency-badge badge-safe">{app.cookieList.filter(c => !c.is_exhausted).length} / {app.cookieList.length} Active</span>
  {:else}
    <span class="concurrency-badge badge-aggressive">No Accounts</span>
  {/if}
</div>

<!-- OAuth Quick Card -->
<div style="background: rgba(56, 139, 253, 0.08); border: 1px solid rgba(56, 139, 253, 0.25); border-radius: 6px; padding: 10px 12px; display: flex; align-items: center; justify-content: space-between; gap: 10px;">
  <div style="display: flex; flex-direction: column; gap: 2px;">
    <div style="display: flex; align-items: center; gap: 6px;">
      <span style="font-weight: 600; font-size: 11.5px; color: var(--accent-blue);">Google OAuth 2.0 Integration (ggdl_temp)</span>
      {#if app.oauthConnected}
        <span class="concurrency-badge badge-safe" style="font-size: 9px; padding: 1px 5px;">Connected</span>
      {/if}
    </div>
    <span style="font-size: 10.5px; color: var(--text-muted); line-height: 1.35;">
      {app.oauthConnected ? `Signed in as ${app.oauthEmail || 'Google User'}. Automated quota bypass is active.` : 'Connect via Rclone or Cloud Console for zero-setup automatic 24-hour quota bypass.'}
    </span>
  </div>
  <button
    class="btn btn-secondary"
    style="font-size: 11px; padding: 4px 10px; white-space: nowrap; flex-shrink: 0;"
    onclick={() => { app.showSettingsModal = false; app.showLoginModal = true; app.accountModalTab = 'oauth'; }}
  >
    {app.oauthConnected ? 'Manage OAuth...' : 'Open OAuth Setup...'}
  </button>
</div>

<!-- Embed Cookie Pool Section -->
<CookiePoolSection />
