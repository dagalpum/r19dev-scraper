/**
 * R19DEV Studio - Movie Details Modal & Lightbox
 * Native ES Module
 */

import { state, elements, escapeHtml, getHighResScreenshotUrl, getMovieLocationInfo, formatBytes } from './state.js';
import { scrapeMovie, fetchMovie, searchTorrents, downloadTorrent } from './api.js';
import { closeHistoryModal } from './history.js';
import { closeActressProfileDrawer } from './actress.js';
import { closeNetworkGraph } from './graph.js';

export function setupModals() {
  elements.btnCloseModal?.addEventListener('click', closeModal);
  elements.modalMovie?.addEventListener('click', (e) => {
    if (e.target === elements.modalMovie) closeModal();
  });

  document.getElementById('btn-close-video-player')?.addEventListener('click', closeVideoPlayer);
  document.getElementById('modal-video-player')?.addEventListener('click', (e) => {
    if (e.target.id === 'modal-video-player') closeVideoPlayer();
  });

  elements.btnCloseLightbox?.addEventListener('click', closeLightbox);
  elements.lightbox?.addEventListener('click', (e) => {
    if (e.target === elements.lightbox) closeLightbox();
  });

  elements.btnModalPrev?.addEventListener('click', () => navigateModalMovie(-1));
  elements.btnModalNext?.addEventListener('click', () => navigateModalMovie(1));

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (!document.getElementById('modal-video-player')?.classList.contains('hidden')) {
        closeVideoPlayer();
      } else if (!elements.lightbox?.classList.contains('hidden')) {
        closeLightbox();
      } else if (!document.getElementById('modal-network-graph')?.classList.contains('hidden')) {
        closeNetworkGraph();
      } else if (!elements.drawerActressProfile?.classList.contains('hidden')) {
        closeActressProfileDrawer();
      } else if (!elements.modalMovie?.classList.contains('hidden')) {
        closeModal();
      } else if (!elements.modalHistory?.classList.contains('hidden')) {
        closeHistoryModal();
      }
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
      if (elements.modalMovie && !elements.modalMovie.classList.contains('hidden')) {
        const activeTag = document.activeElement ? document.activeElement.tagName.toLowerCase() : '';
        if (activeTag === 'input' || activeTag === 'textarea' || activeTag === 'select') {
          return;
        }
        if (document.querySelector('.pswp--open')) {
          return;
        }
        if (e.key === 'ArrowLeft') {
          e.preventDefault();
          navigateModalMovie(-1);
        } else if (e.key === 'ArrowRight') {
          e.preventDefault();
          navigateModalMovie(1);
        }
      }
    }
  });
}

export function getMovieNavigationContext() {
  let list = state.currentMovieNavigationList || [];
  const curId = (state.selectedMovieId || '').trim();

  // If list is empty or current movie is not in list, find suitable context
  if (!list.length || (curId && !list.some(id => id && id.toUpperCase() === curId.toUpperCase()))) {
    if (state.activeTab === 'library' && state.groupedMovies && state.groupedMovies.length > 0) {
      list = state.groupedMovies.map(m => m.id).filter(Boolean);
    } else if (state.activeTab === 'actresses' && state.collectionFilterActress && state.collectionFilterActress !== 'all') {
      const a = state.actresses.find(e => e.actress.name.toLowerCase() === state.collectionFilterActress.toLowerCase());
      if (a && a.releases) {
        list = a.releases.map(r => r.movie_id).filter(Boolean);
      }
    } else if (state.groupedMovies && state.groupedMovies.some(m => m.id && m.id.toUpperCase() === curId.toUpperCase())) {
      list = state.groupedMovies.map(m => m.id).filter(Boolean);
    }
  }

  if (!list.length || !curId) {
    return { hasPrev: false, hasNext: false, currentIndex: -1, totalCount: 0, prevId: null, nextId: null };
  }

  const index = list.findIndex(id => id && id.toUpperCase() === curId.toUpperCase());
  if (index === -1) {
    return { hasPrev: false, hasNext: false, currentIndex: -1, totalCount: list.length, prevId: null, nextId: null };
  }

  return {
    hasPrev: index > 0,
    hasNext: index < list.length - 1,
    currentIndex: index,
    totalCount: list.length,
    prevId: index > 0 ? list[index - 1] : null,
    nextId: index < list.length - 1 ? list[index + 1] : null,
  };
}

