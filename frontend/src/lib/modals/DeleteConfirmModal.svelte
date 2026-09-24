<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Confirm Delete with 2 Options (List vs Disk) -->
  {#if app.showDeleteModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showDeleteModal = false}>
      <div class="modal-window delete-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header delete-modal-header">
          <div class="delete-header-title">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <polyline points="3 6 5 6 21 6"></polyline>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
            </svg>
            <span>Confirm Deletion</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showDeleteModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <p class="delete-prompt-desc">
            {#if app.deleteModalTarget === 'completed'}
              Are you sure you want to remove all <strong>completed & finished</strong> downloads?
            {:else if app.deleteModalTarget === 'selected'}
              Are you sure you want to remove <strong>{app.selectedIds.length}</strong> selected transfer{app.selectedIds.length > 1 ? 's' : ''}?
            {:else}
              Are you sure you want to remove <strong>{app.downloads.find(d => d.id === app.deleteModalSingleId)?.filename || 'this item'}</strong>?
            {/if}
          </p>

          <div class="delete-radio-group">
            <label class="delete-radio-card" class:active={!app.deleteModalWithFile}>
              <input type="radio" name="deleteOptionGroup" value={false} bind:group={app.deleteModalWithFile} />
              <div class="delete-radio-text">
                <span class="delete-radio-title">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                    <line x1="8" y1="6" x2="21" y2="6"></line>
                    <line x1="8" y1="12" x2="21" y2="12"></line>
                    <line x1="8" y1="18" x2="21" y2="18"></line>
                    <line x1="3" y1="6" x2="3.01" y2="6"></line>
                    <line x1="3" y1="12" x2="3.01" y2="12"></line>
                    <line x1="3" y1="18" x2="3.01" y2="18"></line>
                  </svg>Delete from List only
                </span>
                <span class="delete-radio-sub">Removes the task from the download manager. Downloaded files and archives remain safe on your hard disk.</span>
              </div>
            </label>

            <label class="delete-radio-card danger" class:active={app.deleteModalWithFile}>
              <input type="radio" name="deleteOptionGroup" value={true} bind:group={app.deleteModalWithFile} />
              <div class="delete-radio-text">
                <span class="delete-radio-title">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 6px; vertical-align: middle;">
                    <polyline points="3 6 5 6 21 6"></polyline>
                    <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                  </svg>Delete from List AND remove file(s) from disk
                </span>
                <span class="delete-radio-sub">Permanently removes the downloaded .zip, files, and temporary staging folders from your storage.</span>
              </div>
            </label>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" onclick={() => app.showDeleteModal = false}>Cancel</button>
          <button class="btn btn-danger" onclick={app.confirmDeleteModal}>
            {app.deleteModalWithFile ? 'Permanently Delete' : 'Remove from List'}
          </button>
        </div>
      </div>
    </div>
  {/if}

