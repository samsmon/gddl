<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: System & Execution Logs -->
  {#if app.showLogsModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showLogsModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showLogsModal = false)}>
      <div class="modal-window logs-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
            <span style="font-weight: 700;">System Execution & Error Logs</span>
            {#if app.errorLogsCount > 0}
              <span class="log-badge-error">{app.errorLogsCount} error{app.errorLogsCount > 1 ? 's' : ''}</span>
            {/if}
          </div>
          <div class="logs-header-actions">
            <button class="btn-mini btn-secondary" onclick={app.copyLogsToClipboard} title="Copy all logs to clipboard">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px; vertical-align: middle;">
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
              </svg>
              {app.copiedLogs ? 'Copied!' : 'Copy'}
            </button>
            <button class="btn-mini btn-secondary" onclick={app.clearLogs} title="Clear log buffer">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 4px; vertical-align: middle;">
                <polyline points="3 6 5 6 21 6"></polyline>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
              </svg>
              Clear
            </button>
            <button class="btn-mini btn-action" onclick={app.fetchLogs} disabled={app.isFetchingLogs} title="Refresh logs">
              <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="margin-right: 4px; vertical-align: middle;">
                <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
              </svg>
              {app.isFetchingLogs ? 'Refreshing...' : 'Refresh'}
            </button>
            <button class="modal-close" aria-label="Close" onclick={() => app.showLogsModal = false}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>

        <div class="logs-filter-bar">
          <div class="logs-filter-chips">
            <button
              class="log-filter-chip"
              class:active={app.logFilter === 'ALL'}
              onclick={() => app.logFilter = 'ALL'}
            >
              All ({app.logs.length})
            </button>
            <button
              class="log-filter-chip chip-error"
              class:active={app.logFilter === 'ERROR'}
              onclick={() => app.logFilter = 'ERROR'}
            >
              Errors ({app.logs.filter(l => l.level === 'ERROR').length})
            </button>
            <button
              class="log-filter-chip chip-warn"
              class:active={app.logFilter === 'WARN'}
              onclick={() => app.logFilter = 'WARN'}
            >
              Warnings ({app.logs.filter(l => l.level === 'WARN').length})
            </button>
            <button
              class="log-filter-chip chip-info"
              class:active={app.logFilter === 'INFO'}
              onclick={() => app.logFilter = 'INFO'}
            >
              Info ({app.logs.filter(l => l.level === 'INFO').length})
            </button>
            <button
              class="log-filter-chip chip-success"
              class:active={app.logFilter === 'SUCCESS'}
              onclick={() => app.logFilter = 'SUCCESS'}
            >
              Success ({app.logs.filter(l => l.level === 'SUCCESS').length})
            </button>
          </div>

          <input
            type="text"
            class="logs-search-input"
            placeholder="Filter logs by keyword..."
            bind:value={app.logSearch}
          />
        </div>

        <div class="modal-body logs-modal-body">
          {#if app.filteredLogs.length === 0}
            <div class="log-empty">
              <span>No logs found matching filter criteria.</span>
            </div>
          {:else}
            <div class="logs-container">
              {#each app.filteredLogs as log (log.id)}
                <div class="log-row log-row-{log.level}">
                  <span class="log-time">{log.timestamp}</span>
                  <span class="log-level-pill log-level-{log.level}">{log.level}</span>
                  <span class="log-category">[{log.category}]</span>
                  <div class="log-msg-col">
                    <span class="log-msg">{log.message}</span>
                    {#if log.details}
                      <div class="log-details">{log.details}</div>
                    {/if}
                  </div>
                  <button
                    class="btn-copy-log-row"
                    class:copied={app.copiedRowId === log.id}
                    onclick={() => app.copyLogRow(log)}
                    title={app.copiedRowId === log.id ? 'Copied to clipboard' : 'Copy log line'}
                    aria-label="Copy log line"
                  >
                    {#if app.copiedRowId === log.id}
                      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
                        <polyline points="20 6 9 17 4 12"></polyline>
                      </svg>
                      <span>Copied</span>
                    {:else}
                      <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                      </svg>
                    {/if}
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        <div class="modal-footer" style="justify-content: space-between;">
          <span style="font-size: 11px; color: var(--text-dim);">
            Displaying {app.filteredLogs.length} of {app.logs.length} entries • In-memory buffer: last 500 actions
          </span>
          <button class="btn btn-primary" onclick={() => app.showLogsModal = false}>Close</button>
        </div>
      </div>
    </div>
  {/if}

