/**
 * R19DEV Studio - Movie Details Modal & Lightbox
 * Native ES Module
 */

import { state, elements, escapeHtml, getHighResScreenshotUrl, getMovieLocationInfo } from './state.js';
import { scrapeMovie, fetchMovie } from './api.js';
import { closeHistoryModal } from './history.js';
import { closeActressProfileDrawer } from './actress.js';
import { closeNetworkGraph } from './graph.js';

export function setupModals() {
  elements.btnCloseModal?.addEventListener('click', closeModal);
  elements.modalMovie?.addEventListener('click', (e) => {
    if (e.target === elements.modalMovie) closeModal();
  });

  elements.btnCloseLightbox?.addEventListener('click', closeLightbox);
  elements.lightbox?.addEventListener('click', (e) => {
    if (e.target === elements.lightbox) closeLightbox();
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (!elements.lightbox?.classList.contains('hidden')) {
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
    }
  });
}

export async function openMovieDetail(movie) {
  state.selectedMovieId = movie.id;
  elements.modalMovie.classList.remove('hidden');

  let meta = state.metadata[movie.id];
  if (!meta && movie.id) {
    // 1. Check local SQLite first (<2ms) - completely silent, no toast, no network delay!
    meta = await fetchMovie(movie.id);
  }

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

  renderModalContent(movie, meta);
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

export function renderModalContent(movie, meta) {
  const id = movie.id || 'UNMATCHED';
  const uState = state.userStates[id] || {};
  const isOrganized = Boolean(state.organizedStatus[id]);
  const loc = getMovieLocationInfo(id);
  const coverUrl = meta?.cover_url || meta?.poster_url || (id !== 'UNMATCHED' ? '/api/images/' + id : '');

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
      const thumb = act.image_url || 'https://pics.dmm.co.jp/mono/actjpgs/now_printing.jpg';
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
        <div style="display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.6rem;">
          <span class="card-id" style="font-size: 0.95rem; font-weight: 700;">${escapeHtml(id)}</span>
          <button class="btn btn-secondary btn-sm" style="padding: 0.2rem 0.6rem; font-size: 0.75rem; border-radius: 6px;" onclick="window.app.copyMovieId('${escapeHtml(id)}', event)" title="Copy ${escapeHtml(id)}"><span class="material-symbols-outlined icon">content_copy</span> Copy ID</button>
        </div>
        <div class="ja-title">${escapeHtml(meta?.original_title || '')}</div>

        <!-- Location Status Indicator Card -->
        <div class="modal-location-card ${loc.type === 'library' ? 'in-library' : (loc.type === 'external' ? 'outside-library' : 'in-staging')}" role="region" aria-label="Media Storage Location">
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
              <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(loc.folderPath)}" onclick="window.app.openFolderEl(this, event)">
                <span class="material-symbols-outlined icon">folder_open</span> Open in Finder
              </button>
              <button class="btn btn-outline-secondary btn-sm" onclick="window.app.organizeSingle('${id}')" title="Re-run Jellyfin organize">
                <span class="material-symbols-outlined icon">sync</span> Re-organize
              </button>
            ` : (loc.type === 'external' ? `
              <button class="btn btn-primary btn-sm" onclick="window.app.organizeSingle('${id}')" title="Move and organize into Library destination">
                <span class="material-symbols-outlined icon">drive_file_move</span> Move to Library
              </button>
              <button class="btn btn-secondary btn-sm" data-movie-id="${escapeHtml(id)}" data-path="${escapeHtml(loc.folderPath)}" onclick="window.app.openFolderEl(this, event)">
                <span class="material-symbols-outlined icon">folder_open</span> Open in Finder
              </button>
            ` : `
              <button class="btn btn-primary btn-sm" onclick="window.app.organizeSingle('${id}')">
                <span class="material-symbols-outlined icon">folder_zip</span> Organize for Jellyfin
              </button>
            `)}
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
        <div class="gallery-section-title"><span class="material-symbols-outlined icon">photo_library</span> Sample Screenshots (${meta.sample_screenshots.length})</div>
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
