export class CoreState {
  downloads = $state([]);
  activeFilter = $state('all');
  hideCompleted = $state(false);
  searchQuery = $state('');
  selectedIds = $state([]);
  lastClickedId = $state(null);
  defaultFolder = $state('C:\\Users\\Sam\\Downloads');
  maxConcurrency = $state(2);
  chunksPerDownload = $state(4);
  backendConnected = $state(false);
  currentTheme = $state('dark');
  hasLogin = $state(false);
  authChecked = $state(false);
  authEnabled = $state(true);
  isAuthenticated = $state(false);
  currentAuthUser = $state('admin');
  loginUsername = $state('admin');
  loginPassword = $state('');
  loginError = $state('');
  isLoggingIn = $state(false);
  authEnabledToggle = $state(true);
  changeOldPassword = $state('');
  changeNewUsername = $state('admin');
  changeNewPassword = $state('');
  changeConfirmPassword = $state('');
  isUpdatingSecurity = $state(false);
  securityMessage = $state('');
  securityError = $state('');
  sortColumn = $state('added');
  sortDirection = $state('desc');
  colWidths = $state({
    name: 280,
    size: 90,
    done: 90,
    prog: 160,
    status: 110,
    speed: 95,
    eta: 80,
    path: 200,
    added: 135
  });
  openMenu = $state(null);
  showDocModal = $state(false);
  showUpdateModal = $state(false);
  showAboutModal = $state(false);
  updateChecking = $state(false);
  updateStatus = $state('');
  showLogsModal = $state(false);
  logs = $state([]);
  logFilter = $state('ALL');
  logSearch = $state('');
  isFetchingLogs = $state(false);
  logsAutoRefresh = $state(true);
  copiedLogs = $state(false);
  showAddModal = $state(false);
  showSettingsModal = $state(false);
  showLoginModal = $state(false);
  showFolderPicker = $state(false);
  folderPickerTarget = $state('add');
  showDeleteModal = $state(false);
  deleteModalTarget = $state('selected');
  deleteModalSingleId = $state(null);
  deleteModalWithFile = $state(false);
  showDiscordExportModal = $state(false);
  showDiscordRefreshModal = $state(false);
  unfinishedDiscordItems = $state([]);
  isFetchingDiscord = $state(false);
  discordExportFormat = $state('json');
  discordExportCopied = $state(false);
  discordRefreshInput = $state('');
  isRefreshingDiscord = $state(false);
  discordRefreshResult = $state(null);
  showConflictModal = $state(false);
  conflictList = $state([]);
  conflictResolutions = $state({});
  expandedFolderIds = $state([]);
  showContextMenu = $state(false);
  contextMenuX = $state(0);
  contextMenuY = $state(0);
  contextMenuItem = $state(null);
  addLinksInput = $state('');
  addTargetFolder = $state('C:\\Users\\Sam\\Downloads');
  isAdding = $state(false);
  loginCookieInput = $state('');
  isSavingLogin = $state(false);
  detectedFolder = $state(null);
  folderZipMode = $state('zip');
  isResolvingFolder = $state(false);
  resolveError = $state('');
  resolveTimer = null;
  eventSource = null;
  pollInterval = null;
  tableContainerEl = $state(null);
  scrollTop = $state(0);
  viewportHeight = $state(600);
  ROW_HEIGHT = 31;
  OVERSCAN = 15;
  autoWarpEnabled = $state(true);
  autoWarpMinSpeedMB = $state(5);
  warpProxyPort = $state(40000);
  customProxyURL = $state('');
  isRotatingWarp = $state(false);
  warpStatus = $state({
    installed: false,
    binary_path: '',
    auto_enabled: true,
    proxy_active: false,
    warp_connected: false,
    is_rotating: false,
    min_speed_mb: 5.0,
    proxy_port: 40000,
    custom_proxy_url: '',
    active_proxy_url: '',
    rotation_count: 0,
    low_speed_duration_sec: 0,
    last_reason: ''
  });
  confirmDialog = $state({
    show: false,
    title: '',
    message: '',
    confirmText: 'Confirm',
    confirmType: 'danger',
    onConfirm: null
  });
  cookieList = $state([]);
  newCookieLabel = $state('');
  newCookieValue = $state('');
  isAddingCookie = $state(false);
  cookieError = $state('');
  cookieMessage = $state('');
  wasCookieExtracted = $state(false);
  accountModalTab = $state('oauth');
  oauthConfigured = $state(false);
  oauthConnected = $state(false);
  oauthClientID = $state('');
  oauthClientSecret = $state('');
  oauthEmail = $state('');
  oauthAutoBypass = $state(true);
  oauthManualCode = $state('');
  rcloneTokenInput = $state('');
  isSavingOAuthConfig = $state(false);
  isSubmittingManualCode = $state(false);
  isImportingRcloneToken = $state(false);
  isCleaningTempFolder = $state(false);
  oauthMessage = $state('');
  oauthError = $state('');
  oauthAuthURL = $state('');
  showClientSecret = $state(false);
  oauthRedirectURI = $state('');
  oauthRedirectURIOverride = $state('');
  copiedRedirectURI = $state(false);
  copiedRowId = $state(null);
  bulkAddingStatus = $state('');
  relocateItemTargetId = $state(null);

  constructor() {
    try {
      const savedHide = localStorage.getItem('gddl_hide_completed');
      if (savedHide !== null) this.hideCompleted = savedHide === 'true';
    } catch (_) {}
  }
}
