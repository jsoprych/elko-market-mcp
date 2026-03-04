/**
 * renderer.js — parse structured text output from the API and render as HTML.
 *
 * Format dispatch map (driven by result_format field in channel JSON):
 *   "table"    fixed-width columns, header + "---" separator + data rows
 *   "csv"      heading line + CSV header + CSV data rows
 *   "kv"       heading line + "Key:    Value" pairs (28-char left-padded)
 *   "sections" heading + "## Section" subsections each containing kv pairs
 *   (default)  <pre> fallback
 *
 * All extractors share the same structural envelope:
 *   Line 1:     # Title
 *   Lines 2..N: body (format-specific)
 *   Last lines: "Source: ..." / "Values in ..." footer (stripped)
 */

/**
 * Render `text` into `container` using the given format hint.
 * @param {string} text
 * @param {string} format  "table" | "csv" | "kv" | "sections" | ""
 * @param {HTMLElement} container
 */
export function renderResult(text, format, container) {
  container.innerHTML = '';

  if (!text || !text.trim()) {
    const empty = mk('p', 'elko-result-empty');
    empty.textContent = '(no output)';
    container.appendChild(empty);
    return;
  }

  switch (format) {
    case 'csv':      renderCSV(text, container);      break;
    case 'kv':       renderKV(text, container);       break;
    case 'sections': renderSections(text, container); break;
    case 'table':    renderTable(text, container);    break;
    default:         renderPre(text, container);      break;
  }
}

// ── Format renderers ───────────────────────────────────────────────────────────

function renderPre(text, container) {
  const pre = mk('pre', 'elko-result-output');
  pre.textContent = text;
  container.appendChild(pre);
}

/**
 * CSV: "# Title\n\nCol1,Col2,...\nval,val,..."
 */
function renderCSV(text, container) {
  const { title, bodyLines } = parseBlock(text);
  appendTitle(title, container);

  const dataLines = bodyLines.filter(l => l.trim() !== '');
  if (dataLines.length === 0) { renderPre(text, container); return; }

  const headers = splitCSV(dataLines[0]);
  if (headers.length < 2) { renderPre(text, container); return; }

  const rows = dataLines.slice(1).map(splitCSV);
  container.appendChild(buildTable(headers, rows, 'elko-result-table'));
}

/**
 * Fixed-width table: "# Title\n\nH1  H2  H3\n------\nv1  v2  v3"
 * Also handles the edgar_financials matrix (blank first col in header = label col).
 */
function renderTable(text, container) {
  const { title, bodyLines } = parseBlock(text);
  appendTitle(title, container);

  const sepIdx = bodyLines.findIndex(l => /^-{3,}\s*$/.test(l));
  if (sepIdx < 1) { renderPre(text, container); return; }

  const headerLine = bodyLines[sepIdx - 1];
  const dataLines  = bodyLines.slice(sepIdx + 1).filter(l => l.trim() !== '');

  const headers = splitFixed(headerLine);
  const rows    = dataLines.map(splitFixed);
  container.appendChild(buildTable(headers, rows, 'elko-result-table'));
}

/**
 * Key-value pairs: "# Title\n\nKey:          Value"
 * Splits each line on the first run of 2+ spaces.
 */
function renderKV(text, container) {
  const { title, bodyLines } = parseBlock(text);
  appendTitle(title, container);

  const table = mk('table', 'elko-result-table elko-result-table--kv');
  const tbody = document.createElement('tbody');
  let hasRows = false;

  for (const line of bodyLines) {
    if (!line.trim()) continue;
    const parts = line.split(/\s{2,}/);
    if (parts.length < 2) continue;
    hasRows = true;
    const tr = document.createElement('tr');
    appendCell(tr, 'th', parts[0].trim(), 'elko-kv-key');
    appendCell(tr, 'td', parts.slice(1).join('  ').trim(), 'elko-kv-val');
    tbody.appendChild(tr);
  }

  if (!hasRows) { renderPre(text, container); return; }
  table.appendChild(tbody);
  container.appendChild(table);
}

/**
 * Sections: "# Title\n\n## Period: X\n  Key: Value\n..."  (fdic_bank_financials)
 * Each "## " line starts a new section; its body is kv pairs.
 */
