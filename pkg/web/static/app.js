/**
 * R19DEV Studio - Frontend Application Engine
 * Enhanced with Multi-Part File Grouping, Grid Density Control,
 * Live SSE Progress Bars, Full-Width Hero Cover, and WCAG Accessibility.
 */

(function () {
  'use strict';

  // Application State
  const state = {
    activeTab: 'library',
    activeDir: '.',
    rawMatches: [],     // Raw matches from backend
    groupedMovies: [],  // Grouped by JAV ID
    metadata: {},       // ID -> scraper.Movie
    userStates: {},     // ID -> db.UserState
    actresses: [],      // Followed actresses
    filter: 'all',
    sort: 'id-asc',
    searchQuery: '',
    gridCols: localStorage.getItem('r19dev_grid_cols') || 'auto',
    selectedMovieId: null,
    isScanning: false,
    scanEventSource: null,
    orgEventSource: null,
  };

  // DOM Selectors
  const elements = {
    tabs: document.querySelectorAll('.nav-tab'),
    panes: document.querySelectorAll('.tab-pane'),
    countLibrary: document.getElementById('count-library'),
    countActresses: document.getElementById('count-actresses'),
    labelActiveDir: document.getElementById('label-active-dir'),
    btnRescan: document.getElementById('btn-rescan'),
    
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

    // Library Controls
    searchInput: document.getElementById('search-input'),
    searchClear: document.getElementById('search-clear'),
    filterStatus: document.getElementById('filter-status'),
    sortBy: document.getElementById('sort-by'),
    densityButtons: document.querySelectorAll('.btn-density'),
    btnScrapeAll: document.getElementById('btn-scrape-all'),
    btnOrganizeAll: document.getElementById('btn-organize-all'),
    statTotal: document.getElementById('stat-total'),
    statMatched: document.getElementById('stat-matched'),
    statUnmatched: document.getElementById('stat-unmatched'),
    statScraped: document.getElementById('stat-scraped'),
    moviesGrid: document.getElementById('movies-grid'),
    libraryEmpty: document.getElementById('library-empty'),

    // Actresses
    inputActressName: document.getElementById('input-actress-name'),
    btnAddActress: document.getElementById('btn-add-actress'),
    actressesContainer: document.getElementById('actresses-container'),
    actressesEmpty: document.getElementById('actresses-empty'),

    // Organizer
    orgSrcDir: document.getElementById('org-src-dir'),
    orgDestRoot: document.getElementById('org-dest-root'),
    orgDryRun: document.getElementById('org-dry-run'),
    btnStartOrganize: document.getElementById('btn-start-organize'),
    organizerLog: document.getElementById('organizer-log'),
    btnClearLog: document.getElementById('btn-clear-log'),

    // Modals
    modalMovie: document.getElementById('modal-movie'),
    modalContent: document.getElementById('modal-content'),
    btnCloseModal: document.getElementById('btn-close-modal'),
    lightbox: document.getElementById('lightbox'),
    lightboxImg: document.getElementById('lightbox-img'),
    lightboxCaption: document.getElementById('lightbox-caption'),
    btnCloseLightbox: document.getElementById('btn-close-lightbox'),
    toastContainer: document.getElementById('toast-container'),
  };

  // =========================================================================
  // Initialization
  // =========================================================================
  function init() {
    setupTabSwitching();
    setupSearchAndFilters();
    setupDensityControl();
    setupModals();
    setupOrganizer();
    setupActressHub();

    // Initial Data Fetch
    fetchInitialData();
  }

  function setupTabSwitching() {
    elements.tabs.forEach(tab => {
      tab.addEventListener('click', () => {
        const tabId = tab.dataset.tab;
        switchTab(tabId);
      });
    });
  }

  function switchTab(tabId) {
    state.activeTab = tabId;
    elements.tabs.forEach(t => {
      const isCur = t.dataset.tab === tabId;
      t.classList.toggle('active', isCur);
      t.setAttribute('aria-selected', isCur ? 'true' : 'false');
    });
    elements.panes.forEach(p => p.classList.toggle('active', p.id === `tab-${tabId}`));

    if (tabId === 'actresses') {
      loadActressesData();
    }
  }

  function setupDensityControl() {
    // Apply saved density
    applyGridDensity(state.gridCols);

    elements.densityButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        const cols = btn.dataset.cols;
        state.gridCols = cols;
        localStorage.setItem('r19dev_grid_cols', cols);
        applyGridDensity(cols);
      });
    });
  }

  function applyGridDensity(cols) {
    elements.densityButtons.forEach(b => {
      const isCur = b.dataset.cols === cols;
      b.classList.toggle('active', isCur);
      b.setAttribute('aria-pressed', isCur ? 'true' : 'false');
    });

    elements.moviesGrid.className = 'movies-grid ' + (cols === 'auto' ? 'grid-auto' : `grid-cols-${cols}`);
  }

  function setupSearchAndFilters() {
    elements.searchInput.addEventListener('input', (e) => {
      state.searchQuery = e.target.value.trim().toLowerCase();
      elements.searchClear.classList.toggle('hidden', state.searchQuery === '');
      renderMoviesGrid();
    });

    elements.searchClear.addEventListener('click', () => {
      elements.searchInput.value = '';
      state.searchQuery = '';
      elements.searchClear.classList.add('hidden');
      elements.searchInput.focus();
      renderMoviesGrid();
    });

    elements.filterStatus.addEventListener('change', (e) => {
      state.filter = e.target.value;
      renderMoviesGrid();
    });

    elements.sortBy.addEventListener('change', (e) => {
      state.sort = e.target.value;
      renderMoviesGrid();
    });

    elements.btnRescan.addEventListener('click', () => {
      startScanStream();
    });

    elements.btnScrapeAll.addEventListener('click', () => {
      scrapeAllMatched();
    });

    elements.btnOrganizeAll.addEventListener('click', () => {
      switchTab('organizer');
    });
  }

  function setupModals() {
    elements.btnCloseModal.addEventListener('click', closeModal);
    elements.modalMovie.addEventListener('click', (e) => {
      if (e.target === elements.modalMovie) closeModal();
    });

    elements.btnCloseLightbox.addEventListener('click', closeLightbox);
    elements.lightbox.addEventListener('click', (e) => {
      if (e.target === elements.lightbox) closeLightbox();
    });

    document.addEventListener('keydown', (e) => {
      if (e.key === 'Escape') {
        if (!elements.lightbox.classList.contains('hidden')) {
          closeLightbox();
        } else if (!elements.modalMovie.classList.contains('hidden')) {
          closeModal();
        }
      }
    });
  }

  function setupOrganizer() {
    elements.btnClearLog.addEventListener('click', () => {
      elements.organizerLog.textContent = 'Console cleared.';
    });

    elements.btnStartOrganize.addEventListener('click', () => {
      startOrganizeStream();
    });
  }

  function setupActressHub() {
    elements.btnAddActress.addEventListener('click', async () => {
      const name = elements.inputActressName.value.trim();
      if (!name) return;
      try {
        await fetch('/api/actresses/follow', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: name })
        });
        elements.inputActressName.value = '';
        showToast(`Followed ${name} ⭐`, 'success');
        loadActressesData();
      } catch (err) {
        showToast('Error following actress: ' + err.message, 'danger');
      }
    });
  }

  // =========================================================================
  // Multi-Part & Multi-File Aggregation
  // =========================================================================
  function groupMatches(matches) {
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

  // =========================================================================
  // Live Streaming Progress (Scan & Organize)
  // =========================================================================
  async function fetchInitialData() {
    startScanStream();
    loadActressesData();
  }

  function startScanStream() {
    if (state.scanEventSource) {
      state.scanEventSource.close();
    }

    state.isScanning = true;
    elements.scanProgressBox.classList.remove('hidden');
    elements.scanProgressFill.style.width = '10%';
    elements.scanProgressPct.textContent = 'Scanning...';
    elements.scanProgressLabel.textContent = '🔍 Discovering video files...';

    const es = new EventSource('/api/scan/stream?path=' + encodeURIComponent(state.activeDir));
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
          elements.orgDestRoot.value = state.activeDir + '/JAV_Library';
        }

        if (data.metadata) Object.assign(state.metadata, data.metadata);
        if (data.user_states) Object.assign(state.userStates, data.user_states);

        state.groupedMovies = groupMatches(state.rawMatches);

        elements.scanProgressFill.style.width = '100%';
        elements.scanProgressPct.textContent = '100%';
        elements.scanProgressLabel.textContent = `✅ Scan complete: ${state.groupedMovies.length} movies (${state.rawMatches.length} files found)`;

        updateStats();
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

  function startOrganizeStream() {
    const src = elements.orgSrcDir.value.trim();
    const dest = elements.orgDestRoot.value.trim();
    const dryRun = elements.orgDryRun.checked;

    if (!dest) {
      showToast('Destination path is required', 'warning');
      return;
    }

    if (state.orgEventSource) {
      state.orgEventSource.close();
    }

    elements.orgProgressBox.classList.remove('hidden');
    elements.orgProgressFill.style.width = '0%';
    elements.orgProgressPct.textContent = '0%';
    elements.orgProgressLabel.textContent = '🚀 Preparing organize pipeline...';
    elements.btnStartOrganize.disabled = true;

    elements.organizerLog.textContent = `🚀 Starting organize from ${src} -> ${dest} (DryRun: ${dryRun})...\n\n`;

    const url = `/api/organize/stream?source=${encodeURIComponent(src)}&destination=${encodeURIComponent(dest)}&dry_run=${dryRun}`;
    const es = new EventSource(url);
    state.orgEventSource = es;

    es.addEventListener('start', (e) => {
      try {
        const data = JSON.parse(e.data);
        elements.orgProgressLabel.textContent = `Processing 0 of ${data.total} movies...`;
      } catch (err) {}
    });

    es.addEventListener('item', (e) => {
      try {
        const item = JSON.parse(e.data);
        const pct = item.percent || 0;
        elements.orgProgressFill.style.width = `${pct}%`;
        elements.orgProgressPct.textContent = `${pct}%`;
        elements.orgProgressLabel.textContent = `Processing ${item.index} of ${item.total}: ${item.movie_id}...`;

        const status = item.success ? (dryRun ? '[PLAN]' : '[MOVED]') : '[FAIL]';
        elements.organizerLog.textContent += `${status} ${item.movie_id} -> ${item.target_folder || ''}\n`;
        if (item.target_video) {
          elements.organizerLog.textContent += `   Video: ${item.target_video}\n`;
        }
        elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
      } catch (err) {}
    });

    es.addEventListener('done', (e) => {
      try {
        const data = JSON.parse(e.data);
        elements.orgProgressFill.style.width = '100%';
        elements.orgProgressPct.textContent = '100%';
        elements.orgProgressLabel.textContent = `✨ Finished! Organized ${data.success_count} / ${data.total} movies.`;

        elements.organizerLog.textContent += `\n✨ Complete! Successfully processed ${data.success_count}/${data.total} movies.\n`;
        elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
        showToast(`Organize complete: ${data.success_count} movies processed!`, 'success');

        setTimeout(() => {
          elements.orgProgressBox.classList.add('hidden');
        }, 2000);
      } catch (err) {}
      es.close();
      state.orgEventSource = null;
      elements.btnStartOrganize.disabled = false;
    });

    es.addEventListener('error', (e) => {
      elements.organizerLog.textContent += `❌ Connection closed or error occurred.\n`;
      es.close();
      state.orgEventSource = null;
      elements.btnStartOrganize.disabled = false;
      elements.orgProgressBox.classList.add('hidden');
    });
  }

  async function loadActressesData() {
    try {
      const res = await fetch('/api/actresses/releases');
      const data = await res.json();
      state.actresses = data.actresses || [];
      elements.countActresses.textContent = state.actresses.length;
      renderActressHub();
    } catch (err) {
      console.error('Failed to load actresses:', err);
    }
  }

  function updateStats() {
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

  // =========================================================================
  // Library Grid Rendering
  // =========================================================================
  function renderMoviesGrid() {
    let list = [...state.groupedMovies];

    // Search filter
    if (state.searchQuery) {
      list = list.filter(m => {
        const id = (m.id || '').toLowerCase();
        const fnames = m.files.map(f => (f?.name || '').toLowerCase()).join(' ');
        const meta = state.metadata[m.id];
        const title = (meta?.title || '').toLowerCase();
        const studio = (meta?.maker || '').toLowerCase();
        const actNames = (meta?.actresses || []).map(a => a.name.toLowerCase()).join(' ');

        return id.includes(state.searchQuery) ||
               fnames.includes(state.searchQuery) ||
               title.includes(state.searchQuery) ||
               studio.includes(state.searchQuery) ||
               actNames.includes(state.searchQuery);
      });
    }

    // Status filter
    if (state.filter === 'matched') {
      list = list.filter(m => m.id);
    } else if (state.filter === 'unmatched') {
      list = list.filter(m => !m.id);
    } else if (state.filter === 'scraped') {
      list = list.filter(m => m.id && state.metadata[m.id]);
    } else if (state.filter === 'watched') {
      list = list.filter(m => m.id && state.userStates[m.id]?.is_watched);
    } else if (state.filter === 'unwatched') {
      list = list.filter(m => m.id && !state.userStates[m.id]?.is_watched);
    } else if (state.filter === 'favorites') {
      list = list.filter(m => m.id && state.userStates[m.id]?.is_favorite);
    }

    // Sort
    list.sort((a, b) => {
      if (state.sort === 'id-asc') return (a.id || '').localeCompare(b.id || '');
      if (state.sort === 'id-desc') return (b.id || '').localeCompare(a.id || '');
      if (state.sort === 'name-asc') return (a.files[0]?.name || '').localeCompare(b.files[0]?.name || '');
      if (state.sort === 'size-desc') return (b.totalSize || 0) - (a.totalSize || 0);
      if (state.sort === 'date-desc') {
        const dateA = state.metadata[a.id]?.release_date || '';
        const dateB = state.metadata[b.id]?.release_date || '';
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
  }

  function createMovieCard(movie) {
    const card = document.createElement('article');
    card.className = 'movie-card';
    card.tabIndex = 0;
    card.setAttribute('role', 'article');
    card.setAttribute('aria-label', `${movie.id || 'Unmatched'} - ${state.metadata[movie.id]?.title || movie.files[0]?.name || ''}`);

    const id = movie.id || 'UNMATCHED';
    const meta = state.metadata[movie.id];
    const uState = state.userStates[movie.id] || {};
    const sizeMB = Math.round((movie.totalSize || 0) / (1024 * 1024));

    const coverUrl = meta?.cover_url || meta?.poster_url || '/api/images/' + id;
    const title = meta?.title || movie.files[0]?.name || 'Unknown Title';
    const maker = meta?.maker || '';
    const date = meta?.release_date || '';

    // Multi-part indicator
    let partBadge = '';
    if (movie.isMultiPart || movie.files.length > 1) {
      const partStr = movie.partNumbers.length > 0 ? `P${movie.partNumbers.join(', P')}` : `${movie.files.length} parts`;
      partBadge = `<span class="badge-status badge-multipart" title="${movie.files.length} video files">${partStr}</span>`;
    }

    // Badges & Actresses
    const actressesHtml = (meta?.actresses || []).slice(0, 3).map(act => {
      const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
      return `<span class="actress-chip ${isFollowed ? 'followed' : ''}">${isFollowed ? '⭐ ' : ''}${escapeHtml(act.name)}</span>`;
    }).join('');

    const watchedBadge = uState.is_watched ? '<span class="badge-status" style="color: var(--success)" title="Watched">👁️ Watched</span>' : '';
    const favBadge = uState.is_favorite ? '<span class="badge-status" style="color: var(--accent-pink)" title="Favorited">❤️</span>' : '';
    const ratingStr = uState.user_rating ? '⭐'.repeat(uState.user_rating) : '';

    card.innerHTML = `
      <div class="card-cover-wrapper">
        <img class="card-cover-img" src="${coverUrl}" alt="Jacket cover for ${escapeHtml(id)}" onerror="this.src='/placeholder.png'" loading="lazy" />
        <div class="card-overlay-badge">${escapeHtml(id)}</div>
        <div class="card-overlay-status">
          ${partBadge}
          ${watchedBadge}
          ${favBadge}
        </div>
      </div>
      <div class="card-content">
        <h3 class="card-title" title="${escapeHtml(title)}">${escapeHtml(title)}</h3>
        <div class="card-meta">
          <span>${escapeHtml(maker)}</span>
          <span>${sizeMB} MB ${date ? '• ' + date : ''}</span>
        </div>
        <div class="card-actresses">
          ${actressesHtml}
        </div>
        <div class="card-footer">
          <div style="font-size: 0.85rem; color: var(--warning); min-height: 20px;">${ratingStr}</div>
          <div class="card-actions" onclick="event.stopPropagation()">
            <button class="btn-icon-sm btn-scrape" title="Scrape metadata" aria-label="Scrape metadata for ${escapeHtml(id)}">⚡</button>
            <button class="btn-icon-sm btn-fav ${uState.is_favorite ? 'active' : ''}" title="Toggle Favorite" aria-label="Favorite ${escapeHtml(id)}">❤️</button>
            <button class="btn-icon-sm btn-watched" title="Toggle Watched" aria-label="Toggle watched status for ${escapeHtml(id)}">${uState.is_watched ? '👁️' : '👓'}</button>
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

    return card;
  }

  // =========================================================================
  // Full-Width Hero Cover Modal & Lightbox
  // =========================================================================
  async function openMovieDetail(movie) {
    state.selectedMovieId = movie.id;
    elements.modalMovie.classList.remove('hidden');

    let meta = state.metadata[movie.id];
    if (!meta && movie.id) {
      elements.modalContent.innerHTML = '<div style="text-align: center; padding: 4rem;"><span style="font-size: 2rem;">⚡</span><br><br>Fetching metadata from R18.dev...</div>';
      meta = await scrapeMovie(movie.id);
    }

    renderModalContent(movie, meta);
  }

  function closeModal() {
    elements.modalMovie.classList.add('hidden');
    state.selectedMovieId = null;
  }

  function renderModalContent(movie, meta) {
    const id = movie.id || 'UNMATCHED';
    const uState = state.userStates[id] || {};
    const coverUrl = meta?.cover_url || meta?.poster_url || '';

    // Multi-part files list
    let multipartHtml = '';
    if (movie.files && movie.files.length > 0) {
      multipartHtml = `
        <div class="multipart-box">
          <h4>📦 Video Files & Parts (${movie.files.length})</h4>
          ${movie.files.map((f, idx) => `
            <div class="multipart-item">
              <span><strong>Part ${idx + 1}:</strong> ${escapeHtml(f.name)}</span>
              <span>${Math.round((f.size || 0) / (1024 * 1024))} MB</span>
            </div>
          `).join('')}
        </div>
      `;
    }

    // Actress Cards
    let actressGrid = '';
    if (meta?.actresses && meta.actresses.length > 0) {
      actressGrid = meta.actresses.map(act => {
        const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
        const thumb = act.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
        return `
          <div class="actress-card" style="background: var(--bg-card); padding: 1rem; border-radius: var(--radius-md); text-align: center;">
            <img src="${thumb}" alt="${escapeHtml(act.name)}" style="width: 74px; height: 74px; border-radius: 50%; object-fit: cover; margin: 0 auto 0.6rem auto; border: 2px solid var(--primary);" />
            <div style="font-weight: 700; font-size: 0.95rem; color: #fff;">${escapeHtml(act.name)}</div>
            <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.6rem;">${escapeHtml(act.ja_name || '')}</div>
            <button class="btn btn-secondary btn-sm" onclick="window.app.toggleFollowActress('${escapeHtml(act.name)}')">
              ${isFollowed ? '⭐ Following' : '+ Follow'}
            </button>
          </div>
        `;
      }).join('');
    }

    // Sample Screenshots with SAFE High-Res URL & Fallbacks
    let screenshotsGrid = '';
    if (meta?.sample_screenshots && meta.sample_screenshots.length > 0) {
      screenshotsGrid = meta.sample_screenshots.map((url, idx) => {
        const highResUrl = getHighResScreenshotUrl(url);
        return `
          <div class="gallery-thumbnail" tabindex="0" role="button" aria-label="View Screenshot #${idx + 1}"
               onclick="window.app.openLightbox('${highResUrl}', '${escapeHtml(url)}', 'Screenshot #${idx + 1}')"
               onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();window.app.openLightbox('${highResUrl}', '${escapeHtml(url)}', 'Screenshot #${idx + 1}');}">
            <img src="${url}" alt="Screenshot #${idx + 1}" loading="lazy" onerror="this.src='/api/proxy-image?url=${encodeURIComponent(url)}'" />
          </div>
        `;
      }).join('');
    }

    // FULL-WIDTH HERO COVER LAYOUT
    elements.modalContent.innerHTML = `
      <!-- Full-Width Hero Cover Banner -->
      <div class="modal-hero-banner" role="region" aria-label="Full-Width Cover Image">
        <div class="modal-hero-backdrop" style="background-image: url('${coverUrl}');"></div>
        <img class="modal-hero-cover-img" src="${coverUrl}" alt="Full High-Res Cover for ${escapeHtml(id)}"
             onclick="window.app.openLightbox('${coverUrl}', '${coverUrl}', 'Jacket Cover - ${escapeHtml(id)}')"
             onerror="this.src='/placeholder.png'" />
        <div class="modal-hero-overlay">
          <span class="badge-status" style="background: rgba(12,14,20,0.85); font-family: var(--font-mono); font-size: 0.95rem; font-weight: 800; color: #fff;">
            ${escapeHtml(id)}
          </span>
          <button class="btn-zoom-cover" onclick="window.app.openLightbox('${coverUrl}', '${coverUrl}', 'Jacket Cover - ${escapeHtml(id)}')">
            🔍 Zoom Full Cover
          </button>
        </div>
      </div>

      <div class="modal-body">
        <div class="modal-info">
          <h2 id="modal-movie-title">${escapeHtml(meta?.title || movie.files[0]?.name || 'No title')}</h2>
          <div class="ja-title">${escapeHtml(meta?.original_title || '')}</div>

          <!-- Interactive Status Controls -->
          <div class="modal-controls">
            <div style="display: flex; align-items: center; gap: 0.6rem;">
              <span style="font-weight: 700; font-size: 0.85rem;">Rating:</span>
              <div class="star-rating" role="radiogroup" aria-label="Rate 1 to 5 stars">
                ${[1, 2, 3, 4, 5].map(num => `
                  <button class="star-btn ${uState.user_rating >= num ? 'active' : ''}"
                          role="radio" aria-checked="${uState.user_rating >= num ? 'true' : 'false'}"
                          aria-label="Rate ${num} stars"
                          onclick="window.app.setRating('${id}', ${num})">⭐</button>
                `).join('')}
              </div>
            </div>

            <button class="btn btn-secondary ${uState.is_watched ? 'btn-primary' : ''}" onclick="window.app.toggleWatched('${id}')">
              ${uState.is_watched ? '👁️ Watched' : '👓 Mark Watched'}
            </button>

            <button class="btn btn-secondary ${uState.is_favorite ? 'btn-accent' : ''}" onclick="window.app.toggleFavorite('${id}')">
              ${uState.is_favorite ? '❤️ Favorited' : '🤍 Favorite'}
            </button>

            <button class="btn btn-primary" onclick="window.app.organizeSingle('${id}')">
              📂 Organize for Jellyfin
            </button>
          </div>

          <!-- Multi-Part Files -->
          ${multipartHtml}

          <!-- Metadata Grid -->
          <div class="modal-meta-grid">
            <div class="meta-field">
              <div class="label">Studio / Maker</div>
              <div class="value">${escapeHtml(meta?.maker || '-')}</div>
            </div>
            <div class="meta-field">
              <div class="label">Release Date</div>
              <div class="value">${escapeHtml(meta?.release_date || '-')}</div>
            </div>
            <div class="meta-field">
              <div class="label">Runtime</div>
              <div class="value">${meta?.runtime_minutes ? meta.runtime_minutes + ' mins' : '-'}</div>
            </div>
            <div class="meta-field">
              <div class="label">Director</div>
              <div class="value">${escapeHtml(meta?.director || '-')}</div>
            </div>
          </div>

          <div style="display: flex; flex-wrap: wrap; gap: 0.45rem; margin-bottom: 2rem;">
            ${(meta?.genres || []).map(g => `<span class="actress-chip" style="background: rgba(255,255,255,0.06); color: var(--text-main);">${escapeHtml(g)}</span>`).join('')}
          </div>
        </div>

        ${actressGrid ? `
          <div class="gallery-section-title">🎭 Featured Cast</div>
          <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1rem; margin-bottom: 2rem;">
            ${actressGrid}
          </div>
        ` : ''}

        ${screenshotsGrid ? `
          <div class="gallery-section-title">📸 Sample Screenshots (${meta.sample_screenshots.length})</div>
          <div class="modal-gallery-grid">
            ${screenshotsGrid}
          </div>
        ` : ''}
      </div>
    `;
  }

  // Safe DMM high-res URL converter preventing double jpjp-
  function getHighResScreenshotUrl(url) {
    if (!url) return '';
    if (url.includes('jp-')) return url;
    return url.replace(/([a-z0-9]+)-([0-9]+)\.jpg$/, '$1jp-$2.jpg');
  }

  function openLightbox(url, fallbackUrl, caption) {
    elements.lightboxImg.src = url;
    elements.lightboxImg.dataset.fallback = fallbackUrl || '';
    elements.lightboxCaption.textContent = caption || '';
    elements.lightbox.classList.remove('hidden');

    // Fallback if high res fails to load
    elements.lightboxImg.onerror = function () {
      if (this.dataset.fallback && this.src !== this.dataset.fallback) {
        this.src = this.dataset.fallback;
      } else {
        // Last resort: proxy via Go backend
        this.src = '/api/proxy-image?url=' + encodeURIComponent(fallbackUrl || url);
      }
    };
  }

  function closeLightbox() {
    elements.lightbox.classList.add('hidden');
    elements.lightboxImg.src = '';
  }

  // =========================================================================
  // Actress Hub Rendering
  // =========================================================================
  function renderActressHub() {
    elements.actressesContainer.innerHTML = '';
    elements.actressesEmpty.classList.toggle('hidden', state.actresses.length > 0);

    state.actresses.forEach(entry => {
      const a = entry.actress;
      const releases = entry.releases || [];
      const section = document.createElement('section');
      section.className = 'actress-section';
      section.setAttribute('aria-labelledby', `actress-title-${escapeHtml(a.name)}`);

      const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

      let releasesHtml = '';
      if (releases.length > 0) {
        releasesHtml = `
          <div class="movies-grid grid-auto" style="grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));">
            ${releases.map(rel => {
              const statusPill = rel.is_downloaded
                ? '<span class="pill pill-downloaded">🟢 Downloaded</span>'
                : '<span class="pill pill-missing">🔴 Missing / New</span>';
              const watched = rel.is_watched ? ' • 👁️ Watched' : '';
              return `
                <div class="movie-card" style="padding: 0.8rem;" tabindex="0" role="article">
                  <img src="${rel.cover_url || '/placeholder.png'}" alt="Cover for ${escapeHtml(rel.movie_id)}" style="width: 100%; aspect-ratio: 16/10; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.6rem;" />
                  <div style="font-weight: 700; color: #fff; font-size: 0.9rem;">${escapeHtml(rel.movie_id)}</div>
                  <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.5rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">${escapeHtml(rel.title)}</div>
                  <div style="display: flex; justify-content: space-between; align-items: center; font-size: 0.75rem;">
                    ${statusPill}
                    <span style="color: var(--text-muted);">${escapeHtml(rel.release_date || '')}${watched}</span>
                  </div>
                </div>
              `;
            }).join('')}
          </div>
        `;
      } else {
        releasesHtml = '<p style="color: var(--text-muted); font-size: 0.9rem;">No releases recorded in database yet.</p>';
      }

      section.innerHTML = `
        <div class="actress-profile-bar">
          <div class="actress-info-left">
            <img class="actress-avatar-lg" src="${avatar}" alt="Avatar of ${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
            <div class="actress-names">
              <h3 id="actress-title-${escapeHtml(a.name)}">${escapeHtml(a.name)}</h3>
              <div class="ja-name">${escapeHtml(a.ja_name || '')}</div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 1rem; flex-wrap: wrap;">
            <div class="actress-stats-pills">
              <span class="pill pill-downloaded">🟢 ${entry.downloaded} Downloaded</span>
              <span class="pill pill-missing">🔴 ${entry.missing} Missing</span>
              <span class="pill pill-watched">👁️ ${entry.watched} Watched</span>
            </div>
            <button class="btn btn-secondary btn-sm" onclick="window.app.unfollowActress('${escapeHtml(a.name)}')">
              🗑️ Unfollow
            </button>
          </div>
        </div>
        ${releasesHtml}
      `;

      elements.actressesContainer.appendChild(section);
    });
  }

  // =========================================================================
  // API Action Helpers
  // =========================================================================
  async function scrapeMovie(id) {
    if (!id) return null;
    try {
      showToast(`Scraping ${id}...`, 'info');
      const res = await fetch(`/api/scrape/${id}`, { method: 'POST' });
      const movie = await res.json();
      state.metadata[id] = movie;
      updateStats();
      renderMoviesGrid();
      showToast(`Scraped ${id}!`, 'success');
      return movie;
    } catch (err) {
      showToast(`Scrape failed for ${id}: ${err.message}`, 'danger');
      return null;
    }
  }

  async function scrapeAllMatched() {
    const matched = state.groupedMovies.filter(m => m.id && !state.metadata[m.id]);
    if (matched.length === 0) {
      showToast('All matched movies are already scraped!', 'info');
      return;
    }

    showToast(`Batch scraping ${matched.length} movies...`, 'info');
    for (const m of matched) {
      await scrapeMovie(m.id);
    }
    showToast('Batch scraping completed!', 'success');
  }

  async function toggleWatched(id) {
    const cur = state.userStates[id]?.is_watched || false;
    await updateUserState(id, { is_watched: !cur });
    showToast(!cur ? `Marked ${id} as Watched` : `Marked ${id} as Unwatched`, 'info');
  }

  async function toggleFavorite(id) {
    const cur = state.userStates[id]?.is_favorite || false;
    await updateUserState(id, { is_favorite: !cur });
    showToast(!cur ? `Added ${id} to Favorites ❤️` : `Removed ${id} from Favorites`, 'info');
  }

  async function setRating(id, rating) {
    await updateUserState(id, { user_rating: rating });
    showToast(`Rated ${id}: ${'⭐'.repeat(rating)}`, 'success');
  }

  async function updateUserState(id, updates) {
    if (!id) return;
    const existing = state.userStates[id] || { movie_id: id };
    const newState = Object.assign({}, existing, updates);
    state.userStates[id] = newState;

    try {
      await fetch(`/api/movie/${id}/state`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newState)
      });
      renderMoviesGrid();
      if (state.selectedMovieId === id) {
        const movie = state.groupedMovies.find(m => m.id === id);
        if (movie) renderModalContent(movie, state.metadata[id]);
      }
    } catch (err) {
      console.error('Failed to update user state:', err);
    }
  }

  async function toggleFollowActress(name) {
    const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === name.toLowerCase());
    if (isFollowed) {
      await unfollowActress(name);
    } else {
      try {
        await fetch('/api/actresses/follow', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ name: name })
        });
        showToast(`Followed ${name} ⭐`, 'success');
        await loadActressesData();
        renderMoviesGrid();
        if (state.selectedMovieId) {
          const movie = state.groupedMovies.find(m => m.id === state.selectedMovieId);
          if (movie) renderModalContent(movie, state.metadata[state.selectedMovieId]);
        }
      } catch (err) {
        showToast('Error following actress: ' + err.message, 'danger');
      }
    }
  }

  async function unfollowActress(name) {
    try {
      await fetch('/api/actresses/unfollow', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name })
      });
      showToast(`Unfollowed ${name}`, 'info');
      await loadActressesData();
      renderMoviesGrid();
      if (state.selectedMovieId) {
        const movie = state.groupedMovies.find(m => m.id === state.selectedMovieId);
        if (movie) renderModalContent(movie, state.metadata[state.selectedMovieId]);
      }
    } catch (err) {
      showToast('Error unfollowing actress: ' + err.message, 'danger');
    }
  }

  async function organizeSingle(id) {
    const movie = state.groupedMovies.find(m => m.id === id);
    if (!movie) return;

    const dest = elements.orgDestRoot.value.trim() || (state.activeDir + '/JAV_Library');
    showToast(`Organizing ${id} for Jellyfin...`, 'info');

    try {
      // If multipart, organize each file into the single folder
      for (const file of movie.files) {
        await fetch('/api/organize', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            source_file: file?.path,
            destination: dest,
            dry_run: false
          })
        });
      }
      showToast(`✅ Successfully organized ${id} into ${dest}`, 'success');
    } catch (err) {
      showToast(`Error: ${err.message}`, 'danger');
    }
  }

  // =========================================================================
  // Utilities
  // =========================================================================
  function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.setAttribute('role', 'status');
    
    let icon = 'ℹ️';
    if (type === 'success') icon = '✅';
    if (type === 'warning') icon = '⚠️';
    if (type === 'danger') icon = '❌';

    toast.innerHTML = `<span>${icon}</span> <span>${escapeHtml(message)}</span>`;
    elements.toastContainer.appendChild(toast);

    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateY(10px)';
      toast.style.transition = 'all 0.3s ease';
      setTimeout(() => toast.remove(), 300);
    }, 3500);
  }

  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Expose global app object for inline handlers
  window.app = {
    openLightbox,
    toggleWatched,
    toggleFavorite,
    setRating,
    toggleFollowActress,
    unfollowActress,
    organizeSingle
  };

  document.addEventListener('DOMContentLoaded', init);
})();
