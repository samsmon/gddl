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

  let showNewFolderInput = $state(false);
  let newFolderName = $state('');
  let isCreatingFolder = $state(false);

  function startNewFolder() {
    newFolderName = '';
    showNewFolderInput = true;
  }

  function cancelNewFolder() {
    newFolderName = '';
    showNewFolderInput = false;
  }

  async function createFolder() {
    const trimmed = newFolderName.trim();
    if (!trimmed) return;
    isCreatingFolder = true;
    errorMsg = '';
    try {
      const res = await fetch('/api/fs/mkdir', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          parent: currentPath,
          name: trimmed
        })
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: 'Failed to create folder' }));
        throw new Error(err.error || 'Failed to create folder');
      }
      const data = await res.json();
      showNewFolderInput = false;
      newFolderName = '';
      if (data.path) {
        await browse(data.path);
      } else {
        await browse(currentPath);
      }
    } catch (e) {
      errorMsg = 'Error creating folder: ' + e.message;
    } finally {
      isCreatingFolder = false;
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

      <button
        class="nav-btn-action"
        title="Create New Subfolder in Current Directory"
        onclick={startNewFolder}
      >
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
          <line x1="12" y1="11" x2="12" y2="17"></line>
          <line x1="9" y1="14" x2="15" y2="14"></line>
        </svg>
        <span>New Folder</span>
      </button>
    </div>

    {#if showNewFolderInput}
      <div class="new-folder-bar">
        <span class="new-folder-label">Folder Name:</span>
        <input
          type="text"
          bind:value={newFolderName}
          placeholder="e.g. MyDownloads, Anime, FLAC..."
          autofocus
          onkeydown={(e) => {
            if (e.key === 'Enter') createFolder();
            if (e.key === 'Escape') cancelNewFolder();
          }}
        />
        <button class="btn-create" disabled={!newFolderName.trim() || isCreatingFolder} onclick={createFolder}>
          {isCreatingFolder ? 'Creating...' : 'Create'}
        </button>
        <button class="btn-create-cancel" onclick={cancelNewFolder}>Cancel</button>
      </div>
    {/if}

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

