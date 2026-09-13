/**
 * R19DEV Studio - Media Scanner, Filter Engine & Library Grid
 * Native ES Module
 */

import { state, elements, escapeHtml, getMovieLocationInfo, getDefaultOrganizedDestination } from './state.js';
export { getDefaultOrganizedDestination };
import {
  loadActressesData,
  scrapeMovie,
  scrapeAllMatched,
  toggleWatched,
  toggleFavorite,
  organizeSingle,
  openFolder
} from './api.js';
import { openMovieDetail } from './modal.js';
import { renderActressCollection, filterCollectionActress } from './actress.js';

export function setupDensityControl() {
  const savedDensity = localStorage.getItem('r19dev_density') || (state.gridCols === 'compact' ? 'compact' : 'auto');
  applyGridDensity(savedDensity);

  (elements.densityToggles || []).forEach(btn => {
    btn.addEventListener('click', () => {
      const density = btn.dataset.density || 'auto';
      setDensity(density);
    });
  });
}

export function applyGridDensity(density) {
  const activeDensity = (density === 'compact') ? 'compact' : 'auto';
  state.gridCols = activeDensity;

  (elements.densityToggles || []).forEach(b => {
    const isCur = (b.dataset.density || 'auto') === activeDensity;
    b.classList.toggle('active', isCur);
    b.setAttribute('aria-pressed', isCur ? 'true' : 'false');
  });

  if (elements.moviesGrid) {
    elements.moviesGrid.className = 'movies-grid ' + (activeDensity === 'compact' ? 'grid-compact' : 'grid-auto');
  }
}

export function setDensity(density) {
  if (!density) return;
  const activeDensity = (density === 'compact') ? 'compact' : 'auto';
  state.gridCols = activeDensity;
  localStorage.setItem('r19dev_density', activeDensity);
  applyGridDensity(activeDensity);
}

export function setupSearchAndFilters() {
  elements.filterActress?.addEventListener('change', (e) => {
    state.filterActress = e.target.value;
    renderMoviesGrid();
  });

  elements.filterOrganized?.addEventListener('change', (e) => {
    state.filterOrganized = e.target.value;
    renderMoviesGrid();
  });

  elements.filterScraped?.addEventListener('change', (e) => {
    state.filterScraped = e.target.value;
    renderMoviesGrid();
  });

  elements.filterGenre?.addEventListener('change', (e) => {
    state.filterGenre = e.target.value;
    renderMoviesGrid();
  });

  elements.filterWatch?.addEventListener('change', (e) => {
    state.filterWatch = e.target.value;
    renderMoviesGrid();
  });

  elements.btnResetFilters?.addEventListener('click', resetFilters);

  elements.sortSelect?.addEventListener('change', (e) => {
    state.sort = e.target.value;
    renderMoviesGrid();
  });

  elements.btnRescan?.addEventListener('click', (e) => {
    e.stopPropagation();
    rescanDirectory(false);
  });

  if (elements.labelActiveDir) {
    const badge = elements.labelActiveDir.closest('.dir-badge');
    if (badge) {
      badge.style.cursor = 'pointer';
      badge.title = 'Click to change scan directory';
      badge.addEventListener('click', (e) => {
        e.stopPropagation();
        rescanDirectory(true);
      });
    }
  }

  elements.btnScrapeAll?.addEventListener('click', scrapeAllMatched);
  elements.btnOrganizeAll?.addEventListener('click', () => {
    window.app?.openOrganizerDrawer();
  });
  elements.btnNavOrganize?.addEventListener('click', () => {
    window.app?.openOrganizerDrawer();
  });
}

export function resetFilters() {
  state.filterActress = '';
  state.filterOrganized = 'all';
  state.filterScraped = 'all';
  state.filterGenre = '';
  state.filterWatch = 'all';
  state.searchQuery = '';

  if (elements.universalSearchInput) elements.universalSearchInput.value = '';
  if (elements.universalSearchClear) elements.universalSearchClear.classList.add('hidden');
  if (elements.filterActress) elements.filterActress.value = '';
  if (elements.filterOrganized) elements.filterOrganized.value = 'all';
  if (elements.filterScraped) elements.filterScraped.value = 'all';
  if (elements.filterGenre) elements.filterGenre.value = '';
  if (elements.filterWatch) elements.filterWatch.value = 'all';

  renderMoviesGrid();
}

