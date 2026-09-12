/**
 * R19DEV Studio - API Client & Action Helpers
 * Native ES Module
 */

import { state, elements, showToast, escapeHtml } from './state.js';
import { renderMoviesGrid, updateStats, populateFilterDropdowns, getDefaultOrganizedDestination } from './scanner.js';
import { renderModalContent } from './modal.js';
import { renderActressHub, renderActressCollection } from './actress.js';

// =========================================================================
// Live Operation Activity Progress Helpers
// =========================================================================
export function showOpProgress(icon, title, countStr, pct, message) {
  if (!elements.opProgressBox) return;
  elements.opProgressBox.classList.remove('hidden');
  elements.opIcon.textContent = icon;
  elements.opTitle.textContent = title;
  elements.opCounter.textContent = countStr || '';
  elements.opPct.textContent = `${pct}%`;
  elements.opMessage.textContent = message;
  elements.opProgressFill.style.width = `${pct}%`;
}

export function hideOpProgress(delayMs = 1500) {
  if (!elements.opProgressBox) return;
  setTimeout(() => {
    elements.opProgressBox.classList.add('hidden');
  }, delayMs);
}

// =========================================================================
// Scraper API
// =========================================================================
export async function scrapeMovie(id) {
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

export async function scrapeAllMatched() {
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

// =========================================================================
// User State & Interactions
// =========================================================================
export async function toggleWatched(id) {
  const cur = state.userStates[id]?.is_watched || false;
  await updateUserState(id, { is_watched: !cur });
  showToast(!cur ? `Marked ${id} as Watched` : `Marked ${id} as Unwatched`, 'info');
}

export async function toggleFavorite(id) {
  const cur = state.userStates[id]?.is_favorite || false;
  await updateUserState(id, { is_favorite: !cur });
  showToast(!cur ? `Added ${id} to Favorites ❤️` : `Removed ${id} from Favorites`, 'info');
}

export async function setRating(id, rating) {
  await updateUserState(id, { user_rating: rating });
  showToast(`Rated ${id}: ${'⭐'.repeat(rating)}`, 'success');
}

export async function updateUserState(id, updates) {
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

// =========================================================================
// Actress Following & Tracking
// =========================================================================
export async function toggleFollowActress(name) {
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

export async function unfollowActress(name) {
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

export async function loadActressesData() {
  try {
    const res = await fetch('/api/actresses/releases');
    const data = await res.json();
    state.actresses = data.actresses || [];
    elements.countActresses.textContent = state.actresses.length;

    const elFollowed = document.getElementById('count-subtab-followed');
    if (elFollowed) elFollowed.textContent = state.actresses.length;

    // Also load discovered actresses in NAS
    await loadDiscoveredActresses();

    // Populate organized folders & status from all actress releases
    state.actresses.forEach(entry => {
      (entry.releases || []).forEach(rel => {
        if (rel.organized_folder) {
          state.organizedFolders[rel.movie_id] = rel.organized_folder;
          state.organizedStatus[rel.movie_id] = true;
        } else if (rel.is_downloaded && rel.library_path) {
          state.organizedFolders[rel.movie_id] = rel.library_path;
          state.organizedStatus[rel.movie_id] = true;
        }
      });
    });

    renderActressHub();
  } catch (err) {
    console.error('Failed to load actresses:', err);
  }
}

export async function loadDiscoveredActresses() {
  try {
    const res = await fetch('/api/actresses/discovered');
    const data = await res.json();
    state.discoveredActresses = data.actresses || [];
    const elDiscovered = document.getElementById('count-subtab-discovered');
    if (elDiscovered) elDiscovered.textContent = state.discoveredActresses.length;
  } catch (err) {
    console.error('Failed to load discovered actresses:', err);
  }
}

export function switchActressSubTab(tab) {
  state.actressHubTab = tab || 'followed';
  const btnFollowed = document.getElementById('subtab-btn-followed');
  const btnDiscovered = document.getElementById('subtab-btn-discovered');
  const isFollowed = state.actressHubTab === 'followed';

  if (btnFollowed) {
    btnFollowed.classList.toggle('active', isFollowed);
    btnFollowed.setAttribute('aria-selected', String(isFollowed));
  }
  if (btnDiscovered) {
    btnDiscovered.classList.toggle('active', !isFollowed);
    btnDiscovered.setAttribute('aria-selected', String(!isFollowed));
  }

  const titleEl = document.getElementById('actress-hub-title');
  const subtitleEl = document.getElementById('actress-hub-subtitle');
  const sortBox = document.getElementById('actress-sort-box');

  if (isFollowed) {
    if (titleEl) titleEl.textContent = 'Followed Actresses';
    if (subtitleEl) subtitleEl.textContent = 'Solo filmographies & Jellyfin collection progress';
    if (sortBox) sortBox.classList.remove('hidden');
  } else {
    if (titleEl) titleEl.textContent = 'Discovered in NAS Library';
    if (subtitleEl) subtitleEl.textContent = 'Actresses found in your video library who are not yet followed';
    if (sortBox) sortBox.classList.add('hidden');
  }

  renderActressCollection();
}

export async function quickFollowDiscovered(name, jaName, imageUrl, btn) {
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span class="material-symbols-outlined icon" style="font-size: 1rem; animation: spin 1s linear infinite;">sync</span> Following...';
  }
  showToast(`Following ${name}...`, 'info');
  try {
    const res = await fetch('/api/actresses/follow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, ja_name: jaName, image_url: imageUrl })
    });
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`);
    }
    showToast(`Followed ${name}! Checking releases...`, 'success');
    await loadActressesData();
    renderActressCollection();
  } catch (err) {
    showToast(`Failed to follow ${name}: ${err.message}`, 'danger');
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = '<span class="material-symbols-outlined icon">person_add</span> Follow';
    }
  }
}

export async function refreshAllActresses(btn) {
  if (btn) btn.classList.add('loading-spin');
  showToast('Refreshing all actress releases... 🔄', 'info');
  try {
    await loadActressesData();
    showToast('All actress releases updated! ✅', 'success');
  } catch (err) {
    showToast('Error refreshing releases: ' + err.message, 'danger');
  } finally {
    if (btn) btn.classList.remove('loading-spin');
  }
}

export async function refreshSingleActress(name, btn) {
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

export async function trackTitleToActress(actressName) {
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

export async function followActressFromInput() {
  const name = elements.inputActressName?.value?.trim();
  if (!name) {
    showToast('Please enter an actress name', 'warning');
    return;
  }
  try {
    showToast(`Adding and following ${name}... ⏳`, 'info');
    await fetch('/api/actresses/follow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: name })
    });
    if (elements.inputActressName) elements.inputActressName.value = '';
    state.activeActressName = name;
    showToast(`Followed ${name} ⭐`, 'success');
    await loadActressesData();
  } catch (err) {
    showToast('Error following actress: ' + err.message, 'danger');
  }
}

export async function promptAddActress() {
  const name = window.prompt('Enter actress name to follow (e.g. Hayasaka Kanon, Yua Mikami):');
  if (!name || !name.trim()) return;
  try {
    showToast(`Following ${name.trim()}...`, 'info');
    const res = await fetch(`/api/actresses/follow?name=${encodeURIComponent(name.trim())}`, { method: 'POST' });
    if (res.ok) {
      showToast(`Successfully followed ${name.trim()}! Fetching releases...`, 'success');
      await loadActressesData();
    } else {
      const err = await res.json().catch(() => ({}));
      showToast(err.error || 'Failed to follow actress', 'danger');
    }
  } catch (e) {
    showToast(`Error: ${e.message}`, 'danger');
  }
}

export async function openActiveActressFolder() {
  let activeEntry = state.actresses.find(entry => entry.actress.name === state.activeActressName);
  if (!activeEntry && state.actresses.length > 0) {
    activeEntry = state.actresses[0];
    state.activeActressName = activeEntry.actress.name;
  }
  if (!activeEntry) {
    showToast('Please select an actress first', 'warning');
    return;
  }
  const dl = (activeEntry.releases || []).find(r => r.is_downloaded);
  const folderToOpen = dl?.organized_folder || dl?.library_path || '';
  const movieID = dl?.movie_id || '';
  await openFolder(movieID, folderToOpen, activeEntry.actress.name);
}

export async function unfollowActiveActress() {
  let targetName = state.activeActressName;
  if (!targetName && state.actresses.length > 0) {
    targetName = state.actresses[0].actress.name;
  }
  if (!targetName) return;
  if (confirm(`Unfollow ${targetName}?`)) {
    await unfollowActress(targetName);
    state.activeActressName = null;
    await loadActressesData();
  }
}

export function trackTitleActiveActress() {
  let targetName = state.activeActressName;
  if (!targetName && state.actresses.length > 0) {
    targetName = state.actresses[0].actress.name;
    state.activeActressName = targetName;
  }
  if (targetName) {
    trackTitleToActress(targetName);
  } else {
    showToast('Please select or follow an actress first', 'info');
  }
}

export function refreshActiveActress(btn) {
  let targetName = state.activeActressName;
  if (!targetName && state.actresses.length > 0) {
    targetName = state.actresses[0].actress.name;
    state.activeActressName = targetName;
  }
  if (targetName) {
    refreshSingleActress(targetName, btn);
  } else {
    showToast('Please select or follow an actress first', 'info');
  }
}

// =========================================================================
// Single Movie Organize & Finder
// =========================================================================
export async function organizeSingle(id) {
  const movie = state.groupedMovies.find(m => m.id === id);
  if (!movie) return;

  const dest = elements.orgDestRoot.value.trim() || getDefaultOrganizedDestination(state.activeDir);
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
        showOpProgress('✅', `จัดระเบียบ ${id} สำเร็จ!`, '6 / 6', 100, `บันทึกที่: ${data.target_folder || 'organized'}`);
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

export async function openFolder(id, path, actressName) {
  const targetFolder = path || state.organizedFolders[id] || '';
  showToast('📂 กำลังเปิดโฟลเดอร์ใน Finder...', 'info');
  try {
    const res = await fetch('/api/open-folder', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ movie_id: id, path: targetFolder, actress: actressName })
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

export function openFolderByEl(el, e) {
  if (e) e.stopPropagation();
  const id = el.getAttribute('data-movie-id') || '';
  const path = el.getAttribute('data-path') || '';
  const actress = el.getAttribute('data-actress') || '';
  openFolder(id, path, actress);
}

export function copyMovieId(id, e) {
  if (e) e.stopPropagation();
  if (!id) return;
  navigator.clipboard.writeText(id).then(() => {
    showToast(`Copied ${id} to clipboard! 📋`, 'success');
  }).catch(() => {
    showToast(`Failed to copy ${id}`, 'danger');
  });
}
