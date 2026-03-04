/**
 * app.js — entry point.
 *
 * Features wired here:
 *   - Layout skeleton built entirely from JS
 *   - Sidebar ↔ form-builder ↔ runner pipeline
 *   - URL state: ?tool=name&arg=val — bookmarkable, shareable, auto-runs on load
 *   - Result history: in-app back/forward, up to HISTORY_LIMIT entries
 *   - Result toolbar: copy raw text + back/forward navigation
 */

import { fetchCatalogue } from './catalogue.js';
import { buildSidebar }   from './sidebar.js';
import { buildForm }      from './form-builder.js';
import { runTool }        from './runner.js';
import { renderResult }   from './renderer.js';

const HISTORY_LIMIT = 50;

async function init() {
  const app = document.getElementById('app');

  // ── Layout skeleton ─────────────────────────────────────────────────────────
  const sidebar       = mk('aside', 'elko-sidebar');
  const workspace     = mk('main',  'elko-workspace');
  const toolPanel     = mk('div',   'elko-tool-panel');
  const resultWrapper = mk('div',   'elko-result-wrapper');
  const toolbar       = mk('div',   'elko-result-toolbar');
  const resultPanel   = mk('div',   'elko-result-panel');

  const sidebarHeader = mk('div', 'elko-sidebar-header');
  sidebarHeader.textContent = 'elko market';
  sidebar.appendChild(sidebarHeader);

  resultWrapper.appendChild(toolbar);
  resultWrapper.appendChild(resultPanel);
  workspace.appendChild(toolPanel);
  workspace.appendChild(resultWrapper);
  app.appendChild(sidebar);
  app.appendChild(workspace);

  const welcome = mk('div', 'elko-welcome');
  welcome.textContent = 'Select a tool from the sidebar.';
  toolPanel.appendChild(welcome);

  // ── History state ────────────────────────────────────────────────────────────
  // Each entry: { tool, args, format, text }
  const hist = { entries: [], cursor: -1 };

  // ── Load catalogue ───────────────────────────────────────────────────────────
  let groups;
  try {
    groups = await fetchCatalogue();
  } catch (err) {
    const errEl = mk('div', 'elko-load-error');
    errEl.textContent = `Failed to load catalogue: ${err.message}`;
    sidebar.appendChild(errEl);
    return;
  }

  // Flat name → tool lookup (used for URL restore)
  const toolMap = new Map();
  for (const cats of groups.values())
    for (const tools of cats.values())
      for (const tool of tools)
        toolMap.set(tool.name, tool);

  // ── Toolbar ──────────────────────────────────────────────────────────────────

  function renderToolbar(rawText) {
    toolbar.innerHTML = '';

    // Back / forward
    const nav  = mk('div', 'elko-toolbar-nav');
    const back = mk('button', 'elko-toolbar-btn');
    back.textContent = '←';
    back.title       = 'Back';
    back.disabled    = hist.cursor <= 0;
    back.addEventListener('click', () => {
      if (hist.cursor > 0) {
        hist.cursor--;
        const e = hist.entries[hist.cursor];
        renderResult(e.text, e.format, resultPanel);
        replaceURL(e.tool, e.args);
        renderToolbar(e.text);
      }
    });

    const fwd  = mk('button', 'elko-toolbar-btn');
    fwd.textContent = '→';
    fwd.title       = 'Forward';
    fwd.disabled    = hist.cursor >= hist.entries.length - 1;
    fwd.addEventListener('click', () => {
      if (hist.cursor < hist.entries.length - 1) {
        hist.cursor++;
        const e = hist.entries[hist.cursor];
        renderResult(e.text, e.format, resultPanel);
        replaceURL(e.tool, e.args);
        renderToolbar(e.text);
      }
    });

    const counter = mk('span', 'elko-toolbar-counter');
    if (hist.entries.length > 1) {
      counter.textContent = `${hist.cursor + 1} / ${hist.entries.length}`;
    }

    nav.appendChild(back);
    nav.appendChild(fwd);
    nav.appendChild(counter);
    toolbar.appendChild(nav);

    // Copy button (only when there's a result)
    if (rawText) {
      const copy = mk('button', 'elko-toolbar-btn elko-toolbar-btn--copy');
      copy.textContent = 'Copy';
      copy.title       = 'Copy raw text to clipboard';
      copy.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(rawText);
          copy.textContent = 'Copied!';
          setTimeout(() => { copy.textContent = 'Copy'; }, 1500);
        } catch {
          copy.textContent = 'Error';
          setTimeout(() => { copy.textContent = 'Copy'; }, 1500);
        }
      });
      toolbar.appendChild(copy);
    }
  }

  // ── URL helpers ──────────────────────────────────────────────────────────────

  function replaceURL(tool, args) {
    const p = new URLSearchParams({ tool: tool.name });
    for (const [k, v] of Object.entries(args)) p.set(k, String(v));
    window.history.replaceState(null, '', '?' + p.toString());
  }

  function readURL() {
    const p    = new URLSearchParams(location.search);
    const name = p.get('tool');
    if (!name) return null;
    const tool = toolMap.get(name);
    if (!tool) return null;
    const args = {};
    for (const [k, v] of p) {
      if (k === 'tool') continue;
      const prop = tool.schema?.properties?.[k];
      if (prop?.type === 'boolean')                         args[k] = v === 'true';
      else if (prop?.type === 'integer' || prop?.type === 'number') args[k] = Number(v);
      else                                                  args[k] = v;
    }
    return { tool, args };
  }

  // ── Tool selector ────────────────────────────────────────────────────────────

  function selectTool(tool, prefillArgs = {}) {
    // Highlight sidebar item
    sidebar.querySelectorAll('.elko-tool-item.active')
      .forEach(el => el.classList.remove('active'));
    const item = sidebar.querySelector(`[data-tool="${CSS.escape(tool.name)}"]`);
    if (item) item.classList.add('active');

    // Build form
    toolPanel.innerHTML  = '';
    resultPanel.innerHTML = '';
    renderToolbar(null);

    const formContainer = buildForm(tool, async (args) => {
      replaceURL(tool, args);
      const result = await runTool(tool.name, args, tool.result_format || '', resultPanel);
      if (result) {
        pushHistory({ tool, args, format: tool.result_format || '', text: result.text });
        renderToolbar(result.text);
      }
    });

    // Pre-fill from URL args if provided
    if (Object.keys(prefillArgs).length > 0) {
      fillForm(formContainer, prefillArgs, tool);
    }

    toolPanel.appendChild(formContainer);
  }

  // ── History helpers ──────────────────────────────────────────────────────────

  function pushHistory(entry) {
    // Truncate forward branch
    hist.entries = hist.entries.slice(0, hist.cursor + 1);
    hist.entries.push(entry);
    if (hist.entries.length > HISTORY_LIMIT) hist.entries.shift();
    hist.cursor = hist.entries.length - 1;
  }

  // ── Form pre-fill ────────────────────────────────────────────────────────────

  function fillForm(container, args, tool) {
    const form = container.querySelector('form');
    if (!form) return;
    for (const [name, value] of Object.entries(args)) {
      const el = form.elements[name];
      if (!el) continue;
      const prop = tool.schema?.properties?.[name];
      if (prop?.type === 'boolean') el.checked = value === true;
      else el.value = String(value);
    }
  }

  // ── Wire sidebar ─────────────────────────────────────────────────────────────

  buildSidebar(sidebar, groups, tool => {
    replaceURL(tool, {});
    selectTool(tool);
  });

  // ── Restore from URL on load ─────────────────────────────────────────────────

  const urlState = readURL();
  if (urlState) {
    const { tool, args } = urlState;
    selectTool(tool, args);

    // Auto-run if all required params are in the URL
    const required = tool.schema?.required ?? [];
    const haveAll  = required.every(k => args[k] !== undefined && args[k] !== '');
    if (haveAll && (required.length > 0 || Object.keys(args).length > 0)) {
      const result = await runTool(tool.name, args, tool.result_format || '', resultPanel);
      if (result) {
        pushHistory({ tool, args, format: tool.result_format || '', text: result.text });
        renderToolbar(result.text);
      }
    }
  }
}

// ── Utilities ──────────────────────────────────────────────────────────────────

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}

init().catch(err => {
  document.getElementById('app').textContent = `Init error: ${err.message}`;
});