export function clearSearch() {
  if (elements.universalSearchInput) elements.universalSearchInput.value = '';
  state.searchQuery = '';
  elements.universalSearchClear?.classList.add('hidden');
  elements.universalSearchInput?.focus();
  renderMoviesGrid();
}

export function setupUniversalSearch() {
  const isMac = (navigator.platform || '').toUpperCase().indexOf('MAC') >= 0 || (navigator.userAgent || '').toUpperCase().indexOf('MAC') >= 0;
  if (elements.navSearchKbd) {
    elements.navSearchKbd.textContent = isMac ? '⌘K' : 'Ctrl+K';
  }

  elements.universalSearchInput?.addEventListener('input', (e) => {
    onUniversalSearchInput(e.target.value);
  });

  elements.universalSearchClear?.addEventListener('click', clearUniversalSearch);

  // Global keyboard shortcuts: ⌘K / Ctrl+K and '/'
  window.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      elements.universalSearchInput?.focus();
      elements.universalSearchInput?.select();
      return;
    }
    if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(document.activeElement?.tagName)) {
      e.preventDefault();
      elements.universalSearchInput?.focus();
      elements.universalSearchInput?.select();
      return;
    }
    if (e.key === 'Escape' && document.activeElement === elements.universalSearchInput) {
      if (elements.universalSearchInput.value) {
        clearUniversalSearch();
      } else {
        elements.universalSearchInput.blur();
      }
    }
  });

  // Floating Actress Navigation Pill Scroll Listener
  window.addEventListener('scroll', updateFloatingNavVisibility, { passive: true });

  // History popstate listener for back/forward navigation
  window.addEventListener('popstate', (e) => {
    const s = e.state;
    if (s && s.tab) {
      if (state.activeTab !== s.tab && window.app?.switchTab) window.app.switchTab(s.tab);
      if (s.tab === 'actresses') {
        filterCollectionActress(s.actress || 'all', false);
      }
    } else {
      const params = new URLSearchParams(window.location.search);
      const actParam = params.get('actress');
      const tabParam = params.get('tab');
      if (tabParam && tabParam !== state.activeTab && window.app?.switchTab) {
        window.app.switchTab(tabParam);
      }
      if (actParam) {
        if (state.activeTab !== 'actresses' && window.app?.switchTab) window.app.switchTab('actresses');
        filterCollectionActress(actParam, false);
      } else if (state.activeTab === 'actresses' && state.collectionFilterActress !== 'all') {
        filterCollectionActress('all', false);
      }
    }
  });

  // Initial check from query parameters
  const params = new URLSearchParams(window.location.search);
  const initialTab = params.get('tab');
  const initialActress = params.get('actress');
  if (initialTab && ['library', 'actresses', 'organizer'].includes(initialTab) && window.app?.switchTab) {
    window.app.switchTab(initialTab);
  }
  if (initialActress) {
    if (state.activeTab !== 'actresses' && window.app?.switchTab) window.app.switchTab('actresses');
    state.collectionFilterActress = initialActress;
    state.activeActressName = initialActress;
  }

  syncUniversalSearchPlaceholder();
}

export function onUniversalSearchInput(val) {
  const rawVal = val || '';
  const q = rawVal.trim();
  elements.universalSearchClear?.classList.toggle('hidden', rawVal === '');

  if (state.activeTab === 'library') {
    state.searchQuery = q.toLowerCase();
    renderMoviesGrid();
  } else if (state.activeTab === 'catalog') {
    state.allMoviesSearch = q;
    if (window.app?.renderCatalogView) {
      window.app.renderCatalogView();
    }
  } else if (state.activeTab === 'actresses') {
    if (state.collectionFilterActress === 'all') {
      state.actressSearchQuery = q;
      renderActressCollection();
    } else {
      state.actressMovieSearch = q;
      renderActressCollection();
    }
  }
}

