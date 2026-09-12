/**
 * R19DEV Studio - Media Scanner, Filter Engine & Library Grid
 * Native ES Module
 */

import { state, elements, escapeHtml } from './state.js';
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
  applyGridDensity(state.gridCols);

  (elements.densityButtons || []).forEach(btn => {
    btn.addEventListener('click', () => {
      const cols = btn.dataset.cols;
      state.gridCols = cols;
      localStorage.setItem('r19dev_grid_cols', cols);
      applyGridDensity(cols);
    });
  });
}

export function applyGridDensity(cols) {
  (elements.densityButtons || []).forEach(b => {
    const isCur = b.dataset.cols === cols;
    b.classList.toggle('active', isCur);
    b.setAttribute('aria-pressed', isCur ? 'true' : 'false');
  });

  if (elements.moviesGrid) {
    elements.moviesGrid.className = 'movies-grid ' + (cols === 'auto' ? 'grid-auto' : `grid-cols-${cols}`);
  }
}

export function setDensity(cols) {
  if (!cols) return;
  state.gridCols = cols;
  localStorage.setItem('r19dev_grid_cols', cols);
  applyGridDensity(cols);
}

export function setupSearchAndFilters() {
  elements.searchInput?.addEventListener('input', (e) => {
    state.searchQuery = e.target.value.trim().toLowerCase();
    elements.searchClear?.classList.toggle('hidden', state.searchQuery === '');
    if (elements.universalSearchInput && elements.universalSearchInput.value !== e.target.value) {
      elements.universalSearchInput.value = e.target.value;
      elements.universalSearchClear?.classList.toggle('hidden', e.target.value === '');
    }
    renderMoviesGrid();
  });

  elements.searchClear?.addEventListener('click', clearSearch);

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
    rescanDirectory();
  });

  if (elements.labelActiveDir) {
    const badge = elements.labelActiveDir.closest('.dir-badge');
    if (badge) {
      badge.style.cursor = 'pointer';
      badge.title = 'Click to change scan directory';
      badge.addEventListener('click', (e) => {
        e.stopPropagation();
        rescanDirectory();
      });
    }
  }

  elements.btnScrapeAll?.addEventListener('click', scrapeAllMatched);
  elements.btnOrganizeAll?.addEventListener('click', () => {
    window.app?.switchTab('organizer');
  });
}

export function resetFilters() {
  state.filterActress = '';
  state.filterOrganized = 'all';
  state.filterScraped = 'all';
  state.filterGenre = '';
  state.filterWatch = 'all';
  state.searchQuery = '';

  if (elements.searchInput) elements.searchInput.value = '';
  if (elements.universalSearchInput) elements.universalSearchInput.value = '';
  if (elements.searchClear) elements.searchClear.classList.add('hidden');
  if (elements.universalSearchClear) elements.universalSearchClear.classList.add('hidden');
  if (elements.filterActress) elements.filterActress.value = '';
  if (elements.filterOrganized) elements.filterOrganized.value = 'all';
  if (elements.filterScraped) elements.filterScraped.value = 'all';
  if (elements.filterGenre) elements.filterGenre.value = '';
  if (elements.filterWatch) elements.filterWatch.value = 'all';

  renderMoviesGrid();
}

