<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Conflict / Duplicate Resolution -->
  {#if app.showConflictModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showConflictModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showConflictModal = false)}>
      <div class="modal-window modal-conflict" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="8" x2="12" y2="12"></line>
              <line x1="12" y1="16" x2="12.01" y2="16"></line>
            </svg>
            <span>Existing File or Duplicate Transfer Detected ({app.conflictList.length})</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showConflictModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body conflict-modal-body">
          <p class="conflict-intro">
            One or more links match an existing transfer or a file with the same name already exists in your destination folder. Choose how to handle each item:
          </p>

          {#if app.conflictList.length > 1}
            <div class="conflict-quick-bar">
              <span class="quick-bar-label">Apply to all:</span>
              <div class="quick-bar-actions">
                <button type="button" class="btn-mini btn-action" onclick={() => app.setAllConflictActions('rename')}>
                  Keep Both (Rename with Suffix)
                </button>
                <button type="button" class="btn-mini btn-action" onclick={() => app.setAllConflictActions('overwrite')}>
                  Overwrite Existing
                </button>
                <button type="button" class="btn-mini btn-action" onclick={() => app.setAllConflictActions('monitor')}>
                  Re-monitor & Verify Integrity
                </button>
              </div>
            </div>
          {/if}

          <div class="conflict-cards-list">
            {#each app.conflictList as c (c.file_id || c.url)}
              <div class="conflict-card">
                <div class="conflict-card-header">
                  <div class="conflict-header-title">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="flex-shrink: 0;">
                      {#if c.is_folder}
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                      {:else}
                        <path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path>
                        <polyline points="13 2 13 9 20 9"></polyline>
                      {/if}
                    </svg>
                    <span class="conflict-filename" title={c.expected_filename || c.title}>{c.expected_filename || c.title}</span>
                  </div>

                  <div class="conflict-badges">
                    {#if c.exists_in_list}
                      <span class="conflict-badge badge-in-list" title="Item is present in current download history">
                        IN LIST: {(c.existing_status || 'UNKNOWN').toUpperCase()}
                      </span>
                    {/if}
                    {#if c.exists_on_disk}
                      <span class="conflict-badge badge-on-disk" title="File was detected in local download folder">
                        ON DISK: {formatBytes(c.disk_size)}
                      </span>
                    {/if}
                  </div>
                </div>

                <div class="conflict-card-path font-mono" title={c.target_path}>
                  {c.target_path}
                </div>

                <div class="conflict-options">
                  <!-- Option 1: Rename (2) -->
                  <label class="conflict-option" class:selected={app.conflictResolutions[c.file_id || c.url] === 'rename'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="rename"
                      bind:group={app.conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Keep Both</strong> (Save as copy with suffix <code>(2)</code>)
                      </span>
                      <span class="conflict-option-desc">
                        Downloads as a separate file without altering the existing file or list item.
                      </span>
                    </div>
                  </label>

                  <!-- Option 2: Overwrite -->
                  <label class="conflict-option" class:selected={app.conflictResolutions[c.file_id || c.url] === 'overwrite'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="overwrite"
                      bind:group={app.conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Overwrite & Replace</strong>
                      </span>
                      <span class="conflict-option-desc">
                        Replaces the existing file on disk and re-downloads the file from 0%.
                      </span>
                    </div>
                  </label>

                  <!-- Option 3: Re-monitor & Verify -->
                  <label class="conflict-option" class:selected={app.conflictResolutions[c.file_id || c.url] === 'monitor'}>
                    <input
                      type="radio"
                      name="conflict_{c.file_id || c.url}"
                      value="monitor"
                      bind:group={app.conflictResolutions[c.file_id || c.url]}
                    />
                    <div class="conflict-option-content">
                      <span class="conflict-option-title">
                        <strong>Re-monitor & Verify Integrity</strong> (Resume if incomplete)
                      </span>
                      <span class="conflict-option-desc">
                        {#if c.exists_on_disk}
                          Inspects the {formatBytes(c.disk_size)} file on disk. If complete, marks completed. If corrupted or partial, verifies and resumes.
                        {:else}
                          Re-checks file status and resumes download without starting over.
                        {/if}
                      </span>
                    </div>
                  </label>
                </div>
              </div>
            {/each}
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => app.showConflictModal = false}>Cancel</button>
          <button class="btn btn-primary" disabled={app.isAdding} onclick={() => app.executeAddDownloads(app.conflictResolutions)}>
            {app.isAdding ? 'Processing...' : 'Confirm & Proceed'}
          </button>
        </div>
      </div>
    </div>
  {/if}

