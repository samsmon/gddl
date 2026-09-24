import { CoreState } from './coreState.svelte.js';
import { parseCookieInput } from '../utils/formatters.js';

export class DerivedState extends CoreState {
  filteredDownloads = $derived.by(() => {
    let list = this.downloads.filter(item => {
      // Filter out completed downloads if hideCompleted is enabled (except when user explicitly views completed tab)
      if (this.hideCompleted && item.status === 'completed' && this.activeFilter !== 'completed') return false;

      // Filter by status
      if (this.activeFilter === 'downloading' && item.status !== 'downloading' && item.status !== 'compressing' && item.status !== 'moving') return false;
      if (this.activeFilter === 'queued' && item.status !== 'queued') return false;
      if (this.activeFilter === 'paused' && item.status !== 'paused') return false;
      if (this.activeFilter === 'completed' && item.status !== 'completed') return false;
      if (this.activeFilter === 'missing' && item.status !== 'missing') return false;
      if (this.activeFilter === 'corrupted' && item.status !== 'corrupted') return false;
      if (this.activeFilter === 'failed' && item.status !== 'failed' && item.status !== 'cancelled') return false;

      // Filter by search query
      if (this.searchQuery.trim()) {
        const q = this.searchQuery.toLowerCase();
        const name = (item.filename || item.url || '').toLowerCase();
        return name.includes(q);
      }
      return true;
    });

    // Sorting with deterministic stable tie-breaker (like IDM)
    list.sort((a, b) => {
      let valA, valB;
      switch (this.sortColumn) {
        case 'name':
          valA = (a.filename || a.url || '').toLowerCase();
          valB = (b.filename || b.url || '').toLowerCase();
          break;
        case 'size':
          valA = a.total_bytes || 0;
          valB = b.total_bytes || 0;
          break;
        case 'done':
          valA = a.downloaded_bytes || 0;
          valB = b.downloaded_bytes || 0;
          break;
        case 'prog':
          valA = a.percentage || (a.status === 'completed' ? 100 : 0);
          valB = b.percentage || (b.status === 'completed' ? 100 : 0);
          break;
        case 'status':
          valA = a.status;
          valB = b.status;
          break;
        case 'speed':
          valA = a.status === 'downloading' ? (a.speed || 0) : 0;
          valB = b.status === 'downloading' ? (b.speed || 0) : 0;
          break;
        case 'eta':
          valA = a.status === 'downloading' ? (a.eta_seconds || 999999) : 999999;
          valB = b.status === 'downloading' ? (b.eta_seconds || 999999) : 999999;
          break;
        case 'path':
          valA = (a.target_folder || '').toLowerCase();
          valB = (b.target_folder || '').toLowerCase();
          break;
        case 'added':
        default:
          valA = new Date(a.last_try_at || a.created_at).getTime() || 0;
          valB = new Date(b.last_try_at || b.created_at).getTime() || 0;
          break;
      }

      if (valA < valB) return this.sortDirection === 'asc' ? -1 : 1;
      if (valA > valB) return this.sortDirection === 'asc' ? 1 : -1;

      // Stable deterministic tie-breaker (IDM style):
      // Prevents rows with identical values from randomly jumping around when timestamps update
      return (b.id || '').localeCompare(a.id || '');
    });

    return list;
  });

  selectedId = $derived.by(() => this.selectedIds.length > 0 ? this.selectedIds[this.selectedIds.length - 1] : null);

  selectedItems = $derived.by(() => this.downloads.filter(d => this.selectedIds.includes(d.id)));

  selectedItem = $derived.by(() => {
    if (this.selectedIds.length === 0) return null;
    return this.downloads.find(d => d.id === this.selectedIds[this.selectedIds.length - 1]) || null;
  });

  counts = $derived.by(() => ({
    all: this.downloads.length,
    downloading: this.downloads.filter(d => d.status === 'downloading' || d.status === 'compressing' || d.status === 'moving').length,
    queued: this.downloads.filter(d => d.status === 'queued').length,
    paused: this.downloads.filter(d => d.status === 'paused').length,
    completed: this.downloads.filter(d => d.status === 'completed').length,
    missing: this.downloads.filter(d => d.status === 'missing').length,
    corrupted: this.downloads.filter(d => d.status === 'corrupted').length,
    failed: this.downloads.filter(d => d.status === 'failed' || d.status === 'cancelled').length,
    totalSpeed: this.downloads
      .filter(d => d.status === 'downloading')
      .reduce((sum, d) => sum + (d.speed || 0), 0)
  }));

