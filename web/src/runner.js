/**
 * runner.js — calls POST /v1/call/{tool} and renders the result into a container.
 */

/**
 * Invoke a tool with args and stream the result into `container`.
 * @param {string} toolName
 * @param {object} args
 * @param {HTMLElement} container
 */
export async function runTool(toolName, args, container) {
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
      return;
    }

    const pre = mk('pre', 'elko-result-output');
    pre.textContent = typeof data.result === 'string'
      ? data.result
      : JSON.stringify(data.result, null, 2);
    container.appendChild(pre);

  } catch (err) {
    container.innerHTML = '';
    const errEl = mk('div', 'elko-result-status elko-result-status--error');
    errEl.textContent = String(err);
    container.appendChild(errEl);
  }
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
