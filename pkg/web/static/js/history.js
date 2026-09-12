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

export async function loadHistoryDetail(id, summary) {
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

export function closeHistoryModal() {
  elements.modalHistory?.classList.add('hidden');
}

export async function clearHistory() {
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
}

export async function copyHistoryLog() {
  const text = elements.historyLogView?.textContent || '';
  if (!text.trim()) return;
  try {
    await navigator.clipboard.writeText(text);
    showToast('📋 คัดลอกประวัติการทำงานเรียบร้อย', 'success');
  } catch (err) {
    showToast('ไม่สามารถคัดลอกได้: ' + err.message, 'danger');
  }
}
