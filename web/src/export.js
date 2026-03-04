/**
 * export.js — convert tool result text to txt / csv / json.
 *
 * JSON shape:
 *   tabular (csv/table)  → { title, columns: [...], rows: [[...], ...] }  ← split format, no repeated keys
 *   kv                   → { title, data: { key: value, ... } }
 *   sections             → { title, data: { "Section": { key: value }, ... } }
 *   pre / fallback       → { title, text: "..." }
 */

/**
 * Convert result text to the target export format.
 * @param {string} text
 * @param {string} resultFormat  "csv" | "table" | "kv" | "sections" | ""
 * @param {string} fmt           "txt" | "csv" | "json"
 * @returns {string}
 */
export function exportData(text, resultFormat, fmt) {
  if (fmt === 'csv')  return toCSV(text, resultFormat);
  if (fmt === 'json') return toJSON(text, resultFormat);
  return text;
}

/**
 * Build a download filename.
 * @param {string} toolName
 * @param {string} fmt  "txt" | "csv" | "json"
 * @returns {string}
 */
export function exportFilename(toolName, fmt) {
  const date = new Date().toISOString().slice(0, 10);
  const ext  = fmt === 'txt' ? 'txt' : fmt;
  return `${toolName}_${date}.${ext}`;
}

// ── Format converters ──────────────────────────────────────────────────────────

function toCSV(text, resultFormat) {
  if (resultFormat === 'csv') {
    const lines = text.split('\n');
    const start = lines.findIndex(l => l.trim() && !l.startsWith('#'));
    if (start < 0) return text;
    let end = lines.length;
    while (end > start && isFooter(lines[end - 1])) end--;
    return lines.slice(start, end).join('\n');
  }
  if (resultFormat === 'table') {
    const { columns, rows } = parseFixedTable(text);
    if (!columns.length) return text;
    return [columns.join(','), ...rows.map(r => r.map(cell).join(','))].join('\n');
  }
  if (resultFormat === 'kv') {
    const pairs = parseKV(text);
    if (!pairs.length) return text;
    return ['key,value', ...pairs.map(([k, v]) => `${cell(k)},${cell(v)}`)].join('\n');
  }
  if (resultFormat === 'sections') {
    const sections = parseSections(text);
    const rows = sections.flatMap(s => s.pairs.map(([k, v]) => `${cell(s.heading)},${cell(k)},${cell(v)}`));
    return ['section,key,value', ...rows].join('\n');
  }
  return text;
}

function toJSON(text, resultFormat) {
  const title = extractTitle(text);
  if (resultFormat === 'csv') {
    const { columns, rows } = parseCSV(text);
    return JSON.stringify({ title, columns, rows }, null, 2);
  }
  if (resultFormat === 'table') {
    const { columns, rows } = parseFixedTable(text);
    return JSON.stringify({ title, columns, rows }, null, 2);
  }
  if (resultFormat === 'kv') {
    const data = Object.fromEntries(parseKV(text));
    return JSON.stringify({ title, data }, null, 2);
  }
  if (resultFormat === 'sections') {
    const data = Object.fromEntries(
      parseSections(text).map(s => [s.heading, Object.fromEntries(s.pairs)])
    );
    return JSON.stringify({ title, data }, null, 2);
  }
  return JSON.stringify({ title, text }, null, 2);
}

// ── Parsers ────────────────────────────────────────────────────────────────────

function extractTitle(text) {
  const first = (text.split('\n')[0] ?? '');
  return first.startsWith('# ') ? first.slice(2).trim() : '';
}

function parseCSV(text) {
  const lines = text.split('\n').filter(l => l.trim() && !l.startsWith('#'));
  if (lines.length < 2) return { columns: [], rows: [] };
  const columns = lines[0].split(',').map(s => s.trim());
  const rows    = lines.slice(1).map(l => l.split(',').map(s => s.trim()));
  return { columns, rows };
}

function parseFixedTable(text) {
  const lines = text.split('\n');
  const si = lines.findIndex(l => /^-{3,}\s*$/.test(l));
  if (si < 1) return { columns: [], rows: [] };
  const columns = lines[si - 1].split(/\s{2,}/).map(s => s.trim()).filter(Boolean);
  let end = lines.length;
  while (end > si + 1 && isFooter(lines[end - 1])) end--;
  const rows = lines.slice(si + 1, end)
    .filter(l => l.trim())
    .map(l => l.split(/\s{2,}/).map(s => s.trim()));
  return { columns, rows };
}

function parseKV(text) {
  return text.split('\n')
    .filter(l => l.trim() && !l.startsWith('#') && !isFooter(l))
    .map(l => l.split(/\s{2,}/))
    .filter(p => p.length >= 2)
    .map(p => [p[0].trim(), p.slice(1).join('  ').trim()]);
}

function parseSections(text) {
  const sections = [];
  let cur = null;
  for (const line of text.split('\n')) {
    if (line.startsWith('## ')) {
      cur = { heading: line.slice(3).trim(), pairs: [] };
      sections.push(cur);
    } else if (cur && line.trim() && !isFooter(line)) {
      const parts = line.trim().split(/\s{2,}/);
      if (parts.length >= 2) cur.pairs.push([parts[0].trim(), parts.slice(1).join('  ').trim()]);
    }
  }
  return sections;
}

function isFooter(line) {
  const t = line.trimStart();
  return !line.trim() || t.startsWith('Source:') || t.startsWith('Values in');
}

/** Wrap a cell value for CSV — quote if it contains comma, quote, or newline. */
function cell(s) {
  const str = String(s ?? '');
  return (str.includes(',') || str.includes('"') || str.includes('\n'))
    ? '"' + str.replace(/"/g, '""') + '"'
    : str;
}
