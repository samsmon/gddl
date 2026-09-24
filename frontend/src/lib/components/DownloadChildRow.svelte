<script>
  import { app } from '../state/appState.svelte.js';
  import { getItemSize, formatBytes, formatSpeed, formatTime, formatDateTime, parseCookieInput } from '../utils/formatters.js';
  let { child } = $props();
</script>

                  <tr class="child-file-row">
                    <td class="col-name" title={child.filename}>
                      <div class="child-name-cell">
                        <span class="tree-line">└─</span>
                        <span class="child-filename">{child.filename}</span>
                      </div>
                    </td>
                    <td class="col-size font-mono">{child.size ? formatBytes(child.size) : '--'}</td>
                    <td class="col-done font-mono">{child.status === 'completed' && child.size ? formatBytes(child.size) : '--'}</td>
                    <td class="col-prog" style="width: {app.colWidths.prog}px; max-width: {app.colWidths.prog}px;">
                      <div class="child-prog-wrap" title="{child.status === 'completed' ? '100% Completed' : (child.status === 'downloading' ? 'Active downloading' : 'Pending')}">
                        <div class="native-progress-track">
                          <div
                            class="native-progress-fill"
                            class:prog-done={child.status === 'completed'}
                            class:prog-active={child.status === 'downloading'}
                            style="width: {child.status === 'completed' ? 100 : (child.status === 'downloading' ? 65 : 0)}%"
                          ></div>
                        </div>
                        <span class="child-prog-text font-mono">
                          {child.status === 'completed' ? '100%' : (child.status === 'downloading' ? (app.colWidths.prog >= 120 ? 'Active' : '...') : '0%')}
                        </span>
                      </div>
                    </td>
                    <td class="col-status">
                      <span class="child-status-pill child-status-{child.status}">
                        {child.status.toUpperCase()}
                      </span>
                    </td>
                    <td class="col-speed font-mono">--</td>
                    <td class="col-eta font-mono">--</td>
                    <td class="col-path font-mono">
                      <span class="in-archive-label">↳ Inside Archive</span>
                    </td>
                    <td class="col-added font-mono">--</td>
                  </tr>
