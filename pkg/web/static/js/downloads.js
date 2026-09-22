/**
 * R19DEV Studio - Downloads & Transmission Queue Management
 * Native ES Module
 */

import { state, elements, formatBytes, showToast, escapeHtml } from './state.js';
import {
  fetchDownloadQueue,
  deleteQueueItem,
  organizeQueueItem,
  fetchSettings,
  saveSettings,
  testTransmissionConnection
} from './api.js';

let pollTimer = null;

// =========================================================================
// Downloads Queue Drawer & State
// =========================================================================

export function openDownloadsDrawer() {
  state.isDownloadsDrawerOpen = true;
  if (elements.drawerDownloads) {
    elements.drawerDownloads.classList.remove('hidden');
  }
  if (elements.drawerDownloadsBackdrop) {
    elements.drawerDownloadsBackdrop.classList.remove('hidden');
  }
  refreshDownloadsQueue();
  startDownloadPolling(3000);
}

export function closeDownloadsDrawer() {
  state.isDownloadsDrawerOpen = false;
  if (elements.drawerDownloads) {
    elements.drawerDownloads.classList.add('hidden');
  }
  if (elements.drawerDownloadsBackdrop) {
    elements.drawerDownloadsBackdrop.classList.add('hidden');
  }
  // Revert to background polling
  startDownloadPolling(12000);
}

export async function refreshDownloadsQueue() {
  try {
    const items = await fetchDownloadQueue();
    state.downloadQueue = items || [];
    updateDownloadsBadge();
    renderDownloadQueue(state.downloadQueue);
  } catch (err) {
    console.error('Failed to refresh queue:', err);
  }
}

export function updateDownloadsBadge() {
  if (!elements.countDownloads) return;
  const activeItems = (state.downloadQueue || []).filter(
    it => it.status === 'downloading' || it.status === 'staging' || it.status === 'queued'
  );
  const count = activeItems.length;
  state.activeDownloadsCount = count;
  elements.countDownloads.textContent = count;
  if (count > 0) {
    elements.countDownloads.classList.add('active');
  } else {
    elements.countDownloads.classList.remove('active');
  }
}

export function setDownloadsViewMode(mode) {
  state.downloadsViewMode = mode;
  try {
    localStorage.setItem('r19dev_downloads_view', mode);
  } catch (e) {}

  if (elements.btnDownloadsViewKanban) {
    elements.btnDownloadsViewKanban.classList.toggle('active', mode === 'kanban');
  }
  if (elements.btnDownloadsViewList) {
    elements.btnDownloadsViewList.classList.toggle('active', mode === 'list');
  }
  renderDownloadQueue(state.downloadQueue);
}

export function toggleDownloadsExpand() {
  state.isDownloadsDrawerExpanded = !state.isDownloadsDrawerExpanded;
  if (elements.drawerDownloads) {
    elements.drawerDownloads.classList.toggle('expanded', state.isDownloadsDrawerExpanded);
  }
  if (elements.iconExpandDownloads) {
    elements.iconExpandDownloads.textContent = state.isDownloadsDrawerExpanded ? 'close_fullscreen' : 'open_in_full';
  }
  if (elements.btnExpandDownloads) {
    elements.btnExpandDownloads.title = state.isDownloadsDrawerExpanded ? 'Collapse Drawer' : 'Expand to Wide Kanban Board';
  }
}

export function renderDownloadQueue(items) {
  const container = elements.downloadsQueueBody;
  if (!container) return;

  if (elements.btnDownloadsViewKanban) {
    elements.btnDownloadsViewKanban.classList.toggle('active', state.downloadsViewMode === 'kanban');
  }
  if (elements.btnDownloadsViewList) {
    elements.btnDownloadsViewList.classList.toggle('active', state.downloadsViewMode === 'list');
  }

  if (!items || items.length === 0) {
    container.innerHTML = `
      <div class="downloads-empty" id="downloads-empty">
        <span class="material-symbols-outlined icon" style="font-size: 2.8rem; color: var(--text-muted);">cloud_download</span>
        <p style="margin: 0.6rem 0 0.3rem; font-weight: 600; color: #fff; font-size: 1.05rem;">No active downloads in queue</p>
        <span class="empty-hint" style="font-size: 0.85rem; color: var(--text-muted); max-width: 320px; line-height: 1.4;">Search for torrents from any movie details or actress page to download via Transmission.</span>
      </div>
    `;
    return;
  }

  if (state.downloadsViewMode === 'kanban') {
    renderKanbanBoard(items, container);
  } else {
    renderListView(items, container);
  }
}

