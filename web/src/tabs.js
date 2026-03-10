/**
 * tabs.js — generic tab container.
 *
 * Usage:
 *   const { el, setActive } = buildTabs([
 *     { id: 'form',  label: 'Form',     content: formEl  },
 *     { id: 'mcp',   label: 'MCP JSON', content: jsonEl  },
 *   ]);
 *   parent.appendChild(el);
 *
 * When only one tab is provided the tab bar is hidden (no visual noise).
 */

/**
 * @param {Array<{id: string, label: string, content: HTMLElement}>} tabs
 * @param {string} [defaultId]  first tab if omitted
 * @returns {{ el: HTMLElement, setActive: (id: string) => void }}
 */
export function buildTabs(tabs, defaultId) {
  const wrap = mk('div', 'elko-tabs');
  const bar  = mk('div', 'elko-tab-bar');
  const body = mk('div', 'elko-tab-body');

  const paneMap = new Map(); // id → { btn, pane }

  for (const tab of tabs) {
    const btn  = mk('button', 'elko-tab-btn');
    btn.type        = 'button';
    btn.textContent = tab.label;
    btn.dataset.tab = tab.id;

    const pane = mk('div', 'elko-tab-pane');
    pane.dataset.tab = tab.id;
    pane.appendChild(tab.content);

    btn.addEventListener('click', () => setActive(tab.id));
    bar.appendChild(btn);
    body.appendChild(pane);
    paneMap.set(tab.id, { btn, pane });
  }

  // Hide bar when there is only one tab — no need for a tab strip.
  if (tabs.length <= 1) bar.classList.add('elko-tab-bar--hidden');

  wrap.appendChild(bar);
  wrap.appendChild(body);

  function setActive(id) {
    for (const [tid, { btn, pane }] of paneMap) {
      const active = tid === id;
      btn.classList.toggle('elko-tab-btn--active', active);
      pane.classList.toggle('elko-tab-pane--active', active);
    }
  }

  setActive(defaultId ?? tabs[0]?.id);
  return { el: wrap, setActive };
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