export function clearUniversalSearch() {
  if (elements.universalSearchInput) {
    elements.universalSearchInput.value = '';
  }
  elements.universalSearchClear?.classList.add('hidden');
  onUniversalSearchInput('');
  elements.universalSearchInput?.focus();
}

export function updateFloatingNavVisibility() {
  if (!elements.floatingActressNav) return;
  const isActressDetail = state.activeTab === 'actresses' && state.collectionFilterActress && state.collectionFilterActress !== 'all';
  const scrolledDown = window.scrollY > 300;
  elements.floatingActressNav.classList.toggle('hidden', !(isActressDetail && scrolledDown));
}

export function syncUniversalSearchPlaceholder() {
  if (!elements.universalSearchInput) return;
  const isMac = (navigator.platform || '').toUpperCase().indexOf('MAC') >= 0 || (navigator.userAgent || '').toUpperCase().indexOf('MAC') >= 0;
  const kbdText = isMac ? '⌘K' : 'Ctrl+K';

  if (state.activeTab === 'library') {
    if (elements.navSearchScope) elements.navSearchScope.textContent = 'Incoming';
    elements.universalSearchInput.placeholder = `Search incoming videos, SKU, actress... (${kbdText})`;
    elements.universalSearchInput.value = state.searchQuery || '';
  } else if (state.activeTab === 'catalog') {
    if (elements.navSearchScope) elements.navSearchScope.textContent = 'Library';
    elements.universalSearchInput.placeholder = `Search library & catalog movies by ID, title, actress, studio... (${kbdText})`;
    elements.universalSearchInput.value = state.allMoviesSearch || '';
  } else if (state.activeTab === 'actresses') {
    if (state.collectionFilterActress === 'all') {
      if (state.actressHubTab === 'unfollowed' || state.actressHubTab === 'discovered') {
        if (elements.navSearchScope) elements.navSearchScope.textContent = 'Unfollowed';
        elements.universalSearchInput.placeholder = `Search unfollowed actresses in NAS... (${kbdText})`;
      } else {
        if (elements.navSearchScope) elements.navSearchScope.textContent = 'Followed';
        elements.universalSearchInput.placeholder = `Search followed actresses... (${kbdText})`;
      }
      elements.universalSearchInput.value = state.actressSearchQuery || '';
    } else {
      if (elements.navSearchScope) elements.navSearchScope.textContent = state.collectionFilterActress;
      elements.universalSearchInput.placeholder = `Search ${state.collectionFilterActress}'s filmography... (${kbdText})`;
      elements.universalSearchInput.value = state.actressMovieSearch || '';
    }
  } else {
    if (elements.navSearchScope) elements.navSearchScope.textContent = 'Search';
    elements.universalSearchInput.placeholder = `Search videos, SKU, actresses... (${kbdText})`;
    elements.universalSearchInput.value = '';
  }
  elements.universalSearchClear?.classList.toggle('hidden', !elements.universalSearchInput.value);
}

export function groupMatches(matches) {
  const groups = new Map();
  const unmatched = [];

  matches.forEach(item => {
    if (!item.id) {
      unmatched.push({
        id: '',
        matchedBy: 'none',
        files: [item.file],
        totalSize: item.file?.size || 0,
        isMultiPart: false,
        partNumbers: []
      });
      return;
    }

    const key = item.id.toUpperCase();
    if (!groups.has(key)) {
      groups.set(key, {
        id: key,
        matchedBy: item.matched_by,
        files: [],
        totalSize: 0,
        isMultiPart: false,
        partNumbers: []
      });
    }

    const g = groups.get(key);
    g.files.push(item.file);
    g.totalSize += (item.file?.size || 0);

    if (item.is_multi_part) {
      g.isMultiPart = true;
      if (item.part_number && !g.partNumbers.includes(item.part_number)) {
        g.partNumbers.push(item.part_number);
      }
    }
  });

  // If a movie has multiple files even without strict part tags, flag as multipart
  groups.forEach(g => {
    if (g.files.length > 1) {
      g.isMultiPart = true;
    }
    g.partNumbers.sort((a, b) => a - b);
  });

  return [...groups.values(), ...unmatched];
}

