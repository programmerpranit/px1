// web/src/edit.js
// Direct in-place editing: click into an eligible file and type -- no
// separate mode. The first keystroke on a tab fetches the full raw text
// (/api/raw) into d.buf, a plain-text line array that becomes authoritative
// for that tab from then on. renderer.js's paint() sources row bodies from
// d.buf when it is set; everything else (virtualization, gutter, caret,
// selection, agent-range overlay) is unchanged, because all of it already
// reads the *rendered* text, not d.lines directly.
//
// There is no client-side tokenizer, so syntax highlighting during typing is
// "eventually correct": every edit shows plain escaped text immediately, and
// a debounced call to /api/highlight (which never touches disk) re-colors
// the buffer shortly after typing pauses.
import { $, S, doc_, apiPostJson } from './state.js';
import { vp, showToast } from './ui.js';
import { render, layout } from './renderer.js';
import { lineText, updateDomSelection, WORD } from './cursor.js';
import { updateStatus } from './status.js';
import { clearFind } from './find.js';
import { drawTabs } from './tabs.js';

// Mirrors the server's caps (fileops.go: maxEditBytes, maxLiveHighlightBytes).
const MAX_EDIT_BYTES = 2 * 1024 * 1024;
const MAX_LIVE_HIGHLIGHT_BYTES = 256 * 1024;
const UNDO_LIMIT = 100;
const BURST_MS = 500;

export function editAvailable(d) {
  return !!d && !d.isImage && typeof d.size === 'number' && d.size <= MAX_EDIT_BYTES;
}

export function isDirty(d) {
  return !!(d && d.buf && d.buf.dirty);
}

function bufferText(d) {
  return d.buf.lines.join('\n') + (d.buf.trailingNL ? '\n' : '');
}

function colOf(d) {
  return d.col === Infinity ? lineText(d, d.cur).length : (d.col || 0);
}

/* Fetches the file's raw text once per tab and turns it into the plain-text
   line buffer every edit primitive below operates on. Concurrent callers
   (fast typing before the first fetch lands) share one in-flight promise. */
async function ensureBuffer(d) {
  if (d.buf) return d.buf;
  if (!d._bufPromise) {
    d._bufPromise = (async () => {
      try {
        const r = await fetch('/api/raw?path=' + encodeURIComponent(d.path));
        if (!r.ok) throw new Error('failed to load file (status ' + r.status + ')');
        const text = await r.text();
        const trailingNL = text.endsWith('\n');
        const lines = (trailingNL ? text.slice(0, -1) : text).split('\n');
        d.buf = {
          lines, trailingNL, highlighted: [], dirty: false,
          baseMtime: d.mtime || 0, baseSize: typeof d.size === 'number' ? d.size : 0,
          gen: 0, undo: [], redo: [], lastKind: '', lastEditAt: 0,
        };
        d.total = lines.length;
        // Line numbers a splice can invalidate; the buffer is now the
        // authority they'd otherwise be decorating against.
        S.occ = null;
        clearFind();
        scheduleHighlight(d, true);
      } catch (e) {
        showToast('!', 'Could not start editing: ' + e.message);
      } finally {
        d._bufPromise = null;
      }
      return d.buf;
    })();
  }
  return d._bufPromise;
}

function snapshot(d) {
  return { lines: d.buf.lines.slice(), cur: d.cur, col: colOf(d) };
}

/* Groups consecutive plain-character inserts into one undo step (typing a
   whole word undoes at once); every other kind of edit -- newline, backspace
   across a line boundary, paste, a selection replace -- gets its own. */
function beginEdit(d, kind) {
  const now = Date.now();
  const sameBurst = kind === 'char' && d.buf.lastKind === 'char' && (now - d.buf.lastEditAt) < BURST_MS;
  if (!sameBurst) {
    d.buf.undo.push(snapshot(d));
    if (d.buf.undo.length > UNDO_LIMIT) d.buf.undo.shift();
    d.buf.redo = [];
  }
  d.buf.lastKind = kind;
  d.buf.lastEditAt = now;
}

