<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

            <div class="auth-status-banner" style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <span class="status-indicator" class:active-dot={app.cookieList.length > 0} class:failed-dot={app.cookieList.length === 0}></span>
                <span>Pool Status: <strong>{app.cookieList.length > 0 ? `${app.cookieList.filter(c => !c.is_exhausted).length} of ${app.cookieList.length} Accounts Ready` : 'No accounts configured'}</strong></span>
              </div>
              {#if app.cookieList.some(c => c.is_exhausted)}
                <button class="btn-mini btn-action" onclick={app.resetAllCookieCooldowns} title="Reset all exhausted accounts back to active">
                  Reset All Cooldowns
                </button>
              {/if}
            </div>

            <span class="form-hint" style="margin-top: 0.5rem; margin-bottom: 0.75rem;">
              Legacy cookie pool method: Add Google Drive cookies manually. If an account hits Google's 24-hour quota limit, GDDL automatically switches to the next account.
            </span>

            {#if app.cookieError}
              <div style="color: var(--accent-red); font-size: 11px; background: rgba(248,81,73,0.1); padding: 5px 8px; border-radius: 4px; margin-bottom: 8px;">{app.cookieError}</div>
            {/if}
            {#if app.cookieMessage}
              <div style="color: var(--accent-green); font-size: 11px; background: rgba(46,160,67,0.1); padding: 5px 8px; border-radius: 4px; margin-bottom: 8px;">{app.cookieMessage}</div>
            {/if}

            <!-- Existing Accounts List -->
            {#if app.cookieList.length > 0}
              <div class="cookie-pool-container" style="display: flex; flex-direction: column; gap: 6px; margin-bottom: 12px; max-height: 180px; overflow-y: auto;">
                {#each app.cookieList as c}
                  <div style="background: var(--table-row-alt); border: 1px solid var(--border-subtle); padding: 8px 10px; border-radius: 5px; display: flex; align-items: center; justify-content: space-between; gap: 8px;">
                    <div style="display: flex; flex-direction: column; gap: 2px; min-width: 0;">
                      <div style="display: flex; align-items: center; gap: 6px;">
                        <span style="font-weight: 600; font-size: 12px; color: var(--text-main);">{c.label}</span>
                        {#if c.is_exhausted}
                          <span class="concurrency-badge badge-aggressive" style="font-size: 9px; padding: 1px 5px;">Cooldown ({c.cooldown_left})</span>
                        {:else}
                          <span class="concurrency-badge badge-safe" style="font-size: 9px; padding: 1px 5px;">Active</span>
                        {/if}
                      </div>
                      <span style="font-family: var(--font-mono); font-size: 10px; color: var(--text-dim); text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">{c.masked_cookie}</span>
                    </div>

                    <div style="display: flex; gap: 5px; flex-shrink: 0;">
                      {#if c.is_exhausted}
                        <button class="btn btn-secondary" onclick={() => app.resetCookieCooldown(c.id)} style="font-size: 10px; padding: 2px 7px;">
                          Reset
                        </button>
                      {/if}
                      <button class="btn btn-secondary" onclick={() => app.removeCookie(c.id)} style="font-size: 10px; padding: 2px 7px; color: var(--accent-red);">
                        Remove
                      </button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}

            <!-- Add Account Form -->
            <div style="background: var(--input-bg); border: 1px solid var(--border-color); border-radius: 6px; padding: 12px; display: flex; flex-direction: column; gap: 10px;">
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <span style="font-weight: 600; font-size: 12px; color: var(--accent-blue);">+ Add Google Account Cookie</span>
                <span style="font-size: 10px; color: var(--text-dim); background: rgba(255,255,255,0.06); padding: 2px 6px; border-radius: 3px;">Auto-extracts from cURL</span>
              </div>

              {#if app.wasCookieExtracted}
                <div style="font-size: 11px; color: var(--accent-green); background: rgba(46,160,67,0.15); border: 1px solid rgba(46,160,67,0.3); border-radius: 4px; padding: 5px 8px; display: flex; align-items: center; gap: 6px;">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                    <polyline points="20 6 9 17 4 12"></polyline>
                  </svg>
                  <span>Successfully extracted cookie from cURL / Request Headers!</span>
                </div>
              {/if}

              <div style="display: grid; grid-template-columns: 100px 1fr; gap: 8px; align-items: flex-start;">
                <span style="font-size: 11px; color: var(--text-muted); padding-top: 5px;">Account Label:</span>
                <input type="text" bind:value={app.newCookieLabel} placeholder="e.g. Primary Account, Secondary Account..." style="padding: 4px 8px; font-size: 12px;" />

                <span style="font-size: 11px; color: var(--text-muted); padding-top: 5px;">Cookie String:</span>
                <div style="display: flex; flex-direction: column; gap: 6px;">
                  <textarea
                    bind:value={app.newCookieValue}
                    oninput={(e) => app.handleCookieInput(e.target.value)}
                    onpaste={(e) => setTimeout(() => app.handleCookieInput(app.newCookieValue), 15)}
                    rows="3"
                    placeholder="Paste Cookie string or raw cURL from DevTools (F12 > Network > Right click > Copy as cURL)"
                    style="padding: 6px 8px; font-size: 11px; font-family: var(--font-mono); resize: vertical; width: 100%; box-sizing: border-box;"
                  ></textarea>

                  {#if app.cookieValidation}
                    <div style="padding: 6px 8px; border-radius: 4px; font-size: 11px; background: {app.cookieValidation.valid ? 'rgba(46,160,67,0.12)' : 'rgba(210,153,34,0.14)'}; border: 1px solid {app.cookieValidation.valid ? 'rgba(46,160,67,0.3)' : 'rgba(210,153,34,0.35)'};">
                      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 2px; flex-wrap: wrap; gap: 4px;">
                        <span style="font-weight: 600; color: {app.cookieValidation.valid ? 'var(--accent-green)' : '#e3b341'};">
                          {app.cookieValidation.valid ? 'All 3 Core Authentication Cookies Detected' : `Incomplete Cookie (${app.cookieValidation.count}/3 Core Keys Detected)`}
                        </span>
                        <span style="font-family: var(--font-mono); font-size: 10px;">
                          <span style="color: {app.cookieValidation.hasSID ? 'var(--accent-green)' : 'var(--text-dim)'}; font-weight: {app.cookieValidation.hasSID ? 'bold' : 'normal'};">SID {app.cookieValidation.hasSID ? 'OK' : 'MISSING'}</span> |
                          <span style="color: {app.cookieValidation.hasHSID ? 'var(--accent-green)' : 'var(--text-dim)'}; font-weight: {app.cookieValidation.hasHSID ? 'bold' : 'normal'};">HSID {app.cookieValidation.hasHSID ? 'OK' : 'MISSING'}</span> |
                          <span style="color: {app.cookieValidation.hasSSID ? 'var(--accent-green)' : 'var(--text-dim)'}; font-weight: {app.cookieValidation.hasSSID ? 'bold' : 'normal'};">SSID {app.cookieValidation.hasSSID ? 'OK' : 'MISSING'}</span>
                        </span>
                      </div>
                    </div>
                  {/if}
                </div>
              </div>

              <div style="display: flex; justify-content: space-between; align-items: center; margin-top: 4px;">
                <span style="font-size: 10px; color: var(--text-dim);">Supported formats: cURL (bash/cmd), raw headers, JSON</span>
                <button class="btn btn-primary" disabled={!app.newCookieValue.trim() || app.isAddingCookie} onclick={app.addCookieToPool} style="font-size: 11px; padding: 4px 12px;">
                  {app.isAddingCookie ? 'Adding...' : 'Add Account to Pool'}
                </button>
              </div>
            </div>
