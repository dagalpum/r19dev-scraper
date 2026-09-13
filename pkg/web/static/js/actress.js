/**
 * R19DEV Studio - Actress Hub (Collection, Bento Profile & Chat Feed)
 * Native ES Module
 */

import { state, elements, escapeHtml, formatBytes, showToast, getMovieLocationInfo } from './state.js';
import {
  loadActressesData,
  quickFollowDiscovered,
  refreshSingleActress,
  unfollowActress,
  trackTitleToActress,
  promptAddActress,
  openActiveActressFolder,
  refreshActiveActress,
  followActressFromInput,
  unfollowActiveActress,
  trackTitleActiveActress,
  refreshAllActresses,
  fetchDiscoveredActressMovies
} from './api.js';

export function setupActressHub() {
  elements.btnModeCollection?.addEventListener('click', () => setActressViewMode('collection'));
  elements.btnModeChat?.addEventListener('click', () => setActressViewMode('chat'));
  elements.btnHubRefresh?.addEventListener('click', (e) => refreshAllActresses(e.currentTarget));
  elements.btnHubAdd?.addEventListener('click', promptAddActress);

  elements.btnAddActress?.addEventListener('click', followActressFromInput);

  elements.inputActressName?.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      followActressFromInput();
    }
  });

  elements.btnRefreshActresses?.addEventListener('click', (e) => refreshAllActresses(e.currentTarget));

  elements.chatActressSearch?.addEventListener('input', (e) => {
    state.actressSearchQuery = (e.target.value || '').trim().toLowerCase();
    renderActressHub();
  });

  elements.btnOpenProfileDrawer?.addEventListener('click', () => {
    openActressProfileDrawer();
  });

  elements.btnCloseDrawer?.addEventListener('click', closeActressProfileDrawer);
  elements.drawerProfileBackdrop?.addEventListener('click', closeActressProfileDrawer);

  elements.btnHeaderUnfollow?.addEventListener('click', unfollowActiveActress);
  elements.btnChatTrackTitle?.addEventListener('click', trackTitleActiveActress);
  elements.btnChatRefresh?.addEventListener('click', (e) => refreshActiveActress(e.currentTarget));
  elements.btnChatOpenFolder?.addEventListener('click', openActiveActressFolder);
}

export function getActressLocationStats(entry) {
  if (!entry) {
    return {
      total: 0,
      libCount: 0,
      stagingCount: 0,
      missingCount: 0,
      libPct: 0,
      stagingPct: 0,
      totalDlPct: 0,
      isLibraryComplete: false
    };
  }

  const releases = entry.releases || [];
  const total = entry.total || releases.length;
  let libCount = 0;
  let stagingCount = 0;

  releases.forEach(rel => {
    const loc = getMovieLocationInfo(
      rel.movie_id,
      rel.organized_folder || rel.library_path || '',
      rel.is_downloaded
    );
    if (loc.type === 'library') {
      libCount++;
    } else if (loc.type === 'external' || loc.type === 'staging') {
      stagingCount++;
    }
  });

  const missingCount = Math.max(0, total - (libCount + stagingCount));
  const libPct = total > 0 ? Math.round((libCount / total) * 100) : 0;
  const stagingPct = total > 0 ? Math.round((stagingCount / total) * 100) : 0;
  const totalDlPct = total > 0 ? Math.min(100, Math.round(((libCount + stagingCount) / total) * 100)) : 0;
  const isLibraryComplete = total > 0 && libCount >= total;

  return {
    total,
    libCount,
    stagingCount,
    missingCount,
    libPct,
    stagingPct,
    totalDlPct,
    isLibraryComplete
  };
}

export function getGlobalTopGenres(maxCount = 6) {
  const genreTally = {};
  (state.actresses || []).forEach(entry => {
    (entry.releases || []).forEach(rel => {
      (rel.genres || []).forEach(g => {
        const trimmed = (g || '').trim();
        if (!trimmed) return;
        genreTally[trimmed] = (genreTally[trimmed] || 0) + 1;
      });
    });
  });

  const sorted = Object.entries(genreTally)
    .map(([genre, count]) => ({ genre, count }))
    .sort((a, b) => b.count - a.count);

  return sorted.slice(0, maxCount);
}

export function generateSpiderChartSvg(actressReleases, globalTopGenres, activeFilter = null) {
  if (!globalTopGenres || globalTopGenres.length < 3) {
    return '';
  }

  const cx = 135;
  const cy = 116;
  const R = 64;
  const N = globalTopGenres.length;

  // Tally counts for current actress
  const counts = globalTopGenres.map(g => {
    return (actressReleases || []).filter(r => (r.genres || []).includes(g.genre)).length;
  });

  const maxVal = Math.max(...counts, 1);

  // Background Web Polygons (25%, 50%, 75%, 100%)
  const levels = [0.25, 0.5, 0.75, 1.0];
  const gridPolygons = levels.map(lvl => {
    const pts = [];
    for (let i = 0; i < N; i++) {
      const angle = -Math.PI / 2 + (i * 2 * Math.PI) / N;
      const x = cx + R * lvl * Math.cos(angle);
      const y = cy + R * lvl * Math.sin(angle);
      pts.push(`${x.toFixed(1)},${y.toFixed(1)}`);
    }
    return `<polygon points="${pts.join(' ')}" class="radar-web-ring" />`;
  }).join('');

  // Radial spoke lines
  const spokes = [];
  for (let i = 0; i < N; i++) {
    const angle = -Math.PI / 2 + (i * 2 * Math.PI) / N;
    const sx = cx + R * Math.cos(angle);
    const sy = cy + R * Math.sin(angle);
    spokes.push(`<line x1="${cx}" y1="${cy}" x2="${sx.toFixed(1)}" y2="${sy.toFixed(1)}" class="radar-spoke" />`);
  }

  // Data Polygon Points & Dots
  const dataPts = [];
  const dots = [];
  const labels = [];

  for (let i = 0; i < N; i++) {
    const angle = -Math.PI / 2 + (i * 2 * Math.PI) / N;
    const count = counts[i];
    const fraction = count > 0 ? Math.max(0.18, count / maxVal) : 0.08;
    const px = cx + R * fraction * Math.cos(angle);
    const py = cy + R * fraction * Math.sin(angle);
    dataPts.push(`${px.toFixed(1)},${py.toFixed(1)}`);

    const g = globalTopGenres[i];
    const isFiltered = activeFilter === g.genre;

    dots.push(`
      <circle cx="${px.toFixed(1)}" cy="${py.toFixed(1)}" r="${isFiltered ? '5' : '3.8'}" 
              class="radar-dot ${isFiltered ? 'active' : ''} ${count > 0 ? 'has-data' : ''}"
              onclick="window.app.toggleActressGenreFilter('${escapeHtml(g.genre)}')"
              style="cursor: pointer;">
        <title>${escapeHtml(g.genre)}: ${count} releases</title>
      </circle>
    `);

    // Axis Labels with smart anchor
    const cos = Math.cos(angle);
    const sin = Math.sin(angle);
    const dist = R + 18;
    const lx = cx + dist * cos;
    const ly = cy + dist * sin;

    let textAnchor = 'middle';
    if (cos > 0.25) textAnchor = 'start';
    else if (cos < -0.25) textAnchor = 'end';

    let dy = '0.35em';
    if (sin < -0.6) dy = '-0.15em';
    else if (sin > 0.6) dy = '0.85em';

    // Format shortened label
    const shortLabel = g.genre.length > 12 ? g.genre.substring(0, 11) + '…' : g.genre;

    labels.push(`
      <text x="${lx.toFixed(1)}" y="${ly.toFixed(1)}" dy="${dy}" text-anchor="${textAnchor}"
            class="radar-axis-label ${isFiltered ? 'active' : ''} ${count > 0 ? 'has-data' : ''}"
            onclick="window.app.toggleActressGenreFilter('${escapeHtml(g.genre)}')"
            title="Filter by ${escapeHtml(g.genre)} (${count} works)">
        ${escapeHtml(shortLabel)} <tspan class="radar-label-num">(${count})</tspan>
      </text>
    `);
  }

  return `
    <div class="radar-chart-container" title="Top genres comparison across all actresses">
      <svg viewBox="0 0 270 236" class="radar-chart-svg" aria-label="Style Radar Chart">
        <defs>
          <linearGradient id="radar-fill-grad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stop-color="#10b981" stop-opacity="0.5" />
            <stop offset="100%" stop-color="#6366f1" stop-opacity="0.32" />
          </linearGradient>
          <filter id="radar-glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="2" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>
        <g class="radar-grid">
          ${gridPolygons}
          ${spokes.join('')}
        </g>
        <polygon points="${dataPts.join(' ')}" class="radar-polygon" filter="url(#radar-glow)" />
        <g class="radar-dots">
          ${dots.join('')}
        </g>
        <g class="radar-labels">
          ${labels.join('')}
        </g>
      </svg>
    </div>
  `;
}

export function getCollectionOverallStats() {
  const actresses = state.actresses || [];
  let totalActresses = actresses.length;
  let completedActresses = 0;
  let totalWorks = 0;
  let totalLib = 0;
  let totalStaging = 0;
  let totalMissing = 0;
  let totalBytes = 0;

  actresses.forEach(entry => {
    const stats = getActressLocationStats(entry);
    totalWorks += stats.total;
    totalLib += stats.libCount;
    totalStaging += stats.stagingCount;
    totalMissing += stats.missingCount;
    if (stats.isLibraryComplete) completedActresses++;
    totalBytes += (entry.total_size_bytes || 0);
  });

  const libPct = totalWorks > 0 ? Math.round((totalLib / totalWorks) * 100) : 0;
  const stagingPct = totalWorks > 0 ? Math.round((totalStaging / totalWorks) * 100) : 0;
  const totalDlPct = totalWorks > 0 ? Math.min(100, Math.round(((totalLib + totalStaging) / totalWorks) * 100)) : 0;

  return {
    totalActresses,
    completedActresses,
    totalWorks,
    totalLib,
    totalStaging,
    totalMissing,
    libPct,
    stagingPct,
    totalDlPct,
    totalBytes
  };
}