  canResumeAll = $derived(this.counts.paused > 0 || this.counts.failed > 0 || this.counts.queued > 0);

  canStopAll = $derived(this.counts.downloading > 0 || this.counts.queued > 0);

  errorLogsCount = $derived(this.logs.filter(l => l.level === 'ERROR').length);

  filteredLogs = $derived.by(() => {
    return this.logs.filter(l => {
      if (this.logFilter !== 'ALL' && l.level !== this.logFilter) return false;
      if (!this.logSearch.trim()) return true;
      const q = this.logSearch.toLowerCase();
      return (
        (l.message && l.message.toLowerCase().includes(q)) ||
        (l.category && l.category.toLowerCase().includes(q)) ||
        (l.details && l.details.toLowerCase().includes(q)) ||
        (l.level && l.level.toLowerCase().includes(q))
      );
    });
  });

  flattenedRows = $derived.by(() => {
    const rows = [];
    for (const item of this.filteredDownloads) {
      rows.push({ type: 'item', item, id: item.id });
      if (item.is_folder && this.expandedFolderIds.includes(item.id) && item.folder_files && item.folder_files.length > 0) {
        for (let ci = 0; ci < item.folder_files.length; ci++) {
          const child = item.folder_files[ci];
          rows.push({ type: 'child', parent: item, child, id: `${item.id}_child_${child.id || ci}`, index: ci });
        }
      }
    }
    return rows;
  });

  totalRowCount = $derived(this.flattenedRows.length);

  isVirtual = $derived(this.totalRowCount > 60);

  startIndex = $derived(
    this.isVirtual ? Math.max(0, Math.floor(this.scrollTop / this.ROW_HEIGHT) - this.OVERSCAN) : 0
  );

  endIndex = $derived(
    this.isVirtual ? Math.min(this.totalRowCount, Math.ceil((this.scrollTop + this.viewportHeight) / this.ROW_HEIGHT) + this.OVERSCAN) : this.totalRowCount
  );

  visibleRows = $derived(
    this.isVirtual ? this.flattenedRows.slice(this.startIndex, this.endIndex) : this.flattenedRows
  );

  topSpacerHeight = $derived(this.isVirtual ? this.startIndex * this.ROW_HEIGHT : 0);

  bottomSpacerHeight = $derived(this.isVirtual ? Math.max(0, (this.totalRowCount - this.endIndex) * this.ROW_HEIGHT) : 0);

  cookieValidation = $derived.by(() => {
    if (!this.newCookieValue.trim()) return null;
    const val = parseCookieInput(this.newCookieValue);
    const hasSID = /(?:^|;\s*)SID=/.test(val);
    const hasHSID = /(?:^|;\s*)HSID=/.test(val);
    const hasSSID = /(?:^|;\s*)SSID=/.test(val);
    const count = (hasSID ? 1 : 0) + (hasHSID ? 1 : 0) + (hasSSID ? 1 : 0);
    return {
      hasSID,
      hasHSID,
      hasSSID,
      valid: hasSID && hasHSID && hasSSID,
      count
    };
  });

  formattedDiscordExportText = $derived.by(() => {
    if (!this.unfinishedDiscordItems || this.unfinishedDiscordItems.length === 0) {
      return '';
    }
    if (this.discordExportFormat === 'urls') {
      return this.unfinishedDiscordItems.map(i => i.url).join('\n');
    }
    if (this.discordExportFormat === 'detailed') {
      return this.unfinishedDiscordItems.map(i => 
        `Filename: ${i.filename}\nChannel ID: ${i.channel_id || '-'}\nAttachment ID: ${i.attachment_id || '-'}\nStatus: ${i.status}\nError: ${i.error || 'None'}\nURL: ${i.url}\n`
      ).join('\n----------------------------------------\n');
    }
    // Default: Clean formatted JSON designed for AI agents / scrapers
    return JSON.stringify(this.unfinishedDiscordItems, null, 2);
  });
}
