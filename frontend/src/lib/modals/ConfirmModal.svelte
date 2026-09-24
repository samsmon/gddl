<script>
  import { app } from '../state/appState.svelte.js';
</script>

{#if app.confirmDialog.show}
  <div
    class="modal-overlay"
    style="z-index: 1000;"
    role="presentation"
    onclick={() => app.confirmDialog.show = false}
    onkeydown={(e) => e.key === 'Escape' && (app.confirmDialog.show = false)}
  >
    <div
      class="modal-window confirm-modal-window"
      role="dialog"
      aria-modal="true"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      style="max-width: 440px;"
    >
      <div class="modal-header">
        <span>{app.confirmDialog.title || 'Confirm Action'}</span>
        <button class="modal-close" aria-label="Close" onclick={() => app.confirmDialog.show = false}>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>
      <div class="modal-body" style="padding: 18px 20px;">
        <p style="margin: 0; font-size: 13px; line-height: 1.5; color: var(--text-main); white-space: pre-line;">{app.confirmDialog.message}</p>
      </div>
      <div class="modal-footer" style="display: flex; justify-content: flex-end; gap: 8px;">
        <button class="btn btn-secondary" onclick={() => app.confirmDialog.show = false}>Cancel</button>
        <button class="btn btn-{app.confirmDialog.confirmType || 'primary'}" onclick={app.handleConfirmAction}>
          {app.confirmDialog.confirmText || 'Confirm'}
        </button>
      </div>
    </div>
  </div>
{/if}
