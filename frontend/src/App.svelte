<script>
  import { onMount, onDestroy } from 'svelte';
  import { app } from './lib/state/appState.svelte.js';

  import LoginScreen from './lib/components/LoginScreen.svelte';
  import MenuBar from './lib/components/MenuBar.svelte';
  import Toolbar from './lib/components/Toolbar.svelte';
  import Sidebar from './lib/components/Sidebar.svelte';
  import DownloadTable from './lib/components/DownloadTable.svelte';
  import BottomInspector from './lib/components/BottomInspector.svelte';
  import StatusBar from './lib/components/StatusBar.svelte';
  import ContextMenu from './lib/components/ContextMenu.svelte';

  import AddDownloadModal from './lib/modals/AddDownloadModal.svelte';
  import ConflictModal from './lib/modals/ConflictModal.svelte';
  import DeleteConfirmModal from './lib/modals/DeleteConfirmModal.svelte';
  import GoogleAuthModal from './lib/modals/GoogleAuthModal.svelte';
  import SettingsModal from './lib/modals/SettingsModal.svelte';
  import DiscordExportModal from './lib/modals/DiscordExportModal.svelte';
  import DiscordRefreshModal from './lib/modals/DiscordRefreshModal.svelte';
  import LogsModal from './lib/modals/LogsModal.svelte';
  import DocModal from './lib/modals/DocModal.svelte';
  import UpdateAboutModals from './lib/modals/UpdateAboutModals.svelte';
  import ConfirmModal from './lib/modals/ConfirmModal.svelte';

  onMount(() => {
    app.initOnMount();
  });

  onDestroy(() => {
    app.cleanupOnDestroy();
  });
</script>

<svelte:head>
  <title>Google Drive &amp; Discord Direct Downloader</title>
</svelte:head>

{#if app.authRequired && !app.isAuthenticated}
  <LoginScreen />
{:else}
  <div class="app-layout">
    <MenuBar />
    <Toolbar />
    <div class="main-body">
      <Sidebar />
      <div class="content-pane">
        <DownloadTable />
        <BottomInspector />
      </div>
    </div>
    <StatusBar />
    <ContextMenu />

    <!-- Modals -->
    <AddDownloadModal />
    <ConflictModal />
    <DeleteConfirmModal />
    <GoogleAuthModal />
    <SettingsModal />
    <DiscordExportModal />
    <DiscordRefreshModal />
    <LogsModal />
    <DocModal />
    <UpdateAboutModals />
    <ConfirmModal />
  </div>
{/if}
