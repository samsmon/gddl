export function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export function formatSpeed(bytesPerSec) {
  if (!bytesPerSec || bytesPerSec === 0) return '0 KB/s';
  return formatBytes(bytesPerSec) + '/s';
}

export function formatTime(seconds) {
  if (!seconds || seconds <= 0 || !isFinite(seconds)) return '∞';
  if (seconds < 60) return `${Math.ceil(seconds)}s`;
  const mins = Math.floor(seconds / 60);
  const secs = Math.ceil(seconds % 60);
  return `${mins}m ${secs}s`;
}

export const formatETA = formatTime;

export function formatDateTime(dateStr) {
  if (!dateStr) return '--';
  const d = new Date(dateStr);
  if (isNaN(d.getTime())) return '--';
  const day = String(d.getDate()).padStart(2, '0');
  const mon = String(d.getMonth() + 1).padStart(2, '0');
  const yr = d.getFullYear();
  const hr = String(d.getHours()).padStart(2, '0');
  const min = String(d.getMinutes()).padStart(2, '0');
  return `${day}/${mon}/${yr} ${hr}:${min}`;
}

export function getItemSize(item) {
  if (!item) return '--';
  if (item.total_bytes > 0) return formatBytes(item.total_bytes);
  if (item.is_folder) {
    if (item.folder_files && item.folder_files.length > 0) {
      let sum = 0;
      let known = 0;
      for (const f of item.folder_files) {
        if (f.size && f.size > 0) {
          sum += f.size;
          known++;
        }
      }
      if (sum > 0) {
        return known === item.folder_files.length ? formatBytes(sum) : `~${formatBytes(sum)}`;
      }
    }
    return item.downloaded_bytes > 0 ? `~${formatBytes(item.downloaded_bytes)}` : '--';
  }
  return formatBytes(item.total_bytes);
}

export function getRowProgress(item) {
  if (!item) return 0;
  if (item.status === 'completed') return 100;
  return item.percentage ? Math.min(100, Math.max(0, item.percentage)) : 0;
}

export function getStatusInfo(status) {
  return {
    status: status || 'queued',
    label: (status || 'queued').toUpperCase()
  };
}

export function getFileIconInfo(item) {
  return {
    isFolder: Boolean(item?.is_folder),
    filename: item?.filename || item?.url || ''
  };
}

export function parseUrls(text) {
  if (!text) return [];
  return text.split(/\r?\n/).map(u => u.trim()).filter(Boolean);
}

export function parseCookieInput(raw) {
  if (!raw || typeof raw !== 'string') return '';
  let str = raw.trim();
  if (!str) return '';

  // 1. JSON array of cookies (e.g. Cookie-Editor / EditThisCookie export)
  if ((str.startsWith('[') && str.endsWith(']')) || (str.startsWith('{') && str.endsWith('}'))) {
    try {
      const parsed = JSON.parse(str);
      if (Array.isArray(parsed)) {
        const parts = [];
        for (const item of parsed) {
          if (item && item.name && item.value !== undefined) {
            parts.push(`${item.name}=${item.value}`);
          }
        }
        if (parts.length > 0) return parts.join('; ');
      } else if (parsed && typeof parsed === 'object') {
        if (parsed.cookie) return String(parsed.cookie).trim();
        if (parsed.Cookie) return String(parsed.Cookie).trim();
        const parts = [];
        for (const [k, v] of Object.entries(parsed)) {
          if (typeof v === 'string') parts.push(`${k}=${v}`);
        }
        if (parts.length > 0) return parts.join('; ');
      }
    } catch (_) {}
  }

  // 2. cURL (-H 'cookie: ...' or --header "Cookie: ...")
  const curlH = str.match(/(?:-H|--header)\s+[\$]?[\^'"]+(?:[Cc]ookie:\s*)([^\r\n'"\^]+)/i);
  if (curlH && curlH[1]) str = curlH[1].trim();
  else {
    // cURL (-b '...' or --cookie '...')
    const curlB = str.match(/(?:-b|--cookie)\s+[\$]?[\^'"]+([^\r\n'"\^]+)/i);
    if (curlB && curlB[1]) str = curlB[1].trim();
    else {
      // 3. PowerShell Invoke-WebRequest headers
      const psMatch = str.match(/["']cookie["']\s*=\s*["']([^"']+)["']/i);
      if (psMatch && psMatch[1]) str = psMatch[1].trim();
      else {
        // 4. Fetch headers
        const fetchMatch = str.match(/["']cookie["']:\s*["']([^"']+)["']/i);
        if (fetchMatch && fetchMatch[1]) str = fetchMatch[1].trim();
        else {
          // 5. Raw HTTP Request Headers line: Cookie: ...
          const headerMatch = str.match(/(?:^|\n)\s*cookie:\s*([^\r\n]+)/i);
          if (headerMatch && headerMatch[1]) str = headerMatch[1].trim();
        }
      }
    }
  }

  // Clean any invalid header chars (ASCII < 32 or >= 127) that trigger net/http: invalid header field value
  let cleaned = '';
  for (let i = 0; i < str.length; i++) {
    const code = str.charCodeAt(i);
    if (code >= 32 && code < 127) {
      cleaned += str[i];
    }
  }
  return cleaned.trim();
}
