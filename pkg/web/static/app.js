/**
 * R19DEV Studio - Frontend Application Engine
 */

(function () {
  'use strict';

  // Application State
  const state = {
    activeTab: 'library',
    activeDir: '.',
    files: [],       // Scanned files & matches
    metadata: {},    // ID -> scraper.Movie
    userStates: {},  // ID -> db.UserState
    actresses: [],   // Followed actresses
    filter: 'all',
    sort: 'id-asc',
    searchQuery: '',
    selectedMovieId: null,
    isScanning: false,
  };

  // DOM Selectors
  const elements = {
    tabs: document.querySelectorAll('.nav-tab'),
    panes: document.querySelectorAll('.tab-pane'),
    countLibrary: document.getElementById('count-library'),
    countActresses: document.getElementById('count-actresses'),
    labelActiveDir: document.getElementById('label-active-dir'),
    btnRescan: document.getElementById('btn-rescan'),
    
    // Library
    searchInput: document.getElementById('search-input'),
    searchClear: document.getElementById('search-clear'),
    filterStatus: document.getElementById('filter-status'),
    sortBy: document.getElementById('sort-by'),
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
  // Initialization & Event Listeners
  // =========================================================================
  function init() {
    setupTabSwitching();
    setupSearchAndFilters();
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
    elements.tabs.forEach(t => t.classList.toggle('active', t.dataset.tab === tabId));
    elements.panes.forEach(p => p.classList.toggle('active', p.id === `tab-${tabId}`));

    if (tabId === 'actresses') {
      loadActressesData();
    }
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
      triggerScan();
    });

    elements.btnScrapeAll.addEventListener('click', () => {
      scrapeAllMatched();
    });

    elements.btnOrganizeAll.addEventListener('click', () => {
      switchTab('organizer');
    });
  }

  function setupModals() {
    // Close modal on close button or clicking outside
    elements.btnCloseModal.addEventListener('click', closeModal);
    elements.modalMovie.addEventListener('click', (e) => {
      if (e.target === elements.modalMovie) closeModal();
    });

    // Close lightbox
    elements.btnCloseLightbox.addEventListener('click', closeLightbox);
    elements.lightbox.addEventListener('click', (e) => {
      if (e.target === elements.lightbox) closeLightbox();
    });

    // Keyboard shortcuts (Esc)
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

    elements.btnStartOrganize.addEventListener('click', async () => {
      const src = elements.orgSrcDir.value.trim();
      const dest = elements.orgDestRoot.value.trim();
      const dryRun = elements.orgDryRun.checked;

      if (!src || !dest) {
        showToast('Please enter both source and destination directories', 'warning');
        return;
      }

      elements.organizerLog.textContent = `🚀 Starting organize from ${src} -> ${dest} (DryRun: ${dryRun})...\n\n`;
      elements.btnStartOrganize.disabled = true;

      try {
        const res = await fetch('/api/organize', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ source: src, destination: dest, dry_run: dryRun })
        });
        const data = await res.json();
        
        if (data.results && data.results.length > 0) {
          data.results.forEach(item => {
            const status = item.success ? (dryRun ? '[PLAN]' : '[MOVED]') : '[FAIL]';
            elements.organizerLog.textContent += `${status} ${item.movie_id} -> ${item.target_folder}\n`;
            elements.organizerLog.textContent += `   Video: ${item.target_video}\n`;
            elements.organizerLog.textContent += `   Assets: NFO, HTML, poster.jpg, fanart.jpg, extrafanart/ (${item.screenshots_num} screenshots)\n\n`;
          });
          elements.organizerLog.textContent += `\n✨ Finished! Successfully processed ${data.success_count}/${data.total_count} movies.\n`;
          showToast(`Organize complete: ${data.success_count} movies processed!`, 'success');
        } else {
          elements.organizerLog.textContent += 'No matched video files found to organize.\n';
        }
      } catch (err) {
        elements.organizerLog.textContent += `❌ Error: ${err.message}\n`;
        showToast('Failed to organize: ' + err.message, 'danger');
      } finally {
        elements.btnStartOrganize.disabled = false;
      }
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
        showToast(`Followed ${name}`, 'success');
        loadActressesData();
      } catch (err) {
        showToast('Error following actress: ' + err.message, 'danger');
      }
    });
  }

  // =========================================================================
  // Data Fetching & Sync
  // =========================================================================
  async function fetchInitialData() {
    try {
      // Get directory configuration & scan
      await triggerScan();
      await loadActressesData();
    } catch (err) {
      console.error('Initial data load failed:', err);
    }
  }

  async function triggerScan() {
    state.isScanning = true;
    elements.statTotal.textContent = 'Scanning...';

    try {
      const res = await fetch('/api/scan');
      const data = await res.json();
      
      state.files = data.matches || [];
      state.activeDir = data.target_dir || '.';
      elements.labelActiveDir.textContent = state.activeDir;
      elements.orgSrcDir.value = state.activeDir;
      if (!elements.orgDestRoot.value) {
        elements.orgDestRoot.value = state.activeDir + '/JAV_Library';
      }

      // Pre-populate known metadata & user states from response
      if (data.metadata) {
        Object.assign(state.metadata, data.metadata);
      }
      if (data.user_states) {
        Object.assign(state.userStates, data.user_states);
      }

      updateStats();
      renderMoviesGrid();
    } catch (err) {
      showToast('Scan failed: ' + err.message, 'danger');
    } finally {
      state.isScanning = false;
    }
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
    const total = state.files.length;
    let matched = 0;
    let scraped = 0;

    state.files.forEach(f => {
      if (f.id) {
        matched++;
        if (state.metadata[f.id]) scraped++;
      }
    });

    elements.statTotal.textContent = total;
    elements.statMatched.textContent = matched;
    elements.statUnmatched.textContent = total - matched;
    elements.statScraped.textContent = scraped;
    elements.countLibrary.textContent = total;
  }

  // =========================================================================
  // Library Rendering & Grid
  // =========================================================================
  function renderMoviesGrid() {
    let list = [...state.files];

    // 1. Search Filter
    if (state.searchQuery) {
      list = list.filter(item => {
        const id = (item.id || '').toLowerCase();
        const fname = (item.file?.name || '').toLowerCase();
        const meta = state.metadata[item.id];
        const title = (meta?.title || '').toLowerCase();
        const studio = (meta?.maker || '').toLowerCase();
        const actNames = (meta?.actresses || []).map(a => a.name.toLowerCase()).join(' ');

        return id.includes(state.searchQuery) ||
               fname.includes(state.searchQuery) ||
               title.includes(state.searchQuery) ||
               studio.includes(state.searchQuery) ||
               actNames.includes(state.searchQuery);
      });
    }

    // 2. Status Filter
    if (state.filter === 'matched') {
      list = list.filter(item => item.id);
    } else if (state.filter === 'unmatched') {
      list = list.filter(item => !item.id);
    } else if (state.filter === 'scraped') {
      list = list.filter(item => item.id && state.metadata[item.id]);
    } else if (state.filter === 'watched') {
      list = list.filter(item => item.id && state.userStates[item.id]?.is_watched);
    } else if (state.filter === 'unwatched') {
      list = list.filter(item => item.id && !state.userStates[item.id]?.is_watched);
    } else if (state.filter === 'favorites') {
      list = list.filter(item => item.id && state.userStates[item.id]?.is_favorite);
    }

    // 3. Sorting
    list.sort((a, b) => {
      if (state.sort === 'id-asc') return (a.id || '').localeCompare(b.id || '');
      if (state.sort === 'id-desc') return (b.id || '').localeCompare(a.id || '');
      if (state.sort === 'name-asc') return (a.file?.name || '').localeCompare(b.file?.name || '');
      if (state.sort === 'size-desc') return (b.file?.size || 0) - (a.file?.size || 0);
      if (state.sort === 'date-desc') {
        const dateA = state.metadata[a.id]?.release_date || '';
        const dateB = state.metadata[b.id]?.release_date || '';
        return dateB.localeCompare(dateA);
      }
      return 0;
    });

    elements.moviesGrid.innerHTML = '';
    elements.libraryEmpty.classList.toggle('hidden', list.length > 0);

    list.forEach(item => {
      const card = createMovieCard(item);
      elements.moviesGrid.appendChild(card);
    });
  }

  function createMovieCard(item) {
    const card = document.createElement('div');
    card.className = 'movie-card';

    const id = item.id || 'UNMATCHED';
    const meta = state.metadata[item.id];
    const uState = state.userStates[item.id] || {};
    const sizeMB = Math.round((item.file?.size || 0) / (1024 * 1024));

    // Cover image fallback
    const coverUrl = meta?.cover_url || meta?.poster_url || '/api/images/' + id;
    const title = meta?.title || item.file?.name || 'Unknown Title';
    const maker = meta?.maker || '';
    const date = meta?.release_date || '';

    // Badges & Actresses
    const actressesHtml = (meta?.actresses || []).slice(0, 2).map(act => {
      const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
      return `<span class="actress-chip ${isFollowed ? 'followed' : ''}">${isFollowed ? '⭐ ' : ''}${escapeHtml(act.name)}</span>`;
    }).join('');

    const watchedBadge = uState.is_watched ? '<span class="badge-status" style="color: var(--success)">👁️ Watched</span>' : '';
    const favBadge = uState.is_favorite ? '<span class="badge-status" style="color: var(--accent-pink)">❤️</span>' : '';
    const ratingStr = uState.user_rating ? '⭐'.repeat(uState.user_rating) : '';

    card.innerHTML = `
      <div class="card-cover-wrapper">
        <img class="card-cover-img" src="${coverUrl}" alt="${escapeHtml(id)}" onerror="this.src='/placeholder.png'" loading="lazy" />
        <div class="card-overlay-badge">${escapeHtml(id)}</div>
        <div class="card-overlay-status">
          ${watchedBadge}
          ${favBadge}
        </div>
      </div>
      <div class="card-content">
        <div class="card-title" title="${escapeHtml(title)}">${escapeHtml(title)}</div>
        <div class="card-meta">
          <span>${escapeHtml(maker)}</span>
          <span>${sizeMB} MB ${date ? '• ' + date : ''}</span>
        </div>
        <div class="card-actresses">
          ${actressesHtml}
        </div>
        <div class="card-footer">
          <div style="font-size: 0.85rem; color: var(--warning);">${ratingStr}</div>
          <div class="card-actions" onclick="event.stopPropagation()">
            <button class="btn-icon-sm btn-scrape" title="Scrape metadata">⚡</button>
            <button class="btn-icon-sm btn-fav ${uState.is_favorite ? 'active' : ''}" title="Favorite">❤️</button>
            <button class="btn-icon-sm btn-watched" title="Toggle Watched">${uState.is_watched ? '👁️' : '👓'}</button>
          </div>
        </div>
      </div>
    `;

    // Click card to open inspector modal
    card.addEventListener('click', () => {
      openMovieDetail(item);
    });

    // Quick Actions
    card.querySelector('.btn-scrape')?.addEventListener('click', async (e) => {
      e.stopPropagation();
      await scrapeMovie(item.id);
    });

    card.querySelector('.btn-fav')?.addEventListener('click', async (e) => {
      e.stopPropagation();
      await toggleFavorite(item.id);
    });

    card.querySelector('.btn-watched')?.addEventListener('click', async (e) => {
      e.stopPropagation();
      await toggleWatched(item.id);
    });

    return card;
  }

  // =========================================================================
  // Movie Detail Modal & Lightbox
  // =========================================================================
  async function openMovieDetail(item) {
    state.selectedMovieId = item.id;
    elements.modalMovie.classList.remove('hidden');

    let meta = state.metadata[item.id];
    if (!meta && item.id) {
      elements.modalContent.innerHTML = '<div style="text-align: center; padding: 3rem;">⚡ Fetching metadata from R18.dev...</div>';
      meta = await scrapeMovie(item.id);
    }

    renderModalContent(item, meta);
  }

  function closeModal() {
    elements.modalMovie.classList.add('hidden');
    state.selectedMovieId = null;
  }

  function renderModalContent(item, meta) {
    const id = item.id || 'UNMATCHED';
    const uState = state.userStates[id] || {};
    const coverUrl = meta?.cover_url || meta?.poster_url || '';

    let actressGrid = '';
    if (meta?.actresses && meta.actresses.length > 0) {
      actressGrid = meta.actresses.map(act => {
        const isFollowed = state.actresses.some(a => a.actress.name.toLowerCase() === act.name.toLowerCase());
        const thumb = act.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
        return `
          <div class="actress-card" style="background: var(--bg-input); padding: 0.8rem; border-radius: var(--radius-md); text-align: center;">
            <img src="${thumb}" alt="${escapeHtml(act.name)}" style="width: 70px; height: 70px; border-radius: 50%; object-fit: cover; margin: 0 auto 0.5rem auto; border: 2px solid var(--primary);" />
            <div style="font-weight: 700; font-size: 0.9rem; color: #fff;">${escapeHtml(act.name)}</div>
            <div style="font-size: 0.75rem; color: var(--text-muted); margin-bottom: 0.5rem;">${escapeHtml(act.ja_name || '')}</div>
            <button class="btn btn-secondary btn-sm" onclick="window.app.toggleFollowActress('${escapeHtml(act.name)}')">
              ${isFollowed ? '⭐ Following' : '+ Follow'}
            </button>
          </div>
        `;
      }).join('');
    }

    let screenshotsGrid = '';
    if (meta?.sample_screenshots && meta.sample_screenshots.length > 0) {
      screenshotsGrid = meta.sample_screenshots.map((url, idx) => {
        // Upgrade DMM thumbnail url to high res
        const highRes = url.replace(/([a-z0-9]+)-([0-9]+)\.jpg$/, '$1jp-$2.jpg');
        return `
          <div class="gallery-thumbnail" onclick="window.app.openLightbox('${highRes}', 'Screenshot #${idx + 1}')">
            <img src="${url}" alt="Screenshot" loading="lazy" />
          </div>
        `;
      }).join('');
    }

    elements.modalContent.innerHTML = `
      <div class="modal-hero">
        <div class="modal-cover-wrapper">
          <img class="modal-cover-img" src="${coverUrl}" alt="${escapeHtml(id)}" onerror="this.src='/placeholder.png'" />
        </div>
        <div class="modal-info">
          <div style="display: flex; gap: 0.5rem; align-items: center; margin-bottom: 0.8rem;">
            <span class="badge-status" style="background: var(--primary); color: #0c0e14; font-weight: 800; font-size: 0.9rem;">${escapeHtml(id)}</span>
            <span style="font-size: 0.85rem; color: var(--text-muted); font-family: var(--font-mono);">${escapeHtml(item.file?.name || '')}</span>
          </div>

          <h2>${escapeHtml(meta?.title || item.file?.name || 'No title')}</h2>
          <div class="ja-title">${escapeHtml(meta?.original_title || '')}</div>

          <div class="modal-controls">
            <div style="display: flex; align-items: center; gap: 0.6rem;">
              <span style="font-weight: 600; font-size: 0.85rem;">Rating:</span>
              <div class="star-rating">
                ${[1, 2, 3, 4, 5].map(num => `
                  <button class="star-btn ${uState.user_rating >= num ? 'active' : ''}" onclick="window.app.setRating('${id}', ${num})">⭐</button>
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

          <div style="display: flex; flex-wrap: wrap; gap: 0.4rem;">
            ${(meta?.genres || []).map(g => `<span class="actress-chip" style="background: rgba(255,255,255,0.05); color: var(--text-main);">${escapeHtml(g)}</span>`).join('')}
          </div>
        </div>
      </div>

      ${actressGrid ? `
        <div class="gallery-section-title">🎭 Featured Cast</div>
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 1rem; margin-bottom: 2rem;">
          ${actressGrid}
        </div>
      ` : ''}

      ${screenshotsGrid ? `
        <div class="gallery-section-title">📸 Sample Screenshots (${meta.sample_screenshots.length})</div>
        <div class="modal-gallery-grid">
          ${screenshotsGrid}
        </div>
      ` : ''}
    `;
  }

  function openLightbox(url, caption) {
    elements.lightboxImg.src = url;
    elements.lightboxCaption.textContent = caption || '';
    elements.lightbox.classList.remove('hidden');
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
      const section = document.createElement('div');
      section.className = 'actress-section';

      const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

      let releasesHtml = '';
      if (releases.length > 0) {
        releasesHtml = `
          <div class="movies-grid" style="grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));">
            ${releases.map(rel => {
              const statusPill = rel.is_downloaded
                ? '<span class="pill pill-downloaded">🟢 Downloaded</span>'
                : '<span class="pill pill-missing">🔴 Missing / New</span>';
              const watched = rel.is_watched ? ' • 👁️ Watched' : '';
              return `
                <div class="movie-card" style="padding: 0.8rem;">
                  <img src="${rel.cover_url || '/placeholder.png'}" style="width: 100%; aspect-ratio: 16/10; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.6rem;" />
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
            <img class="actress-avatar-lg" src="${avatar}" alt="${escapeHtml(a.name)}" />
            <div class="actress-names">
              <h3>${escapeHtml(a.name)}</h3>
              <div class="ja-name">${escapeHtml(a.ja_name || '')}</div>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 1rem;">
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
    const matched = state.files.filter(f => f.id && !state.metadata[f.id]);
    if (matched.length === 0) {
      showToast('All matched movies are already scraped!', 'info');
      return;
    }

    showToast(`Batch scraping ${matched.length} movies...`, 'info');
    for (const item of matched) {
      await scrapeMovie(item.id);
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
        const item = state.files.find(f => f.id === id);
        if (item) renderModalContent(item, state.metadata[id]);
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
          const item = state.files.find(f => f.id === state.selectedMovieId);
          if (item) renderModalContent(item, state.metadata[state.selectedMovieId]);
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
        const item = state.files.find(f => f.id === state.selectedMovieId);
        if (item) renderModalContent(item, state.metadata[state.selectedMovieId]);
      }
    } catch (err) {
      showToast('Error unfollowing actress: ' + err.message, 'danger');
    }
  }

  async function organizeSingle(id) {
    const item = state.files.find(f => f.id === id);
    if (!item) return;

    const dest = elements.orgDestRoot.value.trim() || (state.activeDir + '/JAV_Library');
    showToast(`Organizing ${id} for Jellyfin...`, 'info');

    try {
      const res = await fetch('/api/organize', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          source_file: item.file?.path,
          destination: dest,
          dry_run: false
        })
      });
      const data = await res.json();
      if (data.results && data.results[0]?.success) {
        showToast(`✅ Successfully organized ${id} into ${data.results[0].target_folder}`, 'success');
      } else {
        showToast(`Failed to organize ${id}`, 'danger');
      }
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

  // Expose global app object for inline HTML handlers
  window.app = {
    openLightbox,
    toggleWatched,
    toggleFavorite,
    setRating,
    toggleFollowActress,
    unfollowActress,
    organizeSingle
  };

  // Launch on DOM ready
  document.addEventListener('DOMContentLoaded', init);
})();