export async function fetchInitialData() {
  // 1. Instantly load actress & library catalog data from DB (0.01s)
  loadActressesData();

  // 2. Check if we have cached scan results in localStorage for 0s instant load
  let hasCached = false;
  try {
    const cachedStr = localStorage.getItem('r19dev_scan_cache');
    if (cachedStr) {
      const cached = JSON.parse(cachedStr);
      if (cached && Array.isArray(cached.rawMatches) && cached.rawMatches.length > 0) {
        state.rawMatches = cached.rawMatches;
        state.activeDir = cached.activeDir || state.activeDir;
        if (elements.labelActiveDir) elements.labelActiveDir.textContent = state.activeDir;
        if (elements.orgSrcDir) elements.orgSrcDir.value = state.activeDir;
        if (elements.orgDestRoot && !elements.orgDestRoot.value) {
          elements.orgDestRoot.value = getDefaultOrganizedDestination(state.activeDir);
        }

        if (cached.metadata) Object.assign(state.metadata, cached.metadata);
        if (cached.userStates) Object.assign(state.userStates, cached.userStates);
        if (cached.organizedStatus) Object.assign(state.organizedStatus, cached.organizedStatus);
        if (cached.organizedFolders) Object.assign(state.organizedFolders, cached.organizedFolders);

        state.groupedMovies = groupMatches(state.rawMatches);
        updateStats();
        populateFilterDropdowns();
        renderMoviesGrid();

        if (elements.countLibrary) {
          elements.countLibrary.textContent = state.groupedMovies.length;
        }
        hasCached = true;
      }
    }
  } catch (e) {
    console.warn('Failed to restore scan cache:', e);
  }

  // If no cached scan data, don't run auto-scan across network NAS; render clean empty state
  if (!hasCached) {
    if (elements.scanProgressBox) elements.scanProgressBox.classList.add('hidden');
    renderMoviesGrid();
  }
}

export function startScanStream(customPath) {
  if (customPath) {
    state.activeDir = customPath;
  }
  if (state.scanEventSource) {
    state.scanEventSource.close();
  }

  state.isScanning = true;
  elements.scanProgressBox.classList.remove('hidden');
  elements.scanProgressFill.style.width = '10%';
  elements.scanProgressPct.textContent = 'Scanning...';
  elements.scanProgressLabel.textContent = state.activeDir
    ? `Scanning ${state.activeDir}...`
    : 'Discovering video files...';

  let url = '/api/scan/stream';
  if (state.activeDir && state.activeDir !== '.') {
    url += '?path=' + encodeURIComponent(state.activeDir);
  }
  const es = new EventSource(url);
  state.scanEventSource = es;

  es.addEventListener('progress', (e) => {
    try {
      const data = JSON.parse(e.data);
      elements.scanProgressLabel.textContent = `Discovered ${data.discovered} videos (${data.matched} matched)...`;
      elements.scanProgressFill.style.width = '60%';
    } catch (err) {}
  });

  es.addEventListener('done', (e) => {
    try {
      const data = JSON.parse(e.data);
      state.rawMatches = data.matches || [];
      state.activeDir = data.target_dir || state.activeDir;
      elements.labelActiveDir.textContent = state.activeDir;
      elements.orgSrcDir.value = state.activeDir;
      if (!elements.orgDestRoot.value) {
        elements.orgDestRoot.value = getDefaultOrganizedDestination(state.activeDir);
      }

      if (data.metadata) Object.assign(state.metadata, data.metadata);
      if (data.user_states) Object.assign(state.userStates, data.user_states);
      if (data.organized_status) Object.assign(state.organizedStatus, data.organized_status);
      if (data.organized_folders) Object.assign(state.organizedFolders, data.organized_folders);

      state.groupedMovies = groupMatches(state.rawMatches);

      elements.scanProgressFill.style.width = '100%';
      elements.scanProgressPct.textContent = '100%';
      elements.scanProgressLabel.textContent = `Scan complete: ${state.groupedMovies.length} movies (${state.rawMatches.length} files found)`;

      updateStats();
      populateFilterDropdowns();
      renderMoviesGrid();

      if (elements.countLibrary) {
        elements.countLibrary.textContent = state.groupedMovies.length;
      }

      // Cache scan results to localStorage for 0s instant load next time
      try {
        const cachePayload = {
          rawMatches: state.rawMatches,
          activeDir: state.activeDir,
          metadata: state.metadata,
          userStates: state.userStates,
          organizedStatus: state.organizedStatus,
          organizedFolders: state.organizedFolders,
          timestamp: Date.now()
        };
        localStorage.setItem('r19dev_scan_cache', JSON.stringify(cachePayload));
      } catch (err) {
        console.warn('Failed to save scan cache:', err);
      }

      setTimeout(() => {
        elements.scanProgressBox.classList.add('hidden');
      }, 1200);
    } catch (err) {
      console.error('Error handling scan done event:', err);
    } finally {
      es.close();
      state.scanEventSource = null;
      state.isScanning = false;
    }
  });

  es.onerror = () => {
    elements.scanProgressBox.classList.add('hidden');
    es.close();
    state.scanEventSource = null;
    state.isScanning = false;
  };
}

