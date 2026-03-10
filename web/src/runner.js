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
      const msg = data.error || `HTTP ${res.status}`;
      container.appendChild(isAuthError(msg) ? authErrorEl(msg) : errorEl(msg));
      return null;
    }

    const text = typeof data.result === 'string'
      ? data.result
      : JSON.stringify(data.result, null, 2);

    renderResult(text, resultFormat, container);
    return { text };

  } catch (err) {
    container.innerHTML = '';
    container.appendChild(errorEl(String(err)));
    return null;
  }
}

/** Returns true if the error message looks like a missing/bad API key. */
function isAuthError(msg) {
  return /api[_ ]key|not set|unauthorized|forbidden|invalid.*(key|token)|rate.?limit/i.test(msg);
}

/** Renders a plain error element. */
function errorEl(msg) {
  const el = mk('div', 'elko-result-status elko-result-status--error');
  el.textContent = msg;
  return el;
}

/**
 * Renders an auth-specific error with the env var name highlighted
 * and a link extracted from the error text if present.
 */
function authErrorEl(msg) {
  const el = mk('div', 'elko-result-status elko-result-status--auth');

  // Extract env var name (ALL_CAPS_WITH_UNDERSCORES).
  const envMatch = msg.match(/\b([A-Z][A-Z0-9_]{3,})\b/);
  const urlMatch  = msg.match(/https?:\/\/\S+/);

  const icon = document.createElement('span');
  icon.textContent = '⚠ ';

  el.appendChild(icon);

  if (envMatch) {
    el.appendChild(document.createTextNode('API key required: '));
    const code = document.createElement('code');
    code.textContent = envMatch[1];
    el.appendChild(code);
    el.appendChild(document.createTextNode(' is not set. '));
  } else {
    el.appendChild(document.createTextNode(msg + ' '));
  }

  if (urlMatch) {
    const a = document.createElement('a');
    a.href = urlMatch[0];
    a.target = '_blank';
    a.rel = 'noopener';
    a.textContent = 'Get a free key →';
    el.appendChild(a);
  }

  return el;
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