function renderKanbanCard(item) {
  const id = item.movie_id || '';
  const title = item.movie_title || item.torrent_title || id;
  const coverUrl = item.cover_url || (id ? `/api/images/${encodeURIComponent(id)}` : '/placeholder.png');
  const pct = (item.progress_pct || 0).toFixed(1);
  const speedStr = item.download_speed > 0 ? `${formatBytes(item.download_speed)}/s` : '';
  const etaStr = item.eta_seconds > 0 ? `ETA: ${formatETA(item.eta_seconds)}` : '';
  const sizeStr = item.file_size_bytes > 0 ? formatBytes(item.file_size_bytes) : '';
  const status = item.status || 'queued';
  const hasError = !!item.error_message;

  return `
    <div class="kanban-card ${escapeHtml(status)} ${hasError ? 'has-error' : ''}" data-id="${item.id}">
      <div class="kanban-card-top">
        <img class="kanban-card-thumb" src="${coverUrl}" alt="${escapeHtml(id)}" onerror="this.src='/placeholder.png';" onclick="if ('${escapeHtml(id)}') window.app.openMovieById('${escapeHtml(id)}')" />
        <div class="kanban-card-info">
          <div class="kanban-card-id-row">
            <span class="kanban-card-id" onclick="if ('${escapeHtml(id)}') window.app.openMovieById('${escapeHtml(id)}')">${escapeHtml(id)}</span>
            ${item.quality_tag ? `<span class="t-badge res-1080p">${escapeHtml(item.quality_tag)}</span>` : ''}
            <button class="kanban-btn-close" title="Remove from queue" onclick="window.app.handleDeleteQueueItem(${item.id})">
              <span class="material-symbols-outlined icon">close</span>
            </button>
          </div>
          <div class="kanban-card-title" title="${escapeHtml(title)}">${escapeHtml(title)}</div>
        </div>
      </div>

      ${hasError ? `
        <div class="kanban-error-banner" title="${escapeHtml(item.error_message)}">
          <span class="material-symbols-outlined icon">error</span>
          <span>${escapeHtml(item.error_message)}</span>
        </div>
      ` : ''}

      ${(status === 'downloading' || status === 'queued') ? `
        <div class="download-progress-track">
          <div class="download-progress-bar downloading" style="width: ${pct}%"></div>
        </div>
        <div class="kanban-meta-row">
          <span>${pct}% ${sizeStr ? `· ${sizeStr}` : ''}</span>
          <span>${speedStr} ${etaStr ? `· ${etaStr}` : ''}</span>
        </div>
      ` : ''}

      <div class="kanban-card-actions">
        ${status === 'staging' ? `
          <button class="btn btn-primary btn-sm" onclick="window.app.handleOrganizeQueueItem(${item.id})">
            <span class="material-symbols-outlined icon">auto_fix_high</span> Organize Now
          </button>
        ` : ''}
        ${status === 'organized' ? `
          <button class="btn btn-primary btn-sm" onclick="window.app.playMovie('${escapeHtml(id)}')">
            <span class="material-symbols-outlined icon">play_arrow</span> Play
          </button>
        ` : ''}
        ${id ? `
          <button class="btn btn-secondary btn-sm" onclick="window.app.openMovieById('${escapeHtml(id)}')">
            <span class="material-symbols-outlined icon">info</span> Details
          </button>
        ` : ''}
      </div>
    </div>
  `;
}

