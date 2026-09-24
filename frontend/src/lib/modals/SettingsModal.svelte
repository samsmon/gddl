<script>
  import { app } from '../state/appState.svelte.js';
  import SettingsGeneralTab from './SettingsGeneralTab.svelte';
  import SettingsWarpTab from './SettingsWarpTab.svelte';
  import SettingsCookiesTab from './SettingsCookiesTab.svelte';
  import SettingsSecurityTab from './SettingsSecurityTab.svelte';
</script>

<!-- Modal: Options / Settings with Sidebar Categories -->
{#if app.showSettingsModal}
  <div class="modal-overlay" role="presentation" onclick={() => app.showSettingsModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showSettingsModal = false)}>
    <div class="modal-window settings-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div style="display: flex; align-items: center; gap: 8px;">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="3"></circle>
            <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
          </svg>
          <span>Options &amp; Preferences</span>
        </div>
        <button class="modal-close" aria-label="Close" onclick={() => app.showSettingsModal = false}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>

      <div class="settings-body-layout">
        <!-- Internal Sidebar Navigation -->
        <nav class="settings-sidebar-nav" aria-label="Settings Categories">
          <button
            type="button"
            class="settings-nav-item"
            class:active={app.settingsTab === 'general'}
            onclick={() => app.settingsTab = 'general'}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
            </svg>
            <span>General</span>
          </button>

          <button
            type="button"
            class="settings-nav-item"
            class:active={app.settingsTab === 'warp'}
            onclick={() => app.settingsTab = 'warp'}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
            </svg>
            <span>WARP &amp; Proxy</span>
            {#if app.warpStatus.proxy_active}
              <span class="settings-nav-badge" style="background: var(--accent-green);">ON</span>
            {/if}
          </button>

          <button
            type="button"
            class="settings-nav-item"
            class:active={app.settingsTab === 'cookies'}
            onclick={() => app.settingsTab = 'cookies'}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"></path>
              <path d="M2 12h20"></path>
            </svg>
            <span>Google Cookies</span>
            {#if app.cookieList.length > 0}
              <span class="settings-nav-badge">{app.cookieList.length}</span>
            {/if}
          </button>

          <button
            type="button"
            class="settings-nav-item"
            class:active={app.settingsTab === 'security'}
            onclick={() => app.settingsTab = 'security'}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect>
              <path d="M7 11V7a5 5 0 0 1 10 0v4"></path>
            </svg>
            <span>Web Security</span>
            {#if app.authEnabled}
              <span class="settings-nav-badge" style="background: var(--accent-blue);">Lock</span>
            {/if}
          </button>
        </nav>

        <!-- Right Content Pane -->
        <div class="settings-pane-container">
          {#if app.settingsTab === 'general'}
            <SettingsGeneralTab />
          {:else if app.settingsTab === 'warp'}
            <SettingsWarpTab />
          {:else if app.settingsTab === 'cookies'}
            <SettingsCookiesTab />
          {:else if app.settingsTab === 'security'}
            <SettingsSecurityTab />
          {/if}
        </div>
      </div>

      <div class="modal-footer">
        {#if app.settingsTab === 'cookies' && app.cookieList.length > 0}
          <button class="btn btn-danger" onclick={app.logoutGoogle} style="margin-right: auto;">Clear All Accounts</button>
        {/if}
        <button class="btn btn-secondary" onclick={() => app.showSettingsModal = false}>Cancel</button>
        <button class="btn btn-primary" onclick={app.saveConfig}>Save Options</button>
      </div>
    </div>
  </div>
{/if}
