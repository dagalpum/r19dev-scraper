/**
 * R19DEV Studio - State Management & Shared Utilities
 * Native ES Module
 */

// Global Application State
export const state = {
  activeTab: 'library',
  activeDir: '',
  rawMatches: [],     // Raw matches from backend
  groupedMovies: [],  // Grouped by JAV ID
  metadata: {},       // ID -> scraper.Movie
  userStates: {},     // ID -> db.UserState
  organizedStatus: {}, // ID -> bool
  organizedFolders: {}, // ID -> folder path
  currentGallery: [],  // PhotoSwipe items for open modal
  actresses: [],      // Followed actresses
  filterActress: '',
  filterOrganized: 'all',
  filterScraped: 'all',
  filterGenre: '',
  filterWatch: 'all',
  sort: 'date-desc',
  searchQuery: '',
  gridCols: localStorage.getItem('r19dev_grid_cols') || 'auto',
  selectedMovieId: null,
  isScanning: false,
  scanEventSource: null,
  orgEventSource: null,
  logAutoScroll: true,
  activeActressName: null,
  actressSearchQuery: '',
  actressViewMode: 'collection',
  actressSort: 'latest-desc',
  collectionFilterActress: 'all',
  collectionSubFilter: 'all',
  actressMovieSearch: '',
  actressMovieSort: 'date-desc',
  actressGenreFilter: null,
  actressHubTab: 'followed', // 'followed' | 'unfollowed'
  discoveredActresses: [],   // Unfollowed actresses with files in NAS
  allMoviesFilterStatus: 'library', // Default to 'library' (Ready to Watch in NAS)
  allMoviesFilterGenre: '',
  allMoviesFilterStudio: '',
  allMoviesFilterActress: '',
  allMoviesSort: 'date-desc',
  allMoviesSearch: '',
  allMoviesViewDensity: 'grid',
  allMoviesPageLimit: 72,
};

