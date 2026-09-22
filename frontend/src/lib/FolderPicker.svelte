<script>
  import { onMount } from 'svelte';

  let { initialPath = '', onSelect, onClose } = $props();

  let currentPath = $state(initialPath || 'C:\\');
  let parentPath = $state('');
  let drives = $state([]);
  let folders = $state([]);
  let loading = $state(false);
  let errorMsg = $state('');

  async function browse(path) {
    loading = true;
    errorMsg = '';
    try {
      const q = path ? `?path=${encodeURIComponent(path)}` : '';
      const res = await fetch(`/api/fs/browse${q}`);
      if (!res.ok) throw new Error('Failed to list directory');
      const data = await res.json();
      currentPath = data.current;
      parentPath = data.parent;
      drives = data.drives || [];
      folders = data.folders || [];
    } catch (e) {
      errorMsg = 'Cannot access folder: ' + e.message;
    } finally {
      loading = false;
    }
  }

  function handleFolderClick(folderPath) {
    browse(folderPath);
  }

  function handleDriveClick(drivePath) {
    browse(drivePath);
  }

  function handleParentClick() {
    if (parentPath) {
      browse(parentPath);
    }
  }

  // Parse path for breadcrumb navigation
  let breadcrumbs = $derived.by(() => {
    if (!currentPath) return [];
    const parts = currentPath.split(/[\\/]+/).filter(Boolean);
    const crumbs = [];
    let acc = '';

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      if (i === 0 && part.includes(':')) {
        acc = part + '\\';
      } else {
        acc = acc.endsWith('\\') ? acc + part : acc + '\\' + part;
      }
      crumbs.push({
        name: part,
        path: acc
      });
    }
    return crumbs;
  });

  onMount(() => {
    browse(currentPath);
  });
</script>

