/**
 * R19DEV Studio - Actress Hub (Collection, Bento Profile & Chat Feed)
 * Native ES Module
 */

import { state, elements, escapeHtml, formatBytes, showToast } from './state.js';
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
  refreshAllActresses
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
          <div class="drawer-stat-val" style="color: var(--success);">🟢 ${dl}</div>
          <div class="drawer-stat-label">In Library</div>
        </div>
        <div class="drawer-stat-card">
          <div class="drawer-stat-val" style="color: var(--accent-pink);">🔴 ${missing}</div>
          <div class="drawer-stat-label">Missing / New</div>
        </div>
        <div class="drawer-stat-card">
          <div class="drawer-stat-val" style="color: var(--primary);">👁️ ${watched}</div>
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
          <span>🌐</span> View Official Profile on R18.dev ↗
        </a>
        <button class="btn btn-secondary" onclick="window.app.trackTitleToActress('${escapeHtml(a.name)}')">
          <span>+</span> Track New JAV-ID
        </button>
        <button class="btn btn-secondary" onclick="window.app.refreshSingleActress('${escapeHtml(a.name)}', this)">
          <span>🔄</span> Refresh Releases
        </button>
        <button class="btn btn-secondary btn-danger-hover" onclick="window.app.unfollowActress('${escapeHtml(a.name)}')">
          <span>🗑️</span> Unfollow Actress
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
  const isOrganized = Boolean(rel.organized_folder) || Boolean(state.organizedFolders[id]);
  const isStaging = !isOrganized && (rel.is_downloaded || Boolean(state.organizedStatus[id]));

  const coverUrl = rel.cover_url || rel.poster_url || meta?.cover_url || meta?.poster_url || (id ? '/api/images/' + id : '');
  const title = meta?.title || rel.title || id;
  const maker = meta?.maker || rel.maker || '';
  const date = rel.release_date || meta?.release_date || '';
  const folderPath = rel.organized_folder || rel.library_path || state.organizedFolders[id] || '';

  const imgHtml = coverUrl ?
    `<img class="poster-img" src="${coverUrl}" alt="${escapeHtml(id)}" loading="lazy" onerror="this.style.display='none'; this.nextElementSibling.style.display='flex';" />
     <div class="poster-fallback" style="display: none;">
       <div class="poster-fallback-icon">🎬</div>
       <div class="poster-fallback-id">${escapeHtml(id)}</div>
     </div>` :
    `<div class="poster-fallback">
       <div class="poster-fallback-icon">🎬</div>
       <div class="poster-fallback-id">${escapeHtml(id)}</div>
     </div>`;

  const statusClass = isOrganized ? 'in-library' : (isStaging ? 'in-staging' : 'is-missing');

  return `
    <div class="collection-poster-card ${statusClass}" onclick="window.app.openMovieById('${escapeHtml(id)}')">
      ${imgHtml}
      <div class="poster-info-overlay">
        <div class="poster-badges-row">
          <span class="poster-id-badge">${escapeHtml(id)}</span>
          ${rel.skip_reason ? `<span class="poster-badge-skip-reason">${escapeHtml(rel.skip_reason)}</span>` : ''}
        </div>
        <h4 class="poster-title" title="${escapeHtml(title)}">${escapeHtml(title)}</h4>
        <div class="poster-sub">
          <span>${escapeHtml(maker || 'R18.dev')}</span>
          <span>${escapeHtml(date)}</span>
        </div>
      </div>
      <div class="poster-hover-actions">
        <button class="poster-btn-action poster-btn-primary" onclick="event.stopPropagation(); window.app.openMovieById('${escapeHtml(id)}')">
          <span>▶</span> Details
        </button>
        <button class="poster-btn-action poster-btn-secondary" onclick="event.stopPropagation(); window.app.copyMovieId('${escapeHtml(id)}', event)">
          <span>📋</span> Copy ID
        </button>
        ${folderPath ? `
          <button class="poster-btn-action poster-btn-secondary" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(folderPath)}" onclick="window.app.openFolderEl(this, event)">
            <span>📂</span> Finder
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
          <div style="font-size: 2.8rem; margin-bottom: 0.6rem;">🎬</div>
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

    // Collect deduplicated releases across all actresses
    const allReleasesMap = new Map();
    state.actresses.forEach(entry => {
      (entry.releases || []).forEach(rel => {
        if (!allReleasesMap.has(rel.movie_id)) {
          allReleasesMap.set(rel.movie_id, rel);
        }
      });
    });
    const allReleases = Array.from(allReleasesMap.values());

    // Shelf 1: Recent releases
    const recentReleases = [...allReleases].sort((a, b) => (b.release_date || '').localeCompare(a.release_date || '')).slice(0, 25);
    const recentCards = recentReleases.map(createPosterCardHtml);

    // Shelf 2: In Library / Ready to watch
    const libraryReleases = allReleases.filter(r => r.organized_folder || r.library_path || r.is_downloaded || state.organizedStatus[r.movie_id]).sort((a, b) => (b.release_date || '').localeCompare(a.release_date || ''));
    const libraryCards = libraryReleases.map(createPosterCardHtml);

    let contentHtml = '';

    if (state.actressHubTab === 'discovered') {
      // Render Discovered Actresses (Unfollowed with files in NAS)
      let discList = [...state.discoveredActresses];
      if (state.actressSearchQuery) {
        const q = state.actressSearchQuery.toLowerCase();
        discList = discList.filter(e => {
          const n = (e.name || '').toLowerCase();
          const j = (e.ja_name || '').toLowerCase();
          return n.includes(q) || j.includes(q);
        });
      }

      contentHtml += `
        <section class="collection-directory-section">
          <div class="directory-section-header">
            <div>
              <h3 class="directory-section-title">Discovered in NAS Library</h3>
              <p class="directory-section-subtitle">Performers found in your video library who are not yet tracked</p>
            </div>
            <span class="directory-section-badge">${discList.length} Discovered</span>
          </div>
          <div class="actresses-directory-grid">
            ${discList.length === 0 ? `
              <div class="shelf-empty-state" style="grid-column: 1 / -1; padding: 3rem 1.5rem; text-align: center;">
                <span class="material-symbols-outlined" style="font-size: 2.5rem; color: var(--text-muted); margin-bottom: 0.5rem;">check_circle</span>
                <div style="font-weight: 600; font-size: 1.1rem; color: #fff;">All Library Actresses Tracked!</div>
                <div style="color: var(--text-muted); font-size: 0.88rem; margin-top: 0.35rem;">Every performer with files in your NAS is currently followed.</div>
              </div>
            ` : discList.map(entry => {
              const avatar = entry.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
              const count = entry.movie_count || 1;
              const latest = entry.latest_release || 'Unknown';

              return `
                <div class="actress-dir-card discovered">
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
          const total = e.total || (e.releases ? e.releases.length : 0);
          const dl = e.downloaded || 0;
          return total > 0 && dl >= total;
        });
      } else if (sortMode === 'in-progress') {
        dirList = dirList.filter(e => {
          const total = e.total || (e.releases ? e.releases.length : 0);
          const dl = e.downloaded || 0;
          return total === 0 || dl < total;
        });
      }

      dirList.sort((a, b) => {
        const totalA = a.total || (a.releases ? a.releases.length : 0);
        const dlA = a.downloaded || 0;
        const pctA = totalA > 0 ? (dlA / totalA) : 0;
        const missingA = a.missing || 0;

        const totalB = b.total || (b.releases ? b.releases.length : 0);
        const dlB = b.downloaded || 0;
        const pctB = totalB > 0 ? (dlB / totalB) : 0;
        const missingB = b.missing || 0;

        if (sortMode === 'pct-desc') {
          return pctB - pctA || totalB - totalA;
        } else if (sortMode === 'missing-desc') {
          return missingB - missingA;
        } else if (sortMode === 'total-desc') {
          return totalB - totalA;
        } else if (sortMode === 'name-asc') {
          return (a.actress.name || '').localeCompare(b.actress.name || '');
        }
        return 0;
      });

      contentHtml += `
        <section class="collection-directory-section">
          <div class="directory-section-header">
            <div>
              <h3 class="directory-section-title">Followed Actresses</h3>
              <p class="directory-section-subtitle">Click an actress to view her dedicated filmography</p>
            </div>
            <span class="directory-section-badge">${dirList.length} Actresses</span>
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
              const total = entry.total || (entry.releases ? entry.releases.length : 0);
              const dl = entry.downloaded || 0;
              const pct = total > 0 ? Math.round((dl / total) * 100) : 0;
              const isCompleted = total > 0 && dl >= total;
              const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';

              const latestDate = entry.latest_date || (entry.releases && entry.releases[0] ? entry.releases[0].release_date : '');
              const latestID = entry.latest_movie_id || (entry.releases && entry.releases[0] ? entry.releases[0].movie_id : '');
              const isLatestDl = entry.latest_is_downloaded !== undefined ? entry.latest_is_downloaded : (entry.releases && entry.releases[0] ? entry.releases[0].is_downloaded : false);

              return `
                <div class="actress-dir-card ${isCompleted ? 'completed' : ''}" onclick="window.app.filterCollectionActress('${escapeHtml(a.name)}')">
                  ${isCompleted ? `
                    <div class="actress-dir-badge-completed">
                      <span class="material-symbols-outlined icon">check_circle</span>
                      <span>100% COMPLETE</span>
                    </div>
                  ` : ''}
                  <div class="actress-dir-avatar-box">
                    <img class="actress-dir-avatar" src="${avatar}" alt="${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
                  </div>
                  <div class="actress-dir-body">
                    <div class="actress-dir-header">
                      <span class="actress-dir-name">${escapeHtml(a.name)}</span>
                    </div>
                    <div class="actress-dir-progress-row">
                      <div class="actress-dir-progress-track">
                        <div class="actress-dir-progress-fill" style="width: ${pct}%;"></div>
                      </div>
                      <div class="actress-dir-stats-line">
                        <span class="actress-dir-fraction">${dl}/${total}</span>
                        <span class="actress-dir-pct">${pct}%</span>
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

    if (recentReleases.length > 0) {
      contentHtml += createShelfHtml('shelf-recent-all', 'Recent Releases Across All Actresses', '<span class="material-symbols-outlined">local_fire_department</span>', recentReleases.length, recentCards);
    }
    if (libraryReleases.length > 0) {
      contentHtml += createShelfHtml('shelf-library-all', 'In Library / Ready to Watch in Jellyfin', '<span class="material-symbols-outlined">check_circle</span>', libraryReleases.length, libraryCards);
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

    const activeEntry = state.actresses.find(e => e.actress.name === state.collectionFilterActress) || state.actresses[0];
    const a = activeEntry.actress;
    const releases = activeEntry.releases || [];
    const total = activeEntry.total || releases.length;
    const dl = activeEntry.downloaded || 0;
    const missing = activeEntry.missing || 0;
    const pct = total > 0 ? Math.round((dl / total) * 100) : 0;
    const avatar = a.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
    const totalBytes = activeEntry.total_size_bytes || 0;
    const avgBytes = dl > 0 && totalBytes > 0 ? Math.round(totalBytes / dl) : 0;
    const topGenres = activeEntry.top_genres || [];
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
      const isOrganized = Boolean(rel.organized_folder) || Boolean(state.organizedFolders[rel.movie_id]);
      const isStaging = !isOrganized && (rel.is_downloaded || Boolean(state.organizedStatus[rel.movie_id]));
      if (isOrganized || isStaging) {
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
          <div class="bento-card bento-card-identity">
            <div class="bento-avatar-wrapper">
              <img class="bento-avatar" src="${avatar}" alt="${escapeHtml(a.name)}" onerror="this.src='/placeholder.png'" />
            </div>
            <div class="bento-identity-info">
              <h2 class="bento-name">${escapeHtml(a.name)}</h2>
              ${a.ja_name ? `<div class="bento-ja-name">${escapeHtml(a.ja_name)}</div>` : ''}
              <div class="bento-badges-row">
                ${careerSpan ? `<span class="bento-career-badge">📅 ${escapeHtml(careerSpan)}</span>` : ''}
                ${a.r18_id ? `<span class="bento-badge-mono">R18 #${a.r18_id}</span>` : ''}
                <span class="bento-badge-count">${total} Works</span>
              </div>
            </div>
          </div>

          <!-- Bento 2: Storage & Library Progress -->
          <div class="bento-card bento-card-storage">
            <div class="bento-card-header">
              <span class="bento-card-title">💾 Library & Storage</span>
              <span class="bento-storage-highlight">${formatBytes(totalBytes)}</span>
            </div>
            <div class="bento-progress-box">
              <div class="bento-progress-track">
                <div class="bento-progress-fill" style="width: ${pct}%;"></div>
              </div>
              <div class="bento-progress-meta">
                <span class="bento-progress-count">${dl}/${total} Collected</span>
                <span class="bento-progress-pct">${pct}%</span>
              </div>
            </div>
            ${dl > 0 && avgBytes > 0 ? `
              <div class="bento-storage-footnote">
                <span>Average size:</span> <strong>${formatBytes(avgBytes)} / file</strong>
              </div>
            ` : ''}
          </div>

          <!-- Bento 3: Top Genres (Clickable Filter Chips) -->
          <div class="bento-card bento-card-genres">
            <div class="bento-card-header">
              <span class="bento-card-title">🏷️ Top Genres</span>
              ${state.actressGenreFilter ? `<button class="bento-chip-reset" onclick="window.app.clearActressGenreFilter()">Clear</button>` : ''}
            </div>
            ${topGenres.length > 0 ? `
              <div class="bento-genres-cloud">
                ${topGenres.map(g => {
                  const isActive = state.actressGenreFilter === g.genre;
                  return `
                    <button class="bento-genre-chip ${isActive ? 'active' : ''}" 
                            onclick="window.app.toggleActressGenreFilter('${escapeHtml(g.genre)}')"
                            title="${isActive ? 'Click to remove filter' : `Filter by ${escapeHtml(g.genre)}`}">
                      <span class="genre-name">#${escapeHtml(g.genre)}</span>
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
              <span class="bento-card-title">⚡ Quick Actions</span>
            </div>
            <div class="bento-action-buttons">
              <button class="btn btn-secondary bento-action-btn" title="Open Actress folder in Finder / NAS" onclick="window.app.openActiveActressFolder()">
                <span class="icon">📂</span> Open in Finder
              </button>
              <a href="${r18Url}" target="_blank" rel="noopener noreferrer" class="btn btn-secondary bento-action-btn" title="Official R18.dev Filmography">
                <span class="icon">🌐</span> R18.dev Profile ↗
              </a>
              <button class="btn btn-secondary bento-action-btn" title="Check & refresh releases from R18.dev" onclick="window.app.refreshActiveActress(this)">
                <span class="icon">🔄</span> Refresh Releases
              </button>
            </div>
          </div>
        </aside>

        <!-- RIGHT COLUMN: Dedicated Main Stage -->
        <main class="actress-main-stage">
          <!-- Stage Toolbar -->
          <div class="actress-stage-toolbar">
            <div class="stage-toolbar-left">
              <!-- In-Page Live Search -->
              <div class="stage-search-box">
                <span class="stage-search-icon">🔍</span>
                <input type="text" 
                       id="actress-movie-search" 
                       class="stage-search-input" 
                       placeholder="Search ID (e.g. SSIS) or title..." 
                       value="${escapeHtml(state.actressMovieSearch || '')}" 
                       oninput="window.app.handleActressMovieSearch(this.value)" />
                ${state.actressMovieSearch ? `
                  <button class="stage-search-clear" onclick="window.app.handleActressMovieSearch('')">✕</button>
                ` : ''}
              </div>

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
                  <span>Genre: <strong>#${escapeHtml(state.actressGenreFilter)}</strong></span>
                  <button class="stage-tag-close" onclick="window.app.clearActressGenreFilter()" title="Clear genre filter">✕</button>
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
                <div class="empty-icon">🎬</div>
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
        <div class="welcome-icon">💬</div>
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
  elements.chatHeaderSubtitle.textContent = `${entry.total} titles tracked • 🟢 ${entry.downloaded} Downloaded • 🔴 ${entry.missing} Missing`;
  elements.chatHeaderControls.classList.remove('hidden');
  elements.chatActionBar.classList.remove('hidden');

  const r18Url = a.r18_id
    ? `https://r18.dev/videos/vod/movies/list/?id=${a.r18_id}&type=actress`
    : `https://r18.dev/videos/vod/movies/list/?search=${encodeURIComponent(a.name)}`;
  elements.linkHeaderR18.href = r18Url;

  if (releases.length === 0) {
    elements.chatMessagesFeed.innerHTML = `
      <div class="chat-welcome-placeholder">
        <div class="welcome-icon">🎬</div>
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
          <span class="chat-date-badge">🗓️ Release Month: ${escapeHtml(dateGroup)}</span>
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
            📢 สวัสดีค่ะ! ผลงานเรื่อง <strong>${escapeHtml(rel.movie_id)}</strong> วางจำหน่ายเมื่อวันที่ ${escapeHtml(rel.release_date || '-')} ค่ะ ฝากติดตามด้วยนะคะ! ✨
          </div>
          <div class="chat-embedded-card">
            <div class="chat-card-cover-box ${isMissing ? 'missing' : ''}" onclick="window.app.openMovie('${escapeHtml(rel.movie_id)}')">
              <img src="${rel.cover_url || (rel.movie_id ? '/api/images/' + rel.movie_id : '/placeholder.png')}" alt="Cover ${escapeHtml(rel.movie_id)}" onerror="this.onerror=null; this.src='/placeholder.png';" />
            </div>
            <div class="chat-card-body">
              <div class="chat-card-id-row">
                <span class="chat-card-id">${escapeHtml(rel.movie_id)}</span>
                <button class="btn-copy-id" onclick="window.app.copyMovieId('${escapeHtml(rel.movie_id)}', event)" title="Copy ${escapeHtml(rel.movie_id)}">
                  📋 Copy ID
                </button>
              </div>
              <div class="chat-card-title">${escapeHtml(rel.title)}</div>
              <div class="chat-card-meta">
                <span>${escapeHtml(rel.maker || 'Studio')}</span>
                <button class="chat-btn-inspect" onclick="window.app.openMovie('${escapeHtml(rel.movie_id)}')">
                  🔍 ดูรายละเอียด
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
            <span>✅ จัดเก็บเข้า Jellyfin เรียบร้อยแล้วครับ!</span>
            <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.organized_folder || '')}" onclick="window.app.openFolderEl(this, event)" style="padding: 0.2rem 0.55rem; font-size: 0.72rem; border-radius: 6px; cursor: pointer; white-space: nowrap;">
              📂 Open in Finder
            </button>
          </div>
          <div class="user-status-path" style="cursor: pointer;" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.organized_folder || '')}" onclick="window.app.openFolderEl(this, event)" title="Click to open in Finder">📁 ${escapeHtml(rel.organized_folder)}</div>
        </div>
      `;
    } else if (isStaging) {
      userStatusHtml = `
        <div class="user-status-box">
          <div class="user-status-title" style="color: var(--warning); display: flex; align-items: center; justify-content: space-between; gap: 0.5rem;">
            <span>📥 ดาวน์โหลดไฟล์แล้ว (รอ Organize เข้า Jellyfin)</span>
            <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.library_path || '')}" onclick="window.app.openFolderEl(this, event)" style="padding: 0.2rem 0.55rem; font-size: 0.72rem; border-radius: 6px; cursor: pointer; white-space: nowrap;">
              📂 Open in Finder
            </button>
          </div>
          <div class="user-status-path" style="cursor: pointer;" data-movie-id="${escapeHtml(rel.movie_id)}" data-path="${escapeHtml(rel.library_path || '')}" onclick="window.app.openFolderEl(this, event)" title="Click to open in Finder">📁 ${escapeHtml(rel.library_path)}</div>
        </div>
      `;
    } else {
      userStatusHtml = `
        <div class="user-status-box">
          <div class="user-status-title" style="color: var(--accent-pink);">
            <span>⏳ ยังไม่มีไฟล์ในคลังครับ</span>
          </div>
          <div style="font-size: 0.78rem; color: var(--text-muted);">
            ยังไม่ได้ดาวน์โหลดลงเครื่อง สามารถกด Copy ID ด้านบนไปค้นหาได้ครับ
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
