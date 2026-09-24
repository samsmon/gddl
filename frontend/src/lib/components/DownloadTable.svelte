<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
  import DownloadChildRow from './DownloadChildRow.svelte';
</script>

      {#if app.bulkAddingStatus}
        <div class="bulk-adding-banner">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="spin-icon">
            <line x1="12" y1="2" x2="12" y2="6"></line>
            <line x1="12" y1="18" x2="12" y2="22"></line>
            <line x1="4.93" y1="4.93" x2="7.76" y2="7.76"></line>
            <line x1="16.24" y1="16.24" x2="19.07" y2="19.07"></line>
            <line x1="2" y1="12" x2="6" y2="12"></line>
            <line x1="18" y1="12" x2="22" y2="12"></line>
            <line x1="4.93" y1="19.07" x2="7.76" y2="16.24"></line>
            <line x1="16.24" y1="7.76" x2="19.07" y2="4.93"></line>
          </svg>
          <span>{app.bulkAddingStatus}</span>
        </div>
      {/if}
      <div class="table-container" bind:this={app.tableContainerEl} onscroll={app.handleTableScroll}>
        <table class="torrent-table">
          <colgroup>
            <col style="width: {app.colWidths.name}px;" />
            <col style="width: {app.colWidths.size}px;" />
            <col style="width: {app.colWidths.done}px;" />
            <col style="width: {app.colWidths.prog}px;" />
            <col style="width: {app.colWidths.status}px;" />
            <col style="width: {app.colWidths.speed}px;" />
            <col style="width: {app.colWidths.eta}px;" />
            <col style="width: {app.colWidths.path}px;" />
            <col style="width: {app.colWidths.added}px;" />
          </colgroup>
          <thead>
            <tr>
              <!-- Name Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.name}px; min-width: {app.colWidths.name}px; max-width: {app.colWidths.name}px;"
                onclick={() => app.toggleSort('name')}
              >
                <span>Name</span>
                {#if app.sortColumn === 'name'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('name', e)} ondblclick={() => app.autoSizeColumn('name')}></span>
              </th>

              <!-- Size Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.size}px; min-width: {app.colWidths.size}px; max-width: {app.colWidths.size}px;"
                onclick={() => app.toggleSort('size')}
              >
                <span>Size</span>
                {#if app.sortColumn === 'size'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('size', e)} ondblclick={() => app.autoSizeColumn('size')}></span>
              </th>

              <!-- Done Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.done}px; min-width: {app.colWidths.done}px; max-width: {app.colWidths.done}px;"
                onclick={() => app.toggleSort('done')}
              >
                <span>Done</span>
                {#if app.sortColumn === 'done'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('done', e)} ondblclick={() => app.autoSizeColumn('done')}></span>
              </th>

              <!-- Progress Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.prog}px; min-width: {app.colWidths.prog}px; max-width: {app.colWidths.prog}px;"
                onclick={() => app.toggleSort('prog')}
              >
                <span>Progress</span>
                {#if app.sortColumn === 'prog'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('prog', e)} ondblclick={() => app.autoSizeColumn('prog')}></span>
              </th>

              <!-- Status Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.status}px; min-width: {app.colWidths.status}px; max-width: {app.colWidths.status}px;"
                onclick={() => app.toggleSort('status')}
              >
                <span>Status</span>
                {#if app.sortColumn === 'status'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('status', e)} ondblclick={() => app.autoSizeColumn('status')}></span>
              </th>

              <!-- Down Speed Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.speed}px; min-width: {app.colWidths.speed}px; max-width: {app.colWidths.speed}px;"
                onclick={() => app.toggleSort('speed')}
              >
                <span>Down Speed</span>
                {#if app.sortColumn === 'speed'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('speed', e)} ondblclick={() => app.autoSizeColumn('speed')}></span>
              </th>

              <!-- ETA Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.eta}px; min-width: {app.colWidths.eta}px; max-width: {app.colWidths.eta}px;"
                onclick={() => app.toggleSort('eta')}
              >
                <span>ETA</span>
                {#if app.sortColumn === 'eta'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('eta', e)} ondblclick={() => app.autoSizeColumn('eta')}></span>
              </th>

              <!-- Save Path Column -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.path}px; min-width: {app.colWidths.path}px; max-width: {app.colWidths.path}px;"
                onclick={() => app.toggleSort('path')}
              >
                <span>Save Path</span>
                {#if app.sortColumn === 'path'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('path', e)} ondblclick={() => app.autoSizeColumn('path')}></span>
              </th>

              <!-- Added / Last Try Date Column (IDM style) -->
              <th
                class="th-sortable"
                style="width: {app.colWidths.added}px; min-width: {app.colWidths.added}px; max-width: {app.colWidths.added}px;"
                onclick={() => app.toggleSort('added')}
              >
                <span>Added / Last Try</span>
                {#if app.sortColumn === 'added'}
                  <span class="sort-indicator">{app.sortDirection === 'asc' ? '▲' : '▼'}</span>
                {/if}
                <span class="col-resizer" role="separator" aria-orientation="vertical" tabindex="-1" onmousedown={(e) => app.startResize('added', e)} ondblclick={() => app.autoSizeColumn('added')}></span>
              </th>
            </tr>
          </thead>
          <tbody>
            {#if app.totalRowCount === 0}
              <tr class="empty-row" onclick={() => { app.selectedIds = []; app.lastClickedId = null; }}>
                <td colspan="9">
                  <div class="table-empty">
                    <span>No transfers in this view. Click <strong>Add Links</strong> to start downloading.</span>
                  </div>
                </td>
              </tr>
            {:else}
              {#if app.topSpacerHeight > 0}
                <tr class="spacer-row" style="height: {app.topSpacerHeight}px;"><td colspan="9" style="height: {app.topSpacerHeight}px; padding: 0; border: none;"></td></tr>
              {/if}
              {#each app.visibleRows as row (row.id)}
                {#if row.type === 'item'}
                  {@const item = row.item}
                  <tr
                    class="torrent-row"
                    class:selected={app.selectedIds.includes(item.id)}
                    class:row-corrupt={item.status === 'corrupted'}
                    onclick={(e) => app.handleRowClick(item, e)}
                    oncontextmenu={(e) => app.handleContextMenu(item, e)}
                  >
                    <td class="col-name" title={item.filename || item.url}>
                      <div class="name-cell">
                        {#if item.is_folder}
                          <button
                            type="button"
                            class="folder-expand-toggle"
                            class:expanded={app.expandedFolderIds.includes(item.id)}
                            onclick={(e) => app.toggleExpandFolder(item.id, e)}
                            title={app.expandedFolderIds.includes(item.id) ? "Collapse folder files" : "Expand to view files"}
                          >
                            <span class="folder-arrow">{app.expandedFolderIds.includes(item.id) ? '▼' : '▶'}</span>
                            {#if item.total_files}
                              <span class="folder-file-badge">{item.total_files}</span>
                            {/if}
                          </button>
                        {/if}
                        {#if item.url && (item.url.includes('cdn.discordapp.com') || item.url.includes('media.discordapp.net'))}
                          <span class="discord-tag" title="Discord CDN Attachment">DISCORD</span>
                        {/if}
                        <span class="file-text">{item.filename || 'Resolving name...'}</span>
                      </div>
                    </td>
                    <td class="col-size font-mono">{getItemSize(item)}</td>
                    <td class="col-done font-mono">{formatBytes(item.downloaded_bytes)}</td>
                    <td class="col-prog" style="width: {app.colWidths.prog}px; max-width: {app.colWidths.prog}px;">
                      <div class="progress-cell" title={item.status === 'moving' ? `Moving file: ${(item.move_progress || 0).toFixed(1)}%` : item.status === 'compressing' ? `Compressing ZIP: ${(item.compression_progress || 0).toFixed(1)}%` : (item.is_folder && item.total_files ? `${(item.percentage || 0).toFixed(1)}% (${item.completed_files || 0} of ${item.total_files} files)` : `${(item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0)).toFixed(1)}%`)}>
                        <div class="native-progress-track">
                          <div
                            class="native-progress-fill"
                            class:prog-done={item.status === 'completed'}
                            class:prog-corrupt={item.status === 'corrupted'}
                            class:prog-paused={item.status === 'paused'}
                            class:prog-error={item.status === 'failed'}
                            class:prog-compressing={item.status === 'compressing'}
                            class:prog-moving={item.status === 'moving'}
                            style="width: {item.status === 'moving' ? (item.move_progress || 0) : item.status === 'compressing' ? (item.compression_progress || 0) : (item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0))}%"
                          ></div>
                        </div>
                        <span class="prog-label font-mono">
                          {#if item.status === 'moving'}
                            {(item.move_progress || 0).toFixed(0)}% (Moving)
                          {:else if item.status === 'compressing'}
                            {app.colWidths.prog >= 130 ? `${(item.compression_progress || item.percentage || 0).toFixed(0)}% (ZIP)` : `${(item.compression_progress || item.percentage || 0).toFixed(0)}%`}
                          {:else if item.is_folder && item.total_files}
                            {#if app.colWidths.prog >= 150}
                              {(item.percentage || 0).toFixed(0)}% ({item.completed_files || 0}/{item.total_files})
                            {:else if app.colWidths.prog >= 115}
                              {(item.percentage || 0).toFixed(0)}% [{item.completed_files || 0}/{item.total_files}]
                            {:else}
                              {(item.percentage || 0).toFixed(0)}%
                            {/if}
                          {:else}
                            {(item.percentage || (item.status === 'completed' || item.status === 'corrupted' ? 100 : 0)).toFixed(app.colWidths.prog >= 110 ? 1 : 0)}%
                          {/if}
                        </span>
                      </div>
                    </td>
                    <td class="col-status">
                      {#if item.status === 'corrupted'}
                        <span class="status-tag status-corrupted" title={item.error}>
                          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path>
                            <line x1="12" y1="9" x2="12" y2="13"></line>
                            <line x1="12" y1="17" x2="12.01" y2="17"></line>
                          </svg>CORRUPTED</span>
                      {:else if item.status === 'missing'}
                        <span class="status-tag status-missing" title={item.error || 'File might be deleted or moved'}>
                          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                            <circle cx="12" cy="12" r="10"></circle>
                            <line x1="12" y1="8" x2="12" y2="12"></line>
                            <line x1="12" y1="16" x2="12.01" y2="16"></line>
                          </svg>MISSING / MOVED</span>
                      {:else if item.status === 'compressing'}
                        <span class="status-tag status-compressing" title="Compressing into ZIP archive">
                          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                            <polyline points="21 8 21 21 3 21 3 8"></polyline>
                            <rect x="1" y="3" width="22" height="5"></rect>
                            <line x1="10" y1="12" x2="14" y2="12"></line>
                          </svg>COMPRESSING</span>
                      {:else if item.status === 'moving'}
                        <span class="status-tag status-moving" title="Moving file across disks/folders">
                          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin-right: 3px; vertical-align: middle;">
                            <path d="M5 12h14"></path>
                            <path d="M12 5l7 7-7 7"></path>
                          </svg>MOVING ({(item.move_progress || 0).toFixed(0)}%)</span>
                      {:else}
                        <span class="status-tag status-{item.status}">{item.status}</span>
                        {#if item.status === 'downloading' && item.chunks && item.chunks > 1}
                          <span class="chunks-badge" title="{item.chunks} Parallel Streams (Multi-Chunk / IDM Style)">{item.chunks} Chunks</span>
                        {/if}
                      {/if}
                    </td>
                    <td class="col-speed font-mono">
                      {item.status === 'downloading' || item.status === 'moving' ? formatSpeed(item.speed) : '--'}
                    </td>
                    <td class="col-eta font-mono">
                      {item.status === 'downloading' ? formatTime(item.eta_seconds) : '--'}
                    </td>
                    <td class="col-path" title={item.target_folder}>
                      {item.target_folder}
                    </td>
                    <td class="col-added font-mono" title="Added: {formatDateTime(item.created_at)} | Last Try: {formatDateTime(item.last_try_at)}">
                      {formatDateTime(item.last_try_at || item.created_at)}
                    </td>
                  </tr>
                {:else if row.type === 'child'}
                  <DownloadChildRow child={row.child} />
                {/if}
              {/each}
              {#if app.bottomSpacerHeight > 0}
                <tr class="spacer-row" style="height: {app.bottomSpacerHeight}px;"><td colspan="9" style="height: {app.bottomSpacerHeight}px; padding: 0; border: none;"></td></tr>
              {/if}
            {/if}
          </tbody>
        </table>
      </div>
