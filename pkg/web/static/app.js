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
    btnRefreshActresses: document.getElementById('btn-refresh-actresses'),
    actressesContainer: document.getElementById('actresses-container'),
    actressesEmpty: document.getElementById('actresses-empty'),

    // Organizer
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
    setupHistoryModal();

    // Initial Data Fetch
    fetchInitialData();

    if (window.lucide) {
      window.lucide.createIcons();
    }
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

    elements.btnResetFilters?.addEventListener('click', () => {
      resetFilters();
    });

    elements.sortBy.addEventListener('change', (e) => {
      state.sort = e.target.value;
      renderMoviesGrid();
    });

    elements.btnRescan.addEventListener('click', () => {
      startScanStream(state.activeDir);
    });

    if (elements.labelActiveDir) {
      const badge = elements.labelActiveDir.closest('.dir-badge');
      if (badge) {
        badge.style.cursor = 'pointer';
        badge.title = 'Click to change scan directory';
        badge.addEventListener('click', () => {
          const newPath = prompt('Enter folder path to scan:', state.activeDir || '/Volumes/home/BT/2026');
          if (newPath && newPath.trim()) {
            startScanStream(newPath.trim());
          }
        });
      }
    }

    elements.btnScrapeAll.addEventListener('click', () => {
      scrapeAllMatched();
    });

    elements.btnOrganizeAll.addEventListener('click', () => {
      switchTab('organizer');
    });
  }

  function resetFilters() {
    state.filterActress = '';
    state.filterOrganized = 'all';
    state.filterScraped = 'all';
    state.filterGenre = '';
    state.filterWatch = 'all';
    state.searchQuery = '';

    if (elements.searchInput) elements.searchInput.value = '';
    if (elements.searchClear) elements.searchClear.classList.add('hidden');
    if (elements.filterActress) elements.filterActress.value = '';
    if (elements.filterOrganized) elements.filterOrganized.value = 'all';
    if (elements.filterScraped) elements.filterScraped.value = 'all';
    if (elements.filterGenre) elements.filterGenre.value = '';
    if (elements.filterWatch) elements.filterWatch.value = 'all';

    renderMoviesGrid();
  }

  function populateFilterDropdowns() {
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
        } else if (!elements.modalHistory?.classList.contains('hidden')) {
          closeHistoryModal();
        }
      }
    });
  }

  function setupHistoryModal() {
    elements.btnViewHistory?.addEventListener('click', () => {
      openHistoryModal();
    });

    elements.btnCloseHistory?.addEventListener('click', () => {
      closeHistoryModal();
    });

    elements.modalHistory?.addEventListener('click', (e) => {
      if (e.target === elements.modalHistory) {
        closeHistoryModal();
      }
    });

    elements.btnClearHistory?.addEventListener('click', async () => {
      if (!confirm('ต้องการล้างประวัติการจัดระเบียบทั้งหมดใช่หรือไม่? (Clear all history?)')) return;
      try {
        const res = await fetch('/api/history', { method: 'DELETE' });
        if (res.ok) {
          showToast('ล้างประวัติเรียบร้อยแล้ว', 'info');
          openHistoryModal();
        }
      } catch (err) {
        showToast('ไม่สามารถล้างประวัติได้: ' + err.message, 'danger');
      }
    });

    elements.btnCopyHistoryLog?.addEventListener('click', async () => {
      const text = elements.historyLogView?.textContent || '';
      if (!text.trim()) return;
      try {
        await navigator.clipboard.writeText(text);
        showToast('📋 Copied history log to clipboard!', 'success');
      } catch (err) {
        showToast('Failed to copy: ' + err.message, 'danger');
      }
    });
  }

  async function openHistoryModal() {
    elements.modalHistory?.classList.remove('hidden');
    elements.historyListContainer.innerHTML = '<div class="history-empty">กำลังโหลดประวัติ...</div>';
    elements.historyLogView.textContent = 'เลือกรายการทางด้านซ้ายเพื่อดู Log ละเอียด';
    elements.historyDetailTitle.textContent = 'รายละเอียดการทำงาน';
    elements.btnCopyHistoryLog.classList.add('hidden');

    try {
      const res = await fetch('/api/history');
      const data = await res.json();
      const history = data.history || [];

      if (history.length === 0) {
        elements.historyListContainer.innerHTML = '<div class="history-empty">ยังไม่มีประวัติการจัดระเบียบที่บันทึกไว้</div>';
        return;
      }

      elements.historyListContainer.innerHTML = '';
      history.forEach((rec, idx) => {
        const item = document.createElement('div');
        item.className = 'history-item';
        if (idx === 0) item.classList.add('active');

        const dateStr = new Date(rec.created_at).toLocaleString('th-TH');
        const opIcon = rec.operation === 'organize' ? '📂' : '⚡';
        const opName = rec.operation === 'organize' ? 'Organize' : 'Scrape';

        item.innerHTML = `
          <div class="history-item-top">
            <span class="history-item-op">${opIcon} ${opName}</span>
            <span class="history-item-time">${dateStr}</span>
          </div>
          <div class="history-item-stats">
            <span class="history-badge history-badge-success">${rec.success_count} ✅</span>
            ${rec.fail_count > 0 ? `<span class="history-badge history-badge-fail">${rec.fail_count} ❌</span>` : ''}
            <span style="color: var(--text-muted); font-size: 0.72rem;">ทั้งหมด: ${rec.total_items}</span>
            ${rec.dry_run ? '<span style="color: var(--warning); font-size: 0.72rem;">[Dry-Run]</span>' : ''}
          </div>
          <div class="history-item-path" title="${escapeHtml(rec.target_path)}">${escapeHtml(rec.target_path)}</div>
        `;

        item.addEventListener('click', () => {
          document.querySelectorAll('.history-item').forEach(el => el.classList.remove('active'));
          item.classList.add('active');
          loadHistoryDetail(rec.id, rec);
        });

        elements.historyListContainer.appendChild(item);
      });

      if (history.length > 0) {
        loadHistoryDetail(history[0].id, history[0]);
      }
    } catch (err) {
      elements.historyListContainer.innerHTML = `<div class="history-empty">เกิดข้อผิดพลาดในการโหลด: ${escapeHtml(err.message)}</div>`;
    }
  }

  async function loadHistoryDetail(id, summary) {
    elements.historyLogView.textContent = 'กำลังโหลดเนื้อหา Log จากฐานข้อมูล SQLite...';
    const dateStr = new Date(summary.created_at).toLocaleString('th-TH');
    elements.historyDetailTitle.textContent = `${summary.operation.toUpperCase()} (${summary.success_count} สำเร็จ, ${summary.fail_count} ล้มเหลว) - ${dateStr}`;
    elements.btnCopyHistoryLog.classList.remove('hidden');

    try {
      const res = await fetch(`/api/history/detail?id=${id}`);
      const data = await res.json();
      elements.historyLogView.textContent = data.log_text || '(ไม่มีบันทึกข้อความสำหรับรายการนี้)';
    } catch (err) {
      elements.historyLogView.textContent = `ไม่สามารถโหลด Log ได้: ${err.message}`;
    }
  }

  function closeHistoryModal() {
    elements.modalHistory?.classList.add('hidden');
  }

  function appendOrganizerLog(text) {
    if (!elements.organizerLog) return;
    elements.organizerLog.textContent += text;
    if (state.logAutoScroll) {
      elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
    } else {
      if (elements.btnResumeScroll) {
        elements.btnResumeScroll.classList.remove('hidden');
      }
    }
  }

  function setupOrganizer() {
    elements.btnClearLog?.addEventListener('click', () => {
      elements.organizerLog.textContent = 'Console cleared.\n';
      if (elements.btnResumeScroll) elements.btnResumeScroll.classList.add('hidden');
    });

    elements.btnCopyLog?.addEventListener('click', async () => {
      const text = elements.organizerLog?.textContent || '';
      if (!text.trim()) {
        showToast('No logs to copy', 'warning');
        return;
      }
      try {
        await navigator.clipboard.writeText(text);
        showToast('📋 Copied console log to clipboard!', 'success');
      } catch (err) {
        showToast('Failed to copy: ' + err.message, 'danger');
      }
    });

    elements.btnResumeScroll?.addEventListener('click', () => {
      state.logAutoScroll = true;
      if (elements.organizerLog) {
        elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
      }
      elements.btnResumeScroll.classList.add('hidden');
      if (elements.logScrollBadge) {
        elements.logScrollBadge.textContent = 'Auto-Scroll: ON';
        elements.logScrollBadge.classList.remove('paused');
      }
      elements.btnToggleAutoscroll?.classList.add('active');
    });

    elements.btnToggleAutoscroll?.addEventListener('click', () => {
      state.logAutoScroll = !state.logAutoScroll;
      if (state.logAutoScroll) {
        if (elements.organizerLog) {
          elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
        }
        elements.btnResumeScroll?.classList.add('hidden');
        if (elements.logScrollBadge) {
          elements.logScrollBadge.textContent = 'Auto-Scroll: ON';
          elements.logScrollBadge.classList.remove('paused');
        }
        elements.btnToggleAutoscroll.classList.add('active');
        showToast('Auto-scroll resumed ⬇️', 'info');
      } else {
        if (elements.logScrollBadge) {
          elements.logScrollBadge.textContent = 'Auto-Scroll: PAUSED';
          elements.logScrollBadge.classList.add('paused');
        }
        elements.btnToggleAutoscroll.classList.remove('active');
        showToast('Auto-scroll paused ⏸️', 'info');
      }
    });

    elements.organizerLog?.addEventListener('scroll', () => {
      const el = elements.organizerLog;
      const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 45;
      if (isNearBottom) {
        if (!state.logAutoScroll) {
          state.logAutoScroll = true;
          if (elements.logScrollBadge) {
            elements.logScrollBadge.textContent = 'Auto-Scroll: ON';
            elements.logScrollBadge.classList.remove('paused');
          }
          elements.btnResumeScroll?.classList.add('hidden');
          elements.btnToggleAutoscroll?.classList.add('active');
        }
      } else {
        if (state.logAutoScroll) {
          state.logAutoScroll = false;
          if (elements.logScrollBadge) {
            elements.logScrollBadge.textContent = 'Auto-Scroll: PAUSED';
            elements.logScrollBadge.classList.add('paused');
          }
          elements.btnResumeScroll?.classList.remove('hidden');
          elements.btnToggleAutoscroll?.classList.remove('active');
        }
      }
    });

    elements.btnStartOrganize.addEventListener('click', () => {
      startOrganizeStream();
    });
  }

  function setupActressHub() {
    elements.btnAddActress?.addEventListener('click', async () => {
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

    elements.btnRefreshActresses?.addEventListener('click', async () => {
      showToast('Refreshing all actress releases... 🔄', 'info');
      await loadActressesData();
      showToast('All actress releases updated! ✅', 'success');
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

  function startScanStream(customPath) {
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
          elements.orgDestRoot.value = state.activeDir + '/JAV_Library';
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
    state.logAutoScroll = true;
    if (elements.logScrollBadge) {
      elements.logScrollBadge.textContent = 'Auto-Scroll: ON';
      elements.logScrollBadge.classList.remove('paused');
    }
    elements.btnResumeScroll?.classList.add('hidden');
    elements.btnToggleAutoscroll?.classList.add('active');

    const url = `/api/organize/stream?source=${encodeURIComponent(src)}&destination=${encodeURIComponent(dest)}&dry_run=${dryRun}`;
    const es = new EventSource(url);
    state.orgEventSource = es;

    es.addEventListener('start', (e) => {
      try {
        const data = JSON.parse(e.data);
        elements.orgProgressLabel.textContent = `Processing 0 of ${data.total} movies...`;
      } catch (err) {}
    });

    es.addEventListener('step', (e) => {
      try {
        const step = JSON.parse(e.data);
        elements.orgProgressLabel.textContent = `[${step.index}/${step.total}] ${step.movie_id}: ${step.message}`;
        appendOrganizerLog(`   → ${step.message}\n`);
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
        if (item.success && !dryRun && item.movie_id) {
          state.organizedStatus[item.movie_id] = true;
          if (item.target_folder) {
            state.organizedFolders[item.movie_id] = item.target_folder;
          }
        }
        let line = `${status} ${item.movie_id} -> ${item.target_folder || ''}\n`;
        if (item.target_video) {
          line += `   Video: ${item.target_video}\n`;
        }
        if (!item.success && item.error) {
          line += `   ❌ ข้อผิดพลาด: ${item.error}\n`;
        }
        appendOrganizerLog(line);
      } catch (err) {}
    });

    es.addEventListener('done', (e) => {
      try {
        const data = JSON.parse(e.data);
        elements.orgProgressFill.style.width = '100%';
        elements.orgProgressPct.textContent = '100%';
        elements.orgProgressLabel.textContent = `✨ Finished! Organized ${data.success_count} / ${data.total} movies.`;

        appendOrganizerLog(`\n✨ Complete! Successfully processed ${data.success_count}/${data.total} movies.\n`);
        showToast(`Organize complete: ${data.success_count} movies processed!`, 'success');

        renderMoviesGrid();

        setTimeout(() => {
          elements.orgProgressBox.classList.add('hidden');
        }, 2000);
      } catch (err) {}
      es.close();
      state.orgEventSource = null;
      elements.btnStartOrganize.disabled = false;
    });

    es.addEventListener('error', (e) => {
      appendOrganizerLog(`❌ Connection closed or error occurred.\n`);
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

  function createMovieCard(movie) {
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

    // Scraped Badge (Clean pill for footer)
    let scrapedBadge = '';
    if (isScraped) {
      scrapedBadge = `<span class="badge-status badge-scraped" title="Metadata scraped from R18.dev"><i data-lucide="check-circle-2"></i> Scraped</span>`;
    } else if (movie.id) {
      scrapedBadge = `<span class="badge-status badge-unscraped" title="Needs metadata scrape"><i data-lucide="sparkles"></i> Unscraped</span>`;
    } else {
      scrapedBadge = `<span class="badge-status badge-staging" title="No JAV ID matched"><i data-lucide="help-circle"></i> Unmatched</span>`;
    }

    // Organized Badge (Clean pill for footer)
    let organizedBadge = '';
    if (isOrganized) {
      organizedBadge = `<span class="badge-status badge-organized clickable" title="Organized in Jellyfin (Click to open in Finder)" onclick="event.stopPropagation(); window.app.openFolder('${id}')" role="button" tabindex="0"><i data-lucide="folder-check"></i> Organized ↗</span>`;
    } else if (movie.id) {
      organizedBadge = `<span class="badge-status badge-staging" title="Pending NAS organize"><i data-lucide="inbox"></i> Staging</span>`;
    }

    // Watched & Fav Badges (Clean indicator on cover overlay)
    const watchedCoverBadge = uState.is_watched ? '<span class="badge-status badge-watched" title="Watched"><i data-lucide="eye"></i></span>' : '';
    const favCoverBadge = uState.is_favorite ? '<span class="badge-status badge-fav" title="Favorited"><i data-lucide="heart"></i></span>' : '';
    const ratingStr = uState.user_rating ? '⭐'.repeat(uState.user_rating) : '';

    // Date Badge with clear labeling
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

  function openMovieById(id) {
    if (!id) return;
    const existing = state.groupedMovies.find(m => m.id && m.id.toUpperCase() === id.toUpperCase());
    const movieObj = existing || { id: id, files: [] };
    openMovieDetail(movieObj);
  }

  function renderModalContent(movie, meta) {
    const id = movie.id || 'UNMATCHED';
    const uState = state.userStates[id] || {};
    const isOrganized = Boolean(state.organizedStatus[id]);
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

    // Build PhotoSwipe 5 gallery dataset
    const galleryItems = [];
    if (coverUrl) {
      galleryItems.push({
        src: coverUrl,
        fallbackSrc: coverUrl,
        w: 800,
        h: 538,
        width: 800,
        height: 538,
        alt: `Jacket Cover - ${id}`
      });
    }

    // Sample Screenshots with SAFE High-Res URL & Fallbacks
    let screenshotsGrid = '';
    if (meta?.sample_screenshots && meta.sample_screenshots.length > 0) {
      screenshotsGrid = meta.sample_screenshots.map((url, idx) => {
        const highResUrl = getHighResScreenshotUrl(url);
        const galleryIdx = galleryItems.length;
        galleryItems.push({
          src: highResUrl,
          fallbackSrc: url,
          w: 800,
          h: 533,
          width: 800,
          height: 533,
          alt: `Screenshot #${idx + 1} - ${id}`
        });

        return `
          <div class="gallery-thumbnail" data-idx="${galleryIdx}" tabindex="0" role="button" aria-label="View Screenshot #${idx + 1} in gallery"
               onclick="window.app.openGallery(${galleryIdx})"
               onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();window.app.openGallery(${galleryIdx});}">
            <img src="${url}" alt="Screenshot #${idx + 1}" loading="lazy" onerror="this.src='/api/proxy-image?url=${encodeURIComponent(url)}'" />
          </div>
        `;
      }).join('');
    }

    state.currentGallery = galleryItems;

    // FULL-WIDTH HERO COVER LAYOUT
    elements.modalContent.innerHTML = `
      <!-- Full-Width Hero Cover Banner -->
      <div class="modal-hero-banner" role="region" aria-label="Full-Width Cover Image">
        <div class="modal-hero-backdrop" style="background-image: url('${coverUrl}');"></div>
        <img class="modal-hero-cover-img" src="${coverUrl}" alt="Full High-Res Cover for ${escapeHtml(id)}"
             onclick="window.app.openGallery(0)"
             onerror="this.src='/placeholder.png'" />
        <div class="modal-hero-overlay">
          <span class="badge-status" style="background: rgba(12,14,20,0.85); font-family: var(--font-mono); font-size: 0.95rem; font-weight: 800; color: #fff;">
            ${escapeHtml(id)}
          </span>
          <button class="btn-zoom-cover" onclick="window.app.openGallery(0)">
            <i data-lucide="maximize-2"></i> View Fullscreen Gallery (${galleryItems.length})
          </button>
        </div>
      </div>

      <div class="modal-body">
        <div class="modal-info">
          <h2 id="modal-movie-title">${escapeHtml(meta?.title || movie.files[0]?.name || 'No title')}</h2>
          <div class="ja-title">${escapeHtml(meta?.original_title || '')}</div>

          ${isOrganized ? `
            <div class="modal-folder-banner" role="region" aria-label="Organized Destination Folder">
              <div class="folder-banner-info">
                <span class="folder-banner-icon">📁</span>
                <div class="folder-banner-text">
                  <span class="folder-banner-label">Jellyfin Destination Folder:</span>
                  <code class="folder-banner-path" title="${escapeHtml(state.organizedFolders[id] || '')}">${escapeHtml(state.organizedFolders[id] || 'Organized in JAV_Library')}</code>
                </div>
              </div>
              <button class="btn btn-secondary btn-sm btn-open-folder" onclick="window.app.openFolder('${id}', '${escapeHtml(state.organizedFolders[id] || '')}')">
                <i data-lucide="external-link"></i> Open in Finder
              </button>
            </div>
          ` : ''}

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

            ${isOrganized ? `
              <button class="btn btn-secondary" onclick="window.app.openFolder('${id}', '${escapeHtml(state.organizedFolders[id] || '')}')">
                📂 Open Folder
              </button>
              <button class="btn btn-outline-secondary btn-sm" onclick="window.app.organizeSingle('${id}')" title="Re-run Jellyfin organize">
                🔄 Re-organize
              </button>
            ` : `
              <button class="btn btn-primary" onclick="window.app.organizeSingle('${id}')">
                📂 Organize for Jellyfin
              </button>
            `}
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

    if (window.lucide) {
      window.lucide.createIcons();
    }
  }

  // Safe DMM high-res URL converter preventing double jpjp-
  function getHighResScreenshotUrl(url) {
    if (!url) return '';
    if (url.includes('jp-')) return url;
    return url.replace(/([a-z0-9]+)-([0-9]+)\.jpg$/, '$1jp-$2.jpg');
  }

  // PhotoSwipe 5 Dynamic Loader & Controller
  let pswpLightboxModule = null;
  let pswpModule = null;

  async function initPhotoSwipe() {
    if (!pswpLightboxModule) {
      const [lbMod, psMod] = await Promise.all([
        import('/vendor/photoswipe-lightbox.esm.min.js'),
        import('/vendor/photoswipe.esm.min.js')
      ]);
      pswpLightboxModule = lbMod.default;
      pswpModule = psMod.default;
    }
  }

  async function openGallery(index = 0) {
    if (!state.currentGallery || state.currentGallery.length === 0) return;
    try {
      await initPhotoSwipe();

      // Probe already-loaded thumbnails in DOM for pixel-perfect aspect ratios
      state.currentGallery.forEach((item, i) => {
        if (i === 0) {
          const cover = document.querySelector('.modal-hero-cover-img');
          if (cover && cover.naturalWidth && cover.naturalHeight) {
            item.w = cover.naturalWidth;
            item.h = cover.naturalHeight;
            item.width = cover.naturalWidth;
            item.height = cover.naturalHeight;
          }
        } else {
          const thumb = document.querySelector(`.gallery-thumbnail[data-idx="${i}"] img`);
          if (thumb && thumb.naturalWidth && thumb.naturalHeight) {
            // High-res screenshots maintain identical aspect ratio as thumbnails
            item.w = thumb.naturalWidth * 2;
            item.h = thumb.naturalHeight * 2;
            item.width = thumb.naturalWidth * 2;
            item.height = thumb.naturalHeight * 2;
          }
        }
      });

      const lightbox = new pswpLightboxModule({
        dataSource: state.currentGallery,
        pswpModule: pswpModule,
        showHideAnimationType: 'fade',
        bgOpacity: 0.96,
        padding: { top: 24, bottom: 24, left: 24, right: 24 },
      });

      // Recalculate natural dimensions dynamically when high-res image loads
      lightbox.on('loadComplete', ({ slide, content }) => {
        if (content && content.element && content.isImageContent()) {
          const img = content.element;
          const nw = img.naturalWidth;
          const nh = img.naturalHeight;
          if (nw && nh && (slide.width !== nw || slide.height !== nh)) {
            slide.width = nw;
            slide.height = nh;
            if (content.data) {
              content.data.width = nw;
              content.data.height = nh;
              content.data.w = nw;
              content.data.h = nh;
            }
            slide.calculateSize();
            slide.updateContentSize(true);
            slide.applyCurrentZoomPan();
          }
        }
      });

      lightbox.init();
      lightbox.loadAndOpen(index);
    } catch (err) {
      console.error('PhotoSwipe open error:', err);
      const item = state.currentGallery[index];
      if (item) openLightbox(item.src, item.fallbackSrc, item.alt);
    }
  }

  function openLightbox(url, fallbackUrl, caption) {
    elements.lightboxImg.src = url;
    elements.lightboxImg.dataset.fallback = fallbackUrl || '';
    elements.lightboxCaption.textContent = caption || '';
    elements.lightbox.classList.remove('hidden');

    elements.lightboxImg.onerror = function () {
      if (this.dataset.fallback && this.src !== this.dataset.fallback) {
        this.src = this.dataset.fallback;
      } else {
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
              const isMissing = !rel.is_downloaded;
              const statusPill = isMissing
                ? '<span class="pill pill-missing">🔴 Missing / New</span>'
                : '<span class="pill pill-downloaded">🟢 Downloaded</span>';
              const watched = rel.is_watched ? ' • 👁️ Watched' : '';
              return `
                <div class="movie-card ${isMissing ? 'movie-card-missing' : ''}" style="padding: 0.8rem; cursor: pointer;" tabindex="0" role="article" onclick="window.app.openMovie('${escapeHtml(rel.movie_id)}')" onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();window.app.openMovie('${escapeHtml(rel.movie_id)}');}">
                  <img src="${rel.cover_url || '/placeholder.png'}" class="${isMissing ? 'cover-missing' : ''}" alt="Cover for ${escapeHtml(rel.movie_id)}" style="width: 100%; aspect-ratio: 16/10; object-fit: cover; border-radius: var(--radius-sm); margin-bottom: 0.6rem;" />
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
          <div style="display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap;">
            <div class="actress-stats-pills">
              <span class="pill pill-downloaded">🟢 ${entry.downloaded} Downloaded</span>
              <span class="pill pill-missing">🔴 ${entry.missing} Missing</span>
              <span class="pill pill-watched">👁️ ${entry.watched} Watched</span>
            </div>
            <a href="https://r18.dev/search/?search=${encodeURIComponent(a.name)}" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm" title="View ${escapeHtml(a.name)} on R18.dev" style="text-decoration: none; display: inline-flex; align-items: center; gap: 0.35rem;">
              <span>🌐</span> R18.dev ↗
            </a>
            <button class="btn btn-secondary btn-sm" onclick="window.app.trackTitleToActress('${escapeHtml(a.name)}')" title="Track a missing or new JAV-ID for ${escapeHtml(a.name)}">
              <span class="icon">+</span> Track Title
            </button>
            <button class="btn btn-secondary btn-sm" onclick="window.app.refreshSingleActress('${escapeHtml(a.name)}', this)" title="Refresh releases for ${escapeHtml(a.name)}">
              <span class="icon">🔄</span> Refresh
            </button>
            <button class="btn btn-secondary btn-sm" onclick="window.app.unfollowActress('${escapeHtml(a.name)}')">
              🗑️ Unfollow
            </button>
          </div>
        </div>
        ${releasesHtml}
      `;

      elements.actressesContainer.appendChild(section);
    });

    if (window.lucide) {
      window.lucide.createIcons();
    }
  }

  // =========================================================================
  // Live Operation Activity Progress Helpers
  // =========================================================================
  function showOpProgress(icon, title, countStr, pct, message) {
    if (!elements.opProgressBox) return;
    elements.opProgressBox.classList.remove('hidden');
    elements.opIcon.textContent = icon;
    elements.opTitle.textContent = title;
    elements.opCounter.textContent = countStr || '';
    elements.opPct.textContent = `${pct}%`;
    elements.opMessage.textContent = message;
    elements.opProgressFill.style.width = `${pct}%`;
  }

  function hideOpProgress(delayMs = 1500) {
    if (!elements.opProgressBox) return;
    setTimeout(() => {
      elements.opProgressBox.classList.add('hidden');
    }, delayMs);
  }

  // =========================================================================
  // API Action Helpers
  // =========================================================================
  async function scrapeMovie(id) {
    if (!id) return null;
    try {
      showOpProgress('⚡', `กำลัง Scrape ${id}`, '1 / 1', 25, `กำลังค้นหาข้อมูล ${id} จาก R18.dev API...`);
      showToast(`Scraping ${id}...`, 'info');

      // Intermediate step: download cover image
      setTimeout(() => {
        showOpProgress('⚡', `กำลัง Scrape ${id}`, '1 / 1', 65, `กำลังดาวน์โหลดและแคชภาพปก ${id}...`);
      }, 500);

      const res = await fetch(`/api/scrape/${id}`, { method: 'POST' });
      const movie = await res.json();
      state.metadata[id] = movie;
      updateStats();
      populateFilterDropdowns();
      renderMoviesGrid();

      showOpProgress('✅', `Scrape ${id} สำเร็จ!`, '1 / 1', 100, `บันทึกข้อมูลและภาพปก ${id} ลงระบบเรียบร้อย`);
      showToast(`Scraped ${id}!`, 'success');
      hideOpProgress(1800);
      return movie;
    } catch (err) {
      showOpProgress('❌', `Scrape ${id} ไม่สำเร็จ`, '1 / 1', 100, `ข้อผิดพลาด: ${err.message}`);
      showToast(`Scrape failed for ${id}: ${err.message}`, 'danger');
      hideOpProgress(3000);
      return null;
    }
  }

  async function scrapeAllMatched() {
    const matched = state.groupedMovies.filter(m => m.id && !state.metadata[m.id]);
    if (matched.length === 0) {
      showToast('All matched movies are already scraped!', 'info');
      return;
    }

    showOpProgress('⚡', 'กำลัง Batch Scrape', `0 / ${matched.length}`, 0, `กำลังเตรียมการ Scrape ทั้งหมด ${matched.length} เรื่อง...`);
    showToast(`Batch scraping ${matched.length} movies...`, 'info');

    let url = `/api/scrape/stream?path=${encodeURIComponent(state.activeDir || '.')}`;
    const es = new EventSource(url);

    es.addEventListener('start', (e) => {
      try {
        const data = JSON.parse(e.data);
        showOpProgress('⚡', 'กำลัง Batch Scrape', `0 / ${data.total}`, 0, `เริ่ม Scrape ข้อมูล ${data.total} เรื่อง...`);
      } catch (err) {}
    });

    es.addEventListener('step', (e) => {
      try {
        const data = JSON.parse(e.data);
        showOpProgress('⚡', `กำลัง Scrape ${data.movie_id}`, `${data.index} / ${data.total}`, data.percent || 0, data.message);
      } catch (err) {}
    });

    es.addEventListener('item', (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.movie) {
          state.metadata[data.movie_id] = data.movie;
          updateStats();
          populateFilterDropdowns();
          renderMoviesGrid();
        }
        showOpProgress('⚡', `Scraped ${data.movie_id}`, `${data.index} / ${data.total}`, data.percent || 0, data.message || `บันทึก ${data.movie_id} สำเร็จ`);
      } catch (err) {}
    });

    es.addEventListener('done', (e) => {
      try {
        const data = JSON.parse(e.data);
        showOpProgress('✅', 'Batch Scrape เสร็จสมบูรณ์', `${data.success_count || data.success} / ${data.total}`, 100, data.message || `สำเร็จทั้งหมด ${data.success_count || data.success} เรื่อง`);
        showToast(`Batch scraping completed: ${data.success_count || data.success}/${data.total} movies!`, 'success');
      } catch (err) {}
      es.close();
      hideOpProgress(3000);
    });

    es.onerror = () => {
      es.close();
      hideOpProgress(2000);
    };
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
    showOpProgress('📂', `กำลังจัดระเบียบ ${id}`, '0 / 6', 5, `กำลังเริ่มจัดระเบียบ ${id} สำหรับ Jellyfin...`);
    showToast(`Organizing ${id} for Jellyfin...`, 'info');

    let url = `/api/organize/stream?movie_id=${encodeURIComponent(id)}&destination=${encodeURIComponent(dest)}&dry_run=false`;
    const es = new EventSource(url);

    es.addEventListener('step', (e) => {
      try {
        const data = JSON.parse(e.data);
        const stepNum = data.step_current || 1;
        const totalSteps = data.step_total || 6;
        const pct = Math.min(95, Math.round((stepNum / totalSteps) * 100));
        showOpProgress('📂', `กำลังจัดระเบียบ ${data.movie_id}`, `${stepNum} / ${totalSteps}`, pct, data.message);
      } catch (err) {}
    });

    es.addEventListener('item', (e) => {
      try {
        const data = JSON.parse(e.data);
        if (data.success) {
          state.organizedStatus[id] = true;
          if (data.target_folder) {
            state.organizedFolders[id] = data.target_folder;
          }
          renderMoviesGrid();
          if (state.selectedMovieId === id) {
            const cur = state.groupedMovies.find(m => m.id === id);
            if (cur) renderModalContent(cur, state.metadata[id]);
          }
          showOpProgress('✅', `จัดระเบียบ ${id} สำเร็จ!`, '6 / 6', 100, `บันทึกที่: ${data.target_folder || 'JAV_Library'}`);
          showToast(`✅ จัดระเบียบ ${id} เรียบร้อยแล้ว!`, 'success');
        } else {
          showOpProgress('❌', `จัดระเบียบ ${id} ไม่สำเร็จ`, '6 / 6', 100, `ข้อผิดพลาด: ${data.error || 'Unknown error'}`);
          showToast(`Error: ${data.error}`, 'danger');
        }
      } catch (err) {}
    });

    es.addEventListener('done', () => {
      es.close();
      hideOpProgress(3000);
    });

    es.onerror = () => {
      es.close();
      hideOpProgress(2000);
    };
  }

  async function openFolder(id, path) {
    const targetFolder = path || state.organizedFolders[id] || '';
    showToast('📂 กำลังเปิดโฟลเดอร์ใน Finder...', 'info');
    try {
      const res = await fetch('/api/open-folder', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ movie_id: id, path: targetFolder })
      });
      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || 'Failed to open folder');
      }
      showToast(`📂 เปิดโฟลเดอร์เรียบร้อย: ${data.path}`, 'success');
    } catch (err) {
      showToast(`ไม่สามารถเปิดโฟลเดอร์ได้: ${err.message}`, 'danger');
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

  async function trackTitleToActress(actressName) {
    const id = prompt(`Enter JAV ID to track for ${actressName} (e.g. SNOS-373):`);
    if (!id || !id.trim()) return;
    const cleanId = id.trim().toUpperCase();
    showToast(`Fetching metadata for ${cleanId} from R18.dev... ⚡`, 'info');
    try {
      const res = await fetch(`/api/scrape/${encodeURIComponent(cleanId)}`);
      if (!res.ok) {
        const err = await res.json();
        throw new Error(err.error || 'Failed to fetch metadata');
      }
      const data = await res.json();
      showToast(`Tracked ${cleanId}: ${data.title ? data.title.slice(0, 35) + '...' : ''} 🎉`, 'success');
      await loadActressesData();
    } catch (err) {
      showToast(`Error tracking ${cleanId}: ${err.message}`, 'danger');
    }
  }

  async function refreshSingleActress(name, btn) {
    if (btn) btn.classList.add('loading-spin');
    showToast(`Checking releases for ${name}... 🔄`, 'info');
    try {
      await loadActressesData();
      showToast(`Releases for ${name} up to date! ✅`, 'success');
    } catch (err) {
      showToast(`Error updating releases: ${err.message}`, 'danger');
    } finally {
      if (btn) btn.classList.remove('loading-spin');
    }
  }

  // Expose global app object for inline handlers
  window.app = {
    openMovie: openMovieById,
    openGallery,
    openLightbox,
    toggleWatched,
    toggleFavorite,
    setRating,
    toggleFollowActress,
    unfollowActress,
    trackTitleToActress,
    refreshSingleActress,
    organizeSingle,
    openFolder
  };

  document.addEventListener('DOMContentLoaded', init);
})();
