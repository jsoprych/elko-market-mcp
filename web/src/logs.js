/**
 * logs.js — call log viewer.
 *
 * Fetches GET /v1/logs and renders a live-refreshing table.
 * Exposes start/stop for the auto-refresh so app.js can pause it
 * when the Logs tab is not active.
 */

const REFRESH_MS  = 5000;
const DEFAULT_LIMIT = 100;

/**
 * Build the log viewer panel.
 * @returns {{ el: HTMLElement, start: () => void, stop: () => void }}
 */
export function buildLogViewer() {
  const root    = mk('div', 'elko-logs');
  const toolbar = mk('div', 'elko-logs-toolbar');
  const content = mk('div', 'elko-logs-content');

  // ── Filter controls ────────────────────────────────────────────────────────

  const toolInput = mk('input', 'elko-input elko-logs-filter');
  toolInput.type        = 'text';
  toolInput.placeholder = 'Filter by tool…';

  const errorChk = mk('input', 'elko-input--checkbox');
  errorChk.type = 'checkbox';
  errorChk.id   = 'elko-logs-errors';
  const errorLbl = mk('label', 'elko-logs-filter-label');
  errorLbl.htmlFor     = 'elko-logs-errors';
  errorLbl.textContent = 'Errors only';

  const limitSel = mk('select', 'elko-input elko-input--select elko-logs-filter');
  for (const n of [50, 100, 250, 500]) {
    const opt = document.createElement('option');
    opt.value       = String(n);
    opt.textContent = `Last ${n}`;
    opt.selected    = n === DEFAULT_LIMIT;
    limitSel.appendChild(opt);
  }

  const refreshBtn = mk('button', 'elko-toolbar-btn');
  refreshBtn.textContent = '↺';
  refreshBtn.title       = 'Refresh now';

  const status = mk('span', 'elko-logs-status');

  toolbar.appendChild(toolInput);
  toolbar.appendChild(errorChk);
  toolbar.appendChild(errorLbl);
  toolbar.appendChild(limitSel);
  toolbar.appendChild(refreshBtn);
  toolbar.appendChild(status);

  root.appendChild(toolbar);
  root.appendChild(content);

  // ── Fetch + render ─────────────────────────────────────────────────────────

  async function load() {
    const tool       = toolInput.value.trim();
    const errorsOnly = errorChk.checked;
    const limit      = Number(limitSel.value) || DEFAULT_LIMIT;

    const params = new URLSearchParams({ limit });
    if (tool)       params.set('tool', tool);
    if (errorsOnly) params.set('error', 'true');

    try {
      const res  = await fetch(`/v1/logs?${params}`);
      const data = await res.json();
      render(data.entries ?? []);
      status.textContent = `${data.count ?? 0} entries`;
      status.className   = 'elko-logs-status';
    } catch (err) {
      status.textContent = `Error: ${err.message}`;
      status.className   = 'elko-logs-status elko-logs-status--error';
    }
  }

  function render(entries) {
    content.innerHTML = '';
    if (entries.length === 0) {
      const empty = mk('p', 'elko-result-empty');
      empty.textContent = 'No log entries yet. Run a tool to start logging.';
      content.appendChild(empty);
      return;
    }

    const table = mk('table', 'elko-result-table elko-logs-table');

    const thead = document.createElement('thead');
    const htr   = document.createElement('tr');
    for (const h of ['Time', 'Tool', 'Source', 'ms', 'Args', 'Result', 'Error']) {
      const th = document.createElement('th');
      th.textContent = h;
      htr.appendChild(th);
    }
    thead.appendChild(htr);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    for (const e of entries) {
      const tr = document.createElement('tr');
      if (e.error) tr.classList.add('elko-log-row--error');

      const ts   = new Date(e.ts).toLocaleTimeString();
      const args = truncate(e.args, 60);
      const res  = truncate(e.result, 80);

      for (const [val, cls] of [
        [ts,        'elko-log-ts'],
        [e.tool,    'elko-log-tool'],
        [e.source,  ''],
        [String(e.duration_ms), 'elko-log-ms'],
        [args,      'elko-log-args'],
        [res,       'elko-log-result'],
        [e.error,   'elko-log-error'],
      ]) {
        const td = document.createElement('td');
        td.textContent = val;
        if (cls) td.className = cls;
        tr.appendChild(td);
      }

      // Expand result on click
      tr.style.cursor = 'pointer';
      tr.title = 'Click to expand result';
      tr.addEventListener('click', () => showDetail(e));

      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    content.appendChild(table);
  }

  function showDetail(e) {
    const existing = root.querySelector('.elko-log-detail');
    if (existing) existing.remove();

    const panel = mk('div', 'elko-log-detail');

    const close = mk('button', 'elko-toolbar-btn elko-log-detail-close');
    close.textContent = '✕ Close';
    close.addEventListener('click', () => panel.remove());
    panel.appendChild(close);

    const sections = [
      ['Tool',      e.tool],
      ['Source',    e.source],
      ['Time',      new Date(e.ts).toLocaleString()],
      ['Duration',  `${e.duration_ms} ms`],
      ['Args',      e.args],
      ['Result',    e.result + (e.result_len > e.result.length ? `\n… [truncated from ${e.result_len} chars]` : '')],
      ['Error',     e.error],
    ];

    for (const [label, value] of sections) {
      if (!value) continue;
      const block = mk('div', 'elko-log-detail-block');
      const lbl   = mk('div', 'elko-log-detail-label');
      lbl.textContent = label;
      const val = mk('pre', 'elko-log-detail-value');
      val.textContent = value;
      block.appendChild(lbl);
      block.appendChild(val);
      panel.appendChild(block);
    }

    root.appendChild(panel);
    panel.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }

  // ── Auto-refresh ───────────────────────────────────────────────────────────

  let timer = null;

  function start() {
    load();
    timer = setInterval(load, REFRESH_MS);
  }

  function stop() {
    clearInterval(timer);
    timer = null;
  }

  // Re-load on filter change (debounced)
  let debounce = null;
  function onFilterChange() {
    clearTimeout(debounce);
    debounce = setTimeout(load, 300);
  }

  toolInput.addEventListener('input',  onFilterChange);
  errorChk.addEventListener('change',  load);
  limitSel.addEventListener('change',  load);
  refreshBtn.addEventListener('click', load);

  return { el: root, start, stop };
}

// ── Helpers ────────────────────────────────────────────────────────────────────

function truncate(str, max) {
  if (!str) return '';
  return str.length > max ? str.slice(0, max) + '…' : str;
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
