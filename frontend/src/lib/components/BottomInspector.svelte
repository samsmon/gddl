<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

      <!-- Bottom Details Pane (qBittorrent style) -->
      {#if app.selectedIds.length > 1}
        <div class="detail-pane">
          <div class="detail-header">
            <span class="detail-title">
              <strong>Batch Selection:</strong> {app.selectedIds.length} items selected
            </span>
            <div class="detail-actions">
              <button class="btn-mini btn-action" onclick={app.startSelected}>
                Start / Resume ({app.selectedIds.length})
              </button>
              <button class="btn-mini btn-secondary" onclick={app.pauseSelected}>
                Pause ({app.selectedIds.length})
              </button>
              <button class="btn-mini btn-secondary" onclick={app.restartSelected}>
                Restart ({app.selectedIds.length})
              </button>
              <button class="btn-mini btn-danger" onclick={() => app.openDeleteModal('selected', null, false)} title="Delete selected items">
                Delete ({app.selectedIds.length})...
              </button>
            </div>
          </div>

          <div class="detail-grid">
            <div class="detail-col">
              <div>
                <span class="prop-label">Selected Count:</span>
                <span class="prop-val font-mono">{app.selectedIds.length} transfers</span>
              </div>
              <div>
                <span class="prop-label">Total Size:</span>
                <span class="prop-val font-mono">{formatBytes(app.selectedItems.reduce((acc, d) => acc + (d.total_bytes || 0), 0))}</span>
              </div>
              <div>
                <span class="prop-label">Downloaded:</span>
                <span class="prop-val font-mono">{formatBytes(app.selectedItems.reduce((acc, d) => acc + (d.downloaded_bytes || 0), 0))}</span>
              </div>
            </div>
            <div class="detail-col">
              <div>
                <span class="prop-label">Combined Speed:</span>
                <span class="prop-val font-mono">{formatSpeed(app.selectedItems.reduce((acc, d) => acc + (d.speed || 0), 0))}</span>
              </div>
              <div>
                <span class="prop-label">Status Summary:</span>
                <span class="prop-val font-mono">
                  {app.selectedItems.filter(d => d.status === 'downloading' || d.status === 'compressing').length} downloading,
                  {app.selectedItems.filter(d => d.status === 'completed').length} completed,
                  {app.selectedItems.filter(d => d.status === 'paused').length} paused
                </span>
              </div>
            </div>
          </div>
        </div>
      {:else if app.selectedItem}
        <div class="detail-pane">
          <div class="detail-header">
            <span class="detail-title">
              <strong>Transfer Details:</strong> {app.selectedItem.filename}
            </span>
            <div class="detail-actions">
              {#if app.selectedItem.status === 'missing'}
                <button class="btn-mini btn-action" onclick={() => app.restartDownload(app.selectedItem.id)} title="Re-download the missing file">
                  Re-download
                </button>
                <button class="btn-mini btn-secondary" onclick={() => app.checkFileStatus(app.selectedItem.id)} title="Verify if file was moved back or restored">
                  Re-check
                </button>
              {:else if app.selectedItem.status === 'downloading' || app.selectedItem.status === 'queued'}
                <button class="btn-mini btn-secondary" onclick={() => app.pauseDownload(app.selectedItem.id)}>
                  Pause
                </button>
              {:else if app.selectedItem.status === 'paused' || app.selectedItem.status === 'failed' || app.selectedItem.status === 'cancelled'}
                <button class="btn-mini btn-action" onclick={() => app.startDownload(app.selectedItem.id)}>
                  Start / Resume
                </button>
              {/if}

              <button class="btn-mini btn-secondary" onclick={() => app.restartDownload(app.selectedItem.id)}>
                Restart
              </button>

              <button class="btn-mini btn-danger" onclick={() => app.openDeleteModal('single', app.selectedItem.id, false)} title="Delete this download">
                Delete...
              </button>
            </div>
          </div>

          <div class="detail-grid">
            <div class="detail-col">
              <div>
                <span class="prop-label">Status:</span>
                <span class="prop-val" class:val-corrupt={app.selectedItem.status === 'corrupted'} class:val-missing={app.selectedItem.status === 'missing'}>
                  {app.selectedItem.status.toUpperCase()}
                </span>
              </div>
              <div>
                <span class="prop-label">Transfer Mode:</span>
                <span class="prop-val font-mono">
                  {#if app.selectedItem.chunks && app.selectedItem.chunks > 1}
                    <span class="badge-chunks-detail">{app.selectedItem.chunks} Parallel Streams (Multi-Chunk)</span>
                  {:else}
                    <span style="color: var(--text-muted);">Single Stream</span>
                  {/if}
                </span>
              </div>
              <div><span class="prop-label">Downloaded:</span> <span class="prop-val font-mono">{formatBytes(app.selectedItem.downloaded_bytes)} / {formatBytes(app.selectedItem.total_bytes)}</span></div>
              <div><span class="prop-label">Speed:</span> <span class="prop-val font-mono">{formatSpeed(app.selectedItem.speed)}</span></div>
              {#if app.selectedItem.is_folder}
                <div>
                  <span class="prop-label">Folder Progress:</span>
                  <span class="prop-val font-mono">{app.selectedItem.completed_files || 0} / {app.selectedItem.total_files || 0} files</span>
                </div>
              {/if}
            </div>
            <div class="detail-col">
              <div><span class="prop-label">ETA:</span> <span class="prop-val font-mono">{formatTime(app.selectedItem.eta_seconds)}</span></div>
              <div>
                <span class="prop-label">Save Path:</span>
                <span class="prop-val">{app.selectedItem.target_folder}</span>
                <button class="btn-browse-mini" style="margin-left: 6px; padding: 1px 6px; font-size: 11px;" disabled={app.selectedItem.status === 'moving'} onclick={() => app.openPickerFor('item', app.selectedItem.id)} title="Change save folder for this download">Change...</button>
              </div>
              <div><span class="prop-label">Added / Last Try:</span> <span class="prop-val font-mono">{formatDateTime(app.selectedItem.created_at)} / {formatDateTime(app.selectedItem.last_try_at)}</span></div>
              {#if app.selectedItem.current_file}
                <div>
                  <span class="prop-label">{app.selectedItem.status === 'compressing' ? 'Compressing:' : 'Active File:'}</span>
                  <span class="prop-val url-truncate font-mono" title={app.selectedItem.current_file}>{app.selectedItem.current_file}</span>
                </div>
              {/if}
            </div>
          </div>

          {#if app.selectedItem.status === 'missing'}
            <div class="detail-missing">
              <div class="detail-missing-header">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0; margin-top: 2px;">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                <div>
                  <strong>File Missing or Moved from Disk:</strong>
                  <div class="error-desc">{app.selectedItem.error || `The file or folder could not be found at "${app.selectedItem.target_folder}". It might have been deleted, moved, or renamed.`}</div>
                </div>
              </div>
              <div class="detail-missing-actions">
                <button class="btn-mini btn-action" onclick={() => app.restartDownload(app.selectedItem.id)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="vertical-align: middle; margin-right: 4px;">
                    <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
                  </svg>Re-download to Save Path
                </button>
                <button class="btn-mini btn-secondary" onclick={() => app.checkFileStatus(app.selectedItem.id)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="vertical-align: middle; margin-right: 4px;">
                    <circle cx="11" cy="11" r="8"></circle>
                    <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                  </svg>Check File Again
                </button>
                <button class="btn-mini btn-secondary" onclick={() => app.openDeleteModal('single', app.selectedItem.id, false)}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                    <line x1="18" y1="6" x2="6" y2="18"></line>
                    <line x1="6" y1="6" x2="18" y2="18"></line>
                  </svg>Remove from List
                </button>
              </div>
            </div>
          {:else if app.selectedItem.status === 'corrupted'}
            <div class="detail-corrupt">
              <div style="display: flex; align-items: center; gap: 6px; margin-bottom: 4px;">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                <strong>Integrity Verification Alert:</strong>
              </div>
              <div class="error-desc">{app.selectedItem.error}</div>
              <button class="btn-mini btn-secondary" onclick={() => app.openLogsModal('ERROR')} style="margin-top: 6px;">
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                  <polyline points="4 17 10 11 4 5"></polyline>
                  <line x1="12" y1="19" x2="20" y2="19"></line>
                </svg>View Logs
              </button>
            </div>
          {:else if app.selectedItem.error}
            <div class="detail-error">
              <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 8px;">
                <div>
                  <strong>Error:</strong> {app.selectedItem.error}
                </div>
                <div style="display: flex; gap: 6px; flex-shrink: 0;">
                  {#if app.selectedItem.error.toLowerCase().includes('quota')}
                    <button class="btn-mini btn-action" onclick={() => app.resetCooldownAndRetry(app.selectedItem.id)} style="white-space: nowrap;">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style="vertical-align: middle; margin-right: 4px;">
                        <path d="M21.5 2v6h-6M21.34 15.57a10 10 0 1 1-.57-8.38l6 5.67"/>
                      </svg>Reset Cooldown & Retry
                    </button>
                  {/if}
                  {#if app.selectedItem.url && (app.selectedItem.url.includes('discordapp.com') || app.selectedItem.url.includes('discordapp.net'))}
                    <button class="btn-mini btn-action" onclick={app.openDiscordRefreshModal} style="white-space: nowrap;" title="Paste fresh Discord CDN URL">
                      <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" style="vertical-align: middle; margin-right: 4px;">
                        <path d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/>
                      </svg>Refresh Link
                    </button>
                  {/if}
                  <button class="btn-mini btn-secondary" onclick={() => app.openLogsModal('ERROR')} style="white-space: nowrap;">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="vertical-align: middle; margin-right: 4px;">
                      <polyline points="4 17 10 11 4 5"></polyline>
                      <line x1="12" y1="19" x2="20" y2="19"></line>
                    </svg>View Logs
                  </button>
                </div>
              </div>
            </div>
          {/if}
        </div>
      {/if}
