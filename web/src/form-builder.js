/**
 * form-builder.js — generates a typed HTML form from a JSON Schema tool definition.
 *
 * Supported property types → rendered input:
 *   boolean              → <input type="checkbox">
 *   string + enum[]      → <select>
 *   integer / number     → <input type="number">
 *   string (default)     → <input type="text">
 *
 * Schema extensions used (all valid JSON Schema):
 *   placeholder  — input placeholder text
 *   default      — pre-selected value
 *   examples[]   — shown as <datalist> options on text inputs
 *   minimum      — min attr on number inputs
 *   maximum      — max attr on number inputs
 */

/**
 * Build a form element from a tool definition.
 * @param {object} tool
 * @param {(args: object) => void} onSubmit  called with typed args on form submit
 * @returns {HTMLElement}
 */
export function buildForm(tool, onSubmit) {
  const schema = tool.schema || { type: 'object', properties: {} };
  const props   = schema.properties || {};
  const required = new Set(schema.required || []);

  const container = mk('div', 'elko-form-container');

  // ── Header
  const header = mk('div', 'elko-form-header');
  const title  = mk('h2',  'elko-tool-title');
  title.textContent = tool.name;
  const desc = mk('p', 'elko-tool-desc');
  desc.textContent = tool.description;
  header.appendChild(title);
  header.appendChild(desc);
  container.appendChild(header);

  // ── Fields
  const entries = Object.entries(props);
  const form = mk('form', 'elko-form');

  if (entries.length === 0) {
    const note = mk('p', 'elko-no-params');
    note.textContent = 'No parameters — click Run to invoke.';
    form.appendChild(note);
  } else {
    for (const [name, prop] of entries) {
      form.appendChild(buildFieldGroup(name, prop, required.has(name)));
    }
  }

  const actions = mk('div', 'elko-form-actions');
  const runBtn  = mk('button', 'elko-btn elko-btn--run');
  runBtn.type = 'submit';
  runBtn.textContent = 'Run';
  actions.appendChild(runBtn);
  form.appendChild(actions);

  form.addEventListener('submit', e => {
    e.preventDefault();
    onSubmit(collectArgs(form, props));
  });

  container.appendChild(form);
  return container;
}

// ── Private helpers ────────────────────────────────────────────────────────────

/**
 * @param {string} name
 * @param {object} prop
 * @param {boolean} isRequired
 * @returns {HTMLElement}
 */
function buildFieldGroup(name, prop, isRequired) {
  const group = mk('div', 'elko-field-group');
  group.dataset.field     = name;
  group.dataset.fieldType = prop.type || 'string';
  if (isRequired) group.dataset.required = 'true';

  const label = mk('label', 'elko-field-label');
  label.htmlFor = fieldId(name);
  if (isRequired) {
    label.innerHTML = `${name} <span class="elko-required">*</span>`;
  } else {
    label.textContent = name;
  }

  const input = buildInput(name, prop, isRequired);

  group.appendChild(label);
  group.appendChild(input);

  if (prop.description) {
    const hint = mk('small', 'elko-field-hint');
    hint.textContent = prop.description;
    group.appendChild(hint);
  }

  return group;
}

/**
 * @param {string} name
 * @param {object} prop
 * @param {boolean} isRequired
 * @returns {HTMLElement}
 */
function buildInput(name, prop, isRequired) {
  const id = fieldId(name);

  // Boolean → checkbox
  if (prop.type === 'boolean') {
    const wrap = mk('span', 'elko-checkbox-wrap');
    const cb   = mk('input', 'elko-input elko-input--checkbox');
    cb.type  = 'checkbox';
    cb.id    = id;
    cb.name  = name;
    if (prop.default === true) cb.defaultChecked = true;
    wrap.appendChild(cb);
    return wrap;
  }

  // Enum → select
  if (Array.isArray(prop.enum) && prop.enum.length > 0) {
    const select = mk('select', 'elko-input elko-input--select');
    select.id   = id;
    select.name = name;
    if (!isRequired) {
      const blank = document.createElement('option');
      blank.value = '';
      blank.textContent = '— default —';
      select.appendChild(blank);
    }
    for (const v of prop.enum) {
      const opt = document.createElement('option');
      opt.value = String(v);
      opt.textContent = String(v);
      if (prop.default !== undefined && String(prop.default) === String(v)) {
        opt.selected = true;
      }
      select.appendChild(opt);
    }
    return select;
  }

  // Integer / number → number input
  if (prop.type === 'integer' || prop.type === 'number') {
    const input = mk('input', 'elko-input elko-input--number');
    input.type = 'number';
    input.id   = id;
    input.name = name;
    input.step = '1';
    if (prop.minimum !== undefined) input.min = String(prop.minimum);
    if (prop.maximum !== undefined) input.max = String(prop.maximum);
    if (prop.default !== undefined) input.defaultValue = String(prop.default);
    if (prop.placeholder) input.placeholder = String(prop.placeholder);
    if (isRequired) input.required = true;
    return input;
  }

  // Default: text (or date picker when format=date)
  const input = mk('input', 'elko-input elko-input--text');
  input.type = prop.format === 'date' ? 'date' : 'text';
  input.id   = id;
  input.name = name;
  if (prop.placeholder) input.placeholder = prop.placeholder;
  if (prop.default !== undefined) input.defaultValue = String(prop.default);
  if (isRequired) input.required = true;

  // examples → datalist for autocomplete hints
  if (Array.isArray(prop.examples) && prop.examples.length > 0) {
    const listId  = `${id}-list`;
    const datalist = document.createElement('datalist');
    datalist.id = listId;
    for (const ex of prop.examples) {
      const opt = document.createElement('option');
      opt.value = ex;
      datalist.appendChild(opt);
    }
    input.setAttribute('list', listId);
    // datalist must be in the DOM; wrap them together
    const wrap = mk('span', 'elko-input-wrap');
    wrap.appendChild(input);
    wrap.appendChild(datalist);
    return wrap;
  }

  return input;
}

/**
 * Walk form elements by prop name, returning only non-empty, typed values.
 * @param {HTMLFormElement} form
 * @param {Record<string, object>} props
 * @returns {object}
 */
function collectArgs(form, props) {
  const args = {};
  for (const [name, prop] of Object.entries(props)) {
    const el = form.elements[name];
    if (!el) continue;

    if (prop.type === 'boolean') {
      if (el.checked) args[name] = true;
    } else if (prop.type === 'integer' || prop.type === 'number') {
      if (el.value !== '') args[name] = Number(el.value);
    } else {
      if (el.value !== '') args[name] = el.value;
    }
  }
  return args;
}

/** @param {string} name @returns {string} */
function fieldId(name) { return `elko-field-${name}`; }

/** @param {string} tag @param {string} cls @returns {HTMLElement} */
function mk(tag, cls) {
  const e = document.createElement(tag);
  e.className = cls;
  return e;
}