export function updateModalNavButtons() {
  const nav = getMovieNavigationContext();
  const prevBtn = elements.btnModalPrev || document.getElementById('btn-modal-prev');
  const nextBtn = elements.btnModalNext || document.getElementById('btn-modal-next');

  if (prevBtn) {
    prevBtn.disabled = !nav.hasPrev;
    prevBtn.title = nav.hasPrev ? `Previous: ${nav.prevId} (←)` : 'No previous movie';
    prevBtn.classList.toggle('hidden', nav.totalCount <= 1);
  }
  if (nextBtn) {
    nextBtn.disabled = !nav.hasNext;
    nextBtn.title = nav.hasNext ? `Next: ${nav.nextId} (→)` : 'No next movie';
    nextBtn.classList.toggle('hidden', nav.totalCount <= 1);
  }
}

export function navigateModalMovie(delta) {
  if (!elements.modalMovie || elements.modalMovie.classList.contains('hidden')) return;
  const nav = getMovieNavigationContext();
  if (delta < 0 && nav.hasPrev && nav.prevId) {
    openMovieById(nav.prevId);
  } else if (delta > 0 && nav.hasNext && nav.nextId) {
    openMovieById(nav.nextId);
  }
}

export async function openMovieDetail(movie) {
  state.selectedMovieId = movie.id;
  const targetId = movie.id;
  elements.modalMovie.classList.remove('hidden');
  updateModalNavButtons();

  let meta = state.metadata[movie.id];
  if (!meta && movie.id) {
    // 1. Check local SQLite first (<2ms) - completely silent, no toast, no network delay!
    meta = await fetchMovie(movie.id);
  }

  if (state.selectedMovieId !== targetId) return;

  if (!meta && movie.id) {
    // 2. Only if truly not in local DB, fetch from scraper silently with in-modal loading
    elements.modalContent.innerHTML = `
      <div style="text-align: center; padding: 4rem;">
        <span class="material-symbols-outlined spin" style="font-size: 2.5rem; color: var(--primary);">progress_activity</span>
        <div style="margin-top: 1rem; color: var(--text-muted); font-size: 0.95rem;">Fetching metadata from R18.dev...</div>
      </div>
    `;
    meta = await scrapeMovie(movie.id, { silent: true });
  }

  if (state.selectedMovieId !== targetId) return;

  // Check for local NAS screenshots in extrafanart/
  let localScreenshots = null;
  if (movie.id) {
    try {
      const gRes = await fetch(`/api/movie-gallery/${encodeURIComponent(movie.id)}`);
      if (gRes.ok) {
        const gData = await gRes.json();
        if (gData.local && Array.isArray(gData.screenshots) && gData.screenshots.length > 0) {
          localScreenshots = gData.screenshots;
        }
      }
    } catch (e) {
      console.warn('Failed to fetch local gallery:', e);
    }
  }

  if (state.selectedMovieId !== targetId) return;

  renderModalContent(movie, meta, localScreenshots);
}

export function closeModal() {
  elements.modalMovie.classList.add('hidden');
  state.selectedMovieId = null;
}

export function openMovieById(id) {
  if (!id) return;
  const existing = state.groupedMovies.find(m => m.id && m.id.toUpperCase() === id.toUpperCase());
  const movieObj = existing || { id: id, files: [] };
  openMovieDetail(movieObj);
}

