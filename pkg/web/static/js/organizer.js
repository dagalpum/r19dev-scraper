/**
 * R19DEV Studio - Jellyfin Organizer Pipeline & Terminal Console
 * Native ES Module
 */

import { state, elements, showToast } from './state.js';
import { renderMoviesGrid } from './scanner.js';

export function appendOrganizerLog(text) {
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

export function openOrganizerDrawer() {
  if (!elements.drawerOrganizer) return;
  elements.drawerOrganizer.classList.remove('hidden');
  elements.drawerOrganizerBackdrop?.classList.remove('hidden');
  document.body.classList.add('drawer-open');

  // Pre-fill source dir if currentDir is available and input is empty
  if (elements.orgSrcDir && !elements.orgSrcDir.value && state.currentDir) {
    elements.orgSrcDir.value = state.currentDir;
  }
}

export function closeOrganizerDrawer() {
  if (!elements.drawerOrganizer) return;
  elements.drawerOrganizer.classList.add('hidden');
  elements.drawerOrganizerBackdrop?.classList.add('hidden');
  document.body.classList.remove('drawer-open');
}

export function toggleOrganizerExpand() {
  if (!elements.drawerOrganizer) return;
  const isExpanded = elements.drawerOrganizer.classList.toggle('expanded');
  if (elements.iconExpandOrganizer) {
    elements.iconExpandOrganizer.textContent = isExpanded ? 'close_fullscreen' : 'open_in_full';
  }
}

export function setupOrganizer() {
  elements.btnCloseOrganizer?.addEventListener('click', closeOrganizerDrawer);
  elements.drawerOrganizerBackdrop?.addEventListener('click', closeOrganizerDrawer);
  elements.btnExpandOrganizer?.addEventListener('click', toggleOrganizerExpand);
  elements.btnClearLog?.addEventListener('click', clearOrganizerLog);
  elements.btnCopyLog?.addEventListener('click', copyOrganizerLog);
  elements.btnResumeScroll?.addEventListener('click', resumeAutoScroll);
  elements.btnToggleAutoscroll?.addEventListener('click', toggleAutoScroll);

  window.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && elements.drawerOrganizer && !elements.drawerOrganizer.classList.contains('hidden')) {
      if (elements.modalHistory && !elements.modalHistory.classList.contains('hidden')) return;
      if (elements.modalMovie && !elements.modalMovie.classList.contains('hidden')) return;
      closeOrganizerDrawer();
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

  elements.btnStartOrganize?.addEventListener('click', () => {
    startOrganizeStream();
  });
}

export function startOrganizeStream() {
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
  elements.orgProgressLabel.textContent = 'Preparing organize pipeline...';
  elements.btnStartOrganize.disabled = true;

  elements.organizerLog.textContent = `Starting organize from ${src} -> ${dest} (DryRun: ${dryRun})...\n\n`;
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
        line += `   [FAIL] Error: ${item.error}\n`;
      }
      appendOrganizerLog(line);
    } catch (err) {}
  });

  es.addEventListener('done', (e) => {
    try {
      const data = JSON.parse(e.data);
      elements.orgProgressFill.style.width = '100%';
      elements.orgProgressPct.textContent = '100%';
      elements.orgProgressLabel.textContent = `Finished! Organized ${data.success_count} / ${data.total} movies.`;
      elements.btnStartOrganize.disabled = false;
      appendOrganizerLog(`\n[DONE] Complete! Successfully processed ${data.success_count}/${data.total} movies.\n`);
      showToast(`Organize complete: ${data.success_count} movies processed!`, 'success');

      renderMoviesGrid();
      try {
        localStorage.removeItem('r19dev_scan_cache');
      } catch (err) {}

      setTimeout(() => {
        elements.orgProgressBox.classList.add('hidden');
      }, 2000);
    } catch (err) {}
    es.close();
    state.orgEventSource = null;
    elements.btnStartOrganize.disabled = false;
  });

  es.addEventListener('error', () => {
    appendOrganizerLog(`[ERROR] Connection closed or error occurred.\n`);
    es.close();
    state.orgEventSource = null;
    elements.btnStartOrganize.disabled = false;
    elements.orgProgressBox.classList.add('hidden');
  });
}

export function startOrganize() {
  startOrganizeStream();
}

export function toggleAutoScroll() {
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
    elements.btnToggleAutoscroll?.classList.add('active');
    showToast('Auto-scroll resumed', 'info');
  } else {
    if (elements.logScrollBadge) {
      elements.logScrollBadge.textContent = 'Auto-Scroll: PAUSED';
      elements.logScrollBadge.classList.add('paused');
    }
    elements.btnToggleAutoscroll?.classList.remove('active');
    showToast('Auto-scroll paused', 'info');
  }
}

export async function copyOrganizerLog() {
  const text = elements.organizerLog?.textContent || '';
  if (!text.trim()) {
    showToast('No logs to copy', 'warning');
    return;
  }
  try {
    await navigator.clipboard.writeText(text);
    showToast('Copied console log to clipboard', 'success');
  } catch (err) {
    showToast('Failed to copy: ' + err.message, 'danger');
  }
}

export function clearOrganizerLog() {
  if (elements.organizerLog) elements.organizerLog.textContent = 'Console cleared.\n';
  if (elements.btnResumeScroll) elements.btnResumeScroll.classList.add('hidden');
}

export function resumeAutoScroll() {
  state.logAutoScroll = true;
  if (elements.organizerLog) {
    elements.organizerLog.scrollTop = elements.organizerLog.scrollHeight;
  }
  elements.btnResumeScroll?.classList.add('hidden');
  if (elements.logScrollBadge) {
    elements.logScrollBadge.textContent = 'Auto-Scroll: ON';
    elements.logScrollBadge.classList.remove('paused');
  }
  elements.btnToggleAutoscroll?.classList.add('active');
}