export function rescanDirectory(promptForPath = false) {
  if (promptForPath) {
    const current = state.activeDir || '';
    const newPath = prompt('Enter folder path to scan for JAV files:', current);
    if (newPath !== null && newPath.trim() !== '') {
      startScanStream(newPath.trim());
    }
    return;
  }
  startScanStream(state.activeDir);
}

export function updateStats() {
  const total = state.groupedMovies.length;
  let matched = 0;
  let scraped = 0;

  state.groupedMovies.forEach(m => {
    if (m.id) {
      matched++;
      if (state.metadata[m.id]) scraped++;
    }
  });

  elements.statTotal.textContent = total;
  elements.statMatched.textContent = matched;
  elements.statUnmatched.textContent = total - matched;
  elements.statScraped.textContent = scraped;
  elements.countLibrary.textContent = total;
}

export function populateFilterDropdowns() {
  if (!elements.filterActress || !elements.filterGenre) return;

  const actressCounts = new Map();
  const genreCounts = new Map();

  state.groupedMovies.forEach(movie => {
    const meta = state.metadata[movie.id];
    if (meta) {
      (meta.actresses || []).forEach(act => {
        if (act && act.name) {
          actressCounts.set(act.name, (actressCounts.get(act.name) || 0) + 1);
        }
      });
      (meta.genres || []).forEach(g => {
        if (g) {
          genreCounts.set(g, (genreCounts.get(g) || 0) + 1);
        }
      });
    }
  });

  // Populate Actress Filter
  const curActress = elements.filterActress.value;
  const sortedActresses = Array.from(actressCounts.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  elements.filterActress.innerHTML = `<option value="">All Actresses (${sortedActresses.length})</option>` +
    sortedActresses.map(([name, count]) => `<option value="${escapeHtml(name)}">${escapeHtml(name)} (${count})</option>`).join('');
  if (actressCounts.has(curActress)) {
    elements.filterActress.value = curActress;
  }

  // Populate Genre / Tag Filter
  const curGenre = elements.filterGenre.value;
  const sortedGenres = Array.from(genreCounts.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  elements.filterGenre.innerHTML = `<option value="">All Tags / Genres (${sortedGenres.length})</option>` +
    sortedGenres.map(([genre, count]) => `<option value="${escapeHtml(genre)}">${escapeHtml(genre)} (${count})</option>`).join('');
  if (genreCounts.has(curGenre)) {
    elements.filterGenre.value = curGenre;
  }
}

export function renderMoviesGrid() {
  let list = state.groupedMovies.filter(movie => {
    const meta = state.metadata[movie.id];
    const uState = state.userStates[movie.id] || {};
    const isOrganized = Boolean(state.organizedStatus[movie.id]);
    const isScraped = Boolean(meta);

    // Search Query
    if (state.searchQuery) {
      const id = (movie.id || '').toLowerCase();
      const fnames = (movie.files || []).map(f => (f.name || '').toLowerCase()).join(' ');
      const title = (meta?.title || '').toLowerCase();
      const studio = (meta?.maker || '').toLowerCase();
      const actNames = (meta?.actresses || []).map(a => a.name.toLowerCase()).join(' ');
      const tags = (meta?.genres || []).map(g => g.toLowerCase()).join(' ');

      const match = id.includes(state.searchQuery) ||
                    fnames.includes(state.searchQuery) ||
                    title.includes(state.searchQuery) ||
                    studio.includes(state.searchQuery) ||
                    actNames.includes(state.searchQuery) ||
                    tags.includes(state.searchQuery);
      if (!match) return false;
    }

    // Actress Filter
    if (state.filterActress) {
      const actNames = (meta?.actresses || []).map(a => a.name);
      if (!actNames.includes(state.filterActress)) return false;
    }

    // Organized Filter
    if (state.filterOrganized === 'organized' && !isOrganized) return false;
    if (state.filterOrganized === 'library') {
      const loc = getMovieLocationInfo(movie.id);
      if (loc.type !== 'library') return false;
    }
    if (state.filterOrganized === 'archive' || state.filterOrganized === 'external') {
      const loc = getMovieLocationInfo(movie.id);
      if (loc.type !== 'external') return false;
    }
    if (state.filterOrganized === 'staging' && isOrganized) return false;

    // Scraped Filter
    if (state.filterScraped === 'scraped' && !isScraped) return false;
    if (state.filterScraped === 'unscraped' && (isScraped || !movie.id)) return false;
    if (state.filterScraped === 'unmatched' && movie.id) return false;

    // Genre / Tag Filter
    if (state.filterGenre) {
      const genres = meta?.genres || [];
      if (!genres.includes(state.filterGenre)) return false;
    }

    // Watch Status Filter
    if (state.filterWatch === 'watched' && !uState.is_watched) return false;
    if (state.filterWatch === 'unwatched' && uState.is_watched) return false;
    if (state.filterWatch === 'favorites' && !uState.is_favorite) return false;

    return true;
  });

  // Check if any filter is active to show Reset button
  const hasActiveFilter = Boolean(
    state.filterActress ||
    (state.filterOrganized && state.filterOrganized !== 'all') ||
    (state.filterScraped && state.filterScraped !== 'all') ||
    state.filterGenre ||
    (state.filterWatch && state.filterWatch !== 'all') ||
    state.searchQuery
  );
  if (elements.btnResetFilters) {
    elements.btnResetFilters.classList.toggle('hidden', !hasActiveFilter);
  }

  // Sort
  list.sort((a, b) => {
    if (state.sort === 'id-asc') return (a.id || '').localeCompare(b.id || '');
    if (state.sort === 'id-desc') return (b.id || '').localeCompare(a.id || '');
    if (state.sort === 'name-asc') return (a.files[0]?.name || '').localeCompare(b.files[0]?.name || '');
    if (state.sort === 'size-desc') return (b.totalSize || 0) - (a.totalSize || 0);
    if (state.sort === 'date-desc') {
      const dateA = state.metadata[a.id]?.release_date || (a.files[0]?.mod_time ? a.files[0].mod_time.slice(0, 10) : '');
      const dateB = state.metadata[b.id]?.release_date || (b.files[0]?.mod_time ? b.files[0].mod_time.slice(0, 10) : '');
      return dateB.localeCompare(dateA);
    }
    return 0;
  });

  elements.moviesGrid.innerHTML = '';
  elements.libraryEmpty.classList.toggle('hidden', list.length > 0);

  list.forEach(movie => {
    const card = createMovieCard(movie);
    elements.moviesGrid.appendChild(card);
  });

  if (window.lucide) {
    window.lucide.createIcons();
  }
}

export function createMovieCard(movie) {
  const card = document.createElement('article');
  card.className = 'movie-card';
  card.tabIndex = 0;
  card.setAttribute('role', 'article');
  card.setAttribute('aria-label', `${movie.id || 'Unmatched'} - ${state.metadata[movie.id]?.title || movie.files[0]?.name || ''}`);

  const id = movie.id || 'UNMATCHED';
  const meta = state.metadata[movie.id];
  const uState = state.userStates[movie.id] || {};
  const isOrganized = Boolean(state.organizedStatus[movie.id]);
  const isScraped = Boolean(meta);
  const sizeMB = Math.round((movie.totalSize || 0) / (1024 * 1024));

  const coverUrl = meta?.cover_url || meta?.poster_url || '/api/images/' + id;
  const title = meta?.title || movie.files[0]?.name || 'Unknown Title';
  const maker = meta?.maker || '';

  // Multi-part indicator
  let partBadge = '';
  if (movie.isMultiPart || movie.files.length > 1) {
    const partStr = movie.partNumbers.length > 0 ? `P${movie.partNumbers.join(', P')}` : `${movie.files.length} parts`;
    partBadge = `<span class="badge-status badge-multipart" title="${movie.files.length} video files"><span class="material-symbols-outlined icon">layers</span> ${partStr}</span>`;
  }

  // Scraped Badge
  let scrapedBadge = '';
  if (isScraped) {
    scrapedBadge = `<span class="badge-status badge-scraped" title="Metadata scraped from R18.dev"><span class="material-symbols-outlined icon">check_circle</span> Scraped</span>`;
  } else if (movie.id) {
    scrapedBadge = `<span class="badge-status badge-unscraped" title="Needs metadata scrape"><span class="material-symbols-outlined icon">bolt</span> Unscraped</span>`;
  } else {
    scrapedBadge = `<span class="badge-status badge-staging" title="No JAV ID matched"><span class="material-symbols-outlined icon">help</span> Unmatched</span>`;
  }

  // Organized & Storage Location Badge
  let organizedBadge = '';
  const loc = getMovieLocationInfo(movie.id);
  if (loc.type === 'library') {
    organizedBadge = `<span class="badge-status badge-library clickable" title="In Library (/organized) - Click to open in Finder" onclick="event.stopPropagation(); window.app.openFolder('${id}')" role="button" tabindex="0"><span class="material-symbols-outlined icon">check_circle</span> In Library ↗</span>`;
  } else if (loc.type === 'external') {
    organizedBadge = `<span class="badge-status badge-archive clickable" title="Outside Library: ${escapeHtml(loc.folderPath)} - Click to open in Finder" onclick="event.stopPropagation(); window.app.openFolder('${id}')" role="button" tabindex="0"><span class="material-symbols-outlined icon">inventory_2</span> ${escapeHtml(loc.label)} ↗</span>`;
  } else if (movie.id) {
    organizedBadge = `<span class="badge-status badge-staging" title="Pending NAS organize"><span class="material-symbols-outlined icon">inbox</span> Staging</span>`;
  }

  // Watched & Fav Badges
  const watchedCoverBadge = uState.is_watched ? '<span class="badge-status badge-watched" title="Watched"><span class="material-symbols-outlined icon">visibility</span></span>' : '';
  const favCoverBadge = uState.is_favorite ? '<span class="badge-status badge-fav" title="Favorited"><span class="material-symbols-outlined icon">favorite</span></span>' : '';
  const ratingStr = uState.user_rating ? Array.from({length: uState.user_rating}).map(() => '<span class="material-symbols-outlined icon star-filled">star</span>').join('') : '';

  // Date Badge
  let dateBadge = '';
  if (meta?.release_date) {
    dateBadge = `<span class="card-date-badge is-release" title="Official Release Date"><span class="material-symbols-outlined icon">calendar_month</span> Rel: ${escapeHtml(meta.release_date)}</span>`;
  } else if (movie.files[0]?.mod_time) {
    dateBadge = `<span class="card-date-badge" title="File Creation / Modification Date"><span class="material-symbols-outlined icon">schedule</span> File: ${escapeHtml(movie.files[0].mod_time.slice(0, 10))}</span>`;
  }

  // Actresses
  const actressesHtml = (meta?.actresses || []).slice(0, 3).map(act => {
    const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
    return `<span class="actress-chip ${isFollowed ? 'followed' : ''}">${isFollowed ? '<span class="material-symbols-outlined icon star-icon">star</span> ' : ''}${escapeHtml(act.name)}</span>`;
  }).join('');

  card.innerHTML = `
    <div class="card-cover-wrapper">
      <img class="card-cover-img" src="${coverUrl}" alt="Jacket cover for ${escapeHtml(id)}" onerror="this.src='/placeholder.png'" loading="lazy" />
      <div class="card-overlay-badge ${loc.badgeClass}" title="${escapeHtml(loc.titleText)}">
        <span class="material-symbols-outlined icon" style="font-size: 0.72rem; vertical-align: -1px; margin-right: 2px;">${loc.icon}</span>${escapeHtml(id)}
      </div>
      <div class="card-overlay-status">
        ${partBadge}
        ${watchedCoverBadge}
        ${favCoverBadge}
      </div>
    </div>
    <div class="card-content">
      <h3 class="card-title" title="${escapeHtml(title)}">${escapeHtml(title)}</h3>
      <div class="card-meta">
        <span>${escapeHtml(maker)}</span>
        <span>${sizeMB} MB</span>
        ${dateBadge}
      </div>
      <div class="card-actresses">
        ${actressesHtml}
      </div>
      <div class="card-footer">
        <div class="card-footer-status">
          ${scrapedBadge}
          ${organizedBadge}
          ${ratingStr ? `<span class="rating-display">${ratingStr}</span>` : ''}
        </div>
        <div class="card-actions" onclick="event.stopPropagation()">
          <button class="btn-icon-sm btn-scrape" title="Scrape metadata from R18.dev" aria-label="Scrape metadata for ${escapeHtml(id)}">
            <i data-lucide="sparkles"></i> Scrape
          </button>
          <button class="btn-icon-sm btn-fav ${uState.is_favorite ? 'active' : ''}" title="Toggle Favorite" aria-label="Favorite ${escapeHtml(id)}">
            <i data-lucide="heart"></i>
          </button>
          <button class="btn-icon-sm btn-watched ${uState.is_watched ? 'active' : ''}" title="Toggle Watched" aria-label="Toggle watched status for ${escapeHtml(id)}">
            <i data-lucide="eye"></i>
          </button>
          <button class="btn-icon-sm btn-organize-quick ${isOrganized ? 'active' : ''}" title="${isOrganized ? 'Open folder in Finder' : 'Organize for NAS Jellyfin'}" aria-label="${isOrganized ? 'Open folder in Finder' : 'Organize'} ${escapeHtml(id)}">
            <i data-lucide="${isOrganized ? 'folder-open' : 'folder-output'}"></i>
          </button>
        </div>
      </div>
    </div>
  `;

  // Click card to open full-width hero cover modal
  card.addEventListener('click', () => {
    openMovieDetail(movie);
  });

  card.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      openMovieDetail(movie);
    }
  });

  // Quick Actions
  card.querySelector('.btn-scrape')?.addEventListener('click', async (e) => {
    e.stopPropagation();
    await scrapeMovie(movie.id);
  });

  card.querySelector('.btn-fav')?.addEventListener('click', async (e) => {
    e.stopPropagation();
    await toggleFavorite(movie.id);
  });

  card.querySelector('.btn-watched')?.addEventListener('click', async (e) => {
    e.stopPropagation();
    await toggleWatched(movie.id);
  });

  card.querySelector('.btn-organize-quick')?.addEventListener('click', async (e) => {
    e.stopPropagation();
    if (isOrganized) {
      await openFolder(movie.id);
    } else {
      await organizeSingle(movie.id);
    }
  });

  return card;
}