function afterEdit(d, structural) {
  d.buf.dirty = true;
  d.buf.gen++;
  // The tokenized HTML from the last /api/highlight response no longer
  // matches d.buf.lines -- without this, paint() keeps showing each edited
  // row's pre-edit highlighted markup (paint() prefers it over plain
  // escaped text) until the 300ms debounce below lands, so edits visibly
  // lag behind the caret until typing pauses.
  d.buf.highlighted = [];
  // Ask the next paint to scroll the caret into view once the DOM actually
  // reflects this edit; see paint() in renderer.js.
  d.buf.revealPending = true;
  d.selAnchor = null;
  d.maxCols = Math.max(d.maxCols || 0, lineText(d, d.cur).length);
  drawTabs();
  if (structural) layout();
  render();
  updateDomSelection();
  updateStatus();
  scheduleHighlight(d);
}

function selRange(d) {
  if (!d.selAnchor) return null;
  const a = d.selAnchor, b = { line: d.cur, col: colOf(d) };
  if (a.line === b.line && a.col === b.col) return null;
  return (a.line < b.line || (a.line === b.line && a.col < b.col)) ? { start: a, end: b } : { start: b, end: a };
}

function deleteSelectionRange(d, { start, end }) {
  if (start.line === end.line) {
    const line = d.buf.lines[start.line - 1];
    d.buf.lines[start.line - 1] = line.slice(0, start.col) + line.slice(end.col);
  } else {
    const first = d.buf.lines[start.line - 1].slice(0, start.col);
    const last = d.buf.lines[end.line - 1].slice(end.col);
    d.buf.lines.splice(start.line - 1, end.line - start.line + 1, first + last);
  }
  d.cur = start.line;
  d.col = start.col;
  d.selAnchor = null;
  d.total = d.buf.lines.length;
}

/* Splices s (one character, or a multi-line paste) in at the caret. Returns
   whether the line count changed, so the caller knows whether layout() (not
   just render()) is needed. */
function applyInsertAt(d, s) {
  const col = colOf(d);
  const parts = s.split('\n');
  const line = d.buf.lines[d.cur - 1];
  const before = line.slice(0, col), after = line.slice(col);
  if (parts.length === 1) {
    d.buf.lines[d.cur - 1] = before + parts[0] + after;
    d.col = col + parts[0].length;
    return false;
  }
  const newLines = [before + parts[0], ...parts.slice(1, -1), parts[parts.length - 1] + after];
  d.buf.lines.splice(d.cur - 1, 1, ...newLines);
  d.cur += parts.length - 1;
  d.col = parts[parts.length - 1].length;
  d.total = d.buf.lines.length;
  return true;
}

export async function insertText(d, s) {
  if (!(await ensureBuffer(d))) return;
  const range = selRange(d);
  const kind = (!range && s.length === 1 && s !== '\n') ? 'char' : 'other';
  beginEdit(d, kind);
  if (range) deleteSelectionRange(d, range);
  const structural = applyInsertAt(d, s);
  afterEdit(d, structural || !!range);
}

export function insertNewline(d) { return insertText(d, '\n'); }
export function insertTab(d) { return insertText(d, '\t'); }

export async function backspace(d) {
  if (!(await ensureBuffer(d))) return;
  const range = selRange(d);
  if (range) {
    beginEdit(d, 'other');
    deleteSelectionRange(d, range);
    afterEdit(d, true);
    return;
  }
  const col = colOf(d);
  if (col > 0) {
    beginEdit(d, 'char-del');
    const line = d.buf.lines[d.cur - 1];
    d.buf.lines[d.cur - 1] = line.slice(0, col - 1) + line.slice(col);
    d.col = col - 1;
    afterEdit(d, false);
  } else if (d.cur > 1) {
    beginEdit(d, 'other');
    const prevLen = d.buf.lines[d.cur - 2].length;
    d.buf.lines[d.cur - 2] += d.buf.lines[d.cur - 1];
    d.buf.lines.splice(d.cur - 1, 1);
    d.cur -= 1;
    d.col = prevLen;
    d.total = d.buf.lines.length;
    afterEdit(d, true);
  }
}

