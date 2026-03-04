/**
 * chart.js — pure-SVG chart renderer. No external dependencies.
 *
 * Chart spec (from channel JSON → /v1/catalogue → tool.chart):
 *   { "type": "line" | "bar", "x": "ColName", "y": "ColName" }
 *
 * Supports result_format: "csv" and "table" (fixed-width).
 * For other formats the chart panel is left empty.
 */

const NS  = 'http://www.w3.org/2000/svg';
const CLR = {
  line:  '#7c6af7',
  area:  'rgba(124,106,247,0.07)',
  bar:   '#7c6af7',
  grid:  '#e8eaf0',
  axis:  '#ccc',
  tick:  '#999',
};
const PAD = { t: 14, r: 18, b: 34, l: 52 };

/**
 * @param {string}      text          raw result text
 * @param {string}      resultFormat  "csv" | "table"
 * @param {{ type: string, x: string, y: string } | null} spec
 * @param {HTMLElement} container
 */
export function renderChart(text, resultFormat, spec, container) {
  container.innerHTML = '';
  if (!spec || !text?.trim()) return;

  const { columns, rows } = parseTabular(text, resultFormat);
  if (!columns.length || !rows.length) return;

  const xi = columns.indexOf(spec.x);
  const yi = columns.indexOf(spec.y);
  if (xi < 0 || yi < 0) return;

  // Keep only rows with a parseable numeric y value
  const valid = rows.filter(r => r[yi] !== undefined && r[yi] !== '—' && r[yi] !== '' && !isNaN(parseFloat(r[yi])));
  if (!valid.length) return;

  const labels = valid.map(r => r[xi] ?? '');
  const values = valid.map(r => parseFloat(r[yi]));

  container.appendChild(buildSVG(labels, values, spec.type || 'line', spec.y));
}

// ── SVG builder ────────────────────────────────────────────────────────────────

function buildSVG(labels, values, type, yLabel) {
  const W  = 480;
  const H  = 190;
  const cW = W - PAD.l - PAD.r;
  const cH = H - PAD.t - PAD.b;

  const minV  = Math.min(...values);
  const maxV  = Math.max(...values);
  const range = (maxV - minV) || 1;
  const vpad  = range * 0.08;             // breathing room top/bottom

  const toY = v => PAD.t + cH - ((v - (minV - vpad)) / (range + 2 * vpad)) * cH;
  const toX = i => PAD.l + (i / Math.max(values.length - 1, 1)) * cW;

  const svg = svgEl('svg');
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  svg.style.cssText = 'width:100%;height:auto;display:block;';

  // ── Gridlines + y-axis labels (5 levels) ──────────────────────────────────
  for (let gi = 0; gi <= 4; gi++) {
    const frac = gi / 4;
    const y    = PAD.t + frac * cH;
    const v    = (maxV + vpad) - frac * (range + 2 * vpad);

    const g = svgEl('line');
    attrs(g, { x1: PAD.l, x2: PAD.l + cW, y1: y, y2: y, stroke: CLR.grid, 'stroke-width': 1 });
    svg.appendChild(g);

    const t = svgEl('text');
    attrs(t, { x: PAD.l - 5, y: y + 3.5, 'text-anchor': 'end', 'font-size': 9, fill: CLR.tick });
    t.textContent = fmtVal(v);
    svg.appendChild(t);
  }

  // ── Data ──────────────────────────────────────────────────────────────────
  if (type === 'bar') {
    const slotW = cW / values.length;
    const gap   = slotW * 0.18;
    const barW  = slotW - gap * 2;
    values.forEach((v, i) => {
      const x = PAD.l + i * slotW + gap;
      const y = toY(v);
      const h = Math.max(1, PAD.t + cH - y);
      const r = svgEl('rect');
      attrs(r, { x, y, width: Math.max(1, barW), height: h, fill: CLR.bar, opacity: 0.78, rx: 1.5 });
      svg.appendChild(r);
    });
  } else {
    const pts  = values.map((v, i) => `${toX(i).toFixed(1)},${toY(v).toFixed(1)}`).join(' ');
    const x0   = toX(0).toFixed(1);
    const xN   = toX(values.length - 1).toFixed(1);
    const base = (PAD.t + cH).toFixed(1);

    // Area fill
    const area = svgEl('polygon');
    area.setAttribute('points', `${x0},${base} ${pts} ${xN},${base}`);
    area.setAttribute('fill', CLR.area);
    svg.appendChild(area);

    // Line
    const line = svgEl('polyline');
    attrs(line, {
      points: pts, fill: 'none', stroke: CLR.line,
      'stroke-width': 1.5, 'stroke-linejoin': 'round', 'stroke-linecap': 'round',
    });
    svg.appendChild(line);
  }

  // ── X-axis labels (sparse, max 8) ─────────────────────────────────────────
  const step = Math.max(1, Math.ceil(labels.length / 8));
  labels.forEach((lbl, i) => {
    if (i % step !== 0 && i !== labels.length - 1) return;
    const x = type === 'bar'
      ? PAD.l + (i + 0.5) * (cW / values.length)
      : toX(i);
    const t = svgEl('text');
    attrs(t, { x: x.toFixed(1), y: H - 5, 'text-anchor': 'middle', 'font-size': 9, fill: CLR.tick });
    t.textContent = lbl.length > 10 ? lbl.slice(0, 10) : lbl;
    svg.appendChild(t);
  });

  // ── Y-axis line ───────────────────────────────────────────────────────────
  const ax = svgEl('line');
  attrs(ax, { x1: PAD.l, x2: PAD.l, y1: PAD.t, y2: PAD.t + cH, stroke: CLR.axis, 'stroke-width': 1 });
  svg.appendChild(ax);

  return svg;
}

// ── Text parsers ───────────────────────────────────────────────────────────────

function parseTabular(text, resultFormat) {
  if (resultFormat === 'csv')   return parseCSV(text);
  if (resultFormat === 'table') return parseFixedTable(text);
  return { columns: [], rows: [] };
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
  const si    = lines.findIndex(l => /^-{3,}\s*$/.test(l));
  if (si < 1) return { columns: [], rows: [] };
  const columns = lines[si - 1].split(/\s{2,}/).map(s => s.trim()).filter(Boolean);
  const rows = lines.slice(si + 1)
    .filter(l => l.trim() && !l.trimStart().startsWith('Source:') && !l.trimStart().startsWith('Values'))
    .map(l => l.split(/\s{2,}/).map(s => s.trim()));
  return { columns, rows };
}

// ── SVG utilities ─────────────────────────────────────────────────────────────

function svgEl(tag) { return document.createElementNS(NS, tag); }

function attrs(el, map) {
  for (const [k, v] of Object.entries(map)) el.setAttribute(k, v);
}

function fmtVal(v) {
  if (Math.abs(v) >= 1e12) return (v / 1e12).toFixed(1) + 'T';
  if (Math.abs(v) >= 1e9)  return (v / 1e9).toFixed(1)  + 'B';
  if (Math.abs(v) >= 1e6)  return (v / 1e6).toFixed(1)  + 'M';
  if (Math.abs(v) >= 1000) return v.toFixed(0);
  return parseFloat(v.toPrecision(3)).toString();
}
