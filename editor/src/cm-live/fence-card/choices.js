// Chat-only options card; `- [ ]` items allow several picks. Static until a host
// calls el._hyChoices.setState({ mode, picked }); picks bubble as `hylo:choice`.
import { icon } from './icons.js';

const STYLE_ID = 'hy-choices-style';

// No opacity transitions: the app's button:active dimming would animate, and WebKit
// composites the animating button on whole pixels, shifting it and leaving a sliver.
const CSS = `
.hy-choices{display:flex;gap:var(--space-xs,8px);padding:2px 0}
.hy-choices-row{align-items:center}
.hy-choices.hy-choices-row>*{flex:none;white-space:nowrap}
.hy-choices-col{flex-direction:column;align-items:flex-start}
.hy-choices-measuring{visibility:hidden}
.hy-choices-probe{position:absolute;top:0;left:0;right:0;visibility:hidden;pointer-events:none}
.hy-choice{all:unset;box-sizing:border-box;display:inline-flex;align-items:center;gap:8px;max-width:100%;padding:7px 12px;border-radius:var(--r-md,8px);font:var(--fw-medium,500) var(--text-sm,.8125rem)/1.4 var(--font-ui,system-ui,sans-serif);color:var(--fg,#18181b);background:var(--btn-bg,rgba(24,24,27,.06));white-space:normal;overflow-wrap:anywhere;transition:background-color var(--motion-fast,100ms)}
.hy-choices-active .hy-choice{cursor:pointer}
.hy-choices-active .hy-choice:hover{background:var(--btn-bg-hov,rgba(24,24,27,.1))}
.hy-choice:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
.hy-choice-box{flex:none;display:flex;opacity:.55}
.hy-choice-box svg{width:16px;height:16px;fill:none;stroke:currentColor;stroke-width:2;stroke-linecap:round;stroke-linejoin:round}
.hy-choice-box svg+svg,.hy-choice-on .hy-choice-box svg:first-child{display:none}
.hy-choice-on .hy-choice-box svg+svg{display:block}
.hy-choices .hy-choice-on{background:var(--btn-bg-on,rgba(24,24,27,.15))}
.hy-choice-on .hy-choice-box{opacity:1}
.hy-choices-answered .hy-choice:not(.hy-choice-on){opacity:.5}
.hy-choices-send{all:unset;box-sizing:border-box;cursor:pointer;padding:7px 12px;border-radius:var(--r-md,8px);font:var(--fw-medium,500) var(--text-sm,.8125rem)/1.4 var(--font-ui,system-ui,sans-serif);color:var(--accent-fg,#fff);background:var(--accent,#5e6ad2)}
.hy-choices-send[disabled]{cursor:default;opacity:.4}
.hy-choices-send:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
`;

function ensureStyle() {
  if (document.getElementById(STYLE_ID)) return;
  const el = document.createElement('style');
  el.id = STYLE_ID;
  el.textContent = CSS;
  document.head.appendChild(el);
}

const ITEM = /^\s*(?:[-*+]|\d+[.)])\s+(\[[ xX]\]\s+)?(.*\S)\s*$/;
const SEP = ', ';

export function parseChoices(source) {
  const options = [];
  let multi = false;
  for (const line of (source || '').split('\n')) {
    const m = ITEM.exec(line);
    if (!m) continue;
    if (m[1]) multi = true;
    options.push(m[2]);
  }
  return { multi, options };
}

