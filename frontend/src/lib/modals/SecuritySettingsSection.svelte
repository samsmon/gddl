<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

          <!-- Web UI Security Section (qBittorrent server style) -->
          <div class="form-group">
            <span class="form-title" style="font-weight: 600; color: var(--accent-blue);">Web UI Security (Server Access)</span>
            <span class="form-hint" style="margin-bottom: 0.75rem;">Protect your server deployment with login authentication, just like qBittorrent WebUI.</span>

            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-bottom: 0.75rem;">
              <input type="checkbox" bind:checked={app.authEnabledToggle} onchange={app.toggleAuthRequirement} style="width: auto;" />
              <span style="font-weight: 500;">Require username and password to access Web UI</span>
            </label>

            {#if app.authEnabledToggle}
              <div style="background: var(--table-row-alt); padding: 12px; border-radius: 6px; border: 1px solid var(--border-subtle); display: flex; flex-direction: column; gap: 8px;">
                <span style="font-weight: 600; font-size: 12px;">Change Admin Credentials:</span>

                {#if app.securityError}
                  <div style="color: var(--accent-red); font-size: 11px; background: rgba(248,81,73,0.1); padding: 4px 8px; border-radius: 4px;">{app.securityError}</div>
                {/if}
                {#if app.securityMessage}
                  <div style="color: var(--accent-green); font-size: 11px; background: rgba(46,160,67,0.1); padding: 4px 8px; border-radius: 4px;">{app.securityMessage}</div>
                {/if}

                <div style="display: grid; grid-template-columns: 130px 1fr; gap: 8px; align-items: center;">
                  <span style="font-size: 12px; color: var(--text-muted);">Username:</span>
                  <input type="text" bind:value={app.changeNewUsername} placeholder="admin" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">Current Password:</span>
                  <input type="password" bind:value={app.changeOldPassword} placeholder="••••••••" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">New Password:</span>
                  <input type="password" bind:value={app.changeNewPassword} placeholder="••••••••" style="padding: 4px 8px;" />

                  <span style="font-size: 12px; color: var(--text-muted);">Confirm Password:</span>
                  <input type="password" bind:value={app.changeConfirmPassword} placeholder="••••••••" style="padding: 4px 8px;" />
                </div>

                <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
                  <button class="btn btn-secondary" onclick={app.updateSecurityCredentials} disabled={app.isUpdatingSecurity} style="font-size: 11px; padding: 4px 12px;">
                    {app.isUpdatingSecurity ? 'Updating...' : 'Update Password'}
                  </button>
                </div>
              </div>
            {/if}
          </div>

          <!-- Divider -->
          <div style="border-top: 1px solid var(--border-color); margin: 1.25rem 0 1rem 0;"></div>

          <!-- Google Account & OAuth Quota Bypass Section -->
          <div class="form-group">
            <div style="display: flex; align-items: center; justify-content: space-between;">
              <span class="form-title" style="font-weight: 600; color: var(--accent-blue);">Google Drive &amp; Quota Bypass</span>
              {#if app.oauthConnected}
                <span class="concurrency-badge badge-safe">OAuth Connected</span>
              {:else if app.cookieList.length > 0}
                <span class="concurrency-badge badge-safe">{app.cookieList.filter(c => !c.is_exhausted).length} of {app.cookieList.length} Cookies Active</span>
              {:else}
                <span class="concurrency-badge badge-aggressive">Not Connected</span>
              {/if}
            </div>
            <span class="form-hint" style="margin-bottom: 0.5rem;">
              Bypass Google's daily "Download quota exceeded" limit automatically using OAuth 2.0 (`ggdl_temp`) or multi-account cookie failover.
            </span>

            <div style="display: flex; align-items: center; gap: 8px;">
              <button class="btn btn-secondary" onclick={() => { app.showSettingsModal = false; app.showLoginModal = true; }} style="display: flex; align-items: center; gap: 6px;">
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"></path>
                  <path d="M12 12v9"></path>
                  <path d="m8 17 4 4 4-4"></path>
                </svg>
                <span>Google Account &amp; OAuth Bypass Settings...</span>
              </button>
            </div>
          </div>
