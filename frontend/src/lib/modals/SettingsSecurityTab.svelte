<script>
  import { app } from '../state/appState.svelte.js';
</script>

<div class="settings-pane-header">
  <div class="settings-pane-title">
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
    </svg>
    <span>Web UI Security &amp; Access</span>
  </div>
  {#if app.authEnabled}
    <span class="concurrency-badge badge-safe">Protected</span>
  {:else}
    <span class="concurrency-badge badge-aggressive">Open Access</span>
  {/if}
</div>

<div class="form-group">
  <span class="form-hint" style="margin-bottom: 0.75rem;">Protect your server deployment with login authentication, just like qBittorrent WebUI.</span>

  <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-bottom: 0.75rem;">
    <input type="checkbox" bind:checked={app.authEnabledToggle} onchange={app.toggleAuthRequirement} style="width: auto;" />
    <span style="font-weight: 500;">Require username and password to access Web UI</span>
  </label>

  {#if app.authEnabledToggle}
    <div style="background: var(--table-row-alt); padding: 14px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: 10px;">
      <span style="font-weight: 600; font-size: 12px; color: var(--text-main);">Change Admin Credentials:</span>

      {#if app.securityError}
        <div style="color: var(--accent-red); font-size: 11px; background: rgba(248,81,73,0.1); padding: 5px 8px; border-radius: 4px;">{app.securityError}</div>
      {/if}
      {#if app.securityMessage}
        <div style="color: var(--accent-green); font-size: 11px; background: rgba(46,160,67,0.1); padding: 5px 8px; border-radius: 4px;">{app.securityMessage}</div>
      {/if}

      <div style="display: grid; grid-template-columns: 130px 1fr; gap: 8px; align-items: center;">
        <span style="font-size: 12px; color: var(--text-muted);">Username:</span>
        <input type="text" bind:value={app.changeNewUsername} placeholder="admin" style="padding: 5px 8px; font-size: 12px;" />

        <span style="font-size: 12px; color: var(--text-muted);">Current Password:</span>
        <input type="password" bind:value={app.changeOldPassword} placeholder="••••••••" style="padding: 5px 8px; font-size: 12px;" />

        <span style="font-size: 12px; color: var(--text-muted);">New Password:</span>
        <input type="password" bind:value={app.changeNewPassword} placeholder="••••••••" style="padding: 5px 8px; font-size: 12px;" />

        <span style="font-size: 12px; color: var(--text-muted);">Confirm Password:</span>
        <input type="password" bind:value={app.changeConfirmPassword} placeholder="••••••••" style="padding: 5px 8px; font-size: 12px;" />
      </div>

      <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
        <button class="btn btn-secondary" onclick={app.updateSecurityCredentials} disabled={app.isUpdatingSecurity} style="font-size: 11px; padding: 5px 14px;">
          {app.isUpdatingSecurity ? 'Updating...' : 'Update Password'}
        </button>
      </div>
    </div>
  {/if}
</div>
