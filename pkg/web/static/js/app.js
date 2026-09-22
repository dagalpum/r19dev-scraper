/**
 * R19DEV Studio - Main Application Bootstrap & Entry Point
 * Native ES Module
 */

import { state, elements, formatBytes } from './state.js';

import {
  scrapeMovie,
  scrapeAllMatched,
  toggleWatched,
  toggleFavorite,
  setRating,
  toggleFollowActress,
  unfollowActress,
  loadActressesData,
  loadDiscoveredActresses,
  switchActressSubTab,
  quickFollowDiscovered,
  refreshAllActresses,
  refreshSingleActress,
  trackTitleToActress,
  followActressFromInput,
  promptAddActress,
  openActiveActressFolder,
  unfollowActiveActress,
  promptUnfollowActress,
  trackTitleActiveActress,
  refreshActiveActress,
  organizeSingle,
  openFolder,
  openFolderByEl,
  copyMovieId,
  playMovie
} from './api.js';

import {
  setupModals,
  openMovieDetail,
  closeModal,
  openMovieById,
  renderModalContent,
  openGallery,
  openLightbox,
  closeLightbox,
  watchInBrowser,
  closeVideoPlayer
} from './modal.js';

import {
  setupDensityControl,
  applyGridDensity,
  setDensity,
  setupSearchAndFilters,
  resetFilters,
  clearSearch,
  setupUniversalSearch,
  onUniversalSearchInput,
  clearUniversalSearch,
  updateFloatingNavVisibility,
  syncUniversalSearchPlaceholder,
  fetchInitialData,
  startScanStream,
  rescanDirectory,
  renderMoviesGrid
} from './scanner.js';

import {
  setupOrganizer,
  startOrganize,
  toggleAutoScroll,
  copyOrganizerLog,
  clearOrganizerLog,
  resumeAutoScroll,
  openOrganizerDrawer,
  closeOrganizerDrawer,
  toggleOrganizerExpand
} from './organizer.js';

import {
  setupHistoryModal,
  openHistoryModal,
  closeHistoryModal,
  clearHistory,
  copyHistoryLog
} from './history.js';

import {
  setupActressHub,
  selectActressContact,
  openActressProfileDrawer,
  closeActressProfileDrawer,
  setActressViewMode,
  applyActressViewMode,
  filterCollectionActress,
  setCollectionSubFilter,
  toggleActressGenreFilter,
  clearActressGenreFilter,
  handleActressMovieSearch,
  setActressMovieSort,
  resetActressStageFilters,
  scrollShelf,
  renderActressCollection,
  renderActressHub,
  renderChatView,
  renderCatalogView,
  renderAllMoviesCatalogHtml,
  setAllMoviesStatus,
  setAllMoviesGenre,
  setAllMoviesStudio,
  setAllMoviesActress,
  setAllMoviesSort,
  setAllMoviesDensity,
  loadMoreAllMovies,
  showAllAllMovies,
  resetAllMoviesFilters
} from './actress.js';

import {
  setupNetworkGraph,
  openNetworkGraph,
  closeNetworkGraph
} from './graph.js';

// =========================================================================
// Tab Navigation
// =========================================================================
export function setupTabSwitching() {
  (elements.tabs || []).forEach(tab => {
    tab.addEventListener('click', () => {
      const tabId = tab.dataset.tab;
      switchTab(tabId);
    });
  });
}

export function switchTab(tabId) {
  if (tabId === 'organizer') {
    openOrganizerDrawer();
    return;
  }

  state.activeTab = tabId;
  try {
    localStorage.setItem('r19dev_active_tab', tabId);
  } catch (e) {}

  (elements.tabs || []).forEach(t => {
    const isCur = t.dataset.tab === tabId;
    t.classList.toggle('active', isCur);
    t.setAttribute('aria-selected', isCur ? 'true' : 'false');
  });
  (elements.panes || []).forEach(p => p.classList.toggle('active', p.id === `tab-${tabId}`));

  if (tabId === 'actresses') {
    loadActressesData();
    if (state.collectionFilterActress && state.collectionFilterActress !== 'all') {
      elements.navBreadcrumb?.classList.remove('hidden');
    } else {
      elements.navBreadcrumb?.classList.add('hidden');
    }
  } else if (tabId === 'catalog') {
    elements.navBreadcrumb?.classList.add('hidden');
    elements.floatingActressNav?.classList.add('hidden');
    loadActressesData().then(() => {
      renderCatalogView();
    });
  } else {
    elements.navBreadcrumb?.classList.add('hidden');
    elements.floatingActressNav?.classList.add('hidden');
  }

  syncUniversalSearchPlaceholder();
  updateFloatingNavVisibility();
}