export function renderHeaderStatsStrip(stats) {
  if (!stats || stats.totalWorks === 0) return '';

  return `
    <div class="dir-header-stats-strip" title="Overall Collection: ${stats.totalLib}/${stats.totalWorks} Lib (${stats.libPct}%) · Staging: +${stats.totalStaging} · Missing: ${stats.totalMissing}">
      <div class="dir-header-progress-wrap">
        <div class="dir-header-progress-track">
          <div class="overall-progress-segment lib-segment" style="width: ${stats.libPct}%;"></div>
          <div class="overall-progress-segment staging-segment" style="width: ${stats.stagingPct}%;"></div>
        </div>
        <div class="dir-header-stat-counts">
          <span class="stat-fraction"><strong>${stats.totalLib}/${stats.totalWorks}</strong>${stats.totalStaging > 0 ? ` <span class="stg-sub">(+${stats.totalStaging})</span>` : ''}</span>
          <span class="stat-pct">${stats.libPct}%</span>
        </div>
      </div>
      ${stats.totalBytes > 0 ? `
        <span class="dir-header-storage" title="Total NAS Storage Used by Followed Actresses">
          <span class="material-symbols-outlined icon">database</span>
          <span>${formatBytes(stats.totalBytes)}</span>
        </span>
      ` : ''}
      ${stats.completedActresses > 0 ? `
        <span class="dir-header-completed" title="${stats.completedActresses} Actresses 100% Completed">
          <span class="material-symbols-outlined icon">verified</span>
          <span>${stats.completedActresses} Done</span>
        </span>
      ` : ''}
    </div>
  `;
}

export function selectActressContact(name) {
  state.activeActressName = name;
  renderActressHub();
}