export async function deleteForward(d) {
  if (!(await ensureBuffer(d))) return;
  const range = selRange(d);
  if (range) {
    beginEdit(d, 'other');
    deleteSelectionRange(d, range);
    afterEdit(d, true);
    return;
  }
  const col = colOf(d);
  const line = d.buf.lines[d.cur - 1];
  if (col < line.length) {
    beginEdit(d, 'char-del');
    d.buf.lines[d.cur - 1] = line.slice(0, col) + line.slice(col + 1);
    afterEdit(d, false);
  } else if (d.cur < d.total) {
    beginEdit(d, 'other');
    d.buf.lines[d.cur - 1] = line + d.buf.lines[d.cur];
    d.buf.lines.splice(d.cur, 1);
    d.total = d.buf.lines.length;
    afterEdit(d, true);
  }
}

/* Word boundary scan, identical to moveWord's in cursor.js so Mod+Backspace
   deletes exactly what Alt+Left would have jumped over. */
function wordStart(text, col) {
  let c = col - 1;
  while (c > 0 && /\s/.test(text[c])) c--;
  if (WORD.test(text[c])) { while (c > 0 && WORD.test(text[c - 1])) c--; }
  else { while (c > 0 && !WORD.test(text[c - 1]) && !/\s/.test(text[c - 1])) c--; }
  return Math.max(0, c);
}

function wordEnd(text, col) {
  const len = text.length;
  let c = col;
  if (WORD.test(text[c])) { while (c < len && WORD.test(text[c])) c++; }
  else if (!/\s/.test(text[c])) { while (c < len && !WORD.test(text[c]) && !/\s/.test(text[c])) c++; }
  while (c < len && /\s/.test(text[c])) c++;
  return c;
}

export async function deleteWordBackward(d) {
  if (!(await ensureBuffer(d))) return;
  const range = selRange(d);
  if (range) { beginEdit(d, 'other'); deleteSelectionRange(d, range); afterEdit(d, true); return; }
  const col = colOf(d);
  if (col === 0) return backspace(d); // merge with previous line, same as a plain backspace there
  const text = d.buf.lines[d.cur - 1];
  const start = wordStart(text, col);
  beginEdit(d, 'other');
  d.buf.lines[d.cur - 1] = text.slice(0, start) + text.slice(col);
  d.col = start;
  afterEdit(d, false);
}

export async function deleteWordForward(d) {
  if (!(await ensureBuffer(d))) return;
  const range = selRange(d);
  if (range) { beginEdit(d, 'other'); deleteSelectionRange(d, range); afterEdit(d, true); return; }
  const col = colOf(d);
  const text = d.buf.lines[d.cur - 1];
  if (col >= text.length) return deleteForward(d); // merge with next line, same as a plain delete there
  const end = wordEnd(text, col);
  beginEdit(d, 'other');
  d.buf.lines[d.cur - 1] = text.slice(0, col) + text.slice(end);
  d.col = col;
  afterEdit(d, false);
}

/* ---------- line comment toggle (Mod+/) ---------- */

