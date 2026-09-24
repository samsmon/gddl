<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
  import CookiePoolSection from './CookiePoolSection.svelte';
</script>

  <!-- Modal: Google Account Manager (OAuth 2.0 & Cookie Pool) -->
  {#if app.showLoginModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showLoginModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showLoginModal = false)}>
      <div class="modal-window" role="dialog" aria-modal="true" tabindex="-1" style="max-width: 620px;" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>Google Drive Integration & Auto-Bypass</span>
          <button class="modal-close" aria-label="Close" onclick={() => app.showLoginModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <!-- Navigation Tabs -->
          <div style="display: flex; gap: 8px; border-bottom: 1px solid var(--border-color); margin-bottom: 14px; padding-bottom: 4px;">
            <button
              class="btn-subtle"
              style="padding: 6px 14px; font-size: 12px; font-weight: 600; border-radius: 4px; display: flex; align-items: center; gap: 6px; {app.accountModalTab === 'oauth' ? 'background: var(--accent-blue); color: white;' : 'background: transparent; color: var(--text-muted);'}"
              onclick={() => app.accountModalTab = 'oauth'}
            >
              <span>Google OAuth 2.0 (Auto-Bypass)</span>
              <span style="font-size: 9px; padding: 1px 5px; border-radius: 3px; {app.accountModalTab === 'oauth' ? 'background: rgba(255,255,255,0.25); color: white;' : 'background: rgba(46,160,67,0.2); color: var(--accent-green);'}">Recommended</span>
            </button>
            <button
              class="btn-subtle"
              style="padding: 6px 14px; font-size: 12px; font-weight: 600; border-radius: 4px; display: flex; align-items: center; gap: 6px; {app.accountModalTab === 'cookies' ? 'background: var(--accent-blue); color: white;' : 'background: transparent; color: var(--text-muted);'}"
              onclick={() => app.accountModalTab = 'cookies'}
            >
              <span>Cookie Pool (Legacy)</span>
              {#if app.cookieList.length > 0}
                <span style="font-size: 9px; padding: 1px 5px; border-radius: 3px; background: rgba(255,255,255,0.2); color: white;">{app.cookieList.length}</span>
              {/if}
            </button>
          </div>

          <!-- TAB 1: GOOGLE OAUTH 2.0 -->
          {#if app.accountModalTab === 'oauth'}
            <!-- Status Banner -->
            <div class="auth-status-banner" style="display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 12px;">
              <div style="display: flex; align-items: center; gap: 8px; min-width: 0;">
                <span class="status-indicator" class:active-dot={app.oauthConnected} class:failed-dot={!app.oauthConnected}></span>
                <span style="text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                  Status: <strong>{app.oauthConnected ? `Connected (${app.oauthEmail || 'Ready'})` : 'Not Connected'}</strong>
                </span>
              </div>
              {#if app.oauthConnected}
                <div style="display: flex; gap: 6px; flex-shrink: 0;">
                  <button class="btn-mini btn-action" disabled={app.isCleaningTempFolder} onclick={app.cleanupTempFolder} title="Purge all temporary files from ggdl_temp in your Google Drive">
                    {app.isCleaningTempFolder ? 'Cleaning...' : 'Clean ggdl_temp'}
                  </button>
                  <button class="btn-mini btn-action" onclick={app.disconnectOAuth} style="color: var(--accent-red);" title="Disconnect Google Account">
                    Disconnect
                  </button>
                </div>
              {/if}
            </div>

            {#if app.oauthMessage}
              <div style="color: var(--accent-green); font-size: 11px; background: rgba(46,160,67,0.12); border: 1px solid rgba(46,160,67,0.3); padding: 7px 10px; border-radius: 5px; margin-bottom: 10px;">
                {app.oauthMessage}
              </div>
            {/if}
            {#if app.oauthError}
              <div style="color: var(--accent-red); font-size: 11px; background: rgba(248,81,73,0.12); border: 1px solid rgba(248,81,73,0.3); padding: 7px 10px; border-radius: 5px; margin-bottom: 10px;">
                {app.oauthError}
              </div>
            {/if}

            <!-- METHOD 1: QUICK CONNECT VIA RCLONE TOKEN (ACEFILE STYLE) -->
            <div style="background: var(--input-bg); border: 1px solid var(--border-color); border-radius: 6px; padding: 14px; display: flex; flex-direction: column; gap: 10px; margin-bottom: 12px;">
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <span style="font-weight: 600; font-size: 12px; color: var(--accent-green);">Option 1: Connect via Rclone Token (Acefile Style)</span>
                <span style="font-size: 10px; color: var(--accent-green); background: rgba(16, 185, 129, 0.12); padding: 2px 6px; border-radius: 3px; font-weight: 600;">Recommended - No Cloud Console Needed</span>
              </div>
              <p style="font-size: 11px; color: var(--text-muted); line-height: 1.45; margin: 0;">
                Connect Google Drive instantly without creating your own Google Cloud Project or dealing with Test Users. Run this command on your computer terminal:
              </p>
              <div style="background: rgba(0, 0, 0, 0.35); border: 1px solid var(--border-subtle); border-radius: 4px; padding: 6px 10px; display: flex; align-items: center; justify-content: space-between;">
                <code style="font-family: var(--font-mono); font-size: 11px; color: #38bdf8;">rclone authorize "drive"</code>
                <button
                  class="btn btn-secondary"
                  style="padding: 2px 8px; font-size: 10px;"
                  onclick={() => app.copyRedirectURIToClipboard('rclone authorize "drive"')}
                >
                  Copy Command
                </button>
              </div>
              <p style="font-size: 10.5px; color: var(--text-dim); line-height: 1.4; margin: 0;">
                Log in when your browser opens, then copy and paste the generated JSON token (or the token line from your <code>rclone.conf</code>) below:
              </p>
              <div style="display: flex; flex-direction: column; gap: 6px;">
                <textarea
                  bind:value={app.rcloneTokenInput}
                  rows="3"
                  placeholder={`Paste token JSON here, e.g.: {"access_token":"ya29...","token_type":"Bearer","refresh_token":"1//0g...","expiry":"..."}`}
                  style="padding: 6px 8px; font-size: 11px; font-family: var(--font-mono); resize: vertical; width: 100%; box-sizing: border-box;"
                ></textarea>
                <div style="display: flex; justify-content: flex-end;">
                  <button
                    class="btn btn-primary"
                    disabled={!app.rcloneTokenInput.trim() || app.isImportingRcloneToken}
                    onclick={app.importRcloneToken}
                    style="font-size: 11px; padding: 5px 16px; background: #10b981;"
                  >
                    {app.isImportingRcloneToken ? 'Importing Token...' : 'Import Rclone Token'}
                  </button>
                </div>
              </div>
            </div>

            <!-- METHOD 2: CUSTOM GOOGLE CLOUD OAUTH 2.0 APP -->
            <div style="background: rgba(16, 185, 129, 0.08); border: 1px solid rgba(16, 185, 129, 0.25); border-radius: 6px; padding: 10px 12px; margin-bottom: 12px;">
              <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px;">
                <span style="font-size: 11px; font-weight: 600; color: #10b981;">Authorized Redirect URI (Required in Google Cloud Console):</span>
                <button
                  class="btn btn-secondary"
                  onclick={() => app.copyRedirectURIToClipboard(app.oauthRedirectURI || `http://localhost:${window.location.port || '8099'}/api/gdrive/oauth/callback`)}
                  style="padding: 2px 8px; font-size: 10px; background: rgba(16, 185, 129, 0.15); color: #10b981; border-color: rgba(16, 185, 129, 0.3);"
                >
                  {app.copiedRedirectURI ? '✓ Copied!' : 'Copy Redirect URI'}
                </button>
              </div>
              <code style="font-size: 11px; color: #38bdf8; word-break: break-all; font-family: var(--font-mono); background: rgba(0,0,0,0.3); padding: 3px 6px; border-radius: 4px; display: block; margin-top: 4px;">
                {app.oauthRedirectURI || `http://localhost:${window.location.port || '8099'}/api/gdrive/oauth/callback`}
              </code>
              <span style="font-size: 9.5px; color: var(--text-dim); display: block; margin-top: 5px; line-height: 1.35;">
                Note: Google rejects private/LAN IPs (e.g. 192.168.x.x) with Error 400. GDDL automatically routes OAuth callbacks through localhost.
              </span>
            </div>

            <div style="background: var(--input-bg); border: 1px solid var(--border-color); border-radius: 6px; padding: 14px; display: flex; flex-direction: column; gap: 10px; margin-bottom: 12px;">
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <span style="font-weight: 600; font-size: 12px; color: var(--accent-blue);">Option 2: Custom Google Cloud OAuth 2.0 App</span>
                <span style="font-size: 10px; color: var(--text-dim); background: rgba(255,255,255,0.06); padding: 2px 6px; border-radius: 3px;">Full Drive Access</span>
              </div>

              <div style="display: grid; grid-template-columns: 95px 1fr; gap: 8px; align-items: center;">
                <span style="font-size: 11px; color: var(--text-muted);">Client ID:</span>
                <input
                  type="text"
                  bind:value={app.oauthClientID}
                  placeholder="e.g. 123456789-abc.apps.googleusercontent.com"
                  style="padding: 5px 8px; font-size: 11px; font-family: var(--font-mono);"
                />

                <span style="font-size: 11px; color: var(--text-muted);">Client Secret:</span>
                <div style="display: flex; gap: 4px;">
                  <input
                    type={app.showClientSecret ? 'text' : 'password'}
                    bind:value={app.oauthClientSecret}
                    placeholder="e.g. GOCSPX-..."
                    style="padding: 5px 8px; font-size: 11px; font-family: var(--font-mono); flex: 1;"
                  />
                  <button class="btn btn-secondary" onclick={() => app.showClientSecret = !app.showClientSecret} style="padding: 4px 8px; font-size: 10px;">
                    {app.showClientSecret ? 'Hide' : 'Show'}
                  </button>
                </div>
              </div>

              <details style="font-size: 10.5px; color: var(--text-muted); margin-top: 2px;">
                <summary style="cursor: pointer; color: var(--text-dim); font-size: 10px;">Advanced Settings: Custom Redirect URI Override (Optional)</summary>
                <div style="margin-top: 6px; display: flex; flex-direction: column; gap: 4px;">
                  <input
                    type="text"
                    bind:value={app.oauthRedirectURIOverride}
                    placeholder="Leave blank for default (auto localhost:[port])"
                    style="padding: 5px 8px; font-size: 11px; font-family: var(--font-mono);"
                  />
                  <span style="font-size: 9.5px; color: var(--text-dim);">Only fill this if using your own public HTTPS domain, Cloudflare Tunnel, or a custom reverse proxy.</span>
                </div>
              </details>

              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; margin-top: 4px; font-size: 11px; color: var(--text-main);">
                <input type="checkbox" bind:checked={app.oauthAutoBypass} style="accent-color: var(--accent-blue);" />
                <span>Automatically bypass Google Drive 24-hour quota limits (Make a copy to <code>ggdl_temp</code> &amp; auto-delete)</span>
              </label>

              <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px;">
                <button class="btn btn-secondary" disabled={app.isSavingOAuthConfig} onclick={app.saveOAuthConfig} style="font-size: 11px; padding: 5px 12px;">
                  {app.isSavingOAuthConfig ? 'Saving...' : 'Save Credentials'}
                </button>
                <button
                  class="btn btn-primary"
                  disabled={!app.oauthClientID.trim()}
                  onclick={app.startGoogleOAuth}
                  style="font-size: 11px; padding: 5px 16px; background: {app.oauthConnected ? 'var(--accent-blue)' : '#10b981'};"
                >
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px;">
                    <path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"></path>
                    <polyline points="10 17 15 12 10 7"></polyline>
                    <line x1="15" y1="12" x2="3" y2="12"></line>
                  </svg>
                  {app.oauthConnected ? 'Re-authenticate Account' : 'Connect Google Drive'}
                </button>
              </div>

              <!-- Quick Setup Help Accordion -->
              <details style="font-size: 10.5px; color: var(--text-muted); border-top: 1px dashed var(--border-subtle); padding-top: 6px; margin-top: 2px;">
                <summary style="cursor: pointer; color: var(--accent-blue); font-weight: 500;">1-Minute Quick Setup Guide: How to Create Free Client ID &amp; Secret</summary>
                <ol style="margin: 6px 0 0 0; padding-left: 18px; line-height: 1.5; color: var(--text-dim);">
                  <li>Open <a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noopener noreferrer" style="color: var(--accent-blue);">Google Cloud Console</a> &gt; create a new project if you haven't already.</li>
                  <li>Under <strong>Enabled APIs &amp; Services</strong>, search for and enable <strong>Google Drive API</strong>.</li>
                  <li>Under <strong>OAuth consent screen</strong>, select <em>External</em> &gt; enter app name (e.g. <em>GDDL</em>) &gt; add your email under Test Users.
                    <br/><strong style="color: #10b981;">Tip:</strong> Click "Publish App" to set Publishing status to <em>In production</em> so any Google account can log in without manual Test User registration!
                  </li>
                  <li>Under <strong>Credentials</strong> &gt; <em>Create Credentials</em> &gt; select <strong>OAuth client ID</strong> &gt; Application type: <strong>Web application</strong>.</li>
                  <li>Under <strong>Authorized redirect URIs</strong>, add:<br/><code style="color: var(--accent-green); background: rgba(0,0,0,0.3); padding: 1px 4px; border-radius: 3px;">{app.oauthRedirectURI || `http://localhost:${window.location.port || '8099'}/api/gdrive/oauth/callback`}</code></li>
                  <li>Copy the Client ID and Client Secret into the form above, then click <strong>Connect Google Drive</strong>.</li>
                </ol>
              </details>
            </div>

            <!-- Manual Callback / Remote Host Fallback Box -->
            <div style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 6px; padding: 12px; display: flex; flex-direction: column; gap: 8px;">
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <span style="font-weight: 600; font-size: 11px; color: var(--text-main);">Manual Authorization / Remote Host (Callback URL Fallback)</span>
                <span style="font-size: 9.5px; color: var(--text-dim);">Fallback Input</span>
              </div>
              <span style="font-size: 10px; color: var(--text-muted); line-height: 1.4;">
                If Google login succeeded in your browser or you are accessing GDDL from another device on your LAN (e.g. phone/tablet), copy the URL from your browser address bar after approving login and paste below:
              </span>
              <div style="display: flex; gap: 6px;">
                <input
                  type="text"
                  bind:value={app.oauthManualCode}
                  placeholder={`Paste full callback URL e.g. ${app.oauthRedirectURI || 'http://localhost:8099/api/gdrive/oauth/callback'}?code=... or authorization code 4/0A...`}
                  style="flex: 1; padding: 5px 8px; font-size: 11px; font-family: var(--font-mono);"
                />
                <button
                  class="btn btn-secondary"
                  disabled={!app.oauthManualCode.trim() || app.isSubmittingManualCode}
                  onclick={app.submitManualCode}
                  style="font-size: 11px; padding: 5px 12px;"
                >
                  {app.isSubmittingManualCode ? 'Verifying...' : 'Submit Code'}
                </button>
              </div>
            </div>

          <!-- TAB 2: COOKIE POOL (LEGACY) -->
          {:else if app.accountModalTab === 'cookies'}
            <CookiePoolSection />
          {/if}
        </div>

        <div class="modal-footer">
          {#if app.accountModalTab === 'cookies' && app.cookieList.length > 0}
            <button class="btn btn-danger" onclick={app.logoutGoogle} style="margin-right: auto;">Clear All Accounts</button>
          {/if}
          <button class="btn btn-secondary" onclick={() => app.showLoginModal = false}>Close</button>
        </div>
      </div>
    </div>
  {/if}