export function renderModalContent(movie, meta, localScreenshots = null) {
  const id = movie.id || 'UNMATCHED';
  const uState = state.userStates[id] || {};
  const isOrganized = Boolean(state.organizedStatus[id]);
  const loc = getMovieLocationInfo(id);
  let rawCover = meta?.cover_url || meta?.poster_url || '';
  if (rawCover.startsWith('digital/') || rawCover.startsWith('mono/')) {
    rawCover = 'https://pics.dmm.co.jp/' + rawCover + (rawCover.endsWith('.jpg') ? '' : '.jpg');
  }

  let coverUrl = '';
  if (loc.type === 'library' || loc.type === 'external' || loc.type === 'staging' || isOrganized) {
    coverUrl = (id && id !== 'UNMATCHED') ? '/api/images/' + encodeURIComponent(id) : rawCover;
  } else {
    coverUrl = rawCover || ((id && id !== 'UNMATCHED') ? '/api/images/' + encodeURIComponent(id) : '');
  }

  const fallbackCdn = rawCover && rawCover.startsWith('http') ? rawCover : '';
  const combinedId = meta?.combined_id || '';
  const r18Url = (meta?.detail_url && meta.detail_url.startsWith('http'))
    ? meta.detail_url
    : (combinedId
      ? `https://r18.dev/videos/vod/movies/detail/-/id=${encodeURIComponent(combinedId)}/`
      : `https://r18.dev/videos/vod/movies/list/?search=${encodeURIComponent(id)}`);

  // Multi-part files list
  let multipartHtml = '';
  if (movie.files && movie.files.length > 0) {
    multipartHtml = `
      <div class="multipart-box">
        <h4><span class="material-symbols-outlined icon">inventory_2</span> Video Files & Parts (${movie.files.length})</h4>
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
      const thumb = `/api/actresses/avatar/${encodeURIComponent(act.name)}`;
      return `
        <div class="actress-card" style="background: var(--bg-card); padding: 1rem; border-radius: var(--radius-md); text-align: center;">
          <img src="${thumb}" alt="${escapeHtml(act.name)}" style="width: 74px; height: 74px; border-radius: 50%; object-fit: cover; margin: 0 auto 0.6rem auto; border: 2px solid var(--primary);" />
          <div style="font-weight: 700; font-size: 0.95rem; color: #fff;">${escapeHtml(act.name)}</div>
          <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 0.6rem;">${escapeHtml(act.ja_name || '')}</div>
          <button class="btn btn-secondary btn-sm" onclick="window.app.toggleFollowActress('${escapeHtml(act.name)}')">
            ${isFollowed ? '<span class="material-symbols-outlined icon">star</span> Following' : '<span class="material-symbols-outlined icon">person_add</span> Follow'}
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
      fallbackSrc: fallbackCdn || coverUrl,
      w: 800,
      h: 538,
      width: 800,
      height: 538,
      alt: `Jacket Cover - ${id}`
    });
  }

  // Sample Screenshots: Use Local NAS extrafanart if available, otherwise remote URLs
  let screenshotsGrid = '';
  const screenshots = (localScreenshots && localScreenshots.length > 0)
    ? localScreenshots
    : (meta?.sample_screenshots || []);

  if (screenshots.length > 0) {
    screenshotsGrid = screenshots.map((url, idx) => {
      const isLocal = url.startsWith('/api/');
      const highResUrl = isLocal ? url : getHighResScreenshotUrl(url);
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
          <img src="${url}" alt="Screenshot #${idx + 1}" loading="lazy" ${isLocal ? '' : `onerror="this.src='/api/proxy-image?url=${encodeURIComponent(url)}'"`} />
        </div>
      `;
    }).join('');
  }

  state.currentGallery = galleryItems;

  const modalFallbackAttr = fallbackCdn
    ? `onerror="if (this.src.indexOf('/api/images/') !== -1) { this.src='${escapeHtml(fallbackCdn)}'; } else { this.src='/placeholder.png'; }"`
    : `onerror="this.src='/placeholder.png';"`;

  updateModalNavButtons();
  const nav = getMovieNavigationContext();
  const navCounterHtml = (nav.totalCount > 1 && nav.currentIndex >= 0) ? `
    <span class="modal-nav-counter-pill" title="Movie ${nav.currentIndex + 1} of ${nav.totalCount} in current list">
      <button class="nav-pill-btn" onclick="window.app.navigateModalMovie(-1)" ${nav.hasPrev ? '' : 'disabled'} title="Previous: ${nav.prevId || ''} (←)" aria-label="Previous movie">
        <span class="material-symbols-outlined icon">chevron_left</span>
      </button>
      <span>${nav.currentIndex + 1} / ${nav.totalCount}</span>
      <button class="nav-pill-btn" onclick="window.app.navigateModalMovie(1)" ${nav.hasNext ? '' : 'disabled'} title="Next: ${nav.nextId || ''} (→)" aria-label="Next movie">
        <span class="material-symbols-outlined icon">chevron_right</span>
      </button>
    </span>
  ` : '';

  // FULL-WIDTH HERO COVER LAYOUT
  elements.modalContent.innerHTML = `
    <!-- Full-Width Hero Cover Banner -->
    <div class="modal-hero-banner" role="region" aria-label="Full-Width Cover Image">
      <div class="modal-hero-backdrop" style="background-image: url('${coverUrl}');"></div>
      <img class="modal-hero-cover-img" src="${coverUrl}" alt="Full High-Res Cover for ${escapeHtml(id)}"
           onclick="window.app.openGallery(0)"
           ${modalFallbackAttr} />
      <div class="modal-hero-overlay">
        <div style="display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap;">
          <span class="badge-status" style="background: rgba(12,14,20,0.85); font-family: var(--font-mono); font-size: 0.95rem; font-weight: 800; color: #fff;">
            ${escapeHtml(id)}
          </span>
          ${navCounterHtml}
        </div>
        <button class="btn-zoom-cover" onclick="window.app.openGallery(0)">
          <i data-lucide="maximize-2"></i> View Fullscreen Gallery (${galleryItems.length})
        </button>
      </div>
    </div>

    <div class="modal-body">
      <div class="modal-info">
        <h2 id="modal-movie-title">${escapeHtml(meta?.title || movie.files[0]?.name || 'No title')}</h2>
        <div style="display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.6rem;">
          <span class="card-id" style="font-size: 0.95rem; font-weight: 700;">${escapeHtml(id)}</span>
          <button class="btn btn-secondary btn-sm" style="padding: 0.2rem 0.6rem; font-size: 0.75rem; border-radius: 6px;" onclick="window.app.copyMovieId('${escapeHtml(id)}', event)" title="Copy ${escapeHtml(id)}"><span class="material-symbols-outlined icon">content_copy</span> Copy ID</button>
          <a href="${r18Url}" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm" style="padding: 0.2rem 0.6rem; font-size: 0.75rem; border-radius: 6px; text-decoration: none; display: inline-flex; align-items: center; gap: 4px;" title="View on R18.dev">
            <span class="material-symbols-outlined icon" style="font-size: 0.85rem;">public</span> R18.dev ↗
          </a>
        </div>
        <div class="ja-title">${escapeHtml(meta?.original_title || '')}</div>

        <!-- Location Status Indicator Card -->
        <div class="modal-location-card ${loc.type === 'library' ? 'in-library' : (loc.type === 'external' ? 'outside-library' : (loc.type === 'staging' ? 'in-staging' : 'missing'))}" role="region" aria-label="Media Storage Location">
          <div class="location-main">
            <div class="location-header-row">
              <span class="location-chip ${loc.badgeClass}">
                <span class="material-symbols-outlined icon">${loc.icon}</span>
                <span>${escapeHtml(loc.label)}</span>
              </span>
              <span class="location-title-text">${escapeHtml(loc.titleText)}</span>
            </div>
            ${loc.folderPath ? `
              <div class="location-path-row">
                <span class="material-symbols-outlined path-icon">folder</span>
                <code class="location-path-code" title="${escapeHtml(loc.folderPath)}">${escapeHtml(loc.folderPath)}</code>
              </div>
            ` : `
              <div class="location-sub-text">${escapeHtml(loc.subText)}</div>
            `}
          </div>

          <div class="location-actions">
            ${loc.type === 'library' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.playMovie('${escapeHtml(id)}')" title="Launch in system default media player (VLC, IINA, QuickTime)">
                <span class="material-symbols-outlined icon">play_arrow</span> Play Video
              </button>
              <button class="btn btn-secondary btn-sm" onclick="window.app.watchInBrowser('${escapeHtml(id)}')" title="Stream video directly in browser modal">
                <span class="material-symbols-outlined icon">smart_display</span> In Browser
              </button>
              <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(loc.folderPath)}" onclick="window.app.openFolderEl(this, event)" title="Open movie directory in Finder">
                <span class="material-symbols-outlined icon">folder_open</span> Finder
              </button>
              <button class="btn btn-outline-secondary btn-sm" onclick="window.app.organizeSingle('${id}')" title="Re-run Jellyfin organize">
                <span class="material-symbols-outlined icon">sync</span>
              </button>
            ` : (loc.type === 'external' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.organizeSingle('${id}')" title="Move and organize into Library destination">
                <span class="material-symbols-outlined icon">drive_file_move</span> Move to Library
              </button>
              <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(loc.folderPath)}" onclick="window.app.openFolderEl(this, event)">
                <span class="material-symbols-outlined icon">folder_open</span> Finder
              </button>
            ` : (loc.type === 'downloading' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.openDownloadsDrawer()" title="View active download">
                <span class="material-symbols-outlined icon spin-pulse">downloading</span> In Queue (${Math.round(loc.progress || 0)}%)
              </button>
              <button class="btn btn-secondary btn-sm" onclick="window.app.toggleModalTorrentSection('${escapeHtml(id)}')">
                <span class="material-symbols-outlined icon">manage_search</span> Torrents
              </button>
            ` : (loc.type === 'staging' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.openDownloadsDrawer()" title="Downloaded, waiting to organize">
                <span class="material-symbols-outlined icon">auto_fix_high</span> Staging Ready
              </button>
              <button class="btn btn-secondary btn-sm" onclick="window.app.toggleModalTorrentSection('${escapeHtml(id)}')">
                <span class="material-symbols-outlined icon">manage_search</span> Torrents
              </button>
            ` : (loc.type === 'staging' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.organizeSingle('${id}')">
                <span class="material-symbols-outlined icon">folder_zip</span> Organize for Jellyfin
              </button>
            ` : `
              <button class="btn btn-primary btn-sm" onclick="window.app.toggleModalTorrentSection('${escapeHtml(id)}')">
                <span class="material-symbols-outlined icon">download</span> Find Torrents
              </button>
              <a href="${r18Url}" target="_blank" rel="noopener noreferrer" class="btn btn-secondary btn-sm" style="text-decoration: none; display: inline-flex; align-items: center; gap: 6px;" title="View movie details and screenshots on R18.dev">
                <span class="material-symbols-outlined icon">public</span> R18.dev ↗
              </a>
              <button class="btn btn-secondary btn-sm" onclick="window.app.copyMovieId('${escapeHtml(id)}', event)" title="Copy ${escapeHtml(id)}">
                <span class="material-symbols-outlined icon">content_copy</span> Copy ID
              </button>
            `))))}
          </div>
        </div>

        <!-- Interactive Status Controls -->
        <div class="modal-controls">
          <div style="display: flex; align-items: center; gap: 0.6rem;">
            <span style="font-weight: 700; font-size: 0.85rem;">Rating:</span>
            <div class="star-rating" role="radiogroup" aria-label="Rate 1 to 5 stars">
              ${[1, 2, 3, 4, 5].map(num => `
                <button class="star-btn ${uState.user_rating >= num ? 'active' : ''}"
                        role="radio" aria-checked="${uState.user_rating >= num ? 'true' : 'false'}"
                        aria-label="Rate ${num} stars"
                        onclick="window.app.setRating('${id}', ${num})"><span class="material-symbols-outlined icon">star</span></button>
              `).join('')}
            </div>
          </div>

          <button class="btn btn-secondary ${uState.is_watched ? 'btn-primary' : ''}" onclick="window.app.toggleWatched('${id}')">
            ${uState.is_watched ? '<span class="material-symbols-outlined icon">visibility</span> Watched' : '<span class="material-symbols-outlined icon">visibility_off</span> Mark Watched'}
          </button>

          <button class="btn btn-secondary ${uState.is_favorite ? 'btn-accent' : ''}" onclick="window.app.toggleFavorite('${id}')">
            ${uState.is_favorite ? '<span class="material-symbols-outlined icon">favorite</span> Favorited' : '<span class="material-symbols-outlined icon">favorite_border</span> Favorite'}
          </button>

          <button class="btn btn-secondary" onclick="window.app.toggleModalTorrentSection('${escapeHtml(id)}')">
            <span class="material-symbols-outlined icon">cloud_download</span> Torrents
          </button>
        </div>

        <!-- Torrent Discovery Section -->
        <div class="modal-torrent-section ${loc.type === 'missing' ? '' : 'hidden'}" id="modal-torrent-section">
          <div class="modal-torrent-header">
            <span class="modal-torrent-title">
              <span class="material-symbols-outlined icon" style="color: #38bdf8;">cloud_download</span>
              <span>Torrent Releases (Sukebei Nyaa)</span>
            </span>
            <button class="btn btn-sm btn-secondary" onclick="window.app.searchMovieTorrents('${escapeHtml(id)}')">
              <span class="material-symbols-outlined icon">sync</span> Refresh
            </button>
          </div>
          <div class="torrent-search-form">
            <input type="text" id="modal-torrent-search-input" class="torrent-search-input" value="${escapeHtml(id)}" placeholder="Search query (JAV ID, Title...)" onkeydown="if(event.key==='Enter'){event.preventDefault();window.app.searchMovieTorrentsCustom();}" />
            <button class="btn btn-primary btn-sm" onclick="window.app.searchMovieTorrentsCustom()">
              <span class="material-symbols-outlined icon">search</span> Search
            </button>
          </div>
          <div id="modal-torrent-results" class="torrent-results-list">
            <div style="text-align: center; color: var(--text-muted); padding: 1.25rem; font-size: 0.85rem;">
              Click "Search" to find download releases.
            </div>
          </div>
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
        <div class="gallery-section-title"><span class="material-symbols-outlined icon">person</span> Featured Cast</div>
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 1rem; margin-bottom: 2rem;">
          ${actressGrid}
        </div>
      ` : ''}

      ${screenshotsGrid ? `
        <div class="gallery-section-title">
          <span class="material-symbols-outlined icon">photo_library</span> Sample Screenshots (${screenshots.length})${localScreenshots ? ' <span class="badge-status" style="font-size: 0.72rem; background: rgba(16,185,129,0.2); color: #34d399; margin-left: 0.5rem;">Local NAS</span>' : ''}
        </div>
        <div class="modal-gallery-grid">
          ${screenshotsGrid}
        </div>
      ` : ''}
    </div>
  `;

  currentModalMovie = movie;
  currentModalMeta = meta;

  if (window.lucide) {
    window.lucide.createIcons();
  }

  // Auto-search torrents if movie is missing from library
  if (loc.type === 'missing') {
    setTimeout(() => {
      searchMovieTorrents(id);
    }, 150);
  }
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

export async function openGallery(index = 0) {
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

export function openLightbox(url, fallbackUrl, caption) {
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

export function closeLightbox() {
  elements.lightbox.classList.add('hidden');
  elements.lightboxImg.src = '';
}

export function watchInBrowser(id) {
  if (!id) return;
  const modalPlayer = document.getElementById('modal-video-player');
  const videoEl = document.getElementById('in-browser-video');
  if (!modalPlayer || !videoEl) return;

  videoEl.src = `/api/video/${encodeURIComponent(id)}`;
  modalPlayer.classList.remove('hidden');
  videoEl.play().catch(() => {});
}

export function closeVideoPlayer() {
  const modalPlayer = document.getElementById('modal-video-player');
  const videoEl = document.getElementById('in-browser-video');
  if (videoEl) {
    videoEl.pause();
    videoEl.removeAttribute('src');
    videoEl.load();
  }
  if (modalPlayer) {
    modalPlayer.classList.add('hidden');
  }
}

// =========================================================================
// Torrent Discovery & Search Helpers
// =========================================================================

let currentModalMovie = null;
let currentModalMeta = null;
let currentTorrents = [];

export function toggleModalTorrentSection(id) {
  const section = document.getElementById('modal-torrent-section');
  if (!section) return;
  const wasHidden = section.classList.contains('hidden');
  section.classList.toggle('hidden');
  if (wasHidden) {
    searchMovieTorrents(id);
  }
}

export async function searchMovieTorrents(query) {
  const q = (query || '').trim();
  if (!q) return;

  const inputEl = document.getElementById('modal-torrent-search-input');
  if (inputEl) inputEl.value = q;

  const container = document.getElementById('modal-torrent-results');
  if (!container) return;

  container.innerHTML = `
    <div style="text-align: center; color: var(--text-muted); padding: 1.5rem;">
      <span class="material-symbols-outlined icon" style="font-size: 2rem; animation: spin 1s linear infinite;">sync</span>
      <p style="margin-top: 0.5rem; font-size: 0.85rem;">Searching Sukebei Nyaa for releases...</p>
    </div>
  `;

  try {
    const res = await searchTorrents(q);
    currentTorrents = res.items || [];
    renderTorrentResults(currentTorrents, container);
  } catch (err) {
    container.innerHTML = `
      <div style="text-align: center; color: #f87171; padding: 1.5rem; font-size: 0.85rem;">
        <span class="material-symbols-outlined icon" style="font-size: 1.8rem;">error</span>
        <p style="margin-top: 0.4rem;">Failed to search torrents: ${escapeHtml(err.message)}</p>
      </div>
    `;
  }
}

export function searchMovieTorrentsCustom() {
  const inputEl = document.getElementById('modal-torrent-search-input');
  const q = inputEl?.value.trim();
  if (q) {
    searchMovieTorrents(q);
  }
}

function stringsEqualFold(a, b) {
  return (a || '').toLowerCase() === (b || '').toLowerCase();
}

function renderTorrentResults(items, container) {
  const curId = currentModalMovie?.id || currentModalMeta?.id || '';
  const loc = getMovieLocationInfo(curId);

  let warningBannerHtml = '';
  if (loc.type === 'library') {
    warningBannerHtml = `
      <div class="torrent-duplicate-warning library">
        <span class="material-symbols-outlined icon" style="color: #34d399;">check_circle</span>
        <div>
          <strong>Already In Library:</strong> เรื่องนี้มีไฟล์พร้อมดูอยู่ในคลัง NAS เรียบร้อยแล้ว (${escapeHtml(loc.folderPath || 'NAS')})
        </div>
      </div>
    `;
  } else if (loc.type === 'downloading') {
    warningBannerHtml = `
      <div class="torrent-duplicate-warning downloading">
        <span class="material-symbols-outlined icon spin-pulse" style="color: #38bdf8;">downloading</span>
        <div style="flex: 1;">
          <strong>Already Downloading:</strong> เรื่องนี้กำลังดาวน์โหลดใน Transmission (${Math.round(loc.progress || 0)}%)
          ${loc.queueItem?.torrent_title ? `<div class="sub-torrent-name" style="font-size: 0.75rem; color: var(--text-muted);">${escapeHtml(loc.queueItem.torrent_title)}</div>` : ''}
        </div>
        <button class="btn btn-secondary btn-sm" onclick="window.app.openDownloadsDrawer()">View in Queue</button>
      </div>
    `;
  } else if (loc.type === 'staging') {
    warningBannerHtml = `
      <div class="torrent-duplicate-warning staging">
        <span class="material-symbols-outlined icon" style="color: #fbbf24;">inventory_2</span>
        <div style="flex: 1;">
          <strong>Downloaded to Staging:</strong> ไฟล์ดาวน์โหลดเสร็จ 100% แล้ว อยู่ระหว่างรอจัดระเบียบเข้าคลัง
        </div>
        <button class="btn btn-primary btn-sm" onclick="window.app.openDownloadsDrawer()">Organize Now</button>
      </div>
    `;
  }

  if (!items || items.length === 0) {
    container.innerHTML = `
      ${warningBannerHtml}
      <div style="text-align: center; color: var(--text-muted); padding: 1.5rem; font-size: 0.85rem;">
        <span class="material-symbols-outlined icon" style="font-size: 1.8rem;">search_off</span>
        <p style="margin-top: 0.4rem;">No torrent releases found on Sukebei.</p>
      </div>
    `;
    return;
  }

  const listHtml = items.map((item, idx) => {
    const isRecommended = item.is_recommended;
    const resBadge = item.resolution ? `<span class="t-badge res-${item.resolution.toLowerCase()}">${escapeHtml(item.resolution)}</span>` : '';
    const uncensoredBadge = item.is_uncensored ? '<span class="t-badge uncensored">🔓 Uncensored</span>' : '';
    const subsBadge = item.has_subtitles ? '<span class="t-badge subtitles">💬 Subtitles</span>' : '';
    const recBadge = isRecommended ? '<span class="t-badge recommended">⭐ BEST MATCH</span>' : '';
    const sizeStr = item.size_formatted || (item.size_bytes > 0 ? formatBytes(item.size_bytes) : '');

    const isExactTorrentActive = (state.downloadQueue || []).some(
      q => q.torrent_hash && item.info_hash && stringsEqualFold(q.torrent_hash, item.info_hash)
    );

    let actionBtnHtml = '';
    if (isExactTorrentActive) {
      actionBtnHtml = `
        <button class="btn btn-secondary btn-sm btn-download-torrent" id="btn-download-torrent-${idx}" disabled style="opacity: 0.85; border-color: rgba(56, 189, 248, 0.4); color: #38bdf8;">
          <span class="material-symbols-outlined icon">check_circle</span> In Transmission
        </button>
      `;
    } else if (loc.type === 'downloading' || loc.type === 'library' || loc.type === 'staging') {
      actionBtnHtml = `
        <button class="btn btn-secondary btn-sm btn-download-torrent" id="btn-download-torrent-${idx}" onclick="window.app.triggerTorrentDownload(${idx}, true)" title="Download alternate version">
          <span class="material-symbols-outlined icon">download</span> Alternate
        </button>
      `;
    } else {
      actionBtnHtml = `
        <button class="btn btn-primary btn-sm btn-download-torrent" id="btn-download-torrent-${idx}" onclick="window.app.triggerTorrentDownload(${idx}, false)">
          <span class="material-symbols-outlined icon">download</span> Download
        </button>
      `;
    }

    return `
      <div class="torrent-card ${isRecommended ? 'recommended' : ''} ${isExactTorrentActive ? 'active-downloading' : ''}" id="torrent-card-${idx}">
        <div class="torrent-card-top">
          <div class="torrent-card-name">${escapeHtml(item.title)}</div>
        </div>

        <div class="torrent-badges-row">
          ${recBadge}
          ${resBadge}
          ${uncensoredBadge}
          ${subsBadge}
          ${isExactTorrentActive ? '<span class="t-badge res-1080p" style="background: rgba(56, 189, 248, 0.15); color: #38bdf8;">⚡ IN QUEUE</span>' : ''}
        </div>

        <div class="torrent-card-bottom">
          <div class="torrent-stats">
            <span class="stat-seeders" title="Seeders">▲ ${item.seeders || 0}</span>
            <span class="stat-leechers" title="Leechers">▼ ${item.leechers || 0}</span>
            ${sizeStr ? `<span>${sizeStr}</span>` : ''}
          </div>
          ${actionBtnHtml}
        </div>
      </div>
    `;
  }).join('');

  container.innerHTML = `${warningBannerHtml}${listHtml}`;
}

export async function triggerTorrentDownload(index, isAlternate = false) {
  const item = currentTorrents[index];
  if (!item) return;

  const id = currentModalMovie?.id || (currentModalMeta?.id) || '';
  const loc = getMovieLocationInfo(id);

  if (isAlternate || loc.type === 'downloading' || loc.type === 'library' || loc.type === 'staging') {
    const statusLabel = loc.type === 'library' ? 'มีอยู่ในคลัง NAS แล้ว' : (loc.type === 'staging' ? 'ดาวน์โหลดเสร็จแล้วใน Staging' : 'กำลังดาวน์โหลดอยู่ใน Transmission');
    const ok = confirm(`เรื่องนี้ (${id}) ${statusLabel}!\n\nคุณแน่ใจหรือไม่ว่าต้องการดาวน์โหลด Torrent ไฟล์นี้เพิ่มอีก?`);
    if (!ok) return;
  }

  const btn = document.getElementById(`btn-download-torrent-${index}`);
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = `<span class="material-symbols-outlined icon" style="animation: spin 1s linear infinite;">sync</span> Adding...`;
  }

  const payload = {
    movie_id: id,
    combined_id: currentModalMeta?.combined_id || '',
    movie_title: currentModalMeta?.title || '',
    cover_url: currentModalMeta?.cover_image_url || '',
    actress_name: (currentModalMeta?.actresses && currentModalMeta.actresses[0]?.name) || '',
    torrent_hash: item.info_hash || '',
    torrent_title: item.title || '',
    torrent_url: item.link || '',
    magnet_url: item.magnet_url || '',
    file_size_bytes: item.size_bytes || 0,
    quality_tag: item.resolution || '',
    force: isAlternate
  };

  try {
    await downloadTorrent(payload);
    if (btn) {
      btn.className = 'btn btn-secondary btn-sm';
      btn.disabled = true;
      btn.innerHTML = `<span class="material-symbols-outlined icon" style="color: #38bdf8;">check_circle</span> In Transmission`;
    }
    if (window.app?.refreshDownloadsQueue) {
      window.app.refreshDownloadsQueue();
    }
  } catch (err) {
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = `<span class="material-symbols-outlined icon">download</span> Retry`;
    }
    alert(`Download failed: ${err.message}`);
  }
}