export function openActressProfileDrawer(entry) {
  if (!entry) {
    entry = state.actresses.find(e => e.actress.name === state.activeActressName);
    if (!entry && state.actresses.length > 0) {
      entry = state.actresses[0];
      state.activeActressName = entry.actress.name;
    }
  }
  if (!entry) {
    showToast('Please select or follow an actress first', 'info');
    return;
  }
  const a = entry.actress;
  const releases = entry.releases || [];
  const total = entry.total || releases.length;
  const dl = entry.downloaded || 0;
  const missing = entry.missing || 0;
  const watched = entry.watched || 0;
  const pct = total > 0 ? Math.round((dl / total) * 100) : 0;

  const r18Url = a.r18_id
    ? `https://r18.dev/videos/vod/movies/list/?id=${a.r18_id}&type=actress`
    : `https://r18.dev/videos/vod/movies/list/?search=${encodeURIComponent(a.name)}`;

  const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

  // Extract unique studios/makers from releases
  const makers = [...new Set(releases.map(r => r.maker).filter(Boolean))];

  if (elements.drawerContent) {
    elements.drawerContent.innerHTML = `
      <div class="drawer-hero">
        <img class="drawer-hero-avatar" src="${avatar}" alt="Avatar of ${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
        <h2>${escapeHtml(a.name)}</h2>
        <div class="drawer-ja">${escapeHtml(a.ja_name || '')}</div>
        ${a.r18_id ? `<div style="font-size: 0.78rem; color: var(--accent-cyan); margin-top: 0.35rem; font-family: monospace;">R18.dev ID: #${a.r18_id}</div>` : ''}
      </div>

      <div class="drawer-progress-box">
        <div class="drawer-progress-header">
          <span>Collection Progress</span>
          <span>${pct}% (${dl}/${total})</span>
        </div>
        <div class="drawer-progress-bar">
          <div class="drawer-progress-fill" style="width: ${pct}%;"></div>
        </div>
      </div>

      <div class="drawer-stats-grid">
        <div class="drawer-stat-card">
          <div class="drawer-stat-val">${total}</div>
          <div class="drawer-stat-label">Total Titles</div>
        </div>
        <div class="drawer-stat-card">
          <div class="drawer-stat-val" style="color: var(--success);"><span class="material-symbols-outlined icon" style="vertical-align: middle;">check_circle</span> ${dl}</div>
          <div class="drawer-stat-label">In Library</div>
        </div>
        <div class="drawer-stat-card">
          <div class="drawer-stat-val" style="color: var(--accent-pink);"><span class="material-symbols-outlined icon" style="vertical-align: middle;">cancel</span> ${missing}</div>
          <div class="drawer-stat-label">Missing / New</div>
        </div>
        <div class="drawer-stat-card">
          <div class="drawer-stat-val" style="color: var(--primary);"><span class="material-symbols-outlined icon" style="vertical-align: middle;">visibility</span> ${watched}</div>
          <div class="drawer-stat-label">Watched</div>
        </div>
      </div>

      ${makers.length > 0 ? `
        <div>
          <h4 style="font-size: 0.85rem; color: var(--text-muted); margin-bottom: 0.6rem; text-transform: uppercase;">Studios & Labels</h4>
          <div style="display: flex; flex-wrap: wrap; gap: 0.4rem;">
            ${makers.map(m => `<span class="pill" style="background: rgba(255,255,255,0.06); font-size: 0.78rem; border: 1px solid rgba(255,255,255,0.1);">${escapeHtml(m)}</span>`).join('')}
          </div>
        </div>
      ` : ''}

      <div class="drawer-links-box" style="margin-top: auto; padding-top: 1rem;">
        <a href="${r18Url}" target="_blank" rel="noopener noreferrer" class="btn btn-primary" style="justify-content: center; text-decoration: none;">
          <span class="material-symbols-outlined icon">public</span> View Official Profile on R18.dev ↗
        </a>
        <button class="btn btn-secondary" onclick="window.app.trackTitleToActress('${escapeHtml(a.name)}')">
          <span class="material-symbols-outlined icon">add</span> Track New JAV-ID
        </button>
        <button class="btn btn-secondary" onclick="window.app.refreshSingleActress('${escapeHtml(a.name)}', this)">
          <span class="material-symbols-outlined icon">sync</span> Refresh Releases
        </button>
        <button class="btn btn-secondary btn-danger-hover" onclick="window.app.unfollowActress('${escapeHtml(a.name)}')">
          <span class="material-symbols-outlined icon">delete</span> Unfollow Actress
        </button>
      </div>
    `;
  }

  elements.drawerActressProfile?.classList.remove('hidden');
  elements.drawerProfileBackdrop?.classList.remove('hidden');
}

export function closeActressProfileDrawer() {
  elements.drawerActressProfile?.classList.add('hidden');
  elements.drawerProfileBackdrop?.classList.add('hidden');
}

export function setActressViewMode(mode) {
  if (!mode) mode = 'collection';
  state.actressViewMode = mode;
  localStorage.setItem('r19dev_actress_view_mode', mode);
  applyActressViewMode();
  if (mode === 'collection') {
    renderActressCollection();
  } else {
    renderChatView();
  }
}

export function applyActressViewMode() {
  const isCollection = state.actressViewMode === 'collection';

  elements.btnModeCollection?.classList.toggle('active', isCollection);
  elements.btnModeCollection?.setAttribute('aria-selected', String(isCollection));
  elements.btnModeChat?.classList.toggle('active', !isCollection);
  elements.btnModeChat?.setAttribute('aria-selected', String(!isCollection));

  elements.actressCollectionView?.classList.toggle('hidden', !isCollection);
  elements.actressChatView?.classList.toggle('hidden', isCollection);
}

export function filterCollectionActress(name, pushHistory = true) {
  const actName = name || 'all';
  state.collectionFilterActress = actName;
  state.collectionSubFilter = 'all';
  state.actressMovieSearch = '';
  state.actressGenreFilter = null;
  state.actressMovieSort = 'date-desc';
  if (actName !== 'all') {
    state.activeActressName = actName;
  }

  window.scrollTo({ top: 0, behavior: 'smooth' });

  if (pushHistory && window.history && window.history.pushState) {
    const newUrl = actName !== 'all' 
      ? `${window.location.pathname}?tab=actresses&actress=${encodeURIComponent(actName)}`
      : `${window.location.pathname}?tab=actresses`;
    window.history.pushState({ tab: 'actresses', actress: actName }, '', newUrl);
  }

  renderActressCollection();
  if (elements.navBreadcrumbActressName) {
    elements.navBreadcrumbActressName.textContent = actName;
  }
}

export function setCollectionSubFilter(sub) {
  state.collectionSubFilter = sub || 'all';
  renderActressCollection();
}

export function toggleActressGenreFilter(genre) {
  if (state.actressGenreFilter === genre) {
    state.actressGenreFilter = null;
  } else {
    state.actressGenreFilter = genre;
  }
  renderActressCollection();
}

export function clearActressGenreFilter() {
  state.actressGenreFilter = null;
  renderActressCollection();
}

export function handleActressMovieSearch(q) {
  state.actressMovieSearch = (q || '').trim();
  if (elements.universalSearchInput && elements.universalSearchInput.value !== (q || '')) {
    elements.universalSearchInput.value = q || '';
    elements.universalSearchClear?.classList.toggle('hidden', !q);
  }
  renderActressCollection();
  const el = document.getElementById('actress-movie-search');
  if (el) {
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  }
}

export function setActressMovieSort(sortKey) {
  state.actressMovieSort = sortKey || 'date-desc';
  renderActressCollection();
}

export function resetActressStageFilters() {
  state.collectionSubFilter = 'all';
  state.actressMovieSearch = '';
  state.actressGenreFilter = null;
  state.actressMovieSort = 'date-desc';
  renderActressCollection();
}

export function scrollShelf(shelfId, direction) {
  const track = document.getElementById(shelfId);
  if (!track) return;
  const distance = track.clientWidth * 0.75;
  track.scrollBy({
    left: direction * distance,
    behavior: 'smooth'
  });
}

export function createPosterCardHtml(rel) {
  const id = rel.movie_id || '';
  const meta = state.metadata[id];
  const folderPath = rel.organized_folder || rel.library_path || state.organizedFolders[id] || '';
  const loc = getMovieLocationInfo(id, folderPath, rel.is_downloaded);

  const coverUrl = rel.cover_url || rel.poster_url || meta?.cover_url || meta?.poster_url || (id ? '/api/images/' + id : '');
  const title = meta?.title || rel.title || id;
  const maker = meta?.maker || rel.maker || '';
  const date = rel.release_date || meta?.release_date || '';
  const actressName = rel.actress_name || (rel.actresses && rel.actresses.length > 0 ? rel.actresses.join(', ') : '');
  const subText = actressName ? `${actressName} • ${maker || 'Studio'}` : (maker || 'R18.dev');

  const imgHtml = coverUrl ?
    `<img class="poster-img" src="${coverUrl}" alt="${escapeHtml(id)}" loading="lazy" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';" />
     <div class="poster-fallback" style="display: none;">
       <div class="poster-fallback-icon"><span class="material-symbols-outlined icon" style="font-size: 2.2rem; color: var(--text-muted);">movie</span></div>
       <div class="poster-fallback-id">${escapeHtml(id)}</div>
     </div>` :
    `<div class="poster-fallback">
       <div class="poster-fallback-icon"><span class="material-symbols-outlined icon" style="font-size: 2.2rem; color: var(--text-muted);">movie</span></div>
       <div class="poster-fallback-id">${escapeHtml(id)}</div>
     </div>`;

  const statusClass = loc.type === 'library' ? 'in-library' : (loc.type === 'external' ? 'in-external' : (loc.type === 'staging' ? 'in-staging' : 'is-missing'));

  return `
    <div class="collection-poster-card ${statusClass}" onclick="window.app.openMovieById('${escapeHtml(id)}')">
      ${imgHtml}
      <div class="poster-info-overlay">
        <div class="poster-badges-row">
          <span class="poster-id-badge ${loc.badgeClass}" title="${escapeHtml(loc.titleText)}">
            <span class="material-symbols-outlined icon" style="font-size: 0.72rem; vertical-align: -1px; margin-right: 2px;">${loc.icon}</span>${escapeHtml(id)}
          </span>
          ${rel.skip_reason ? `<span class="poster-badge-skip-reason">${escapeHtml(rel.skip_reason)}</span>` : ''}
        </div>
        <h4 class="poster-title" title="${escapeHtml(title)}">${escapeHtml(title)}</h4>
        <div class="poster-sub">
          <span title="${escapeHtml(subText)}">${escapeHtml(subText)}</span>
          <span>${escapeHtml(date)}</span>
        </div>
      </div>
      <div class="poster-hover-actions">
        <button class="poster-btn-action poster-btn-primary" onclick="event.stopPropagation(); window.app.openMovieById('${escapeHtml(id)}')">
          <span class="material-symbols-outlined icon">play_arrow</span> Details
        </button>
        <button class="poster-btn-action poster-btn-secondary" onclick="event.stopPropagation(); window.app.copyMovieId('${escapeHtml(id)}', event)">
          <span class="material-symbols-outlined icon">content_copy</span> Copy ID
        </button>
        ${folderPath ? `
          <button class="poster-btn-action poster-btn-secondary" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(folderPath)}" onclick="window.app.openFolderEl(this, event)">
            <span class="material-symbols-outlined icon">folder_open</span> Finder
          </button>
        ` : ''}
      </div>
    </div>
  `;
}

export function createShelfHtml(shelfId, title, icon, count, cardsHtml) {
  return `
    <section class="movie-shelf" aria-labelledby="shelf-title-${shelfId}">
      <div class="shelf-header">
        <div class="shelf-title-group">
          <h3 id="shelf-title-${shelfId}" class="shelf-title">
            <span>${icon}</span>
            <span>${escapeHtml(title)}</span>
          </h3>
          <span class="shelf-count">${count}</span>
        </div>
        <div class="shelf-nav-controls">
          <button class="shelf-nav-btn shelf-nav-prev" aria-label="Scroll left" onclick="window.app.scrollShelf('${shelfId}', -1)">‹</button>
          <button class="shelf-nav-btn shelf-nav-next" aria-label="Scroll right" onclick="window.app.scrollShelf('${shelfId}', 1)">›</button>
        </div>
      </div>
      <div id="${shelfId}" class="shelf-track" role="region" aria-label="${escapeHtml(title)} shelf">
        ${cardsHtml && cardsHtml.length > 0 ? cardsHtml.join('') : '<div class="shelf-empty-state">No releases in this category yet.</div>'}
      </div>
    </section>
  `;
}

export function renderActressCollection() {
  if (!elements.collectionShelvesContainer) return;

  if (!state.actresses || state.actresses.length === 0) {
    if (elements.collectionActressChips) elements.collectionActressChips.innerHTML = '';
    if (elements.collectionActressHero) {
      elements.collectionActressHero.innerHTML = `
        <div style="text-align: center; padding: 2.5rem 1.5rem; color: var(--text-muted);">
          <div style="font-size: 2.8rem; margin-bottom: 0.6rem;"><span class="material-symbols-outlined icon" style="font-size: 3rem;">movie</span></div>
          <h3 style="color: #fff; margin-bottom: 0.5rem; font-size: 1.2rem;">No Followed Actresses Yet</h3>
          <p style="margin-bottom: 1.25rem; font-size: 0.9rem;">Follow your favorite actresses to browse their streaming collections and track missing releases!</p>
          <button class="btn btn-primary" onclick="window.app.promptAddActress()">+ Follow Actress</button>
        </div>
      `;
    }
    elements.collectionShelvesContainer.innerHTML = '';
    return;
  }

  const isAll = state.collectionFilterActress === 'all';

  if (isAll) {
    if (elements.navBreadcrumb) elements.navBreadcrumb.classList.add('hidden');
    if (elements.floatingActressNav) elements.floatingActressNav.classList.add('hidden');
    if (elements.collectionActressHero) {
      elements.collectionActressHero.innerHTML = '';
      elements.collectionActressHero.classList.add('hidden');
    }

    let contentHtml = '';

    if (state.actressHubTab === 'unfollowed' || state.actressHubTab === 'discovered') {
      // Render Unfollowed Actresses (with files in NAS)
      let discList = [...state.discoveredActresses];
      if (state.actressSearchQuery) {
        const q = state.actressSearchQuery.toLowerCase();
        discList = discList.filter(e => {
          const n = (e.name || '').toLowerCase();
          const j = (e.ja_name || '').toLowerCase();
          return n.includes(q) || j.includes(q);
        });
      }

      if (state.actressSort === 'latest-desc') {
        discList.sort((a, b) => (b.latest_release || '').localeCompare(a.latest_release || ''));
      } else {
        discList.sort((a, b) => (b.movie_count || 1) - (a.movie_count || 1));
      }

      contentHtml += `
        <section class="collection-directory-section">
          <div class="directory-section-header">
            <div>
              <h3 class="directory-section-title">Unfollowed Actresses</h3>
              <p class="directory-section-subtitle">Actresses found in your NAS storage who are not yet tracked</p>
            </div>
            <span class="directory-section-badge">${discList.length} Actresses</span>
          </div>
          <div class="actresses-directory-grid">
            ${discList.length === 0 ? `
              <div class="shelf-empty-state" style="grid-column: 1 / -1; padding: 3rem 1.5rem; text-align: center;">
                <span class="material-symbols-outlined" style="font-size: 2.5rem; color: var(--text-muted); margin-bottom: 0.5rem;">check_circle</span>
                <div style="font-weight: 600; font-size: 1.1rem; color: #fff;">All Library Actresses Followed!</div>
                <div style="color: var(--text-muted); font-size: 0.88rem; margin-top: 0.35rem;">Every actress with files in your NAS is currently followed.</div>
              </div>
            ` : discList.map(entry => {
              const avatar = entry.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
              const count = entry.movie_count || 1;
              const latest = entry.latest_release || 'Unknown';

              return `
                <div class="actress-dir-card discovered" onclick="window.app.filterCollectionActress('${escapeHtml(entry.name)}')">
                  <div class="actress-dir-avatar-box">
                    <img class="actress-dir-avatar" src="${avatar}" alt="${escapeHtml(entry.name)}" onerror="this.src='/placeholder.png'" />
                  </div>
                  <div class="actress-dir-body">
                    <div class="actress-dir-header">
                      <span class="actress-dir-name">${escapeHtml(entry.name)}</span>
                    </div>
                    <div class="actress-dir-nas-info">
                      <span class="material-symbols-outlined icon">video_library</span>
                      <span>${count} ${count === 1 ? 'title' : 'titles'} in NAS library</span>
                    </div>
                    <div class="actress-dir-latest-row">
                      <div class="actress-dir-latest-pill nas-info">
                        <span class="material-symbols-outlined icon">calendar_today</span>
                        <span class="actress-dir-latest-id">LATEST</span>
                      </div>
                      <span class="actress-dir-latest-date">${escapeHtml(latest || 'Recent')}</span>
                    </div>
                    <div class="actress-dir-actions">
                      <button class="btn-follow-quick" onclick="event.stopPropagation(); window.app.quickFollowDiscovered('${escapeHtml(entry.name)}', '${escapeHtml(entry.ja_name || '')}', '${escapeHtml(entry.image_url || '')}', this)">
                        <span class="material-symbols-outlined icon">person_add</span> Follow
                      </button>
                    </div>
                  </div>
                </div>
              `;
            }).join('')}
          </div>
        </section>
      `;
    } else {
      // Followed Actresses Directory Grid
      let dirList = [...state.actresses];
      if (state.actressSearchQuery) {
        const q = state.actressSearchQuery.toLowerCase();
        dirList = dirList.filter(e => {
          const n = (e.actress.name || '').toLowerCase();
          const j = (e.actress.ja_name || '').toLowerCase();
          return n.includes(q) || j.includes(q);
        });
      }

      const sortMode = state.actressSort || 'pct-desc';
      if (sortMode === 'completed-first') {
        dirList = dirList.filter(e => {
          const stats = getActressLocationStats(e);
          return stats.isLibraryComplete;
        });
      } else if (sortMode === 'in-progress') {
        dirList = dirList.filter(e => {
          const stats = getActressLocationStats(e);
          return !stats.isLibraryComplete;
        });
      }

      dirList.sort((a, b) => {
        const statsA = getActressLocationStats(a);
        const statsB = getActressLocationStats(b);

        if (sortMode === 'latest-desc') {
          const dateA = a.latest_date || (a.releases && a.releases[0] ? a.releases[0].release_date : '') || '';
          const dateB = b.latest_date || (b.releases && b.releases[0] ? b.releases[0].release_date : '') || '';
          return dateB.localeCompare(dateA) || statsB.libCount - statsA.libCount;
        } else if (sortMode === 'lib-desc') {
          return statsB.libCount - statsA.libCount || statsB.libPct - statsA.libPct;
        } else if (sortMode === 'size-desc') {
          return (b.total_size_bytes || 0) - (a.total_size_bytes || 0) || statsB.libCount - statsA.libCount;
        } else if (sortMode === 'pct-desc') {
          return statsB.libPct - statsA.libPct || statsB.totalDlPct - statsA.totalDlPct || statsB.total - statsA.total;
        } else if (sortMode === 'missing-desc') {
          return statsB.missingCount - statsA.missingCount;
        } else if (sortMode === 'total-desc') {
          return statsB.total - statsA.total;
        } else if (sortMode === 'name-asc') {
          return (a.actress.name || '').localeCompare(b.actress.name || '');
        }
        return 0;
      });

      const overallStats = getCollectionOverallStats();

      contentHtml += `
        <section class="collection-directory-section">
          <div class="directory-section-header with-overall-stats">
            <div class="dir-header-title-group">
              <h3 class="directory-section-title">Followed Actresses</h3>
              <p class="directory-section-subtitle">Click an actress to view her dedicated filmography</p>
            </div>
            ${renderHeaderStatsStrip(overallStats)}
            <div class="dir-header-actions" style="display: flex; align-items: center; gap: 0.65rem;">
              <button class="btn btn-secondary btn-sm" onclick="window.app.openNetworkGraph()" title="Explore Relationship Network Graph">
                <span class="material-symbols-outlined icon">hub</span> Network Graph
              </button>
              <span class="directory-section-badge">${dirList.length} Actresses</span>
            </div>
          </div>
          <div class="actresses-directory-grid">
            ${dirList.length === 0 ? `
              <div class="shelf-empty-state" style="grid-column: 1 / -1; padding: 3rem 1.5rem; text-align: center;">
                <span class="material-symbols-outlined" style="font-size: 2.5rem; color: var(--text-muted); margin-bottom: 0.5rem;">person_search</span>
                <div style="font-weight: 600; font-size: 1.1rem; color: #fff;">No Actresses Match Filter</div>
                <div style="color: var(--text-muted); font-size: 0.88rem; margin-top: 0.35rem;">Try adjusting your search query or sort filter.</div>
              </div>
            ` : dirList.map(entry => {
              const a = entry.actress;
              const stats = getActressLocationStats(entry);
              const isCompleted = stats.isLibraryComplete;
              const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

              const latestDate = entry.latest_date || (entry.releases && entry.releases[0] ? entry.releases[0].release_date : '');
              const latestID = entry.latest_movie_id || (entry.releases && entry.releases[0] ? entry.releases[0].movie_id : '');
              const isLatestDl = entry.latest_is_downloaded !== undefined ? entry.latest_is_downloaded : (entry.releases && entry.releases[0] ? entry.releases[0].is_downloaded : false);

              return `
                <div class="actress-dir-card ${isCompleted ? 'completed' : ''}" onclick="window.app.filterCollectionActress('${escapeHtml(a.name)}')">
                  <div class="actress-dir-avatar-box">
                    <img class="actress-dir-avatar" src="${avatar}" alt="${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
                    ${isCompleted ? `
                      <div class="actress-dir-avatar-badge-verified" title="100% Library Complete (All titles organized in library)">
                        <span class="material-symbols-outlined icon">verified</span>
                      </div>
                    ` : ''}
                  </div>
                  <div class="actress-dir-body">
                    <div class="actress-dir-header">
                      <span class="actress-dir-name" title="${escapeHtml(a.name)}">${escapeHtml(a.name)}</span>
                    </div>
                    <div class="actress-dir-progress-row" title="In Library: ${stats.libCount} | Staging/Archive: ${stats.stagingCount} | Missing: ${stats.missingCount} (Total: ${stats.total})">
                      <div class="actress-dir-progress-track">
                        <div class="actress-dir-progress-segment lib-segment" style="width: ${stats.libPct}%;"></div>
                        <div class="actress-dir-progress-segment staging-segment" style="width: ${stats.stagingPct}%;"></div>
                      </div>
                      <div class="actress-dir-stats-line">
                        <span class="actress-dir-fraction">
                          ${isCompleted ? `
                            <span class="stat-lib-complete"><span class="material-symbols-outlined icon" style="font-size: 0.8rem; vertical-align: -1px;">check_circle</span> ${stats.libCount}/${stats.total}</span>
                          ` : `
                            <span class="stat-lib">${stats.libCount}/${stats.total}</span>${stats.stagingCount > 0 ? ` <span class="stat-staging">(+${stats.stagingCount})</span>` : ''}
                          `}
                        </span>
                        <span class="actress-dir-pct">
                          ${isCompleted ? `
                            <span class="badge-pct-complete">100%</span>
                          ` : `
                            ${stats.libPct}%
                          `}
                        </span>
                      </div>
                    </div>
                    <div class="actress-dir-latest-row">
                      <div class="actress-dir-latest-pill ${isLatestDl ? 'downloaded' : 'missing'}" title="${isLatestDl ? 'Downloaded in NAS' : 'Missing from NAS'}">
                        <span class="material-symbols-outlined icon">${isLatestDl ? 'check_circle' : 'download'}</span>
                        <span class="actress-dir-latest-id">${escapeHtml(latestID || 'LATEST')}</span>
                      </div>
                      <span class="actress-dir-latest-date">${escapeHtml(latestDate || 'Recent')}</span>
                    </div>
                  </div>
                </div>
              `;
            }).join('')}
          </div>
        </section>
      `;
    }

    elements.collectionShelvesContainer.innerHTML = contentHtml;

  } else {
    // Specific Actress Selected -> 2-Column Bento Sidebar & Dedicated Filmography Stage
    if (elements.navBreadcrumb) {
      elements.navBreadcrumb.classList.remove('hidden');
      if (elements.navBreadcrumbActressName) {
        elements.navBreadcrumbActressName.textContent = state.collectionFilterActress;
      }
    }

    if (elements.collectionActressHero) {
      elements.collectionActressHero.innerHTML = '';
      elements.collectionActressHero.classList.add('hidden');
    }

    const activeEntry = state.actresses.find(e => (e.actress.name || '').toLowerCase() === state.collectionFilterActress.toLowerCase());

    if (!activeEntry) {
      // Unfollowed / Discovered in NAS Actress View
      const targetName = state.collectionFilterActress.toLowerCase();
      const discEntry = state.discoveredActresses.find(d => (d.name || '').toLowerCase() === targetName);
      const actName = discEntry ? discEntry.name : state.collectionFilterActress;
      const jaName = discEntry?.ja_name || '';
      const avatar = discEntry?.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

      state.discoveredMoviesCache = state.discoveredMoviesCache || {};
      const cachedMovies = state.discoveredMoviesCache[targetName];

      if (cachedMovies === undefined) {
        if (!state.fetchingDiscoveredMovies) state.fetchingDiscoveredMovies = {};
        if (!state.fetchingDiscoveredMovies[targetName]) {
          state.fetchingDiscoveredMovies[targetName] = true;
          fetchDiscoveredActressMovies(actName).then(movies => {
            state.discoveredMoviesCache[targetName] = movies || [];
            delete state.fetchingDiscoveredMovies[targetName];
            if ((state.collectionFilterActress || '').toLowerCase() === targetName) {
              renderActressCollection();
            }
          });
        }

        elements.collectionShelvesContainer.innerHTML = `
          <div class="discovered-actress-container">
            <div class="discovered-hero-card">
              <div class="discovered-hero-avatar-wrap">
                <img class="discovered-hero-avatar" src="${avatar}" alt="${escapeHtml(actName)}" onerror="this.src='/placeholder.png'" />
              </div>
              <div class="discovered-hero-info">
                <div class="discovered-hero-tag-row">
                  <span class="discovered-badge-nas"><span class="material-symbols-outlined icon">inventory_2</span> Local NAS Titles</span>
                </div>
                <h2 class="discovered-hero-name">${escapeHtml(actName)}</h2>
                ${jaName ? `<div class="discovered-hero-ja">${escapeHtml(jaName)}</div>` : ''}
              </div>
            </div>
            <div style="text-align: center; padding: 4rem 1rem;">
              <span class="material-symbols-outlined spin" style="font-size: 2.5rem; color: var(--primary);">progress_activity</span>
              <p style="margin-top: 1rem; color: var(--text-muted);">Finding local NAS titles for ${escapeHtml(actName)}...</p>
            </div>
          </div>
        `;
        return;
      }

      // Find all titles in local NAS matching this actress
      const localMatches = [];
      const seenIDs = new Set();

      // 1. First add all titles returned by the backend for this actress
      (cachedMovies || []).forEach(rel => {
        if (!rel.movie_id || seenIDs.has(rel.movie_id)) return;
        seenIDs.add(rel.movie_id);
        localMatches.push({
          movie_id: rel.movie_id,
          title: rel.title || rel.movie_id,
          maker: rel.maker || '',
          release_date: rel.release_date || '',
          cover_url: rel.cover_url || `/api/images/${rel.movie_id}`,
          is_downloaded: true,
          genres: rel.genres || [],
          organized_folder: rel.organized_folder || rel.library_path || '',
          library_path: rel.library_path || ''
        });
      });

      // 2. Also check active staging folder (state.groupedMovies)
      (state.groupedMovies || []).forEach(m => {
        if (!m.id || seenIDs.has(m.id)) return;
        const meta = state.metadata[m.id];
        const acts = meta?.actresses || [];
        const hasActress = acts.some(a => (a.name || '').toLowerCase() === targetName || (a.ja_name || '').toLowerCase() === targetName);
        if (hasActress) {
          seenIDs.add(m.id);
          localMatches.push({
            movie_id: m.id,
            title: meta?.title || m.id,
            maker: meta?.maker || '',
            release_date: meta?.release_date || '',
            cover_url: meta?.cover_url || meta?.poster_url || `/api/images/${m.id}`,
            is_downloaded: true,
            genres: meta?.genres || [],
            organized_folder: state.organizedFolders[m.id] || ''
          });
        }
      });

      // 3. Also check other followed actresses in state.actresses for any shared titles
      state.actresses.forEach(entry => {
        (entry.releases || []).forEach(rel => {
          if (!rel.movie_id || seenIDs.has(rel.movie_id)) return;
          const meta = state.metadata[rel.movie_id];
          const acts = meta?.actresses || [];
          const hasActress = acts.some(a => (a.name || '').toLowerCase() === targetName || (a.ja_name || '').toLowerCase() === targetName);
          if (hasActress && (rel.organized_folder || rel.is_downloaded || state.organizedStatus[rel.movie_id])) {
            seenIDs.add(rel.movie_id);
            localMatches.push({
              movie_id: rel.movie_id,
              title: rel.title || rel.movie_id,
              maker: rel.maker || '',
              release_date: rel.release_date || '',
              cover_url: rel.cover_url || `/api/images/${rel.movie_id}`,
              is_downloaded: true,
              genres: rel.genres || [],
              organized_folder: rel.organized_folder || ''
            });
          }
        });
      });

      // Apply search filter if user typed in search input
      let displayMatches = localMatches;
      if (state.actressMovieSearch) {
        const q = state.actressMovieSearch.toLowerCase();
        displayMatches = displayMatches.filter(m => 
          (m.movie_id || '').toLowerCase().includes(q) ||
          (m.title || '').toLowerCase().includes(q) ||
          (m.maker || '').toLowerCase().includes(q)
        );
      }

      // Sort
      if (state.actressMovieSort === 'date-asc') {
        displayMatches.sort((a, b) => (a.release_date || '').localeCompare(b.release_date || ''));
      } else if (state.actressMovieSort === 'id-asc') {
        displayMatches.sort((a, b) => (a.movie_id || '').localeCompare(b.movie_id || ''));
      } else {
        displayMatches.sort((a, b) => (b.release_date || '').localeCompare(a.release_date || ''));
      }

      const cards = displayMatches.map(createPosterCardHtml);

      elements.collectionShelvesContainer.innerHTML = `
        <div class="discovered-actress-container">
          <div class="discovered-hero-card">
            <div class="discovered-hero-avatar-wrap">
              <img class="discovered-hero-avatar" src="${avatar}" alt="${escapeHtml(actName)}" onerror="this.src='/placeholder.png'" />
            </div>
            <div class="discovered-hero-info">
              <div class="discovered-hero-tag-row">
                <span class="discovered-badge-nas"><span class="material-symbols-outlined icon">inventory_2</span> Local NAS Titles</span>
                <span class="discovered-badge-count">${localMatches.length} ${localMatches.length === 1 ? 'Title' : 'Titles'} Available</span>
              </div>
              <h2 class="discovered-hero-name">${escapeHtml(actName)}</h2>
              ${jaName ? `<div class="discovered-hero-ja">${escapeHtml(jaName)}</div>` : ''}
              <p class="discovered-hero-desc">
                Currently displaying all titles featuring this actress found in your local NAS storage. Follow her to load her complete official filmography from R18.dev and track missing releases.
              </p>
            </div>
            <div class="discovered-hero-actions">
              <button class="btn btn-primary btn-follow-large" onclick="window.app.quickFollowDiscovered('${escapeHtml(actName)}', '${escapeHtml(jaName)}', '${escapeHtml(avatar)}', this)">
                <span class="material-symbols-outlined icon">person_add</span> Follow Actress
              </button>
              <button class="btn btn-secondary" onclick="window.app.filterCollectionActress('all')">
                <span class="material-symbols-outlined icon">arrow_back</span> All Actresses
              </button>
            </div>
          </div>

          <section class="discovered-movies-section">
            <div class="directory-section-header">
              <div>
                <h3 class="directory-section-title">Titles in Your Library</h3>
                <p class="directory-section-subtitle">Playable media files located in local NAS storage</p>
              </div>
              <span class="directory-section-badge">${localMatches.length} Titles</span>
            </div>
            <div class="collection-stage-grid">
              ${cards.length > 0 ? cards.join('') : '<div class="shelf-empty-state" style="grid-column: 1 / -1; padding: 3rem 1.5rem; text-align: center;">No matching video files found in local storage.</div>'}
            </div>
          </section>
        </div>
      `;
      return;
    }

    const a = activeEntry.actress;
    const releases = activeEntry.releases || [];
    const heroStats = getActressLocationStats(activeEntry);
    const total = heroStats.total;
    const dl = heroStats.libCount + heroStats.stagingCount;
    const missing = heroStats.missingCount;
    const pct = heroStats.libPct;
    const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
    const totalBytes = activeEntry.total_size_bytes || 0;
    const avgBytes = dl > 0 && totalBytes > 0 ? Math.round(totalBytes / dl) : 0;
    const topGenres = activeEntry.top_genres || [];
    const globalTopGenres = getGlobalTopGenres(6);
    const spiderChartHtml = generateSpiderChartSvg(releases, globalTopGenres, state.actressGenreFilter);
    const debutDate = activeEntry.debut_date || '';
    const latestDate = activeEntry.latest_date || '';

    let careerSpan = '';
    if (debutDate) {
      const dYear = debutDate.substring(0, 4);
      const lYear = latestDate ? latestDate.substring(0, 4) : '';
      if (dYear && lYear && dYear !== lYear) {
        careerSpan = `${dYear} – ${lYear}`;
      } else if (dYear) {
        careerSpan = `Debut ${dYear}`;
      }
    }

    const r18Url = a.r18_id
      ? `https://r18.dev/videos/vod/movies/list/?id=${a.r18_id}&type=actress`
      : `https://r18.dev/videos/vod/movies/list/?search=${encodeURIComponent(a.name)}`;

    // Group into In Library vs Missing
    const inLibraryReleases = [];
    const missingReleases = [];
    const skippedReleases = activeEntry.skipped_releases || [];
    releases.forEach(rel => {
      const loc = getMovieLocationInfo(rel.movie_id);
      if (loc.type === 'library' || loc.type === 'external' || loc.type === 'staging' || rel.is_downloaded || Boolean(state.organizedStatus[rel.movie_id])) {
        inLibraryReleases.push(rel);
      } else {
        missingReleases.push(rel);
      }
    });

    // Filter pipeline: Sub-filter -> Genre -> Search -> Sort
    const subFilter = state.collectionSubFilter || 'all';
    let filtered = releases;
    if (subFilter === 'in_library') {
      filtered = inLibraryReleases;
    } else if (subFilter === 'missing') {
      filtered = missingReleases;
    } else if (subFilter === 'skipped') {
      filtered = skippedReleases;
    }

    if (state.actressGenreFilter) {
      filtered = filtered.filter(rel => (rel.genres || []).includes(state.actressGenreFilter));
    }

    if (state.actressMovieSearch) {
      const q = state.actressMovieSearch.toLowerCase();
      filtered = filtered.filter(rel => {
        const mid = (rel.movie_id || '').toLowerCase();
        const title = (rel.title || '').toLowerCase();
        const maker = (rel.maker || '').toLowerCase();
        return mid.includes(q) || title.includes(q) || maker.includes(q);
      });
    }

    const sortMode = state.actressMovieSort || 'date-desc';
    filtered = [...filtered].sort((x, y) => {
      if (sortMode === 'date-desc') {
        return (y.release_date || '').localeCompare(x.release_date || '');
      } else if (sortMode === 'date-asc') {
        return (x.release_date || '').localeCompare(y.release_date || '');
      } else if (sortMode === 'size-desc') {
        return (y.size_bytes || 0) - (x.size_bytes || 0);
      } else if (sortMode === 'rating-desc') {
        const rX = state.userStates[x.movie_id]?.user_rating || 0;
        const rY = state.userStates[y.movie_id]?.user_rating || 0;
        return rY - rX;
      } else if (sortMode === 'id-asc') {
        return (x.movie_id || '').localeCompare(y.movie_id || '');
      }
      return 0;
    });

    const cardsHtml = filtered.map(createPosterCardHtml);

    const contentHtml = `
      <div class="actress-detail-layout">
        <!-- LEFT COLUMN: Sticky Bento Profile Sidebar -->
        <aside class="actress-bento-sidebar">
          <button class="bento-back-btn" onclick="window.app.filterCollectionActress('all')">
            <span class="back-arrow">←</span> All Followed Actresses
          </button>

          <!-- Bento 1: Identity Card -->
          <div class="bento-card bento-card-identity ${heroStats.isLibraryComplete ? 'completed' : ''}">
            <div class="bento-avatar-wrapper" style="position: relative;">
              <img class="bento-avatar" src="${avatar}" alt="${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
              ${heroStats.isLibraryComplete ? `
                <div class="actress-dir-avatar-badge-verified" style="bottom: -2px; right: -2px;" title="100% Library Complete">
                  <span class="material-symbols-outlined icon">verified</span>
                </div>
              ` : ''}
            </div>
            <div class="bento-identity-info">
              <h2 class="bento-name">${escapeHtml(a.name)}</h2>
              ${a.ja_name ? `<div class="bento-ja-name">${escapeHtml(a.ja_name)}</div>` : ''}
              <div class="bento-badges-row">
                ${careerSpan ? `<span class="bento-career-badge"><span class="material-symbols-outlined icon">calendar_month</span> ${escapeHtml(careerSpan)}</span>` : ''}
                ${a.r18_id ? `<span class="bento-badge-mono">R18 #${a.r18_id}</span>` : ''}
                <span class="bento-badge-count">${total} Works</span>
              </div>
            </div>
          </div>

          <!-- Bento 2: Storage & Library Progress -->
          <div class="bento-card bento-card-storage">
            <div class="bento-card-header">
              <span class="bento-card-title"><span class="material-symbols-outlined icon">database</span> Library & Storage</span>
              <span class="bento-storage-highlight">${formatBytes(totalBytes)}</span>
            </div>
            <div class="bento-progress-box" title="In Library: ${heroStats.libCount} | Staging/Archive: ${heroStats.stagingCount} | Missing: ${heroStats.missingCount} (Total: ${heroStats.total})">
              <div class="actress-dir-progress-track">
                <div class="actress-dir-progress-segment lib-segment" style="width: ${heroStats.libPct}%;"></div>
                <div class="actress-dir-progress-segment staging-segment" style="width: ${heroStats.stagingPct}%;"></div>
              </div>
              <div class="bento-progress-meta">
                <span class="bento-progress-count">
                  ${heroStats.isLibraryComplete 
                    ? `<span style="color: #34d399; font-weight: 700;"><span class="material-symbols-outlined icon" style="font-size: 0.82rem; vertical-align: -1px;">check_circle</span> ${heroStats.libCount}/${heroStats.total}</span>`
                    : `<span style="color: #34d399; font-weight: 600;">${heroStats.libCount}/${heroStats.total}</span>${heroStats.stagingCount > 0 ? ` <span style="color: #fbbf24; font-weight: 600;">(+${heroStats.stagingCount})</span>` : ''}`
                  }
                </span>
                <span class="bento-progress-pct">${heroStats.isLibraryComplete ? '<span style="color: #34d399; font-weight: 800;">100%</span>' : `${heroStats.libPct}%`}</span>
              </div>
            </div>
            ${dl > 0 && avgBytes > 0 ? `
              <div class="bento-storage-footnote">
                <span>Average size:</span> <strong>${formatBytes(avgBytes)} / file</strong>
              </div>
            ` : ''}
          </div>

          <!-- Bento 3: Style Radar (Spider Chart) & Top Genres -->
          <div class="bento-card bento-card-genres">
            <div class="bento-card-header">
              <span class="bento-card-title"><span class="material-symbols-outlined icon">radar</span> Style Radar</span>
              <div class="bento-card-header-actions">
                ${state.actressGenreFilter ? `<button class="bento-chip-reset" onclick="window.app.clearActressGenreFilter()">Clear</button>` : ''}
                <button class="bento-graph-btn" onclick="window.app.openNetworkGraph('${escapeHtml(a.name)}')" title="Explore Relationship Network Graph">
                  <span class="material-symbols-outlined icon">hub</span> Graph
                </button>
              </div>
            </div>
            ${spiderChartHtml}
            ${topGenres.length > 0 ? `
              <div class="bento-genres-cloud">
                ${topGenres.map(g => {
                  const isActive = state.actressGenreFilter === g.genre;
                  return `
                    <button class="bento-genre-chip ${isActive ? 'active' : ''}" 
                            onclick="window.app.toggleActressGenreFilter('${escapeHtml(g.genre)}')"
                            title="${isActive ? 'Click to remove filter' : `Filter by ${escapeHtml(g.genre)}`}">
                      <span class="genre-name">${escapeHtml(g.genre)}</span>
                      <span class="genre-count">${g.count}</span>
                    </button>
                  `;
                }).join('')}
              </div>
            ` : `
              <div class="bento-empty-text">Detailed genres unlock as movies are scraped.</div>
            `}
          </div>

          <!-- Bento 4: Quick Actions -->
          <div class="bento-card bento-card-actions">
            <div class="bento-card-header">
              <span class="bento-card-title"><span class="material-symbols-outlined icon">bolt</span> Quick Actions</span>
            </div>
            <div class="bento-action-buttons">
              <button class="btn btn-secondary bento-action-btn" title="Open Actress folder in Finder / NAS" onclick="window.app.openActiveActressFolder()">
                <span class="material-symbols-outlined icon">folder_open</span> Open in Finder
              </button>
              <a href="${r18Url}" target="_blank" rel="noopener noreferrer" class="btn btn-secondary bento-action-btn" title="Official R18.dev Filmography">
                <span class="material-symbols-outlined icon">public</span> R18.dev Profile ↗
              </a>
              <button class="btn btn-secondary bento-action-btn" title="Check & refresh releases from R18.dev" onclick="window.app.refreshActiveActress(this)">
                <span class="material-symbols-outlined icon">sync</span> Refresh Releases
              </button>
            </div>
          </div>
        </aside>

        <!-- RIGHT COLUMN: Dedicated Main Stage -->
        <main class="actress-main-stage">
          <!-- Stage Toolbar -->
          <div class="actress-stage-toolbar">
            <div class="stage-toolbar-left">
              <!-- Sub-Filter Pills -->
              <div class="stage-filter-pills">
                <button class="stage-pill ${subFilter === 'all' ? 'active' : ''}" onclick="window.app.setCollectionSubFilter('all')">
                  All Works <span class="pill-badge">${total}</span>
                </button>
                <button class="stage-pill ${subFilter === 'in_library' ? 'active' : ''}" onclick="window.app.setCollectionSubFilter('in_library')">
                  In Library <span class="pill-badge">${dl}</span>
                </button>
                <button class="stage-pill ${subFilter === 'missing' ? 'active' : ''}" onclick="window.app.setCollectionSubFilter('missing')">
                  Missing <span class="pill-badge">${missing}</span>
                </button>
                ${skippedReleases.length > 0 ? `
                <button class="stage-pill ${subFilter === 'skipped' ? 'active' : ''}" onclick="window.app.setCollectionSubFilter('skipped')">
                  <span class="material-symbols-outlined icon" style="font-size: 0.95rem; vertical-align: -2px; margin-right: 0.2rem;">filter_list_off</span>
                  Skipped <span class="pill-badge">${skippedReleases.length}</span>
                </button>
                ` : ''}
              </div>

              <!-- Active Genre Badge (if clicked from Bento) -->
              ${state.actressGenreFilter ? `
                <div class="stage-active-genre-tag">
                  <span>Genre: <strong>${escapeHtml(state.actressGenreFilter)}</strong></span>
                  <button class="stage-tag-close" onclick="window.app.clearActressGenreFilter()" title="Clear genre filter"><span class="material-symbols-outlined icon">close</span></button>
                </div>
              ` : ''}
            </div>

            <!-- Stage Toolbar Right: Sort Dropdown -->
            <div class="stage-toolbar-right">
              <label class="stage-sort-label" for="actress-sort-select">Sort:</label>
              <select id="actress-sort-select" class="stage-sort-select" onchange="window.app.setActressMovieSort(this.value)">
                <option value="date-desc" ${sortMode === 'date-desc' ? 'selected' : ''}>Release Date (Newest)</option>
                <option value="date-asc" ${sortMode === 'date-asc' ? 'selected' : ''}>Release Date (Oldest)</option>
                <option value="size-desc" ${sortMode === 'size-desc' ? 'selected' : ''}>File Size (Largest)</option>
                <option value="id-asc" ${sortMode === 'id-asc' ? 'selected' : ''}>Movie ID (A-Z)</option>
              </select>
            </div>
          </div>

          <!-- Stage Poster Grid -->
          <div class="collection-vertical-grid">
            ${cardsHtml.length > 0 ? cardsHtml.join('') : `
              <div class="grid-empty-state">
                <div class="empty-icon"><span class="material-symbols-outlined icon" style="font-size: 3rem;">movie</span></div>
                <h4>No Releases Found</h4>
                <p>No movies match your selected filters for ${escapeHtml(a.name)}.</p>
                <button class="btn btn-secondary btn-sm" onclick="window.app.resetActressStageFilters()">Clear Filters</button>
              </div>
            `}
          </div>
        </main>
      </div>
    `;

    elements.collectionShelvesContainer.innerHTML = contentHtml;
  }
}

export function renderActressHub() {
  applyActressViewMode();
  renderChatView();
  renderActressCollection();
}

export function renderChatView() {
  if (!elements.chatContactsList) return;

  elements.chatFriendsCount.textContent = state.actresses.length;
  elements.countActresses.textContent = state.actresses.length;

  let list = state.actresses;
  if (state.actressSearchQuery) {
    const q = state.actressSearchQuery;
    list = list.filter(entry => {
      const name = (entry.actress.name || '').toLowerCase();
      const ja = (entry.actress.ja_name || '').toLowerCase();
      return name.includes(q) || ja.includes(q);
    });
  }

  elements.actressesEmpty.classList.toggle('hidden', state.actresses.length > 0);

  if (!state.activeActressName && state.actresses.length > 0) {
    const haya = state.actresses.find(e => e.actress.name.toLowerCase().includes('hayasaka'));
    state.activeActressName = haya ? haya.actress.name : state.actresses[0].actress.name;
  }

  // 1. Render Left Sidebar Contacts
  elements.chatContactsList.innerHTML = '';
  list.forEach(entry => {
    const a = entry.actress;
    const releases = entry.releases || [];
    const isActive = a.name === state.activeActressName;
    const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

    let snippet = 'No releases recorded';
    if (releases.length > 0) {
      const latest = releases[0];
      snippet = `${latest.movie_id} (${latest.release_date || 'Recent'})`;
    }

    const item = document.createElement('div');
    item.className = `chat-contact-item ${isActive ? 'active' : ''}`;
    item.setAttribute('role', 'listitem');
    item.tabIndex = 0;
    item.onclick = () => selectActressContact(a.name);
    item.onkeydown = (e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        selectActressContact(a.name);
      }
    };

    item.innerHTML = `
      <div class="chat-avatar-wrapper">
        <img class="chat-avatar-img" src="${avatar}" alt="Avatar of ${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
        <span class="status-dot online"></span>
      </div>
      <div class="chat-contact-info">
        <div class="chat-contact-top">
          <span class="chat-contact-name">${escapeHtml(a.name)}<span class="chat-contact-ja">${escapeHtml(a.ja_name || '')}</span></span>
          ${entry.missing > 0 ? `<span class="chat-badge-missing" title="${entry.missing} titles missing">${entry.missing} New</span>` : ''}
        </div>
        <div class="chat-contact-snippet">${escapeHtml(snippet)}</div>
      </div>
    `;

    elements.chatContactsList.appendChild(item);
  });

  // 2. Render Active Chat Feed & Header
  const activeEntry = state.actresses.find(entry => entry.actress.name === state.activeActressName);
  renderActiveActressChat(activeEntry);
}

export function renderActiveActressChat(entry) {
  if (!entry) {
    elements.chatHeaderAvatar.src = '/placeholder.png';
    elements.chatHeaderName.textContent = 'Select an Actress';
    elements.chatHeaderJa.textContent = '';
    elements.chatHeaderSubtitle.textContent = 'Select a contact on the left to view release timeline';
    elements.chatHeaderControls.classList.add('hidden');
    elements.chatActionBar.classList.add('hidden');
    elements.chatMessagesFeed.innerHTML = `
      <div class="chat-welcome-placeholder">
        <div class="welcome-icon"><span class="material-symbols-outlined" style="font-size: 3rem; color: var(--primary);">forum</span></div>
        <h3>Welcome to Actress Release Chat</h3>
        <p>Select an actress on the left to see her releases, download status, and Jellyfin history!</p>
      </div>
    `;
    return;
  }

  const a = entry.actress;
  const releases = [...(entry.releases || [])];
  const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

  elements.chatHeaderAvatar.src = avatar;
  elements.chatHeaderName.textContent = a.name;
  elements.chatHeaderJa.textContent = a.ja_name || '';
  elements.chatHeaderSubtitle.innerHTML = `${entry.total} titles tracked • <span class="material-symbols-outlined icon" style="font-size: 1rem; vertical-align: middle; color: var(--success);">check_circle</span> ${entry.downloaded} Downloaded • <span class="material-symbols-outlined icon" style="font-size: 1rem; vertical-align: middle; color: var(--accent-pink);">cancel</span> ${entry.missing} Missing`;
  elements.chatHeaderControls.classList.remove('hidden');
  elements.chatActionBar.classList.remove('hidden');

  const r18Url = a.r18_id
    ? `https://r18.dev/videos/vod/movies/list/?id=${a.r18_id}&type=actress`
    : `https://r18.dev/videos/vod/movies/list/?search=${encodeURIComponent(a.name)}`;
  elements.linkHeaderR18.href = r18Url;

  if (releases.length === 0) {
    elements.chatMessagesFeed.innerHTML = `
      <div class="chat-welcome-placeholder">
        <div class="welcome-icon"><span class="material-symbols-outlined icon" style="font-size: 3rem;">movie</span></div>
        <h3>No releases tracked yet for ${escapeHtml(a.name)}</h3>
        <p>Click "+ Track JAV-ID" below to add her releases, or refresh from database.</p>
      </div>
    `;
    return;
  }

  releases.sort((x, y) => (x.release_date || '').localeCompare(y.release_date || ''));

  let feedHtml = '';
  let lastDateGroup = '';

  releases.forEach((rel) => {
    const relDate = rel.release_date || 'Unknown Date';
    const dateGroup = relDate.slice(0, 7); // YYYY-MM
    if (dateGroup !== lastDateGroup) {
      feedHtml += `
        <div class="chat-date-divider">
          <span class="chat-date-badge"><span class="material-symbols-outlined icon" style="font-size: 0.95rem; vertical-align: middle;">calendar_month</span> Release Month: ${escapeHtml(dateGroup)}</span>
        </div>
      `;
      lastDateGroup = dateGroup;
    }

    const isMissing = !rel.is_downloaded;
    const isOrganized = Boolean(rel.organized_folder);
    const isStaging = rel.is_downloaded && !isOrganized;

    feedHtml += `
      <div class="chat-msg-row actress">
        <img class="chat-msg-avatar" src="${avatar}" alt="${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
        <div class="chat-bubble">
          <div class="chat-bubble-text">
            Hello! Release <strong>${escapeHtml(rel.movie_id)}</strong> was released on ${escapeHtml(rel.release_date || '-')}.
          </div>
          <div class="chat-embedded-card">
            <div class="chat-card-cover-box ${isMissing ? 'missing' : ''}" onclick="window.app.openMovie('${escapeHtml(rel.movie_id)}')">
              <img src="${rel.cover_url || (rel.movie_id ? '/api/images/' + rel.movie_id : '/placeholder.png')}" alt="Cover ${escapeHtml(rel.movie_id)}" onerror="this.onerror=null; this.src='/placeholder.png';" />
            </div>
            <div class="chat-card-body">
              <div class="chat-card-id-row">
                <span class="chat-card-id">${escapeHtml(rel.movie_id)}</span>
                <button class="btn-copy-id" onclick="window.app.copyMovieId('${escapeHtml(rel.movie_id)}', event)" title="Copy ${escapeHtml(rel.movie_id)}">
                  <span class="material-symbols-outlined icon">content_copy</span> Copy ID
                </button>
              </div>
              <div class="chat-card-title">${escapeHtml(rel.title)}</div>
              <div class="chat-card-meta">
                <span>${escapeHtml(rel.maker || 'Studio')}</span>
                <button class="chat-btn-inspect" onclick="window.app.openMovie('${escapeHtml(rel.movie_id)}')">
                  <span class="material-symbols-outlined icon">search</span> View Details
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    `;

    let userStatusHtml = '';
    if (isOrganized) {
      userStatusHtml = `
        <div class="user-status-box">
          <div class="user-status-title" style="color: var(--success); display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;">
            <span><span class="material-symbols-outlined icon" style="vertical-align: middle;">check_circle</span> In Jellyfin Library</span>
            <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.organized_folder || '')}" onclick="window.app.openFolderEl(this, event)" style="padding: 0.2rem 0.55rem; font-size: 0.72rem; border-radius: 6px; cursor: pointer; white-space: nowrap;">
              <span class="material-symbols-outlined icon">folder_open</span> Open in Finder
            </button>
          </div>
          <div class="user-status-path" style="cursor: pointer;" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.organized_folder || '')}" onclick="window.app.openFolderEl(this, event)" title="Click to open in Finder"><span class="material-symbols-outlined icon" style="vertical-align: middle;">folder</span> ${escapeHtml(rel.organized_folder)}</div>
        </div>
      `;
    } else if (isStaging) {
      userStatusHtml = `
        <div class="user-status-box">
          <div class="user-status-title" style="color: var(--warning); display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;">
            <span><span class="material-symbols-outlined icon" style="vertical-align: middle;">inbox</span> Downloaded (Pending Organization)</span>
            <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.library_path || '')}" onclick="window.app.openFolderEl(this, event)" style="padding: 0.2rem 0.55rem; font-size: 0.72rem; border-radius: 6px; cursor: pointer; white-space: nowrap;">
              <span class="material-symbols-outlined icon">folder_open</span> Open in Finder
            </button>
          </div>
          <div class="user-status-path" style="cursor: pointer;" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.library_path || '')}" onclick="window.app.openFolderEl(this, event)" title="Click to open in Finder"><span class="material-symbols-outlined icon" style="vertical-align: middle;">folder</span> ${escapeHtml(rel.library_path)}</div>
        </div>
      `;
    } else {
      userStatusHtml = `
        <div class="user-status-box">
          <div class="user-status-title" style="color: var(--accent-pink);">
            <span><span class="material-symbols-outlined icon" style="vertical-align: middle;">schedule</span> Missing from Storage</span>
          </div>
          <div style="font-size: 0.78rem; color: var(--text-muted);">
            Not yet downloaded. Click Copy ID above to search and download.
          </div>
        </div>
      `;
    }

    feedHtml += `
      <div class="chat-msg-row user">
        <div class="chat-bubble">
          ${userStatusHtml}
        </div>
      </div>
    `;
  });

  elements.chatMessagesFeed.innerHTML = feedHtml;

  setTimeout(() => {
    elements.chatMessagesFeed.scrollTop = elements.chatMessagesFeed.scrollHeight;
  }, 50);
}

export function renderAllMoviesCatalogHtml() {
  // Aggregate all unique releases across all followed actresses
  const moviesMap = new Map();
  const genreCountMap = new Map();
  const studioCountMap = new Map();
  const actressCountMap = new Map();

  state.actresses.forEach(entry => {
    const actName = entry.actress.name;
    (entry.releases || []).forEach(rel => {
      const id = rel.movie_id;
      if (!id) return;
      if (!moviesMap.has(id)) {
        moviesMap.set(id, {
          ...rel,
          actresses: [actName]
        });
      } else {
        const existing = moviesMap.get(id);
        if (existing.actresses && !existing.actresses.includes(actName)) {
          existing.actresses.push(actName);
        }
      }
    });
  });

  const allMovies = Array.from(moviesMap.values());

  // Count library vs missing vs watched vs favorites
  let libraryCount = 0;
  let watchedCount = 0;
  let favCount = 0;
  allMovies.forEach(m => {
    const inLib = !!(m.organized_folder || m.library_path || m.is_downloaded || state.organizedStatus[m.movie_id]);
    if (inLib) libraryCount++;

    const isWatched = !!(m.is_watched || state.userStates?.[m.movie_id]?.is_watched);
    const isFav = !!(m.is_favorite || state.userStates?.[m.movie_id]?.is_favorite);
    if (isWatched) watchedCount++;
    if (isFav) favCount++;

    // Tally genres
    (m.genres || []).forEach(g => {
      if (g) genreCountMap.set(g, (genreCountMap.get(g) || 0) + 1);
    });
    // Tally studios
    if (m.maker) {
      studioCountMap.set(m.maker, (studioCountMap.get(m.maker) || 0) + 1);
    }
    // Tally actresses
    (m.actresses || []).forEach(a => {
      if (a) actressCountMap.set(a, (actressCountMap.get(a) || 0) + 1);
    });
  });
  const missingCount = allMovies.length - libraryCount;
  if (elements.countCatalog) {
    elements.countCatalog.textContent = String(libraryCount);
  }

  // Filter list
  let filtered = allMovies;

  // 1. Status Filter
  if (state.allMoviesFilterStatus === 'library') {
    filtered = filtered.filter(m => m.organized_folder || m.library_path || m.is_downloaded || state.organizedStatus[m.movie_id]);
  } else if (state.allMoviesFilterStatus === 'missing') {
    filtered = filtered.filter(m => !(m.organized_folder || m.library_path || m.is_downloaded || state.organizedStatus[m.movie_id]));
  } else if (state.allMoviesFilterStatus === 'watched') {
    filtered = filtered.filter(m => !!(m.is_watched || state.userStates?.[m.movie_id]?.is_watched));
  } else if (state.allMoviesFilterStatus === 'favorite') {
    filtered = filtered.filter(m => !!(m.is_favorite || state.userStates?.[m.movie_id]?.is_favorite));
  }

  // 2. Genre Filter
  if (state.allMoviesFilterGenre) {
    filtered = filtered.filter(m => m.genres && m.genres.includes(state.allMoviesFilterGenre));
  }

  // 3. Studio Filter
  if (state.allMoviesFilterStudio) {
    const sLow = state.allMoviesFilterStudio.toLowerCase();
    filtered = filtered.filter(m => (m.maker || '').toLowerCase() === sLow);
  }

  // 4. Actress Filter
  if (state.allMoviesFilterActress) {
    filtered = filtered.filter(m => (m.actresses || []).includes(state.allMoviesFilterActress));
  }

  // 5. Search Query
  const searchQ = (state.allMoviesSearch || state.actressSearchQuery || '').toLowerCase().trim();
  if (searchQ) {
    filtered = filtered.filter(m => {
      const idMatch = (m.movie_id || '').toLowerCase().includes(searchQ);
      const titleMatch = (m.title || '').toLowerCase().includes(searchQ);
      const makerMatch = (m.maker || '').toLowerCase().includes(searchQ);
      const actMatch = (m.actresses || []).some(a => a.toLowerCase().includes(searchQ));
      return idMatch || titleMatch || makerMatch || actMatch;
    });
  }

  // 6. Sort
  const sortMode = state.allMoviesSort || 'date-desc';
  filtered.sort((a, b) => {
    if (sortMode === 'date-desc') {
      return (b.release_date || '').localeCompare(a.release_date || '');
    } else if (sortMode === 'date-asc') {
      return (a.release_date || '').localeCompare(b.release_date || '');
    } else if (sortMode === 'rating-desc') {
      const rA = (state.userStates?.[a.movie_id]?.user_rating || a.user_rating || 0);
      const rB = (state.userStates?.[b.movie_id]?.user_rating || b.user_rating || 0);
      if (rB !== rA) return rB - rA;
      return (b.release_date || '').localeCompare(a.release_date || '');
    } else if (sortMode === 'maker-asc') {
      const mA = (a.maker || '').toLowerCase();
      const mB = (b.maker || '').toLowerCase();
      if (mA !== mB) return mA.localeCompare(mB);
      return (b.release_date || '').localeCompare(a.release_date || '');
    } else if (sortMode === 'id-asc') {
      return (a.movie_id || '').localeCompare(b.movie_id || '');
    } else if (sortMode === 'title-asc') {
      return (a.title || '').localeCompare(b.title || '');
    }
    return 0;
  });

  // Prepare dropdown lists
  const genresList = Array.from(genreCountMap.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));

  const studiosList = Array.from(studioCountMap.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));

  const actressesList = Array.from(actressCountMap.entries())
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));

  const isFiltered = state.allMoviesFilterStatus !== 'all' ||
    !!state.allMoviesFilterGenre ||
    !!state.allMoviesFilterStudio ||
    !!state.allMoviesFilterActress ||
    !!searchQ;

  const pageLimit = state.allMoviesPageLimit || 72;
  const sliced = filtered.slice(0, pageLimit);
  const cardsHtml = sliced.map(createPosterCardHtml);

  return `
    <section class="all-movies-catalog-section">
      <div class="all-movies-toolbar">
        <div class="all-movies-toolbar-top">
          <!-- Status Filter Pills -->
          <div class="all-movies-status-pills" role="tablist" aria-label="Filter by availability">
            <button class="status-pill-btn ${state.allMoviesFilterStatus === 'all' ? 'active' : ''}" onclick="window.app.setAllMoviesStatus('all')">
              All Works <span class="pill-count">${allMovies.length}</span>
            </button>
            <button class="status-pill-btn ${state.allMoviesFilterStatus === 'library' ? 'active' : ''}" onclick="window.app.setAllMoviesStatus('library')">
              <span class="material-symbols-outlined icon">check_circle</span> In Library <span class="pill-count">${libraryCount}</span>
            </button>
            <button class="status-pill-btn ${state.allMoviesFilterStatus === 'missing' ? 'active' : ''}" onclick="window.app.setAllMoviesStatus('missing')">
              <span class="material-symbols-outlined icon">schedule</span> Missing <span class="pill-count">${missingCount}</span>
            </button>
            <button class="status-pill-btn ${state.allMoviesFilterStatus === 'watched' ? 'active' : ''}" onclick="window.app.setAllMoviesStatus('watched')">
              <span class="material-symbols-outlined icon">visibility</span> Watched <span class="pill-count">${watchedCount}</span>
            </button>
            <button class="status-pill-btn ${state.allMoviesFilterStatus === 'favorite' ? 'active' : ''}" onclick="window.app.setAllMoviesStatus('favorite')">
              <span class="material-symbols-outlined icon">favorite</span> Favorites <span class="pill-count">${favCount}</span>
            </button>
          </div>

          <!-- Live Results Counter & Reset -->
          <div class="all-movies-stats-box">
            <span class="catalog-showing-counter">Showing <strong>${Math.min(filtered.length, pageLimit)}</strong> of <strong>${filtered.length}</strong> titles</span>
            ${isFiltered ? `
              <button class="btn btn-secondary btn-sm" onclick="window.app.resetAllMoviesFilters()" title="Reset all catalog filters">
                <span class="material-symbols-outlined icon">filter_alt_off</span> Reset
              </button>
            ` : ''}
          </div>
        </div>

        <!-- Filter Controls Bar -->
        <div class="all-movies-filters-bar">
          <!-- Genre Dropdown -->
          <div class="filter-dropdown-wrap">
            <select id="select-filter-genre" class="select-dropdown" aria-label="Filter by Genre" onchange="window.app.setAllMoviesGenre(this.value)">
              <option value="">All Genres (${genresList.length})</option>
              ${genresList.map(g => `<option value="${escapeHtml(g.name)}" ${state.allMoviesFilterGenre === g.name ? 'selected' : ''}>${escapeHtml(g.name)} (${g.count})</option>`).join('')}
            </select>
          </div>

          <!-- Studio / Maker Dropdown -->
          <div class="filter-dropdown-wrap">
            <select id="select-filter-studio" class="select-dropdown" aria-label="Filter by Studio" onchange="window.app.setAllMoviesStudio(this.value)">
              <option value="">All Studios (${studiosList.length})</option>
              ${studiosList.map(s => `<option value="${escapeHtml(s.name)}" ${state.allMoviesFilterStudio === s.name ? 'selected' : ''}>${escapeHtml(s.name)} (${s.count})</option>`).join('')}
            </select>
          </div>

          <!-- Actress Dropdown -->
          <div class="filter-dropdown-wrap">
            <select id="select-filter-actress" class="select-dropdown" aria-label="Filter by Actress" onchange="window.app.setAllMoviesActress(this.value)">
              <option value="">All Actresses (${actressesList.length})</option>
              ${actressesList.map(a => `<option value="${escapeHtml(a.name)}" ${state.allMoviesFilterActress === a.name ? 'selected' : ''}>${escapeHtml(a.name)} (${a.count})</option>`).join('')}
            </select>
          </div>

          <!-- Sort Dropdown -->
          <div class="filter-dropdown-wrap">
            <select id="select-filter-sort" class="select-dropdown" aria-label="Sort catalog by" onchange="window.app.setAllMoviesSort(this.value)">
              <option value="date-desc" ${state.allMoviesSort === 'date-desc' ? 'selected' : ''}>Release Date (Newest First)</option>
              <option value="date-asc" ${state.allMoviesSort === 'date-asc' ? 'selected' : ''}>Release Date (Oldest First)</option>
              <option value="rating-desc" ${state.allMoviesSort === 'rating-desc' ? 'selected' : ''}>User Rating (Highest First)</option>
              <option value="maker-asc" ${state.allMoviesSort === 'maker-asc' ? 'selected' : ''}>Studio / Maker (A → Z)</option>
              <option value="id-asc" ${state.allMoviesSort === 'id-asc' ? 'selected' : ''}>JAV ID (A → Z)</option>
              <option value="title-asc" ${state.allMoviesSort === 'title-asc' ? 'selected' : ''}>Title (A → Z)</option>
            </select>
          </div>

          <!-- View Density Toggle -->
          <div class="view-density-group" role="group" aria-label="Grid Density">
            <button class="btn-icon-density ${state.allMoviesViewDensity === 'grid' ? 'active' : ''}" title="Poster Grid View" aria-label="Poster Grid View" onclick="window.app.setAllMoviesDensity('grid')">
              <span class="material-symbols-outlined icon">grid_view</span>
            </button>
            <button class="btn-icon-density ${state.allMoviesViewDensity === 'compact' ? 'active' : ''}" title="Compact Grid View" aria-label="Compact Grid View" onclick="window.app.setAllMoviesDensity('compact')">
              <span class="material-symbols-outlined icon">view_module</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Movies Grid -->
      <div class="all-movies-grid ${state.allMoviesViewDensity === 'compact' ? 'density-compact' : ''}">
        ${filtered.length === 0 ? `
          <div class="shelf-empty-state" style="grid-column: 1 / -1; padding: 4rem 2rem; text-align: center;">
            <span class="material-symbols-outlined" style="font-size: 3rem; color: var(--text-muted); margin-bottom: 0.75rem;">search_off</span>
            <h3 style="color: #fff; font-size: 1.25rem; margin-bottom: 0.5rem;">No Movies Match Criteria</h3>
            <p style="color: var(--text-muted); font-size: 0.9rem; margin-bottom: 1.25rem;">Try adjusting your search keywords, status filter, genre, or studio selection.</p>
            <button class="btn btn-primary" onclick="window.app.resetAllMoviesFilters()">Reset All Filters</button>
          </div>
        ` : cardsHtml.join('')}
      </div>

      <!-- Load More / Pagination -->
      ${filtered.length > pageLimit ? `
        <div class="all-movies-load-more-box">
          <button class="btn btn-secondary btn-lg" onclick="window.app.loadMoreAllMovies()">
            <span class="material-symbols-outlined icon">expand_more</span> Load More (${Math.min(72, filtered.length - pageLimit)} remaining)
          </button>
          <button class="btn btn-secondary" onclick="window.app.showAllAllMovies()">
            Show All (${filtered.length})
          </button>
        </div>
      ` : ''}
    </section>
  `;
}