// A button shows an option's inline markdown as plain text; a click still sends the option as written.
export function choiceLabel(option) {
  return option
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[\[([^\]|]+)(?:\|([^\]]+))?\]\]/g, (_, target, alias) => alias || target)
    .replace(/(\*\*|~~|`)(.+?)\1/g, '$2')
    .replace(/(?<![\w*])\*(?!\s)(.+?)(?<!\s)\*(?![\w*])/g, '$1')
    .replace(/\\([\\`*_[\]()#+\-.!~|])/g, '$1')
    .trim() || option;
}

// Which options a reply picked: the whole reply for one, its parts for several.
export function pickedOptions(options, reply) {
  const text = (reply || '').trim();
  if (!text) return new Set();
  if (options.includes(text)) return new Set([text]);
  const parts = new Set(text.split(SEP).map((s) => s.trim()));
  return new Set(options.filter((o) => parts.has(o)));
}

export function mount(el, source, ctx = {}) {
  ensureStyle();
  el.classList.add('hy-choices-host');
  let parsed = parseChoices(source);
  let state = { mode: 'static', picked: '' };
  let selected = new Set();
  let width = -1;

  const emit = (text) => el.dispatchEvent(new CustomEvent('hylo:choice', { bubbles: true, detail: { text } }));

  function render() {
    const active = state.mode === 'active';
    const picked = state.mode === 'answered' ? pickedOptions(parsed.options, state.picked) : selected;
    const list = document.createElement('div');
    list.className = `hy-choices hy-choices-${state.mode} hy-choices-measuring`;
    list.setAttribute('role', parsed.multi ? 'group' : 'list');
    for (const text of parsed.options) {
      const b = document.createElement(active ? 'button' : 'span');
      b.className = 'hy-choice';
      if (picked.has(text)) b.classList.add('hy-choice-on');
      if (parsed.multi) {
        const box = document.createElement('span');
        box.className = 'hy-choice-box';
        // Both glyphs share one frame, so toggling in place never shifts the label.
        box.innerHTML = icon('square') + icon('square-check');
        b.appendChild(box);
        if (active) b.setAttribute('aria-pressed', String(picked.has(text)));
      }
      b.appendChild(document.createTextNode(choiceLabel(text)));
      if (active) {
        b.type = 'button';
        b.addEventListener('click', () => {
          if (!parsed.multi) return emit(text);
          // Toggle in place: rebuilding and re-measuring the list would shift the chat's scroll.
          const on = !selected.has(text);
          if (on) selected.add(text);
          else selected.delete(text);
          b.classList.toggle('hy-choice-on', on);
          b.setAttribute('aria-pressed', String(on));
          const send = list.querySelector('.hy-choices-send');
          if (send) send.disabled = selected.size === 0;
        });
      }
      list.appendChild(b);
    }
    if (active && parsed.multi) {
      const send = document.createElement('button');
      send.type = 'button';
      send.className = 'hy-choices-send';
      send.textContent = 'Send';
      send.disabled = selected.size === 0;
      send.addEventListener('click', () => emit(parsed.options.filter((o) => selected.has(o)).join(SEP)));
      list.appendChild(send);
    }
    el.replaceChildren(list);
    width = -1;
    layout();
  }

  // One row if it fits, else one per line, never a ragged wrap. Measured on an
  // out-of-flow copy so the list's height, and the chat's scroll, never jump.
  function layout() {
    const list = el.firstChild;
    if (!list || !el.isConnected || el.clientWidth === width) return;
    width = el.clientWidth;
    const probe = list.cloneNode(true);
    probe.className = 'hy-choices hy-choices-row hy-choices-probe';
    el.appendChild(probe);
    const fits = probe.scrollWidth <= probe.clientWidth + 1;
    probe.remove();
    list.classList.toggle('hy-choices-row', fits);
    list.classList.toggle('hy-choices-col', !fits);
    list.classList.remove('hy-choices-measuring');
  }
  const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(() => layout());
  if (ro) ro.observe(el);

  el._hyChoices = {
    setState(next) {
      const was = state.mode;
      const mode = next.mode || 'static';
      const picked = next.picked || '';
      // Hosts call this on every render; redrawing would drop focus and hover.
      if (mode === was && picked === state.picked) return;
      state = { mode, picked };
      if (was !== 'active') selected = new Set();
      render();
    },
  };
  render();
  // Hosts that make it interactive hear about it once it's in their DOM, before it paints if it already is.
  const announce = () => el.dispatchEvent(new CustomEvent('hylo:choices-mount', { bubbles: true }));
  if (el.isConnected) announce();
  else requestAnimationFrame(() => {
    if (!el.isConnected) return;
    layout();
    announce();
  });

  return {
    update(next) {
      parsed = parseChoices(next);
      selected = new Set([...selected].filter((o) => parsed.options.includes(o)));
      render();
    },
    destroy() {
      if (ro) ro.disconnect();
      el.classList.remove('hy-choices-host');
      delete el._hyChoices;
    },
  };
}
