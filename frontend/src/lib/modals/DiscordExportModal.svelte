<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Export Unfinished Discord Downloads (AI Agent Scraper) -->
  {#if app.showDiscordExportModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showDiscordExportModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showDiscordExportModal = false)}>
      <div class="modal-window discord-export-modal" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
              <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994.021-.041.001-.09-.041-.106a13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.929 1.793 8.18 1.793 12.061 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.894.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.028zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
            </svg>
            <span style="font-weight: 700;">Export Unfinished Discord Downloads (AI Agent)</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showDiscordExportModal = false}>
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
              <strong>For AI Scraping Agent:</strong> Give this exported JSON or URL list to your scraper. Each entry contains <code>filename</code>, <code>channel_id</code>, <code>attachment_id</code>, <code>status</code>, and <code>error</code> so the agent can quickly look up fresh download links from Discord.
            </div>
          </div>

          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px;">
            <div class="logs-filter-chips">
              <button
                class="log-filter-chip"
                class:active={app.discordExportFormat === 'json'}
                onclick={() => app.discordExportFormat = 'json'}
              >
                JSON (Full AI Metadata)
              </button>
              <button
                class="log-filter-chip"
                class:active={app.discordExportFormat === 'urls'}
                onclick={() => app.discordExportFormat = 'urls'}
              >
                Plain URLs Only
              </button>
              <button
                class="log-filter-chip"
                class:active={app.discordExportFormat === 'detailed'}
                onclick={() => app.discordExportFormat = 'detailed'}
              >
                Text Summary
              </button>
            </div>
            
            <div style="font-size: 11px; color: var(--text-dim);">
              {app.unfinishedDiscordItems.length} unfinished file{app.unfinishedDiscordItems.length === 1 ? '' : 's'}
            </div>
          </div>

          {#if app.isFetchingDiscord}
            <div style="padding: 40px; text-align: center; color: var(--text-muted);">
              <div class="mini-spinner" style="margin: 0 auto 10px auto;"></div>
              <span>Scanning queue for unfinished Discord downloads...</span>
            </div>
          {:else if app.unfinishedDiscordItems.length === 0}
            <div class="log-empty" style="padding: 30px; text-align: center;">
              <span style="color: var(--accent-green, #3fb950); font-weight: 600;">✓ No unfinished Discord downloads found!</span>
              <p style="margin: 6px 0 0 0; font-size: 12px; color: var(--text-dim);">All Discord downloads are either completed or queue is empty.</p>
            </div>
          {:else}
            <textarea
              class="discord-export-textarea"
              readonly
              rows="12"
              value={app.formattedDiscordExportText}
              onclick={(e) => e.target.select()}
            ></textarea>
          {/if}
        </div>

        <div class="modal-footer" style="display: flex; justify-content: space-between; align-items: center;">
          <div style="display: flex; gap: 8px;">
            <button
              class="btn btn-secondary"
              onclick={() => { app.showDiscordExportModal = false; app.openDiscordRefreshModal(); }}
              title="Switch to Refresh Links modal"
            >
              Paste Fresh Links →
            </button>
          </div>
          <div style="display: flex; gap: 8px;">
            <button
              class="btn btn-secondary"
              disabled={app.unfinishedDiscordItems.length === 0}
              onclick={() => app.downloadDiscordExport(app.discordExportFormat)}
              title="Save as file on disk"
            >
              Download {app.discordExportFormat === 'json' ? '.json' : '.txt'}
            </button>
            <button
              class="btn btn-primary"
              disabled={app.unfinishedDiscordItems.length === 0}
              onclick={app.copyDiscordExport}
            >
              {app.discordExportCopied ? 'Copied to Clipboard!' : 'Copy to Clipboard'}
            </button>
            <button class="btn btn-secondary" onclick={() => app.showDiscordExportModal = false}>
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  {/if}