export function renderCatalogView() {
  const container = document.getElementById('catalog-collection-view');
  if (!container) return;
  container.innerHTML = renderAllMoviesCatalogHtml();
}

export function setAllMoviesStatus(status) {
  state.allMoviesFilterStatus = status;
  state.allMoviesPageLimit = 72;
  renderCatalogView();
}

export function setAllMoviesGenre(genre) {
  state.allMoviesFilterGenre = genre;
  state.allMoviesPageLimit = 72;
  renderCatalogView();
}

export function setAllMoviesStudio(studio) {
  state.allMoviesFilterStudio = studio;
  state.allMoviesPageLimit = 72;
  renderCatalogView();
}

export function setAllMoviesActress(actress) {
  state.allMoviesFilterActress = actress;
  state.allMoviesPageLimit = 72;
  renderCatalogView();
}

export function setAllMoviesSort(sort) {
  state.allMoviesSort = sort;
  renderCatalogView();
}

export function setAllMoviesDensity(density) {
  state.allMoviesViewDensity = density;
  renderCatalogView();
}

export function loadMoreAllMovies() {
  state.allMoviesPageLimit = (state.allMoviesPageLimit || 72) + 72;
  renderCatalogView();
}

export function showAllAllMovies() {
  state.allMoviesPageLimit = 99999;
  renderCatalogView();
}

export function resetAllMoviesFilters() {
  state.allMoviesFilterStatus = 'library';
  state.allMoviesFilterGenre = '';
  state.allMoviesFilterStudio = '';
  state.allMoviesFilterActress = '';
  state.allMoviesSort = 'date-desc';
  state.allMoviesSearch = '';
  state.actressSearchQuery = '';
  state.allMoviesPageLimit = 72;
  if (elements.universalSearchInput) elements.universalSearchInput.value = '';
  elements.universalSearchClear?.classList.add('hidden');
  renderCatalogView();
}

