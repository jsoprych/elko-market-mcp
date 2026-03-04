/**
 * app.js — entry point. Builds the layout skeleton, loads the catalogue,
 * and wires the sidebar ↔ form-builder ↔ runner together.
 */

import { fetchCatalogue } from './catalogue.js';
import { buildSidebar }   from './sidebar.js';
import { buildForm }      from './form-builder.js';
import { runTool }        from './runner.js';

async function init() {
  const app = document.getElementById('app');

  // ── Layout skeleton (all built from JS — index.html is just a shell)
  const sidebar    = mk('aside', 'elko-sidebar');
  const workspace  = mk('main',  'elko-workspace');
  const toolPanel  = mk('div',   'elko-tool-panel');
  const resultPanel = mk('div',  'elko-result-panel');

  const sidebarHeader = mk('div', 'elko-sidebar-header');
  sidebarHeader.textContent = 'elko market';
  sidebar.appendChild(sidebarHeader);

  workspace.appendChild(toolPanel);
  workspace.appendChild(resultPanel);
  app.appendChild(sidebar);
  app.appendChild(workspace);

  // ── Initial placeholder
  const welcome = mk('div', 'elko-welcome');
  welcome.textContent = 'Select a tool from the sidebar.';
  toolPanel.appendChild(welcome);

  // ── Load catalogue
  let groups;
  try {
    groups = await fetchCatalogue();
  } catch (err) {
    const errEl = mk('div', 'elko-load-error');
    errEl.textContent = `Failed to load catalogue: ${err.message}`;
    sidebar.appendChild(errEl);
    return;
  }

  // ── Wire sidebar → form → runner
  buildSidebar(sidebar, groups, tool => {
    toolPanel.innerHTML  = '';
    resultPanel.innerHTML = '';
    toolPanel.appendChild(
      buildForm(tool, args => runTool(tool.name, args, tool.result_format || '', resultPanel))
    );
  });
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