// Cached DOM Selectors
export const elements = {
  tabs: document.querySelectorAll('.nav-tab'),
  panes: document.querySelectorAll('.tab-pane'),
  countLibrary: document.getElementById('count-library'),
  countActresses: document.getElementById('count-actresses'),
  countCatalog: document.getElementById('count-catalog'),
  labelActiveDir: document.getElementById('label-active-dir'),
  btnRescan: document.getElementById('btn-rescan'),

  // Universal Search & Sticky Navigation
  navBreadcrumb: document.getElementById('nav-breadcrumb'),
  navBreadcrumbActressName: document.getElementById('nav-breadcrumb-actress-name'),
  navSearchBox: document.getElementById('nav-search-box'),
  navSearchScope: document.getElementById('nav-search-scope'),
  universalSearchInput: document.getElementById('universal-search-input'),
  navSearchKbd: document.getElementById('nav-search-kbd'),
  universalSearchClear: document.getElementById('universal-search-clear'),
  floatingActressNav: document.getElementById('floating-actress-nav'),
  btnNavOrganize: document.getElementById('btn-nav-organize'),

  // Progress Bars
  scanProgressBox: document.getElementById('scan-progress-box'),
  scanProgressLabel: document.getElementById('scan-progress-label'),
  scanProgressPct: document.getElementById('scan-progress-pct'),
  scanProgressFill: document.getElementById('scan-progress-fill'),
  scanProgressBar: document.getElementById('scan-progress-bar'),

  orgProgressBox: document.getElementById('org-progress-box'),
  orgProgressLabel: document.getElementById('org-progress-label'),
  orgProgressPct: document.getElementById('org-progress-pct'),
  orgProgressFill: document.getElementById('org-progress-fill'),
  orgProgressBar: document.getElementById('org-progress-bar'),

  // Live Operation Activity Progress
  opProgressBox: document.getElementById('op-progress-box'),
  opIcon: document.getElementById('op-icon'),
  opTitle: document.getElementById('op-title'),
  opCounter: document.getElementById('op-counter'),
  opPct: document.getElementById('op-pct'),
  opMessage: document.getElementById('op-message'),
  opProgressFill: document.getElementById('op-progress-fill'),

  // Library Controls
  searchInput: document.getElementById('search-input'),
  searchClear: document.getElementById('search-clear'),
  filterActress: document.getElementById('filter-actress'),
  filterOrganized: document.getElementById('filter-organized'),
  filterScraped: document.getElementById('filter-scraped'),
  filterGenre: document.getElementById('filter-genre'),
  filterWatch: document.getElementById('filter-watch'),
  btnResetFilters: document.getElementById('btn-reset-filters'),
  sortSelect: document.getElementById('sort-by') || document.getElementById('sort-select'),
  densityButtons: document.querySelectorAll('.btn-density'),
  densityToggles: document.querySelectorAll('.btn-density-toggle'),
  btnGridShowcase: document.getElementById('btn-grid-showcase'),
  btnGridDense: document.getElementById('btn-grid-dense'),
  btnScrapeAll: document.getElementById('btn-scrape-all'),
  btnOrganizeAll: document.getElementById('btn-organize-all'),
  statTotal: document.getElementById('stat-total'),
  statMatched: document.getElementById('stat-matched'),
  statUnmatched: document.getElementById('stat-unmatched'),
  statScraped: document.getElementById('stat-scraped'),
  moviesGrid: document.getElementById('movies-grid'),
  libraryEmpty: document.getElementById('library-empty'),

  // Actress Hub (Dual Mode: Collection & Chat)
  btnModeCollection: document.getElementById('btn-mode-collection'),
  btnModeChat: document.getElementById('btn-mode-chat'),
  btnHubRefresh: document.getElementById('btn-hub-refresh'),
  btnHubAdd: document.getElementById('btn-hub-add'),
  actressCollectionView: document.getElementById('actress-collection-view'),
  actressChatView: document.getElementById('actress-chat-view'),
  collectionActressChips: document.getElementById('collection-actress-chips'),
  collectionActressHero: document.getElementById('collection-actress-hero'),
  collectionShelvesContainer: document.getElementById('collection-shelves-container'),

  // Actress Chat Hub
  inputActressName: document.getElementById('input-actress-name'),
  btnAddActress: document.getElementById('btn-add-actress'),
  btnRefreshActresses: document.getElementById('btn-refresh-actresses'),
  chatFriendsCount: document.getElementById('chat-friends-count'),
  chatActressSearch: document.getElementById('chat-actress-search'),
  chatContactsList: document.getElementById('chat-contacts-list'),
  actressesEmpty: document.getElementById('actresses-empty'),

  chatActiveHeader: document.getElementById('chat-active-header'),
  chatHeaderAvatar: document.getElementById('chat-header-avatar'),
  chatHeaderName: document.getElementById('chat-header-name'),
  chatHeaderJa: document.getElementById('chat-header-ja'),
  chatHeaderSubtitle: document.getElementById('chat-header-subtitle'),
  chatHeaderControls: document.getElementById('chat-header-controls'),
  btnOpenProfileDrawer: document.getElementById('btn-open-profile-drawer'),
  linkHeaderR18: document.getElementById('link-header-r18'),
  btnHeaderUnfollow: document.getElementById('btn-header-unfollow'),

  chatMessagesFeed: document.getElementById('chat-messages-feed'),
  chatActionBar: document.getElementById('chat-action-bar'),
  btnChatTrackTitle: document.getElementById('btn-chat-track-title'),
  btnChatRefresh: document.getElementById('btn-chat-refresh'),
  btnChatOpenFolder: document.getElementById('btn-chat-open-folder'),

  drawerProfileBackdrop: document.getElementById('drawer-profile-backdrop'),
  drawerActressProfile: document.getElementById('drawer-actress-profile'),
  btnCloseDrawer: document.getElementById('btn-close-drawer'),
  drawerContent: document.getElementById('drawer-content'),

  // NAS Organizer Drawer
  drawerOrganizer: document.getElementById('drawer-organizer'),
  drawerOrganizerBackdrop: document.getElementById('drawer-organizer-backdrop'),
  btnCloseOrganizer: document.getElementById('btn-close-organizer'),
  btnExpandOrganizer: document.getElementById('btn-expand-organizer'),
  iconExpandOrganizer: document.getElementById('icon-expand-organizer'),
  orgSrcDir: document.getElementById('org-src-dir'),
  orgDestRoot: document.getElementById('org-dest-root'),
  orgDryRun: document.getElementById('org-dry-run'),
  btnStartOrganize: document.getElementById('btn-start-organize'),
  organizerLog: document.getElementById('organizer-log'),
  btnClearLog: document.getElementById('btn-clear-log'),
  btnCopyLog: document.getElementById('btn-copy-log'),
  btnViewHistory: document.getElementById('btn-view-history'),
  btnToggleAutoscroll: document.getElementById('btn-toggle-autoscroll'),
  btnResumeScroll: document.getElementById('btn-resume-scroll'),
  logScrollBadge: document.getElementById('log-scroll-badge'),

  // Modals
  modalMovie: document.getElementById('modal-movie'),
  modalContent: document.getElementById('modal-content'),
  btnCloseModal: document.getElementById('btn-close-modal'),
  modalHistory: document.getElementById('modal-history'),
  btnCloseHistory: document.getElementById('btn-close-history'),
  btnClearHistory: document.getElementById('btn-clear-history'),
  historyListContainer: document.getElementById('history-list-container'),
  historyDetailTitle: document.getElementById('history-detail-title'),
  historyLogView: document.getElementById('history-log-view'),
  btnCopyHistoryLog: document.getElementById('btn-copy-history-log'),
  lightbox: document.getElementById('lightbox'),
  lightboxImg: document.getElementById('lightbox-img'),
  lightboxCaption: document.getElementById('lightbox-caption'),
  btnCloseLightbox: document.getElementById('btn-close-lightbox'),
  toastContainer: document.getElementById('toast-container'),
};