function renderKanbanBoard(items, container) {
  const colDownloading = items.filter(it => it.status === 'downloading' || it.status === 'queued');
  const colStaging = items.filter(it => it.status === 'staging');
  const colOrganized = items.filter(it => it.status === 'organized');

  const totalSpeed = colDownloading.reduce((acc, it) => acc + (it.download_speed || 0), 0);
  const speedStr = totalSpeed > 0 ? `· ${formatBytes(totalSpeed)}/s` : '';

  container.innerHTML = `
    <div class="kanban-board">
      <!-- Column 1: Downloading & Queued -->
      <div class="kanban-column col-downloading">
        <div class="kanban-col-header">
          <div class="col-header-left">
            <span class="col-indicator downloading"></span>
            <span class="col-title">Downloading</span>
            <span class="col-count-badge">${colDownloading.length}</span>
          </div>
          <span class="col-speed">${speedStr}</span>
        </div>
        <div class="kanban-col-cards">
          ${colDownloading.length === 0 ? `
            <div class="kanban-col-empty">
              <span class="material-symbols-outlined icon">cloud_download</span>
              <p>No active downloads</p>
            </div>
          ` : colDownloading.map(renderKanbanCard).join('')}
        </div>
      </div>

      <!-- Column 2: Ready to Organize (Staging) -->
      <div class="kanban-column col-staging">
        <div class="kanban-col-header">
          <div class="col-header-left">
            <span class="col-indicator staging"></span>
            <span class="col-title">Ready to Organize</span>
            <span class="col-count-badge">${colStaging.length}</span>
          </div>
          ${colStaging.length > 1 ? `
            <button class="btn-col-action" onclick="window.app.organizeAllStaging()" title="Organize all staging items">
              <span class="material-symbols-outlined icon">bolt</span> All
            </button>
          ` : ''}
        </div>
        <div class="kanban-col-cards">
          ${colStaging.length === 0 ? `
            <div class="kanban-col-empty">
              <span class="material-symbols-outlined icon">inventory_2</span>
              <p>No files waiting to organize</p>
            </div>
          ` : colStaging.map(renderKanbanCard).join('')}
        </div>
      </div>

      <!-- Column 3: In Library (Organized) -->
      <div class="kanban-column col-organized">
        <div class="kanban-col-header">
          <div class="col-header-left">
            <span class="col-indicator organized"></span>
            <span class="col-title">In Library</span>
            <span class="col-count-badge">${colOrganized.length}</span>
          </div>
        </div>
        <div class="kanban-col-cards">
          ${colOrganized.length === 0 ? `
            <div class="kanban-col-empty">
              <span class="material-symbols-outlined icon">check_circle</span>
              <p>No organized items yet</p>
            </div>
          ` : colOrganized.map(renderKanbanCard).join('')}
        </div>
      </div>
    </div>
  `;
}