export function clearSearch() {
  if (elements.searchInput) elements.searchInput.value = '';
  if (elements.universalSearchInput) elements.universalSearchInput.value = '';
  state.searchQuery = '';
  elements.searchClear?.classList.add('hidden');
  elements.universalSearchClear?.classList.add('hidden');
  elements.searchInput?.focus();
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
    if (elements.searchInput && elements.searchInput.value !== rawVal) {
      elements.searchInput.value = rawVal;
      elements.searchClear?.classList.toggle('hidden', rawVal === '');
    }
    renderMoviesGrid();
  } else if (state.activeTab === 'actresses') {
    if (state.collectionFilterActress === 'all') {
      state.actressSearchQuery = q;
      renderActressCollection();
    } else {
      state.actressMovieSearch = q;
      renderActressCollection();
      const stageInput = document.getElementById('actress-movie-search');
      if (stageInput && stageInput.value !== rawVal) {
        stageInput.value = rawVal;
      }
    }
  } else if (state.activeTab === 'organizer') {
    if (window.app?.switchTab) window.app.switchTab('library');
    state.searchQuery = q.toLowerCase();
    if (elements.searchInput) {
      elements.searchInput.value = rawVal;
      elements.searchClear?.classList.toggle('hidden', rawVal === '');
    }
    renderMoviesGrid();
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
    elements.universalSearchInput.placeholder = `Search library, SKU, actress... (${kbdText})`;
    elements.universalSearchInput.value = elements.searchInput ? elements.searchInput.value : (state.searchQuery || '');
  } else if (state.activeTab === 'actresses') {
    if (state.collectionFilterActress === 'all') {
      elements.universalSearchInput.placeholder = `Search followed actresses... (${kbdText})`;
      elements.universalSearchInput.value = state.actressSearchQuery || '';
    } else {
      elements.universalSearchInput.placeholder = `Search ${state.collectionFilterActress}'s filmography... (${kbdText})`;
      elements.universalSearchInput.value = state.actressMovieSearch || '';
    }
  } else {
    elements.universalSearchInput.placeholder = `Search movies, SKU, actresses... (${kbdText})`;
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

export async function fetchInitialData() {
  startScanStream();
  loadActressesData();
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
    ? `🔍 Scanning ${state.activeDir}...`
    : '🔍 Discovering video files...';

  let url = '/api/scan/stream';
  if (state.activeDir && state.activeDir !== '.') {
    url += '?path=' + encodeURIComponent(state.activeDir);
  }
  const es = new EventSource(url);
  state.scanEventSource = es;

  es.addEventListener('progress', (e) => {
    try {
      const data = JSON.parse(e.data);
      elements.scanProgressLabel.textContent = `🔍 Discovered ${data.discovered} videos (${data.matched} matched)...`;
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
      elements.scanProgressLabel.textContent = `✅ Scan complete: ${state.groupedMovies.length} movies (${state.rawMatches.length} files found)`;

      updateStats();
      populateFilterDropdowns();
      renderMoviesGrid();

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

export function rescanDirectory() {
  const current = state.activeDir || '';
  const newPath = prompt('Enter folder path to scan for JAV files:', current);
  if (newPath !== null && newPath.trim() !== '') {
    startScanStream(newPath.trim());
  }
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
    partBadge = `<span class="badge-status badge-multipart" title="${movie.files.length} video files"><i data-lucide="layers"></i> ${partStr}</span>`;
  }

  // Scraped Badge
  let scrapedBadge = '';
  if (isScraped) {
    scrapedBadge = `<span class="badge-status badge-scraped" title="Metadata scraped from R18.dev"><i data-lucide="check-circle-2"></i> Scraped</span>`;
  } else if (movie.id) {
    scrapedBadge = `<span class="badge-status badge-unscraped" title="Needs metadata scrape"><i data-lucide="sparkles"></i> Unscraped</span>`;
  } else {
    scrapedBadge = `<span class="badge-status badge-staging" title="No JAV ID matched"><i data-lucide="help-circle"></i> Unmatched</span>`;
  }

  // Organized Badge
  let organizedBadge = '';
  if (isOrganized) {
    organizedBadge = `<span class="badge-status badge-organized clickable" title="Organized in Jellyfin (Click to open in Finder)" onclick="event.stopPropagation(); window.app.openFolder('${id}')" role="button" tabindex="0"><i data-lucide="folder-check"></i> Organized ↗</span>`;
  } else if (movie.id) {
    organizedBadge = `<span class="badge-status badge-staging" title="Pending NAS organize"><i data-lucide="inbox"></i> Staging</span>`;
  }

  // Watched & Fav Badges
  const watchedCoverBadge = uState.is_watched ? '<span class="badge-status badge-watched" title="Watched"><i data-lucide="eye"></i></span>' : '';
  const favCoverBadge = uState.is_favorite ? '<span class="badge-status badge-fav" title="Favorited"><i data-lucide="heart"></i></span>' : '';
  const ratingStr = uState.user_rating ? '⭐'.repeat(uState.user_rating) : '';

  // Date Badge
  let dateBadge = '';
  if (meta?.release_date) {
    dateBadge = `<span class="card-date-badge is-release" title="Official Release Date"><i data-lucide="calendar"></i> Rel: ${escapeHtml(meta.release_date)}</span>`;
  } else if (movie.files[0]?.mod_time) {
    dateBadge = `<span class="card-date-badge" title="File Creation / Modification Date"><i data-lucide="clock"></i> File: ${escapeHtml(movie.files[0].mod_time.slice(0, 10))}</span>`;
  }

  // Actresses
  const actressesHtml = (meta?.actresses || []).slice(0, 3).map(act => {
    const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
    return `<span class="actress-chip ${isFollowed ? 'followed' : ''}">${isFollowed ? '⭐ ' : ''}${escapeHtml(act.name)}</span>`;
  }).join('');

  card.innerHTML = `
    <div class="card-cover-wrapper">
      <img class="card-cover-img" src="${coverUrl}" alt="Jacket cover for ${escapeHtml(id)}" onerror="this.src='/placeholder.png'" loading="lazy" />
      <div class="card-overlay-badge">${escapeHtml(id)}</div>
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
