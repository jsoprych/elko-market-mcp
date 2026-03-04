/**
 * catalogue.js — fetch tools from /v1/catalogue and group by source → category.
 *
 * @typedef {{
 *   name: string,
 *   description: string,
 *   source: string,
 *   category: string,
 *   schema: { type: string, properties: Record<string,PropDef>, required?: string[] }
 * }} Tool
 *
 * @typedef {{
 *   type: string,
 *   description?: string,
 *   placeholder?: string,
 *   default?: any,
 *   enum?: string[],
 *   examples?: string[],
 *   minimum?: number,
 *   maximum?: number
 * }} PropDef
 */

/**
 * Fetch the catalogue and return tools grouped as Map<source, Map<category, Tool[]>>.
 * @returns {Promise<Map<string, Map<string, Tool[]>>>}
 */
export async function fetchCatalogue() {
  const res = await fetch('/v1/catalogue');
  if (!res.ok) throw new Error(`catalogue fetch failed: ${res.status}`);
  const { tools } = await res.json();
  return groupTools(tools);
}

/**
 * @param {Tool[]} tools
 * @returns {Map<string, Map<string, Tool[]>>}
 */
function groupTools(tools) {
  /** @type {Map<string, Map<string, Tool[]>>} */
  const groups = new Map();
  for (const tool of tools) {
    if (!groups.has(tool.source)) groups.set(tool.source, new Map());
    const cats = groups.get(tool.source);
    if (!cats.has(tool.category)) cats.set(tool.category, []);
    cats.get(tool.category).push(tool);
  }
  return groups;
}