function renderListView(items, container) {
  const cardsHtml = items.map(item => {
    const id = item.movie_id || '';
    const title = item.movie_title || item.torrent_title || id;
    const coverUrl = item.cover_url || (id ? `/api/images/${encodeURIComponent(id)}` : '/placeholder.png');
    const pct = (item.progress_pct || 0).toFixed(1);
    const speedStr = item.download_speed > 0 ? `${formatBytes(item.download_speed)}/s` : '';
    const etaStr = item.eta_seconds > 0 ? `ETA: ${formatETA(item.eta_seconds)}` : '';
    const sizeStr = item.file_size_bytes > 0 ? formatBytes(item.file_size_bytes) : '';
    const status = item.status || 'queued';
    const hasError = !!item.error_message;

    let statusText = status.toUpperCase();
    let statusIcon = 'hourglass_empty';
    if (status === 'downloading') {
      statusIcon = 'downloading';
    } else if (status === 'staging') {
      statusIcon = 'folder_zip';
      statusText = 'READY TO ORGANIZE';
    } else if (status === 'organized') {
      statusIcon = 'check_circle';
      statusText = 'IN LIBRARY';
    } else if (status === 'error') {
      statusIcon = 'error';
    }

    return `
      <div class="download-item-card ${escapeHtml(status)} ${hasError ? 'has-error' : ''}" data-id="${item.id}">
        <div class="download-item-header">
          <img class="download-item-thumb" src="${coverUrl}" alt="${escapeHtml(id)}" onerror="this.src='/placeholder.png';" onclick="if ('${escapeHtml(id)}') window.app.openMovieById('${escapeHtml(id)}')" />
          <div class="download-item-details">
            <div class="download-item-title-row">
              <span class="download-item-id" onclick="if ('${escapeHtml(id)}') window.app.openMovieById('${escapeHtml(id)}')">${escapeHtml(id)}</span>
              <span class="download-item-status-badge ${escapeHtml(status)}">
                <span class="material-symbols-outlined icon" style="font-size: 0.8rem;">${statusIcon}</span>
                ${statusText}
              </span>
              ${item.quality_tag ? `<span class="t-badge res-1080p">${escapeHtml(item.quality_tag)}</span>` : ''}
            </div>
            <div class="download-item-title" title="${escapeHtml(title)}">${escapeHtml(title)}</div>
            <div class="download-item-torrent-title" title="${escapeHtml(item.torrent_title)}">${escapeHtml(item.torrent_title)}</div>
          </div>
        </div>

        ${hasError ? `
          <div class="kanban-error-banner" title="${escapeHtml(item.error_message)}">
            <span class="material-symbols-outlined icon">error</span>
            <span>${escapeHtml(item.error_message)}</span>
          </div>
        ` : ''}

        <!-- Progress bar -->
        ${(status === 'downloading' || status === 'queued') ? `
          <div class="download-progress-track">
            <div class="download-progress-bar ${escapeHtml(status)}" style="width: ${pct}%"></div>
          </div>

          <div class="download-item-meta-row">
            <span>${pct}% ${sizeStr ? `· ${sizeStr}` : ''}</span>
            <span>${speedStr} ${etaStr ? `· ${etaStr}` : ''}</span>
          </div>
        ` : ''}

        <!-- Action Buttons -->
        <div class="download-item-actions">
          ${status === 'staging' ? `
            <button class="btn btn-primary btn-sm" onclick="window.app.handleOrganizeQueueItem(${item.id})">
              <span class="material-symbols-outlined icon">auto_fix_high</span> Organize Now
            </button>
          ` : ''}
          ${status === 'organized' ? `
            <button class="btn btn-primary btn-sm" onclick="window.app.playMovie('${escapeHtml(id)}')">
              <span class="material-symbols-outlined icon">play_arrow</span> Play
            </button>
          ` : ''}
          ${item.movie_id ? `
            <button class="btn btn-secondary btn-sm" onclick="window.app.openMovieById('${escapeHtml(item.movie_id)}')">
              <span class="material-symbols-outlined icon">info</span> Details
            </button>
          ` : ''}
          <button class="btn btn-secondary btn-sm btn-icon-only" title="Remove from queue" onclick="window.app.handleDeleteQueueItem(${item.id})">
            <span class="material-symbols-outlined icon">close</span>
          </button>
        </div>
      </div>
    `;
  }).join('');

  container.innerHTML = `<div class="downloads-queue-list">${cardsHtml}</div>`;
}

export async function organizeAllStaging() {
  const stagingItems = (state.downloadQueue || []).filter(it => it.status === 'staging');
  if (!stagingItems.length) return;
  showToast(`Organizing ${stagingItems.length} items to library...`, 'info');
  for (const it of stagingItems) {
    try {
      await organizeQueueItem(it.id);
    } catch (e) {
      console.warn('Failed to organize:', it.id, e);
    }
  }
  await refreshDownloadsQueue();
}

export async function handleOrganizeQueueItem(id) {
  try {
    await organizeQueueItem(id);
    await refreshDownloadsQueue();
  } catch (err) {
    // Error handled in API
  }
}

export async function handleDeleteQueueItem(id) {
  const confirmDelete = confirm('Remove this item from the download queue?');
  if (!confirmDelete) return;

  const deleteFromTransmission = confirm('Also remove torrent from Transmission client?');
  try {
    await deleteQueueItem(id, deleteFromTransmission);
    await refreshDownloadsQueue();
  } catch (err) {
    // Error handled in API
  }
}

export function startDownloadPolling(intervalMs = 10000) {
  if (pollTimer) clearInterval(pollTimer);
  pollTimer = setInterval(() => {
    refreshDownloadsQueue();
  }, intervalMs);
}

export function stopDownloadPolling() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

// =========================================================================
// Settings Modal
// =========================================================================

export async function openSettingsModal() {
  state.isSettingsModalOpen = true;
  if (elements.modalSettings) {
    elements.modalSettings.classList.remove('hidden');
  }
  loadAndRenderSettings();
}

