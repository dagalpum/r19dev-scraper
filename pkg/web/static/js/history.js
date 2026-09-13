/**
 * R19DEV Studio - Operation History Modal & Audit Log
 * Native ES Module
 */

import { elements, escapeHtml, showToast } from './state.js';

export function setupHistoryModal() {
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

  elements.btnClearHistory?.addEventListener('click', clearHistory);
  elements.btnCopyHistoryLog?.addEventListener('click', copyHistoryLog);
}

export async function openHistoryModal() {
  elements.modalHistory?.classList.remove('hidden');
  elements.historyListContainer.innerHTML = '<div class="history-empty">Loading history...</div>';
  elements.historyLogView.textContent = 'Select an operation from the list to view audit logs';
  elements.historyDetailTitle.textContent = 'Operation Details';
  elements.btnCopyHistoryLog.classList.add('hidden');

  try {
    const res = await fetch('/api/history');
    const data = await res.json();
    const history = data.history || [];

    if (history.length === 0) {
      elements.historyListContainer.innerHTML = '<div class="history-empty">No recorded operations in history</div>';
      return;
    }

    elements.historyListContainer.innerHTML = '';
    history.forEach((rec, idx) => {
      const item = document.createElement('div');
      item.className = 'history-item';
      if (idx === 0) item.classList.add('active');

      const dateStr = new Date(rec.created_at).toLocaleString();
      const opIcon = rec.operation === 'organize'
        ? '<span class="material-symbols-outlined icon" style="font-size: 1.1rem; vertical-align: middle;">folder</span>'
        : '<span class="material-symbols-outlined icon" style="font-size: 1.1rem; vertical-align: middle;">bolt</span>';
      const opName = rec.operation === 'organize' ? 'Organize' : 'Scrape';

      item.innerHTML = `
        <div class="history-item-top">
          <span class="history-item-op">${opIcon} ${opName}</span>
          <span class="history-item-time">${dateStr}</span>
        </div>
        <div class="history-item-stats">
          <span class="history-badge history-badge-success"><span class="material-symbols-outlined icon" style="font-size: 0.8rem; vertical-align: -1px;">check_circle</span> ${rec.success_count}</span>
          ${rec.fail_count > 0 ? `<span class="history-badge history-badge-fail"><span class="material-symbols-outlined icon" style="font-size: 0.8rem; vertical-align: -1px;">cancel</span> ${rec.fail_count}</span>` : ''}
          <span style="color: var(--text-muted); font-size: 0.72rem;">Total: ${rec.total_items}</span>
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
    elements.historyListContainer.innerHTML = `<div class="history-empty">Failed to load history: ${escapeHtml(err.message)}</div>`;
  }
}

export async function loadHistoryDetail(id, summary) {
  elements.historyLogView.textContent = 'Loading log details from database...';
  const dateStr = new Date(summary.created_at).toLocaleString();
  elements.historyDetailTitle.textContent = `${summary.operation.toUpperCase()} (${summary.success_count} Succeeded, ${summary.fail_count} Failed) - ${dateStr}`;
  elements.btnCopyHistoryLog.classList.remove('hidden');

  try {
    const res = await fetch(`/api/history/detail?id=${id}`);
    const data = await res.json();
    elements.historyLogView.textContent = data.log_text || '(No log messages recorded for this operation)';
  } catch (err) {
    elements.historyLogView.textContent = `Unable to load log: ${err.message}`;
  }
}

export function closeHistoryModal() {
  elements.modalHistory?.classList.add('hidden');
}

export async function clearHistory() {
  if (!confirm('Are you sure you want to clear all operation history?')) return;
  try {
    const res = await fetch('/api/history', { method: 'DELETE' });
    if (res.ok) {
      showToast('History cleared successfully', 'info');
      openHistoryModal();
    }
  } catch (err) {
    showToast('Failed to clear history: ' + err.message, 'danger');
  }
}

export async function copyHistoryLog() {
  const text = elements.historyLogView?.textContent || '';
  if (!text.trim()) return;
  try {
    await navigator.clipboard.writeText(text);
    showToast('Log copied to clipboard', 'success');
  } catch (err) {
    showToast('Failed to copy log: ' + err.message, 'danger');
  }
}
