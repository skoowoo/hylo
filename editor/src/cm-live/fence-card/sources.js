// Chat-only line naming the notes and skill a reply drew on. The skill is a
// button only while the reply can still be answered, like a choices card.
import { parseFields, injectStyle, noteName, noteLink } from './fields.js';
import { lineIconNode } from '../../line-icons.js';

const CSS = `
.hy-sources{display:flex;flex-wrap:wrap;align-items:center;gap:2px var(--space-xs,8px);font:var(--text-xs,.75rem)/1.6 var(--font-ui,system-ui,sans-serif);color:var(--muted,#6d7080)}
.hy-sources-group{display:inline-flex;flex-wrap:wrap;align-items:center;gap:0 6px}
.hy-sources .line-icon{width:12px;height:12px;flex:none}
.hy-sources a{color:inherit;text-decoration:none}
.hy-sources a:hover{color:var(--fg,#18181b);text-decoration:underline}
.hy-sources-sep{opacity:.6}
.hy-sources-skill{all:unset;cursor:pointer;text-decoration:underline dotted;text-underline-offset:3px}
.hy-sources-skill:hover{color:var(--fg,#18181b)}
.hy-sources-skill:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px;border-radius:var(--r-xs,4px)}
`;

export function mount(el, source) {
  injectStyle('hy-sources-style', CSS);
  el.classList.add('hy-choices-host');
  let parsed = parseFields(source);
  let mode = 'static';

  function render() {
    const line = document.createElement('div');
    line.className = 'hy-sources';
    const notes = parsed.all('note').map(noteName).filter(Boolean);
    if (notes.length) {
      const g = group('note');
      notes.forEach((n, i) => {
        if (i) g.appendChild(sep());
        g.appendChild(noteLink(n));
      });
      line.appendChild(g);
    }
    for (const skill of parsed.all('skill')) {
      const g = group('sparkles');
      const swap = parsed.field('swap');
      if (swap && mode === 'active') {
        const b = document.createElement('button');
        b.type = 'button';
        b.className = 'hy-sources-skill';
        b.textContent = skill;
        b.title = swap;
        b.addEventListener('click', () => el.dispatchEvent(new CustomEvent('hylo:choice', { bubbles: true, detail: { text: swap } })));
        g.appendChild(b);
      } else g.appendChild(document.createTextNode(skill));
      line.appendChild(g);
    }
    el.replaceChildren(line);
  }

  el._hyChoices = {
    setState(next) {
      const m = next.mode === 'active' ? 'active' : 'static';
      if (m === mode) return;
      mode = m;
      render();
    },
  };
  render();
  if (el.isConnected) el.dispatchEvent(new CustomEvent('hylo:choices-mount', { bubbles: true }));
  else requestAnimationFrame(() => el.isConnected && el.dispatchEvent(new CustomEvent('hylo:choices-mount', { bubbles: true })));

  return {
    update(next) {
      parsed = parseFields(next);
      render();
    },
    destroy() {
      el.classList.remove('hy-choices-host');
      delete el._hyChoices;
    },
  };
}

function group(name) {
  const g = document.createElement('span');
  g.className = 'hy-sources-group';
  g.append(lineIconNode(name));
  return g;
}

function sep() {
  const s = document.createElement('span');
  s.className = 'hy-sources-sep';
  s.textContent = '·';
  return s;
}
