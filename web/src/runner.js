/**
 * runner.js — calls POST /v1/call/{tool} and renders the result.
 * Returns { text } on success, null on error (caller handles history/toolbar).
 */

import { renderResult } from './renderer.js';

/**
 * @param {string} toolName
 * @param {object} args
 * @param {string} resultFormat
 * @param {HTMLElement} container
 * @returns {Promise<{text: string}|null>}
 */
export async function runTool(toolName, args, resultFormat, container) {
  container.innerHTML = '';

  const loader = mk('div', 'elko-result-status elko-result-status--loading');
  loader.textContent = `Calling ${toolName}…`;
  container.appendChild(loader);

  try {
    const res  = await fetch(`/v1/call/${encodeURIComponent(toolName)}`, {
      method:  'POST',
      headers: { 'Content-Type': 'application/json' },
      body:    JSON.stringify(args),
    });
    const data = await res.json();
    container.innerHTML = '';

    if (!res.ok) {
      const errEl = mk('div', 'elko-result-status elko-result-status--error');
      errEl.textContent = data.error || `HTTP ${res.status}`;
      container.appendChild(errEl);
      return null;
    }

    const text = typeof data.result === 'string'
      ? data.result
      : JSON.stringify(data.result, null, 2);

    renderResult(text, resultFormat, container);
    return { text };

  } catch (err) {
    container.innerHTML = '';
    const errEl = mk('div', 'elko-result-status elko-result-status--error');
    errEl.textContent = String(err);
    container.appendChild(errEl);
    return null;
  }
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