function renderSections(text, container) {
  const lines = text.split('\n');

  if (lines[0]?.startsWith('# ')) {
    appendTitle(lines[0].slice(2).trim(), container);
  }

  const sections = [];
  let current = null;
  for (const line of lines.slice(1)) {
    if (line.startsWith('## ')) {
      current = { heading: line.slice(3).trim(), lines: [] };
      sections.push(current);
    } else if (current) {
      current.lines.push(line);
    }
  }

  if (sections.length === 0) { renderPre(text, container); return; }

  for (const section of sections) {
    const wrap = mk('div', 'elko-result-section');
    const sh   = mk('h4', 'elko-result-section-title');
    sh.textContent = section.heading;
    wrap.appendChild(sh);

    const table = mk('table', 'elko-result-table elko-result-table--kv');
    const tbody = document.createElement('tbody');

    for (const line of section.lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('Source:')) continue;
      const parts = trimmed.split(/\s{2,}/);
      if (parts.length < 2) continue;
      const tr = document.createElement('tr');
      appendCell(tr, 'th', parts[0].trim(), 'elko-kv-key');
      appendCell(tr, 'td', parts.slice(1).join('  ').trim(), 'elko-kv-val');
      tbody.appendChild(tr);
    }

    table.appendChild(tbody);
    wrap.appendChild(table);
    container.appendChild(wrap);
  }
}

// ── Shared helpers ─────────────────────────────────────────────────────────────

/**
 * Extract the `# Title` first line and a clean body (no footer lines).
 * @param {string} text
 * @returns {{ title: string, bodyLines: string[] }}
 */
function parseBlock(text) {
  const lines = text.split('\n');
  let title = '';
  let start = 0;

  if (lines[0]?.startsWith('# ')) {
    title = lines[0].slice(2).trim();
    start = 1;
  }

  // Skip blank lines after title
  while (start < lines.length && lines[start].trim() === '') start++;

  // Strip footer (Source:, Values in USD..., trailing blanks)
  let end = lines.length;
  while (end > start && (
    lines[end - 1].trim() === '' ||
    lines[end - 1].trimStart().startsWith('Source:') ||
    lines[end - 1].trimStart().startsWith('Values in')
  )) end--;

  return { title, bodyLines: lines.slice(start, end) };
}

/** Rows beyond this limit are hidden behind a toggle. */
const ROW_LIMIT = 100;

/**
 * Build an HTML table from headers + rows arrays.
 * Rows beyond ROW_LIMIT are hidden; a toggle button reveals them.
 * @param {string[]} headers
 * @param {string[][]} rows
 * @param {string} cls
 * @returns {HTMLTableElement}
 */
function buildTable(headers, rows, cls) {
  const table = mk('table', cls);

  const thead = document.createElement('thead');
  const htr   = document.createElement('tr');
  for (const h of headers) appendCell(htr, 'th', h);
  thead.appendChild(htr);
  table.appendChild(thead);

  const visibleRows  = rows.slice(0, ROW_LIMIT);
  const overflowRows = rows.slice(ROW_LIMIT);

  const tbody = document.createElement('tbody');
  for (const row of visibleRows) {
    const tr = document.createElement('tr');
    for (let i = 0; i < Math.max(headers.length, row.length); i++) {
      appendCell(tr, 'td', row[i] ?? '');
    }
    tbody.appendChild(tr);
  }
  table.appendChild(tbody);

  if (overflowRows.length > 0) {
    const overflowBody = document.createElement('tbody');
    overflowBody.className = 'elko-overflow-body';
    for (const row of overflowRows) {
      const tr = document.createElement('tr');
      for (let i = 0; i < Math.max(headers.length, row.length); i++) {
        appendCell(tr, 'td', row[i] ?? '');
      }
      overflowBody.appendChild(tr);
    }
    table.appendChild(overflowBody);

    const tfoot = document.createElement('tfoot');
    const ftr   = document.createElement('tr');
    const ftd   = document.createElement('td');
    ftd.colSpan = Math.max(headers.length, 1);
    ftd.className = 'elko-overflow-cell';

    const btn = mk('button', 'elko-overflow-toggle');
    btn.textContent = `Show all ${rows.length} rows`;
    btn.addEventListener('click', () => {
      const hidden = overflowBody.classList.toggle('elko-overflow-hidden');
      btn.textContent = hidden
        ? `Show all ${rows.length} rows`
        : `Show fewer rows`;
    });
    // Start collapsed
    overflowBody.classList.add('elko-overflow-hidden');

    ftd.appendChild(btn);
    ftr.appendChild(ftd);
    tfoot.appendChild(ftr);
    table.appendChild(tfoot);
  }

  return table;
}

/** Append a title <h3> to container if title is non-empty. */
function appendTitle(title, container) {
  if (!title) return;
  const h = mk('h3', 'elko-result-title');
  h.textContent = title;
  container.appendChild(h);
}

/** Append a <th> or <td> cell to a row. */
function appendCell(tr, tag, text, cls = '') {
  const cell = document.createElement(tag);
  if (cls) cell.className = cls;
  cell.textContent = text;
  tr.appendChild(cell);
}

/** Split a fixed-width line on runs of 2+ spaces. */
function splitFixed(line) {
  return line.split(/\s{2,}/).map(s => s.trim());
}

/** Split a CSV line (no quoted-comma support needed for our outputs). */
function splitCSV(line) {
  return line.split(',').map(s => s.trim());
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