<div class="picker-overlay" role="presentation" onclick={onClose} onkeydown={(e) => e.key === 'Escape' && onClose()}>
  <div class="picker-dialog" role="dialog" aria-modal="true" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
    <!-- Dialog Header -->
    <div class="picker-header">
      <div class="picker-title">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
        </svg>
        <span>Select Destination Folder</span>
      </div>
      <button class="btn-close" aria-label="Close" onclick={onClose}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <line x1="18" y1="6" x2="6" y2="18"></line>
          <line x1="6" y1="6" x2="18" y2="18"></line>
        </svg>
      </button>
    </div>

    <!-- Quick Drive Selector (Jellyfin Style) -->
    <div class="drives-bar">
      <span class="drives-label">Drives:</span>
      <div class="drives-list">
        {#each drives as drive}
          <button
            class="drive-chip"
            class:active={currentPath.toUpperCase().startsWith(drive.name.toUpperCase())}
            onclick={() => handleDriveClick(drive.path)}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="2" y="4" width="20" height="16" rx="2"></rect>
              <line x1="6" y1="12" x2="6.01" y2="12"></line>
              <line x1="10" y1="12" x2="10.01" y2="12"></line>
            </svg>
            <span>{drive.name}</span>
          </button>
        {/each}
      </div>
    </div>

    <!-- Breadcrumb Path Bar -->
    <div class="breadcrumb-bar">
      <button
        class="nav-btn up-btn"
        title="Go Up One Level"
        disabled={!parentPath || parentPath === currentPath}
        onclick={handleParentClick}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <line x1="12" y1="19" x2="12" y2="5"></line>
          <polyline points="5 12 12 5 19 12"></polyline>
        </svg>
      </button>

      <div class="breadcrumb-trail">
        {#each breadcrumbs as crumb, i}
          {#if i > 0}
            <span class="crumb-separator">/</span>
          {/if}
          <button class="crumb-item" onclick={() => browse(crumb.path)}>
            {crumb.name}
          </button>
        {/each}
      </div>
    </div>

    <!-- Folder Browser List -->
    <div class="folder-list-container">
      {#if loading}
        <div class="loading-state">
          <div class="spinner"></div>
          <span>Loading directory...</span>
        </div>
      {:else if errorMsg}
        <div class="error-state">
          <span>{errorMsg}</span>
          <button class="btn-retry" onclick={() => browse('C:\\')}>Go to C:\</button>
        </div>
      {:else if folders.length === 0}
        <div class="empty-state">
          <span>No subfolders found in this directory.</span>
        </div>
      {:else}
        <div class="folder-grid">
          {#each folders as folder}
            <button
              class="folder-row"
              onclick={() => handleFolderClick(folder.path)}
            >
              <svg class="folder-icon" width="18" height="18" viewBox="0 0 24 24" fill="currentColor" stroke="none">
                <path d="M10 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-8l-2-2z"></path>
              </svg>
              <span class="folder-name">{folder.name}</span>
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Selected Path Input & Actions -->
    <div class="picker-footer">
      <div class="current-path-display">
        <label for="picker-path-input">Selected Path:</label>
        <input
          id="picker-path-input"
          type="text"
          bind:value={currentPath}
          onkeydown={(e) => e.key === 'Enter' && browse(currentPath)}
        />
      </div>

      <div class="footer-actions">
        <button class="btn btn-cancel" onclick={onClose}>Cancel</button>
        <button class="btn btn-select" onclick={() => onSelect(currentPath)}>
          Select Folder
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  .picker-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 999;
  }

  .picker-dialog {
    width: 90%;
    max-width: 680px;
    height: 540px;
    background: var(--modal-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4);
    font-family: inherit;
    color: var(--text-main);
  }

  .picker-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border-color);
    background: var(--modal-header-bg);
  }

  .picker-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--text-main);
  }

  .btn-close {
    background: transparent;
    border: none;
    color: var(--text-muted);
    font-size: 1rem;
    padding: 0.25rem 0.5rem;
    cursor: pointer;
    border-radius: 4px;
  }
  .btn-close:hover {
    color: var(--text-main);
    background: var(--btn-hover);
  }

  /* Drives bar */
  .drives-bar {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 1rem;
    background: var(--sidebar-bg);
    border-bottom: 1px solid var(--border-color);
  }
  .drives-label {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    font-weight: 600;
  }
  .drives-list {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .drive-chip {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0.25rem 0.6rem;
    border-radius: 4px;
    font-size: 0.8rem;
    font-family: var(--font-mono);
    cursor: pointer;
  }
  .drive-chip:hover {
    background: var(--btn-hover);
  }
  .drive-chip.active {
    background: var(--accent-blue);
    border-color: var(--accent-blue-hover);
    color: #ffffff;
    font-weight: 600;
  }

  /* Breadcrumb */
  .breadcrumb-bar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 1rem;
    background: var(--table-bg);
    border-bottom: 1px solid var(--border-color);
  }
  .nav-btn {
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    width: 28px;
    height: 28px;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
  }
  .nav-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }
  .nav-btn:hover:not(:disabled) {
    background: var(--btn-hover);
  }
  .breadcrumb-trail {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    overflow-x: auto;
    font-size: 0.85rem;
    color: var(--text-main);
    white-space: nowrap;
    padding-bottom: 2px;
  }
  .crumb-item {
    background: transparent;
    border: none;
    color: var(--accent-blue);
    padding: 0.2rem 0.4rem;
    border-radius: 3px;
    cursor: pointer;
    font-size: 0.85rem;
  }
  .crumb-item:hover {
    background: var(--btn-hover);
    text-decoration: underline;
  }
  .crumb-separator {
    color: var(--text-dim);
  }

  /* Folder List */
  .folder-list-container {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem 1rem;
    background: var(--table-bg);
  }

  .folder-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 0.4rem;
  }

  .folder-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.45rem 0.6rem;
    background: var(--sidebar-bg);
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-main);
    text-align: left;
    cursor: pointer;
    overflow: hidden;
    font-size: 0.85rem;
  }
  .folder-row:hover {
    background: var(--table-row-hover);
    border-color: var(--border-color);
  }
  .folder-icon {
    flex-shrink: 0;
    color: #94a3b8;
  }
  .folder-name {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .loading-state, .empty-state, .error-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
    gap: 0.75rem;
    font-size: 0.9rem;
  }
  .error-state {
    color: var(--accent-red);
  }
  .btn-retry {
    background: var(--btn-bg);
    color: var(--text-main);
    border: 1px solid var(--border-color);
    padding: 0.4rem 0.8rem;
    border-radius: 4px;
    cursor: pointer;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid var(--border-color);
    border-top-color: var(--accent-blue);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* Footer */
  .picker-footer {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    padding: 0.75rem 1rem;
    background: var(--modal-header-bg);
    border-top: 1px solid var(--border-color);
  }
  .current-path-display {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8rem;
    color: var(--text-muted);
  }
  .current-path-display input {
    flex: 1;
    background: var(--input-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    padding: 0.4rem 0.6rem;
    border-radius: 4px;
    font-family: var(--font-mono);
    font-size: 0.85rem;
  }
  .footer-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
  }
  .btn {
    padding: 0.45rem 1rem;
    border-radius: 4px;
    font-size: 0.85rem;
    font-weight: 500;
    cursor: pointer;
    border: none;
  }
  .btn-cancel {
    background: var(--btn-bg);
    border: 1px solid var(--border-color);
    color: var(--text-main);
  }
  .btn-cancel:hover {
    background: var(--btn-hover);
  }
  .btn-select {
    background: var(--accent-green);
    color: #ffffff;
    font-weight: 600;
  }
  .btn-select:hover {
    opacity: 0.9;
  }
</style>