// Chroma lexer name (d.lang, as reported by /api/file and /api/highlight) ->
// line comment token. Unlisted languages report via showToast rather than
// guessing a wrong prefix.
const LINE_COMMENT = {
  go: '//', c: '//', 'c++': '//', 'c#': '//', java: '//', javascript: '//', jsx: '//',
  typescript: '//', tsx: '//', rust: '//', swift: '//', kotlin: '//', scala: '//',
  php: '//', dart: '//', groovy: '//', 'objective-c': '//', 'objective-c++': '//', zig: '//',
  sass: '//', scss: '//', less: '//',
  python: '#', ruby: '#', perl: '#', bash: '#', shell: '#', 'shell session': '#',
  makefile: '#', dockerfile: '#', yaml: '#', toml: '#', r: '#', elixir: '#',
  julia: '#', nim: '#', crystal: '#', powershell: '#', tcl: '#', ini: '#', properties: '#',
  lua: '--', haskell: '--', sql: '--', applescript: '--', ada: '--', vhdl: '--',
  clojure: ';', 'common lisp': ';', scheme: ';', racket: ';', 'emacs lisp': ';',
  'vim script': '"', matlab: '%', erlang: '%', latex: '%', tex: '%',
  fortran: '!', batchfile: 'REM',
};

export async function toggleLineComment(d) {
  if (!(await ensureBuffer(d))) return;
  const prefix = LINE_COMMENT[(d.lang || '').toLowerCase()];
  if (!prefix) { showToast('!', 'No comment syntax known for ' + (d.lang || 'this file')); return; }

  const range = selRange(d);
  let startLine = d.cur, endLine = d.cur;
  if (range) {
    startLine = range.start.line;
    endLine = (range.end.col === 0 && range.end.line > range.start.line) ? range.end.line - 1 : range.end.line;
  }

  const pat = new RegExp('^(\\s*)' + prefix.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ' ?');
  let allCommented = true;
  for (let l = startLine; l <= endLine; l++) {
    const line = d.buf.lines[l - 1];
    if (line.trim() === '') continue;
    if (!pat.test(line)) { allCommented = false; break; }
  }

  beginEdit(d, 'other');
  for (let l = startLine; l <= endLine; l++) {
    const line = d.buf.lines[l - 1];
    if (allCommented) {
      d.buf.lines[l - 1] = line.replace(pat, '$1');
    } else if (line.trim() !== '') {
      const indent = line.match(/^\s*/)[0];
      d.buf.lines[l - 1] = indent + prefix + ' ' + line.slice(indent.length);
    }
  }
  afterEdit(d, true);
}

export function undo(d) {
  if (!d.buf || !d.buf.undo.length) return;
  d.buf.redo.push(snapshot(d));
  const snap = d.buf.undo.pop();
  d.buf.lines = snap.lines;
  d.cur = snap.cur;
  d.col = snap.col;
  d.selAnchor = null;
  d.total = d.buf.lines.length;
  d.buf.dirty = true;
  d.buf.gen++;
  d.buf.highlighted = [];
  d.buf.lastKind = '';
  drawTabs();
  layout();
  render();
  updateDomSelection();
  updateStatus();
  scheduleHighlight(d);
}

export function redo(d) {
  if (!d.buf || !d.buf.redo.length) return;
  d.buf.undo.push(snapshot(d));
  const snap = d.buf.redo.pop();
  d.buf.lines = snap.lines;
  d.cur = snap.cur;
  d.col = snap.col;
  d.selAnchor = null;
  d.total = d.buf.lines.length;
  d.buf.dirty = true;
  d.buf.gen++;
  d.buf.highlighted = [];
  d.buf.lastKind = '';
  drawTabs();
  layout();
  render();
  updateDomSelection();
  updateStatus();
  scheduleHighlight(d);
}

/* ---------- debounced re-highlight: never touches disk, purely a POST of
   the in-memory buffer to /api/highlight for fresh tokenised HTML. ---------- */

const highlightTimers = new WeakMap();

