/**
 * app.js — entry point.
 *
 * Features:
 *   - Layout skeleton: sidebar | (form + chart) | result
 *   - Sidebar ↔ form-builder ↔ runner pipeline
 *   - Pure-SVG chart panel rendered from channel's chart spec
 *   - Export toolbar: format selector (txt/csv/json) + download + copy
 *   - URL state: ?tool=name&arg=val — bookmarkable, auto-runs on load
 *   - Result history: in-app back/forward, up to HISTORY_LIMIT entries
 *   - Light / dark theme toggle (persisted to localStorage)
 */

import { fetchCatalogue }             from './catalogue.js';
import { buildSidebar }               from './sidebar.js';
import { buildForm }                  from './form-builder.js';
import { runTool }                    from './runner.js';
import { renderResult }               from './renderer.js';
import { renderChart }                from './chart.js';
import { exportData, exportFilename } from './export.js';

const HISTORY_LIMIT = 50;
const THEME_KEY     = 'elko-theme';

// ── Theme helpers (run before render to avoid flash) ──────────────────────────

function getStoredTheme() {
  const v = localStorage.getItem(THEME_KEY);
  return v === 'dark' || v === 'light' ? v : null;
}

function applyTheme(theme) {
  document.body.dataset.theme = theme;
  localStorage.setItem(THEME_KEY, theme);
}

// Apply before first paint: stored preference → system preference → light
applyTheme(getStoredTheme() ?? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));

