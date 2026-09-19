// web/src/scm.js
// Source control panel: stage/unstage files and commit, VS Code-style.
import { $, esc, api, apiPost, apiPostJson } from './state.js';
import { layout, render } from './renderer.js';
import { openFile } from './tabs.js';
import { refreshTree } from './tree.js';
import { reloadOpenTabs } from './tabs.js';
import { showToast } from './ui.js';

const groupsEl = $('#scm-groups');
const messageEl = $('#scm-message');
const errEl = $('#scm-err');
const badgeEl = $('#scm-badge');
const syncEl = $('#scm-sync');
const syncTextEl = $('#scm-sync-text');
const pushBtn = $('#scm-push-btn');

const STATUS_LABEL = { A: 'A', M: 'M', D: 'D', R: 'R', C: 'C', '!': '!', U: 'U' };

function setErr(msg) {
  if (!errEl) return;
  errEl.textContent = msg || '';
  errEl.hidden = !msg;
}

export function isScmActive() {
  return $('#side-tab-scm')?.classList.contains('active');
}

export function showScm() {
  $('#side-tab-files')?.classList.remove('active');
  $('#side-tab-scm')?.classList.add('active');
  $('#tree').hidden = true;
  $('#scm').hidden = false;
  loadScmStatus();
}

export function showFiles() {
  $('#side-tab-scm')?.classList.remove('active');
  $('#side-tab-files')?.classList.add('active');
  $('#scm').hidden = true;
  $('#tree').hidden = false;
}

function fileRow(f, staged) {
  const code = staged ? f.staged : f.unstaged;
  const label = STATUS_LABEL[code] || code;
  const action = staged ? 'unstage' : 'stage';
  const actionLabel = staged ? 'Unstage' : 'Stage';
  const discardBtn = staged ? '' :
    '<button class="scm-action mini" data-scm-action="discard" data-path="' + esc(f.path) + '" title="Discard changes">&#8634;</button>';
  return '<div class="scm-row" data-path="' + esc(f.path) + '">' +
    '<span class="scm-status scm-' + esc(code) + '" title="' + esc(code) + '">' + esc(label) + '</span>' +
    '<span class="scm-path" title="' + esc(f.path) + '">' + esc(f.path) + '</span>' +
    discardBtn +
    '<button class="scm-action mini" data-scm-action="' + action + '" data-path="' + esc(f.path) + '" title="' + esc(actionLabel) + '">' +
    (staged ? '&#8722;' : '&#43;') + '</button></div>';
}

function groupHead(title, count, groupActions) {
  return '<div class="scm-group-head"><span>' + esc(title) + ' <span class="scm-count">' + count + '</span></span>' +
    '<span class="scm-group-actions">' + groupActions + '</span></div>';
}

export async function loadScmStatus() {
  if (!groupsEl) return;
  let j;
  try {
    j = await api('/api/git/status');
  } catch (e) {
    groupsEl.innerHTML = '<div class="hint">' + esc(e.message) + '</div>';
    return;
  }
  const files = j.files || [];
  if (!j.available) {
    groupsEl.innerHTML = '<div class="hint">Not a git repository.</div>';
    if (badgeEl) badgeEl.hidden = true;
    return;
  }
  const staged = files.filter(f => f.staged);
  const unstaged = files.filter(f => f.unstaged);

  if (badgeEl) {
    const n = files.length;
    badgeEl.textContent = String(n);
    badgeEl.hidden = n === 0;
  }

  if (!files.length) {
    groupsEl.innerHTML = '<div class="hint">No changes.</div>';
    await checkSyncStatus();
    return;
  }

  const stagedActions = staged.length ?
    '<button class="scm-group-action mini" data-scm-group-action="unstage-all" title="Unstage all">&#8722;</button>' : '';
  const unstagedActions = unstaged.length ?
    '<button class="scm-group-action mini" data-scm-group-action="discard-all" title="Discard all changes">&#8634;</button>' +
    '<button class="scm-group-action mini" data-scm-group-action="stage-all" title="Stage all">&#43;</button>' : '';

  let html = '';
  html += groupHead('Staged Changes', staged.length, stagedActions);
  html += staged.length ? staged.map(f => fileRow(f, true)).join('') : '<div class="hint">Nothing staged.</div>';
  html += groupHead('Changes', unstaged.length, unstagedActions);
  html += unstaged.length ? unstaged.map(f => fileRow(f, false)).join('') : '<div class="hint">No unstaged changes.</div>';
  groupsEl.innerHTML = html;

  await checkSyncStatus();
}

// Shown after a commit (or any panel refresh) when the branch has a remote
// tracking branch and has commits to push. Behind-only (nothing to push, e.g.
// someone else pushed) is surfaced as text with no action -- pulling isn't
// something this panel does.
async function checkSyncStatus() {
  if (!syncEl) return;
  let st;
  try {
    st = await api('/api/git/sync-status');
  } catch {
    syncEl.hidden = true;
    return;
  }
  if (!st.hasUpstream || (st.ahead === 0 && st.behind === 0)) {
    syncEl.hidden = true;
    return;
  }
  const parts = [];
  if (st.ahead > 0) parts.push('↑' + st.ahead);
  if (st.behind > 0) parts.push('↓' + st.behind);
  syncTextEl.textContent = parts.join(' ') + ' vs ' + st.upstream;
  if (pushBtn) pushBtn.hidden = st.ahead === 0;
  syncEl.hidden = false;
}

