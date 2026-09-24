<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Add Downloads -->
  {#if app.showAddModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showAddModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showAddModal = false)}>
      <div class="modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <span>Add Google Drive & Discord CDN Links</span>
          <button class="modal-close" aria-label="Close" onclick={() => app.showAddModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <label class="form-group">
            <span class="form-title">Enter Google Drive or Discord CDN URLs (one per line):</span>
            <textarea
              bind:value={app.addLinksInput}
              oninput={app.handleLinksInput}
              rows="5"
              placeholder="https://drive.google.com/file/d/1A2B3C.../view&#10;https://drive.google.com/drive/folders/1sSPph9ml0...&#10;https://cdn.discordapp.com/attachments/141.../154.../file.rar?ex=...&#10;https://media.discordapp.net/attachments/..."
            ></textarea>
          </label>

          {#if app.isResolvingFolder}
            <div class="folder-preview-loading">
              <div class="mini-spinner"></div>
              <span>Inspecting Google Drive folder contents...</span>
            </div>
          {:else if app.detectedFolder}
            <div class="folder-preview-card">
              <div class="folder-preview-header">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; flex-shrink: 0;">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <div class="folder-preview-info">
                  <div class="folder-preview-title" title={app.detectedFolder.title}>{app.detectedFolder.title}</div>
                  <div class="folder-preview-subtitle">{app.detectedFolder.files_count} files detected</div>
                </div>
              </div>
            </div>
          {:else if app.resolveError}
            <div class="folder-resolve-error">
              <span style="display: flex; align-items: center; gap: 6px;">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                  <line x1="12" y1="9" x2="12" y2="13"></line>
                  <line x1="12" y1="17" x2="12.01" y2="17"></line>
                </svg>
                {app.resolveError}
              </span>
            </div>
          {/if}

          <!-- Folder Download Format (Always prominent, defaults to ZIP) -->
          <div class="form-group" style="margin-top: 0.75rem;">
            <span class="form-title">Folder Download Mode:</span>
            <div class="folder-mode-options">
              <label class="radio-option">
                <input type="radio" name="folderMode" value="zip" bind:group={app.folderZipMode} />
                <div class="radio-text">
                  <span class="radio-label-title">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                      <polyline points="21 8 21 21 3 21 3 8"></polyline>
                      <rect x="1" y="3" width="22" height="5"></rect>
                      <line x1="10" y1="12" x2="14" y2="12"></line>
                    </svg>Compress to ZIP Archive (.zip) [Default]
                  </span>
                  <span class="radio-label-desc">Downloads files to local staging and compresses locally into a single verified ZIP file with live progress.</span>
                </div>
              </label>

              <label class="radio-option">
                <input type="radio" name="folderMode" value="folder" bind:group={app.folderZipMode} />
                <div class="radio-text">
                  <span class="radio-label-title">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                      <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                    </svg>Download as Subfolder
                  </span>
                  <span class="radio-label-desc">Downloads each file individually into a subfolder named after the folder.</span>
                </div>
              </label>
            </div>
          </div>

          <!-- Jellyfin-style Folder Picker Input Trigger -->
          <div class="form-group" style="margin-top: 0.75rem;">
            <span class="form-title">Save Path:</span>
            <div class="path-picker-row">
              <input type="text" bind:value={app.addTargetFolder} />
              <button class="btn-browse" onclick={() => app.openPickerFor('add')}>
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
                </svg>
                <span>Browse...</span>
              </button>
            </div>
            <span class="form-hint">Files will be saved into this directory.</span>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => app.showAddModal = false}>Cancel</button>
          <button class="btn btn-primary" disabled={app.isAdding || !app.addLinksInput.trim()} onclick={app.submitAddDownloads}>
            {app.isAdding ? 'Adding...' : 'Start Download'}
          </button>
        </div>
      </div>
    </div>
  {/if}

