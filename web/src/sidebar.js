/**
 * sidebar.js — builds a collapsible source → category → tool navigation tree.
 */

/**
 * Populate `container` with a nav tree from the grouped catalogue.
 * Groups and categories start collapsed; call onSelect(tool) on tool click.
 *
 * @param {HTMLElement} container
 * @param {Map<string, Map<string, any[]>>} groups
 * @param {(tool: any) => void} onSelect
 */
export function buildSidebar(container, groups, onSelect) {
  const nav = mk('nav', 'elko-nav');

  for (const [source, categories] of groups) {
    const sourceGroup = mk('div', 'elko-source-group');

    const sourceBtn = mk('button', 'elko-source-header');
    sourceBtn.type = 'button';
    sourceBtn.textContent = source;
    sourceBtn.addEventListener('click', () => sourceGroup.classList.toggle('open'));
    sourceGroup.appendChild(sourceBtn);

    const sourceBody = mk('div', 'elko-source-body');

    for (const [category, tools] of categories) {
      const catGroup = mk('div', 'elko-cat-group');

      const catBtn = mk('button', 'elko-cat-header');
      catBtn.type = 'button';
      catBtn.textContent = category;
      catBtn.addEventListener('click', () => catGroup.classList.toggle('open'));
      catGroup.appendChild(catBtn);

      const toolList = mk('ul', 'elko-tool-list');
      for (const tool of tools) {
        const item = mk('li', 'elko-tool-item');
        item.textContent = tool.name;
        item.dataset.tool = tool.name;
        item.title = tool.description;
        item.addEventListener('click', () => {
          container.querySelectorAll('.elko-tool-item.active')
            .forEach(el => el.classList.remove('active'));
          item.classList.add('active');
          onSelect(tool);
        });
        toolList.appendChild(item);
      }

      catGroup.appendChild(toolList);
      catGroup.classList.add('open'); // categories open by default
      sourceBody.appendChild(catGroup);
    }

    sourceGroup.appendChild(sourceBody);
    sourceGroup.classList.add('open'); // sources open by default
    nav.appendChild(sourceGroup);
  }

  container.appendChild(nav);
}

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