async function push() {
  if (!pushBtn) return;
  pushBtn.disabled = true;
  const label = pushBtn.textContent;
  pushBtn.textContent = 'Pushing…';
  try {
    await apiPost('/api/git/push');
    showToast('✓', 'Pushed');
    await checkSyncStatus();
  } catch (e) {
    showToast('!', 'Push failed: ' + e.message);
  } finally {
    pushBtn.disabled = false;
    pushBtn.textContent = label;
  }
}

async function stage(path) {
  try {
    await apiPost('/api/git/stage', { path });
    await loadScmStatus();
    await refreshTree();
  } catch (e) {
    showToast('!', 'Stage failed: ' + e.message);
  }
}

async function unstage(path) {
  try {
    await apiPost('/api/git/unstage', { path });
    await loadScmStatus();
    await refreshTree();
  } catch (e) {
    showToast('!', 'Unstage failed: ' + e.message);
  }
}

async function discard(path) {
  if (!confirm('Discard changes to ' + path + '? This cannot be undone.')) return;
  try {
    await apiPost('/api/git/discard', { path });
    await loadScmStatus();
    await refreshTree();
    await reloadOpenTabs();
  } catch (e) {
    showToast('!', 'Discard failed: ' + e.message);
  }
}

async function currentPaths(staged) {
  const j = await api('/api/git/status');
  return (j.files || []).filter(f => staged ? f.staged : f.unstaged).map(f => f.path);
}

// Bulk actions post the whole path list to one endpoint that runs a single
// git invocation server-side (git.go: gitStageAll/gitUnstageAll/gitDiscardAll).
// Firing one HTTP request per file here, each triggering its own `git
// add`/`reset`/`rm` subprocess, used to race on .git/index.lock -- git
// doesn't queue or retry for it, so most of a large batch silently failed.
async function stageAll() {
  const paths = await currentPaths(false);
  if (!paths.length) return;
  try {
    await apiPostJson('/api/git/stage-all', { paths });
  } catch (e) {
    showToast('!', 'Stage all failed: ' + e.message);
  }
  await loadScmStatus();
  await refreshTree();
}

async function unstageAll() {
  const paths = await currentPaths(true);
  if (!paths.length) return;
  try {
    await apiPostJson('/api/git/unstage-all', { paths });
  } catch (e) {
    showToast('!', 'Unstage all failed: ' + e.message);
  }
  await loadScmStatus();
  await refreshTree();
}

async function discardAll() {
  const paths = await currentPaths(false);
  if (!paths.length) return;
  if (!confirm('Discard changes to ' + paths.length + ' file' + (paths.length === 1 ? '' : 's') + '? This cannot be undone.')) return;
  try {
    await apiPostJson('/api/git/discard-all', { paths });
  } catch (e) {
    showToast('!', 'Discard all failed: ' + e.message);
  }
  await loadScmStatus();
  await refreshTree();
  await reloadOpenTabs();
}

async function generateMessage() {
  setErr('');
  const btn = $('#scm-generate');
  if (btn) { btn.disabled = true; btn.textContent = 'Generating…'; }
  try {
    const j = await apiPostJson('/api/git/commit-message', {});
    if (messageEl) messageEl.value = j.message || '';
  } catch (e) {
    setErr(e.message);
  } finally {
    if (btn) { btn.disabled = false; btn.textContent = 'Generate'; }
  }
}

async function commit() {
  setErr('');
  const message = (messageEl?.value || '').trim();
  if (!message) { setErr('Commit message is empty.'); return; }
  const btn = $('#scm-commit-btn');
  if (btn) btn.disabled = true;
  try {
    await apiPostJson('/api/git/commit', { message });
    if (messageEl) messageEl.value = '';
    showToast('✓', 'Committed');
    await loadScmStatus();
    await refreshTree();
    await reloadOpenTabs();
  } catch (e) {
    setErr(e.message);
  } finally {
    if (btn) btn.disabled = false;
  }
}

export function initScm() {
  if (!groupsEl) return;

  $('#side-tab-files')?.addEventListener('click', () => { showFiles(); layout(); render(); });
  $('#side-tab-scm')?.addEventListener('click', () => { showScm(); layout(); render(); });
  pushBtn?.addEventListener('click', push);

  groupsEl.addEventListener('click', e => {
    const groupBtn = e.target.closest('[data-scm-group-action]');
    if (groupBtn) {
      const act = groupBtn.dataset.scmGroupAction;
      if (act === 'stage-all') stageAll();
      else if (act === 'unstage-all') unstageAll();
      else if (act === 'discard-all') discardAll();
      return;
    }
    const btn = e.target.closest('[data-scm-action]');
    if (btn) {
      const path = btn.dataset.path;
      const act = btn.dataset.scmAction;
      if (act === 'stage') stage(path);
      else if (act === 'unstage') unstage(path);
      else if (act === 'discard') discard(path);
      return;
    }
    const row = e.target.closest('.scm-row');
    if (row) openFile(row.dataset.path);
  });

  $('#scm-generate')?.addEventListener('click', generateMessage);
  $('#scm-commit-btn')?.addEventListener('click', commit);
  messageEl?.addEventListener('keydown', e => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') { e.preventDefault(); commit(); }
  });
}