// UI Formatting Utilities
export function escapeHtml(str) {
  if (!str) return '';
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

export function formatBytes(bytes, decimals = 1) {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

export function cleanMovieId(id) {
  if (!id) return '';
  return id.replace(/[-_]/g, '').toLowerCase();
}

export function getHighResScreenshotUrl(url) {
  if (!url) return '';
  return url.replace(/-(\d+)\.jpg$/i, 'jp-$1.jpg');
}

export function getDefaultOrganizedDestination(activeDir) {
  if (!activeDir) return '/Volumes/home/BT/organized';
  if (activeDir.startsWith('/Volumes/home/BT')) {
    return '/Volumes/home/BT/organized';
  }
  const parts = activeDir.replace(/\\/g, '/').split('/').filter(Boolean);
  if (parts.length > 1) {
    parts.pop();
    return '/' + parts.join('/') + '/organized';
  }
  return activeDir + '/organized';
}

export function getMovieLocationInfo(movieId, explicitFolder = '', isDownloaded = null) {
  if (!movieId) {
    return {
      type: 'missing',
      label: 'Missing',
      icon: 'cancel',
      badgeClass: 'badge-missing',
      folderPath: '',
      destRoot: '',
      isStandard: false,
      titleText: 'Missing from Storage',
      subText: 'No local video file found for this title'
    };
  }

  const folderPath = explicitFolder || state.organizedFolders[movieId] || '';
  const destRoot = elements.orgDestRoot?.value?.trim() || getDefaultOrganizedDestination(state.activeDir);

  const cleanFolder = (folderPath || '').replace(/\\/g, '/');
  const cleanDest = (destRoot || '').replace(/\\/g, '/');

  if (cleanFolder) {
    // Standard Jellyfin Library location
    if (cleanFolder.startsWith(cleanDest)) {
      return {
        type: 'library',
        label: 'In Library',
        icon: 'check_circle',
        badgeClass: 'badge-library',
        folderPath: folderPath,
        destRoot: destRoot,
        isStandard: true,
        titleText: 'In Library (Ready for playback)',
        subText: 'Stored in Jellyfin NAS Library'
      };
    }

    // Outside standard library root (e.g. /Archive/..., /JD/..., /Misc/...)
    let locName = 'Archive';
    const lower = cleanFolder.toLowerCase();
    if (lower.includes('/archive')) {
      locName = 'Archive';
    } else if (lower.includes('/misc')) {
      locName = 'Misc';
    } else if (lower.includes('/sorted')) {
      locName = 'Sorted';
    } else if (lower.includes('/jd')) {
      locName = 'JD';
    } else {
      const parts = cleanFolder.split('/').filter(Boolean);
      if (parts.length > 2) {
        locName = parts[parts.length - 2];
      } else {
        locName = 'External';
      }
    }

    return {
      type: 'external',
      label: `In ${locName}`,
      locName: locName,
      icon: 'inventory_2',
      badgeClass: 'badge-archive',
      folderPath: folderPath,
      destRoot: destRoot,
      isStandard: false,
      titleText: `External Storage (Stored in ${locName})`,
      subText: `Metadata ready, located outside ${destRoot}`
    };
  }

  const isOrganized = Boolean(state.organizedStatus[movieId]);
  if (isOrganized) {
    return {
      type: 'library',
      label: 'In Library',
      icon: 'check_circle',
      badgeClass: 'badge-library',
      folderPath: destRoot,
      destRoot: destRoot,
      isStandard: true,
      titleText: 'In Library (Ready for playback)',
      subText: 'Stored in Jellyfin NAS Library'
    };
  }

  // Check if file exists in scanned active staging batch or explicit dl flag
  const hasLocalMatch = state.groupedMovies.some(m => m.id && m.id.toUpperCase() === movieId.toUpperCase());
  const isDl = isDownloaded !== null ? Boolean(isDownloaded) : hasLocalMatch;

  if (!isDl && !hasLocalMatch) {
    return {
      type: 'missing',
      label: 'Missing',
      icon: 'cancel',
      badgeClass: 'badge-missing',
      folderPath: '',
      destRoot: destRoot,
      isStandard: false,
      titleText: 'Missing from Storage',
      subText: 'No local video file found for this title'
    };
  }

  // Staging / pending organization (file is present in scanned staging directory)
  return {
    type: 'staging',
    label: 'Staging',
    icon: 'inbox',
    badgeClass: 'badge-staging',
    folderPath: '',
    destRoot: destRoot,
    isStandard: false,
    titleText: 'Pending Organization',
    subText: state.activeDir ? `Located in incoming folder: ${state.activeDir}` : 'Waiting to organize into Jellyfin'
  };
}

export function showToast(message, type = 'info') {
  const container = elements.toastContainer || document.getElementById('toast-container');
  if (!container) return;

  const toast = document.createElement('div');
  toast.className = 'toast';
  toast.setAttribute('role', 'status');

  let iconName = 'info';
  if (type === 'success') iconName = 'check_circle';
  if (type === 'warning') iconName = 'warning';
  if (type === 'danger') iconName = 'error';

  toast.innerHTML = `<span class="material-symbols-outlined icon">${iconName}</span> <span>${escapeHtml(message)}</span>`;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateY(10px)';
    toast.style.transition = 'all 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}