function scheduleHighlight(d, immediate = false) {
  clearTimeout(highlightTimers.get(d));
  const fire = async () => {
    if (!d.buf) return;
    const content = bufferText(d);
    if (content.length > MAX_LIVE_HIGHLIGHT_BYTES) return; // stays plain-escaped; re-highlighted properly on save
    const gen = d.buf.gen;
    let j;
    try {
      j = await apiPostJson('/api/highlight', { path: d.path, content });
    } catch {
      return;
    }
    if (!d.buf || d.buf.gen !== gen) return; // superseded by further edits
    d.buf.highlighted = j.lines;
    d.maxCols = Math.max(d.maxCols || 0, j.maxCols || 0);
    render();
  };
  if (immediate) fire();
  else highlightTimers.set(d, setTimeout(fire, 300));
}

/* ---------- save + conflict banner ---------- */

const editBanner = $('#edit-banner');

function hideConflictBanner() {
  if (editBanner) editBanner.hidden = true;
}

function showConflict(d, body) {
  if (!editBanner) return;
  editBanner.hidden = false;
  editBanner.replaceChildren();
  const msg = document.createElement('span');
  msg.className = 'edit-banner-msg';
  msg.textContent = d.path + ' changed on disk since you started editing.';
  const overwrite = document.createElement('button');
  overwrite.className = 'edit-banner-btn';
  overwrite.textContent = 'Overwrite anyway';
  overwrite.addEventListener('click', () => {
    d.buf.baseMtime = body.mtime;
    d.buf.baseSize = body.size;
    hideConflictBanner();
    saveBuffer(d);
  });
  const discard = document.createElement('button');
  discard.className = 'edit-banner-btn';
  discard.textContent = 'Discard my edits';
  discard.addEventListener('click', () => {
    const trailingNL = body.content.endsWith('\n');
    d.buf.lines = (trailingNL ? body.content.slice(0, -1) : body.content).split('\n');
    d.buf.trailingNL = trailingNL;
    d.buf.highlighted = [];
    d.buf.baseMtime = body.mtime;
    d.buf.baseSize = body.size;
    d.buf.dirty = false;
    d.buf.undo = [];
    d.buf.redo = [];
    d.total = d.buf.lines.length;
    d.cur = Math.min(d.cur, d.total);
    d.selAnchor = null;
    hideConflictBanner();
    drawTabs();
    layout();
    render();
    updateStatus();
    scheduleHighlight(d, true);
  });
  editBanner.append(msg, overwrite, discard);
}

export async function saveBuffer(d) {
  if (!d.buf) return;
  const content = bufferText(d);
  try {
    const j = await apiPostJson('/api/file/save', {
      path: d.path, content, mtime: d.buf.baseMtime, size: d.buf.baseSize,
    });
    d.buf.baseMtime = j.mtime;
    d.buf.baseSize = j.size;
    d.mtime = j.mtime;
    d.size = j.size;
    d.buf.dirty = false;
    hideConflictBanner();
    showToast('✓', 'Saved');
    drawTabs();
  } catch (e) {
    if (e.body && typeof e.body.content === 'string') showConflict(d, e.body);
    else showToast('!', 'Save failed: ' + e.message);
  }
}

/* ---------- paste / cut: not routed through keydown, since there is no
   contenteditable surface for the browser to deliver a native edit to. ---------- */

export function initEdit() {
  vp.addEventListener('paste', e => {
    const d = doc_();
    if (!d || !editAvailable(d)) return;
    const text = e.clipboardData?.getData('text/plain');
    if (!text) return;
    e.preventDefault();
    insertText(d, text);
  });

  vp.addEventListener('cut', async e => {
    const d = doc_();
    if (!d || !editAvailable(d)) return;
    const range = selRange(d);
    if (!range) return;
    // Default cut behavior is left alone so the browser copies the live
    // Selection (kept correct by updateDomSelection()) to the clipboard;
    // the view was never contenteditable, so the browser won't remove the
    // text on its own -- that part is done here, against the buffer.
    if (!(await ensureBuffer(d))) return;
    beginEdit(d, 'other');
    deleteSelectionRange(d, range);
    afterEdit(d, true);
  });
}