// =========================================================================
// Initialization
// =========================================================================
export function init() {
  try { setupTabSwitching(); } catch (e) { console.error('setupTabSwitching failed:', e); }
  try { setupSearchAndFilters(); } catch (e) { console.error('setupSearchAndFilters failed:', e); }
  try { setupUniversalSearch(); } catch (e) { console.error('setupUniversalSearch failed:', e); }
  try { setupDensityControl(); } catch (e) { console.error('setupDensityControl failed:', e); }
  try { setupModals(); } catch (e) { console.error('setupModals failed:', e); }
  try { setupOrganizer(); } catch (e) { console.error('setupOrganizer failed:', e); }
  try { setupActressHub(); } catch (e) { console.error('setupActressHub failed:', e); }
  try { setupHistoryModal(); } catch (e) { console.error('setupHistoryModal failed:', e); }
  try { setupNetworkGraph(); } catch (e) { console.error('setupNetworkGraph failed:', e); }

  // Restore last active tab (default to 'catalog' so Library is shown instantly!)
  let savedTab = 'catalog';
  try {
    const stored = localStorage.getItem('r19dev_active_tab');
    if (stored && ['library', 'actresses', 'catalog'].includes(stored)) {
      savedTab = stored;
    }
  } catch (e) {}
  switchTab(savedTab);

  // Initial Data Fetch
  try { fetchInitialData(); } catch (e) { console.error('fetchInitialData failed:', e); }

  if (window.lucide) {
    try { window.lucide.createIcons(); } catch (e) {}
  }
}

// Expose global window.app for inline HTML event handlers & debugging
window.app = {
  // Navigation & Layout
  switchTab,
  setDensity,
  rescanDirectory,
  startScanStream,
  clearUniversalSearch,
  onUniversalSearchInput,

  // Library Controls
  clearSearch,
  scrapeAll: scrapeAllMatched,
  resetFilters,

  // Modals
  openMovie: openMovieById,
  closeModal,
  openGallery,
  openLightbox,
  closeLightbox,
  openHistoryModal,
  closeHistoryModal,
  clearHistory,
  copyHistoryLog,
  openNetworkGraph,
  closeNetworkGraph,

  // Movie Detail Actions
  toggleWatched,
  toggleFavorite,
  setRating,
  toggleFollowActress,
  organizeSingle,
  copyMovieId,
  playMovie,
  watchInBrowser,
  closeVideoPlayer,

  // Actress Hub: Collection & Filmography
  searchActresses: (q) => {
    state.actressSearchQuery = (q || '').trim();
    renderActressCollection();
  },
  setActressSort: (s) => {
    state.actressSort = s;
    const el = document.getElementById('actress-sort-by');
    if (el && el.value !== s) el.value = s;
    renderActressCollection();
  },
  setActressViewMode,
  filterCollectionActress,
  setCollectionSubFilter,
  toggleActressGenreFilter,
  clearActressGenreFilter,
  handleActressMovieSearch,
  setActressMovieSort,
  resetActressStageFilters,
  formatBytes,
  scrollShelf,
  promptAddActress,
  openMovieById,

  // Actress Hub & Chat UI Actions
  selectActressContact,
  followActressFromInput,
  refreshAllActresses,
  openActressProfileDrawer,
  closeActressProfileDrawer,
  unfollowActiveActress,
  unfollowActress,
  promptUnfollowActress,
  trackTitleActiveActress,
  trackTitleToActress,
  refreshActiveActress,
  refreshSingleActress,
  openActiveActressFolder,
  switchActressSubTab,
  quickFollowDiscovered,
  loadDiscoveredActresses,

  // All Movies Catalog Actions
  renderCatalogView,
  setAllMoviesStatus,
  setAllMoviesGenre,
  setAllMoviesStudio,
  setAllMoviesActress,
  setAllMoviesSort,
  setAllMoviesDensity,
  loadMoreAllMovies,
  showAllAllMovies,
  resetAllMoviesFilters,

  // File Management
  openFolder,
  openFolderEl: openFolderByEl,

  // NAS Organizer Drawer
  openOrganizerDrawer,
  closeOrganizerDrawer,
  toggleOrganizerExpand,
  startOrganize,
  toggleAutoScroll,
  copyOrganizerLog,
  clearOrganizerLog,
  resumeAutoScroll
};

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
