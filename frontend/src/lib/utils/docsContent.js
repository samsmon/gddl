export const APP_VERSION = 'v1.2.0';
export const APP_EDITION = 'Persistent Desktop Edition';

export const WHATS_NEW_ITEMS = [
  'Desktop application menu bar (File, Edit, View, Tools, Help)',
  'Persistent real-time download history (downloads.json)',
  'Real-time file presence detection (Missing / Moved / Deleted)',
  'One-click re-download for missing files'
];

export const SHORTCUTS_LIST = [
  { key: 'Ctrl+N', desc: 'Open Add Links dialog' },
  { key: 'Ctrl+A', desc: 'Select all transfers in current view' },
  { key: 'Esc', desc: 'Deselect all / Close open menus & modals' },
  { key: 'Space', desc: 'Start or pause selected transfer(s)' },
  { key: 'Del', desc: 'Delete selected transfer(s) from list only' },
  { key: 'Shift+Del', desc: 'Delete transfer(s) from list AND remove file from disk' }
];

export const ABOUT_SPECS = [
  { label: 'Backend:', value: 'Go 1.23 Concurrency' },
  { label: 'Frontend:', value: 'Svelte 5 Runes + Vite' },
  { label: 'Storage:', value: 'Realtime downloads.json' },
  { label: 'License:', value: 'MIT Open Source' }
];
