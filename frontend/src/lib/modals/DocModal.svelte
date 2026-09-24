<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
</script>

  <!-- Modal: Documentation & Quick Guide -->
  {#if app.showDocModal}
    <div class="modal-overlay" role="presentation" onclick={() => app.showDocModal = false} onkeydown={(e) => e.key === 'Escape' && (app.showDocModal = false)}>
      <div class="modal-window doc-modal-window" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
        <div class="modal-header">
          <div style="display: flex; align-items: center; gap: 8px;">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path>
              <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path>
            </svg>
            <span style="font-weight: 700;">Google Drive Client - Documentation & User Guide</span>
          </div>
          <button class="modal-close" aria-label="Close" onclick={() => app.showDocModal = false}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </div>

        <div class="modal-body doc-modal-body">
          <section class="doc-section">
            <h4>Getting Started & Downloading</h4>
            <p>Paste any Google Drive link into <strong>Add Links</strong> (or press <kbd>Ctrl+N</kbd>). Multiple links can be added at once by separating them with newlines.</p>
            <ul>
              <li><strong>Single File URLs:</strong> <code>https://drive.google.com/file/d/&lt;FILE_ID&gt;/view</code></li>
              <li><strong>Direct Download URLs:</strong> <code>https://drive.usercontent.google.com/download?id=&lt;FILE_ID&gt;&amp;export=download</code></li>
              <li><strong>Folder URLs:</strong> <code>https://drive.google.com/drive/folders/&lt;FOLDER_ID&gt;</code></li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Folder Download Modes</h4>
            <ul>
              <li><strong>Compress to ZIP Archive (.zip) [Recommended]:</strong> Downloads all files inside the folder and compresses them locally into a verified <code>.zip</code> archive. You can monitor live Turn-by-turn and compression progress.</li>
              <li><strong>Download as Subfolder:</strong> Creates a local folder named after the Drive directory and queues each file individually into the transfer list.</li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Real-Time File Sync & Missing File Detection</h4>
            <p>If a downloaded file is moved, renamed, or deleted from your local disk:</p>
            <ul>
              <li>Clicking or checking the row in the table instantly detects that the file is missing and marks it as <span class="status-tag status-missing">MISSING / MOVED</span>.</li>
              <li>You can click <strong>Re-download</strong> from the Transfer Details pane or context menu to immediately re-fetch the file.</li>
              <li>All download history is safely preserved in <code>downloads.json</code> and survives server reboots, git pulls, and software updates.</li>
            </ul>
          </section>

          <section class="doc-section">
            <h4>Downloading Restricted / Quota-Exceeded Files</h4>
            <p>If a file requires Google account login or has exceeded its public quota, open <strong>Tools &gt; Google Session Cookie</strong> and paste your session cookie from Chrome/Edge Developer Tools (<kbd>F12</kbd> &gt; Network/Application &gt; Cookies).</p>
          </section>

          <section class="doc-section">
            <h4>Keyboard Shortcuts Cheat Sheet</h4>
            <div class="shortcut-table">
              <div><kbd>Ctrl+N</kbd> <span>Open Add Links dialog</span></div>
              <div><kbd>Ctrl+A</kbd> <span>Select all transfers in current view</span></div>
              <div><kbd>Esc</kbd> <span>Deselect all / Close open menus & modals</span></div>
              <div><kbd>Space</kbd> <span>Start or pause selected transfer(s)</span></div>
              <div><kbd>Del</kbd> <span>Delete selected transfer(s) from list only</span></div>
              <div><kbd>Shift+Del</kbd> <span>Delete transfer(s) from list AND remove file from disk</span></div>
            </div>
          </section>
        </div>

        <div class="modal-footer">
          <button class="btn btn-primary" onclick={() => app.showDocModal = false}>Got it</button>
        </div>
      </div>
    </div>
  {/if}

