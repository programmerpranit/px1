// web/src/tree.js
import { $, $$, esc, api, S, apiPostJson } from './state.js';
import { openFile } from './tabs.js';
import { showToast } from './ui.js';
import { reloadWorkspace } from './agent.js';

export const treeEl = $('#tree');
export const openDirs = new Set();

/* git status letter -> CSS class + label. Empty/absent = clean, no badge. */
const GIT_STATUS = {
  M: ['git-M', 'modified'], A: ['git-A', 'added'], D: ['git-D', 'deleted'],
  U: ['git-untracked', 'untracked'], R: ['git-R', 'renamed'],
  C: ['git-A', 'copied'], '!': ['git-M', 'unmerged'],
};

export async function drawTree(dir, container, depth) {
  let j;
  try { j = await api('/api/tree', { dir }); } catch { return; }
  container.innerHTML = j.children.map(c => {
    const pad = 8 + depth * 12;
    // Ignored by .gitignore: still browsable, dimmed, and absent from search.
    const ig = c.ignored ? ' ignored' : '';
    const note = c.ignored ? ' (ignored by .gitignore, not searched)' : '';
    if (c.dir) {
      const dc = c.dirty ? ' dirty' : ''; // backend marks any ancestor of a change
      return '<div class="tw"><div class="tr dir' + ig + dc + '" data-dir="' + esc(c.path) + '" style="padding-left:' + pad + 'px" title="Folder: ' + esc(c.path) + note + '">' +
        '<span class="ar"></span><span class="nm">' + esc(c.name) + '</span></div>' +
        '<div class="kids" data-kids="' + esc(c.path) + '"></div></div>';
    }
    const g = GIT_STATUS[c.status];
    const gc = g ? ' dirty ' + g[0] : '';
    const badge = g ? '<span class="gs" title="git: ' + g[1] + '">' + esc(c.status) + '</span>' : '';
    return '<div class="tr file' + ig + gc + '" data-file="' + esc(c.path) + '" style="padding-left:' + (pad + 12) + 'px" title="Open ' + esc(c.path) + note + '">' +
      '<span class="ic" data-t="' + fileKind(c.name) + '"></span><span class="nm">' + esc(c.name) + '</span>' + badge + '</div>';
  }).join('');
}

/* A colour family per file kind, drawn in CSS. Emoji or icon fonts would be at
   the mercy of whatever the viewer has installed. */
export const FILE_KIND = {
  go: 'code', js: 'code', mjs: 'code', cjs: 'code', ts: 'code', tsx: 'code', jsx: 'code',
  py: 'code', rb: 'code', rs: 'code', java: 'code', kt: 'code', c: 'code', h: 'code',
  cc: 'code', cpp: 'code', hpp: 'code', cs: 'code', php: 'code', swift: 'code',
  lua: 'code', ex: 'code', exs: 'code', scala: 'code', dart: 'code', sh: 'code',
  bash: 'code', zsh: 'code', sql: 'code',
  json: 'data', yaml: 'data', yml: 'data', toml: 'data', ini: 'data', xml: 'data',
  csv: 'data', env: 'data', lock: 'data', mod: 'data', sum: 'data',
  md: 'doc', markdown: 'doc', txt: 'doc', rst: 'doc', adoc: 'doc',
  html: 'web', htm: 'web', css: 'web', scss: 'web', less: 'web', svg: 'web', vue: 'web',
  png: 'img', jpg: 'img', jpeg: 'img', gif: 'img', webp: 'img', ico: 'img', avif: 'img',
};

export function fileKind(name) {
  const i = name.lastIndexOf('.');
  return (i > 0 && FILE_KIND[name.slice(i + 1).toLowerCase()]) || 'other';
}

export async function refreshTree() {
  await drawTree('', treeEl, 0);
  const dirs = Array.from(openDirs).sort((a, b) => a.split('/').length - b.split('/').length);
  for (const path of dirs) {
    const dirRow = treeEl.querySelector('[data-dir="' + CSS.escape(path) + '"]');
    const kids = treeEl.querySelector('[data-kids="' + CSS.escape(path) + '"]');
    if (kids && dirRow) {
      dirRow.classList.add('open');
      kids.classList.add('open');
      kids.dataset.loaded = '1';
      await drawTree(path, kids, path.split('/').length);
    } else {
      openDirs.delete(path);
    }
  }
}

export function restoreOpenDirs(dirs) {
  if (Array.isArray(dirs)) {
    for (const d of dirs) {
      if (typeof d === 'string') openDirs.add(d);
    }
  }
}

/* Expand the tree down to dir and scroll it into view. */
export async function revealDir(dir) {
  const parts = dir.split('/');
  for (let i = 0; i < parts.length; i++) {
    const p = parts.slice(0, i + 1).join('/');
    const row = treeEl.querySelector('[data-dir="' + CSS.escape(p) + '"]');
    if (!row) break;
    if (!row.classList.contains('open')) {
      row.classList.add('open');
      const kids = treeEl.querySelector('[data-kids="' + CSS.escape(p) + '"]');
      if (kids) {
        kids.classList.add('open');
        openDirs.add(p);
        kids.dataset.loaded = '1';
        await drawTree(p, kids, p.split('/').length);
      }
    }
  }
  const last = treeEl.querySelector('[data-dir="' + CSS.escape(dir) + '"]');
  if (last) last.scrollIntoView({ block: 'center' });
  try {
    sessionStorage.setItem('px0.openDirs', JSON.stringify(Array.from(openDirs)));
  } catch {}
}

