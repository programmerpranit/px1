// web/src/main.js
import { $, S, api, applyKeyLabels } from './state.js';
import { measure, layout, render, initRenderer, updateEditorOptionControls } from './renderer.js';
import { initTabs, openFile, restoreWorkspaceTabs } from './tabs.js';
import { initCursor } from './cursor.js';
import { initSelectionBar } from './selbar.js';
import { initTree, revealFile, refreshTree, restoreOpenDirs } from './tree.js';
import { initSearch } from './search.js';
import { initScm, loadScmStatus } from './scm.js';
import { initPanels } from './panels.js';
import { initFind } from './find.js';
import { initPalette } from './palette.js';
import { initShortcuts } from './shortcuts.js';
import { initTheme } from './theme.js';
import { initDiff } from './diff.js';
import { initStatusFit, updateStatus } from './status.js';
import { initEdit } from './edit.js';

// Initialize all subsystems
initRenderer();
initTabs();
initCursor();
initSelectionBar();
initTree();
initSearch();
initScm();
initPanels();
initFind();
initPalette();
initShortcuts();
initDiff();
initStatusFit();
initEdit();

// Bootstrap application lifecycle
(async function boot() {
  try {
    initTheme();

    // Restore word wrap (default ON)
    const wrapPref = localStorage.getItem('px1.wrap');
    S.wrap = wrapPref !== null ? wrapPref === 'true' : true;
    document.body.classList.toggle('word-wrap', S.wrap);

    // Line numbers are always ON
    S.lineNumbers = true;
    document.body.classList.remove('hide-lines');

    updateEditorOptionControls();
  } catch {}

  applyKeyLabels();

  measure();
  S.meta = await api('/api/meta');
  if (S.meta.git) { const b = $('#btn-changed'); if (b) b.hidden = false; loadScmStatus(); }
  document.title = S.meta.name + ' - px1';
  $('#root-name').textContent = S.meta.name;
  $('#root-name').title = S.meta.root;
  if (S.meta.version) {
    const emptyVerEl = $('#empty-ver');
    if (emptyVerEl) emptyVerEl.textContent = 'v' + S.meta.version;
  }
  try {
    const savedDirs = JSON.parse(sessionStorage.getItem('px1.openDirs') || '[]');
    restoreOpenDirs(savedDirs);
  } catch {}
  await refreshTree();

  const params = new URLSearchParams(window.location.search);
  const initialPath = params.get('path');
  const initialLine = parseInt(params.get('line'), 10) || undefined;
  if (initialPath) {
    await openFile(initialPath, { line: initialLine });
    await revealFile(initialPath);
    try {
      const u = new URL(window.location.href);
      u.searchParams.delete('path');
      u.searchParams.delete('line');
      const cleanSearch = u.searchParams.toString();
      const cleanUrl = u.pathname + (cleanSearch ? '?' + cleanSearch : '') + u.hash;
      window.history.replaceState({}, '', cleanUrl);
    } catch {}
  } else {
    await restoreWorkspaceTabs();
  }

  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(() => { measure(); layout(); render(); });
  }

  // If the background indexer was still running when the UI loaded, poll briefly
  // until complete to update the total file count and index time in the status bar.
  if (S.meta && !S.meta.ready) {
    const timer = setInterval(async () => {
      try {
        const m = await api('/api/meta');
        if (m.ready) {
          clearInterval(timer);
          S.meta = m;
          updateStatus();
        }
      } catch {
        clearInterval(timer);
      }
    }, 150);
  }
})();