export function closeSettingsModal() {
  state.isSettingsModalOpen = false;
  if (elements.modalSettings) {
    elements.modalSettings.classList.add('hidden');
  }
}

export async function loadAndRenderSettings() {
  try {
    const settings = await fetchSettings();
    const urlInput = document.getElementById('setting-transmission-url');
    const userInput = document.getElementById('setting-transmission-username');
    const passInput = document.getElementById('setting-transmission-password');
    const dirInput = document.getElementById('setting-transmission-download-dir');
    const sukebeiInput = document.getElementById('setting-sukebei-url');
    const autoOrgInput = document.getElementById('setting-auto-organize');
    const testStatus = document.getElementById('transmission-test-status');

    if (urlInput) urlInput.value = settings.transmission_url || 'http://192.168.1.189:9091';
    if (userInput) userInput.value = settings.transmission_username || '';
    if (passInput) passInput.value = settings.transmission_password || '';
    if (dirInput) dirInput.value = settings.transmission_download_dir || '';
    if (sukebeiInput) sukebeiInput.value = settings.sukebei_url || 'https://sukebei.nyaa.si';
    if (autoOrgInput) autoOrgInput.checked = (settings.auto_organize_completed === 'true');
    if (testStatus) {
      testStatus.classList.add('hidden');
      testStatus.textContent = '';
    }
  } catch (err) {
    showToast(`Failed to load settings: ${err.message}`, 'danger');
  }
}

export async function saveSettingsFromForm() {
  const urlInput = document.getElementById('setting-transmission-url');
  const userInput = document.getElementById('setting-transmission-username');
  const passInput = document.getElementById('setting-transmission-password');
  const dirInput = document.getElementById('setting-transmission-download-dir');
  const sukebeiInput = document.getElementById('setting-sukebei-url');
  const autoOrgInput = document.getElementById('setting-auto-organize');

  let rawDir = dirInput?.value.trim() || '';
  if (rawDir.startsWith('/Volumes/')) {
    rawDir = '/volume1/' + rawDir.replace(/^\/Volumes\//, '');
    if (dirInput) dirInput.value = rawDir;
    showToast(`Normalized Mac path to Synology NAS path: ${rawDir}`, 'info');
  }

  const settingsPayload = {
    transmission_url: urlInput?.value.trim() || '',
    transmission_username: userInput?.value.trim() || '',
    transmission_password: passInput?.value || '',
    transmission_download_dir: rawDir,
    sukebei_url: sukebeiInput?.value.trim() || 'https://sukebei.nyaa.si',
    auto_organize_completed: autoOrgInput?.checked ? 'true' : 'false'
  };

  try {
    await saveSettings(settingsPayload);
    closeSettingsModal();
    showToast('Settings saved successfully', 'success');
  } catch (err) {
    // Error handled in API
  }
}

export async function handleTestTransmission() {
  const urlInput = document.getElementById('setting-transmission-url');
  const userInput = document.getElementById('setting-transmission-username');
  const passInput = document.getElementById('setting-transmission-password');
  const testStatus = document.getElementById('transmission-test-status');

  if (!testStatus) return;
  testStatus.className = 'test-status-pill';
  testStatus.classList.remove('hidden');
  testStatus.textContent = 'Connecting...';

  const config = {
    url: urlInput?.value.trim() || '',
    username: userInput?.value.trim() || '',
    password: passInput?.value || ''
  };

  const res = await testTransmissionConnection(config);
  if (res.success) {
    testStatus.classList.add('success');
    testStatus.textContent = `Connected! (v${res.version || 'unknown'})`;
    showToast(`Transmission connected! Download dir: ${res.download_dir || 'Default'}`, 'success');
  } else {
    testStatus.classList.add('error');
    testStatus.textContent = `Failed: ${res.error || 'Connection error'}`;
    showToast(`Transmission connection failed: ${res.error}`, 'danger');
  }
}

// =========================================================================
// Helpers
// =========================================================================

function formatETA(seconds) {
  if (!seconds || seconds < 0) return 'unknown';
  if (seconds < 60) return `${seconds}s`;
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (mins < 60) return `${mins}m ${secs}s`;
  const hours = Math.floor(mins / 60);
  const remMins = mins % 60;
  return `${hours}h ${remMins}m`;
}