export async function revealFile(path) {
  const idx = path.lastIndexOf('/');
  if (idx > 0) await revealDir(path.slice(0, idx));
  const row = treeEl.querySelector('[data-file="' + CSS.escape(path) + '"]');
  if (row) {
    $$('.tr.sel', treeEl).forEach(x => x.classList.remove('sel'));
    row.classList.add('sel');
    row.scrollIntoView({ block: 'center' });
  }
}

/* ---------- rename: right-click a row for a one-item menu, or Enter/Esc an
   inline text field swapped in for the row's name. Move is just a rename to
   a path in a different directory, typed the same way. ---------- */

const treeMenu = $('#tree-menu');
let menuTarget = null; // the .tr row the open menu applies to

function closeTreeMenu() {
  if (treeMenu && !treeMenu.hidden) treeMenu.hidden = true;
  menuTarget = null;
}

function openTreeMenu(row, x, y) {
  if (!treeMenu) return;
  menuTarget = row;
  treeMenu.replaceChildren();
  const btn = document.createElement('button');
  btn.className = 'sel-menu-item';
  btn.setAttribute('role', 'menuitem');
  btn.textContent = 'Rename';
  btn.addEventListener('click', () => { closeTreeMenu(); startRename(row); });
  treeMenu.append(btn);
  treeMenu.hidden = false;
  const w = treeMenu.offsetWidth, h = treeMenu.offsetHeight;
  treeMenu.style.left = Math.max(4, x + w > innerWidth - 4 ? x - w : x) + 'px';
  treeMenu.style.top = Math.max(4, y + h > innerHeight - 4 ? y - h : y) + 'px';
}

function rowPath(row) {
  return row.dataset.file ?? row.dataset.dir;
}

function startRename(row) {
  const path = rowPath(row);
  if (!path) return;
  const nameEl = row.querySelector('.nm');
  if (!nameEl) return;
  const oldName = nameEl.textContent;
  const input = document.createElement('input');
  input.className = 'tr-rename-input';
  input.setAttribute('aria-label', 'Rename ' + path);
  input.value = oldName;
  nameEl.replaceWith(input);
  input.focus();
  input.setSelectionRange(0, input.value.lastIndexOf('.') > 0 ? input.value.lastIndexOf('.') : input.value.length);

  let done = false;
  const finish = async commit => {
    if (done) return;
    done = true;
    const newName = input.value.trim();
    if (!commit || !newName || newName === oldName) {
      input.replaceWith(nameEl);
      return;
    }
    const slash = path.lastIndexOf('/');
    const newPath = (slash < 0 ? '' : path.slice(0, slash + 1)) + newName;
    try {
      await apiPostJson('/api/file/rename', { path, newPath });
      const t = S.tabs.find(t => t.path === path);
      if (t) { t.path = newPath; t.name = newName; }
      showToast('✓', 'Renamed');
      await reloadWorkspace();
    } catch (e) {
      showToast('!', 'Rename failed: ' + e.message);
      nameEl.textContent = oldName;
      input.replaceWith(nameEl);
    }
  };
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter') { e.preventDefault(); finish(true); }
    else if (e.key === 'Escape') { e.preventDefault(); finish(false); }
    e.stopPropagation();
  });
  input.addEventListener('blur', () => finish(true));
  input.addEventListener('click', e => e.stopPropagation());
}

export function initTree() {
  // "Changed only" filter: hide clean files and known-clean folders (CSS-driven).
  $('#btn-changed')?.addEventListener('click', e => {
    const on = treeEl.classList.toggle('changed-only');
    e.currentTarget.classList.toggle('active', on);
  });

  treeEl.addEventListener('click', async e => {
    const dirRow = e.target.closest('[data-dir]');
    if (dirRow) {
      const path = dirRow.dataset.dir;
      const kids = treeEl.querySelector('[data-kids="' + CSS.escape(path) + '"]');
      const open = dirRow.classList.toggle('open');
      kids.classList.toggle('open', open);
      if (open) {
        openDirs.add(path);
        kids.dataset.loaded = '1';
        await drawTree(path, kids, path.split('/').length);
      } else openDirs.delete(path);
      try {
        sessionStorage.setItem('px0.openDirs', JSON.stringify(Array.from(openDirs)));
      } catch {}
      return;
    }
    const f = e.target.closest('[data-file]');
    if (f) {
      $$('.tr.sel', treeEl).forEach(x => x.classList.remove('sel'));
      f.classList.add('sel');
      openFile(f.dataset.file);
    }
  });

  treeEl.addEventListener('contextmenu', e => {
    const row = e.target.closest('.tr');
    if (!row) return;
    e.preventDefault();
    openTreeMenu(row, e.clientX, e.clientY);
  });
  if (treeMenu) {
    treeMenu.addEventListener('mousedown', e => e.preventDefault());
    document.addEventListener('mousedown', e => {
      if (!treeMenu.hidden && !e.target.closest('#tree-menu')) closeTreeMenu();
    });
    addEventListener('keydown', e => { if (e.key === 'Escape') closeTreeMenu(); });
  }
}
