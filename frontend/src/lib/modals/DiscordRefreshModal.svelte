<script>
  import { app } from '../state/appState.svelte.js';
</script>

{#if app.showDiscordRefreshModal}
  <div class="modal-overlay" role="presentation" onclick={() => app.showDiscordRefreshModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showDiscordRefreshModal = false)}>
    <div class="modal-window discord-refresh-modal" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div style="display: flex; align-items: center; gap: 8px;">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
            <path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>
          </svg>
          <span style="font-weight: 700;">Refresh Expired Discord Links</span>
        </div>
        <button class="modal-close" aria-label="Close" onclick={() => app.showDiscordRefreshModal = false}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>

      <div class="modal-body discord-modal-body">
        <div class="discord-info-banner">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0; margin-top: 1px;">
            <circle cx="12" cy="12" r="10"></circle>
            <line x1="12" y1="16" x2="12" y2="12"></line>
            <line x1="12" y1="8" x2="12.01" y2="8"></line>
          </svg>
          <div>
            <strong>Auto-Matching &amp; Resume:</strong> Paste fresh Discord URLs or the output JSON from your scraper. GDDL matches items by <strong>Attachment ID</strong> or <strong>Filename</strong>, updates the URL, resets status to queued, and resumes seamlessly from existing <code>.part</code> files.
          </div>
        </div>

        <label class="form-group" style="margin-bottom: 8px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px;">
            <span class="form-title" style="margin-bottom: 0;">Paste fresh URLs or AI Scraper JSON:</span>
            <button
              type="button"
              class="btn-subtle"
              style="font-size: 11px; color: var(--accent-blue); padding: 0; text-decoration: underline; background: transparent; border: none; cursor: pointer;"
              onclick={() => { app.showDiscordRefreshModal = false; app.openDiscordExportModal(); }}
            >
              Export unfinished list first
            </button>
          </div>
          <textarea
            class="discord-refresh-textarea"
            rows="9"
            bind:value={app.discordRefreshInput}
            placeholder={`Paste refreshed Discord URLs (one per line) or JSON array:
https://cdn.discordapp.com/attachments/141.../154.../song.rar?ex=66f...&is=...
https://cdn.discordapp.com/attachments/141.../155.../video.mp4?ex=66f...

Or paste AI scraper JSON array:
[{"url": "https://cdn.discordapp.com/..."}]`}
          ></textarea>
        </label>

        {#if app.discordRefreshResult}
          <div class={app.discordRefreshResult.success ? 'discord-alert-success' : 'discord-alert-error'}>
            <div style="display: flex; align-items: center; gap: 8px;">
              {#if app.discordRefreshResult.success}
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
              {:else}
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                  <circle cx="12" cy="12" r="10"></circle>
                  <line x1="12" y1="8" x2="12" y2="12"></line>
                  <line x1="12" y1="16" x2="12.01" y2="16"></line>
                </svg>
              {/if}
              <span>{app.discordRefreshResult.message}</span>
            </div>
            {#if app.discordRefreshResult.success && app.discordRefreshResult.not_found > 0}
              <div style="font-size: 11px; margin-top: 4px; opacity: 0.85;">
                Note: {app.discordRefreshResult.not_found} URL(s) did not match any unfinished items currently in the queue.
              </div>
            {/if}
          </div>
        {/if}
      </div>

      <div class="modal-footer" style="display: flex; justify-content: flex-end; gap: 8px;">
        <button class="btn btn-secondary" onclick={() => app.showDiscordRefreshModal = false}>
          Cancel
        </button>
        <button
          class="btn btn-primary"
          disabled={!app.discordRefreshInput.trim() || app.isRefreshingDiscord}
          onclick={app.submitRefreshDiscordURLs}
        >
          {#if app.isRefreshingDiscord}
            <div class="mini-spinner" style="display: inline-block; vertical-align: middle; margin-right: 6px; width: 12px; height: 12px;"></div>
            <span>Updating Queue...</span>
          {:else}
            <span>Update &amp; Resume Downloads</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