async function init() {
  const app = document.getElementById('app');

  // ── Layout skeleton ─────────────────────────────────────────────────────────
  const sidebar       = mk('aside', 'elko-sidebar');
  const workspace     = mk('main',  'elko-workspace');
  const upper         = mk('div',   'elko-upper');       // form + chart side-by-side
  const toolPanel     = mk('div',   'elko-tool-panel');
  const chartPanel    = mk('div',   'elko-chart-panel');
  const resultWrapper = mk('div',   'elko-result-wrapper');
  const toolbar       = mk('div',   'elko-result-toolbar');
  const resultPanel   = mk('div',   'elko-result-panel');

  const sidebarHeader = mk('div', 'elko-sidebar-header');

  const sidebarTitle = mk('span', 'elko-sidebar-title');
  sidebarTitle.textContent = 'elko market';

  const themeBtn = mk('button', 'elko-theme-toggle');
  themeBtn.type  = 'button';
  themeBtn.title = 'Toggle light / dark mode';

  function syncThemeBtn() {
    const dark = document.body.dataset.theme === 'dark';
    themeBtn.textContent = dark ? '☀' : '☾';
    themeBtn.setAttribute('aria-label', dark ? 'Switch to light mode' : 'Switch to dark mode');
  }
  syncThemeBtn();

  themeBtn.addEventListener('click', () => {
    applyTheme(document.body.dataset.theme === 'dark' ? 'light' : 'dark');
    syncThemeBtn();
  });

  sidebarHeader.appendChild(sidebarTitle);
  sidebarHeader.appendChild(themeBtn);
  sidebar.appendChild(sidebarHeader);

  upper.appendChild(toolPanel);
  upper.appendChild(chartPanel);
  resultWrapper.appendChild(toolbar);
  resultWrapper.appendChild(resultPanel);
  workspace.appendChild(upper);
  workspace.appendChild(resultWrapper);
  app.appendChild(sidebar);
  app.appendChild(workspace);

  const welcome = mk('div', 'elko-welcome');
  welcome.textContent = 'Select a tool from the sidebar.';
  toolPanel.appendChild(welcome);

  // ── Export format state ──────────────────────────────────────────────────────
  // Persists across navigations; controls both Download and Copy.
  let exportFmt = 'txt';

  // ── History state ────────────────────────────────────────────────────────────
  // Each entry: { tool, args, format, text, chart }
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

  function renderToolbar(rawText, resultFormat, toolName) {
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
        renderChart(e.text, e.format, e.chart, chartPanel);
        replaceURL(e.tool, e.args);
        renderToolbar(e.text, e.format, e.tool.name);
      }
    });

    const fwd = mk('button', 'elko-toolbar-btn');
    fwd.textContent = '→';
    fwd.title       = 'Forward';
    fwd.disabled    = hist.cursor >= hist.entries.length - 1;
    fwd.addEventListener('click', () => {
      if (hist.cursor < hist.entries.length - 1) {
        hist.cursor++;
        const e = hist.entries[hist.cursor];
        renderResult(e.text, e.format, resultPanel);
        renderChart(e.text, e.format, e.chart, chartPanel);
        replaceURL(e.tool, e.args);
        renderToolbar(e.text, e.format, e.tool.name);
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

    // Export controls (only when there's a result)
    if (rawText) {
      const actions = mk('div', 'elko-toolbar-actions');

      // Format selector
      const fmt = mk('select', 'elko-toolbar-fmt');
      fmt.title = 'Export format';
      for (const f of ['txt', 'csv', 'json']) {
        const opt = document.createElement('option');
        opt.value = f;
        opt.textContent = f;
        opt.selected = f === exportFmt;
        fmt.appendChild(opt);
      }
      fmt.addEventListener('change', () => { exportFmt = fmt.value; });

      // Download button
      const dl = mk('button', 'elko-toolbar-btn');
      dl.textContent = '↓';
      dl.title       = 'Download';
      dl.addEventListener('click', () => {
        const content  = exportData(rawText, resultFormat, exportFmt);
        const filename = exportFilename(toolName || 'result', exportFmt);
        triggerDownload(content, filename);
      });

      // Copy button
      const copy = mk('button', 'elko-toolbar-btn elko-toolbar-btn--copy');
      copy.textContent = 'Copy';
      copy.title       = 'Copy to clipboard';
      copy.addEventListener('click', async () => {
        try {
          await navigator.clipboard.writeText(exportData(rawText, resultFormat, exportFmt));
          copy.textContent = 'Copied!';
          setTimeout(() => { copy.textContent = 'Copy'; }, 1500);
        } catch {
          copy.textContent = 'Error';
          setTimeout(() => { copy.textContent = 'Copy'; }, 1500);
        }
      });

      actions.appendChild(fmt);
      actions.appendChild(dl);
      actions.appendChild(copy);
      toolbar.appendChild(actions);
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
      if (prop?.type === 'boolean')                              args[k] = v === 'true';
      else if (prop?.type === 'integer' || prop?.type === 'number') args[k] = Number(v);
      else                                                       args[k] = v;
    }
    return { tool, args };
  }

  // ── Tool selector ─────────────────────────────────────────────────────────────

  function selectTool(tool, prefillArgs = {}) {
    // Highlight sidebar item
    sidebar.querySelectorAll('.elko-tool-item.active')
      .forEach(el => el.classList.remove('active'));
    const item = sidebar.querySelector(`[data-tool="${CSS.escape(tool.name)}"]`);
    if (item) item.classList.add('active');

    // Clear panels
    toolPanel.innerHTML   = '';
    resultPanel.innerHTML = '';
    chartPanel.innerHTML  = '';
    renderToolbar(null, '', tool.name);

    // Show chart panel only when tool has a chart spec
    chartPanel.style.display = tool.chart ? '' : 'none';

    const formContainer = buildForm(tool, async (args) => {
      replaceURL(tool, args);
      const result = await runTool(tool.name, args, tool.result_format || '', resultPanel);
      if (result) {
        renderChart(result.text, tool.result_format || '', tool.chart || null, chartPanel);
        pushHistory({ tool, args, format: tool.result_format || '', text: result.text, chart: tool.chart || null });
        renderToolbar(result.text, tool.result_format || '', tool.name);
      }
    });

    if (Object.keys(prefillArgs).length > 0) {
      fillForm(formContainer, prefillArgs, tool);
    }

    toolPanel.appendChild(formContainer);
  }

  // ── History helpers ───────────────────────────────────────────────────────────

  function pushHistory(entry) {
    hist.entries = hist.entries.slice(0, hist.cursor + 1);
    hist.entries.push(entry);
    if (hist.entries.length > HISTORY_LIMIT) hist.entries.shift();
    hist.cursor = hist.entries.length - 1;
  }

  // ── Form pre-fill ─────────────────────────────────────────────────────────────

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

  // ── Wire sidebar ──────────────────────────────────────────────────────────────

  buildSidebar(sidebar, groups, tool => {
    replaceURL(tool, {});
    selectTool(tool);
  });

  // ── Restore from URL on load ──────────────────────────────────────────────────

  const urlState = readURL();
  if (urlState) {
    const { tool, args } = urlState;
    selectTool(tool, args);

    const required = tool.schema?.required ?? [];
    const haveAll  = required.every(k => args[k] !== undefined && args[k] !== '');
    if (haveAll && (required.length > 0 || Object.keys(args).length > 0)) {
      const result = await runTool(tool.name, args, tool.result_format || '', resultPanel);
      if (result) {
        renderChart(result.text, tool.result_format || '', tool.chart || null, chartPanel);
        pushHistory({ tool, args, format: tool.result_format || '', text: result.text, chart: tool.chart || null });
        renderToolbar(result.text, tool.result_format || '', tool.name);
      }
    }
  }
}

// ── Utilities ───────────────────────────────────────────────────────────────────

function triggerDownload(content, filename) {
  const a = document.createElement('a');
  a.href     = URL.createObjectURL(new Blob([content], { type: 'text/plain' }));
  a.download = filename;
  a.click();
  URL.revokeObjectURL(a.href);
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}

init().catch(err => {
  document.getElementById('app').textContent = `Init error: ${err.message}`;
});
