// Chat-only automation offer: what it will do, the mechanics on demand, and
// choices-style buttons that answer like a choices block does.
import { parseFields, injectStyle, appendText } from './fields.js';
import { mount as mountChoices } from './choices.js';

const CSS = `
.hy-proposal{display:flex;flex-direction:column;gap:var(--space-xs,8px);padding:var(--space-sm,12px);border:1px solid var(--line-edge,rgba(24,24,27,.16));border-radius:var(--r-md,8px);font:var(--text-sm,.8125rem)/1.5 var(--font-ui,system-ui,sans-serif);color:var(--fg,#18181b)}
.hy-proposal-title{font-weight:var(--fw-semibold,600)}
.hy-proposal-summary{color:var(--prose-body,#27272a)}
.hy-proposal-more{all:unset;cursor:pointer;align-self:flex-start;font-size:var(--text-xs,.75rem);color:var(--muted,#6d7080)}
.hy-proposal-more:hover{color:var(--fg,#18181b)}
.hy-proposal-more:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px;border-radius:2px}
.hy-proposal-details{font-size:var(--text-xs,.75rem);color:var(--muted,#6d7080)}
.hy-proposal-details[hidden]{display:none}
.hy-proposal .hy-choices{padding-top:4px}
`;

export function mount(el, source) {
  injectStyle('hy-proposal-style', CSS);
  const card = document.createElement('div');
  card.className = 'hy-proposal';
  el.replaceChildren(card);
  let open = false;
  let choices = null;
  const options = (p) => p.items.map((t) => '- ' + t).join('\n');

  function render() {
    const p = parseFields(source);
    const head = [];
    if (p.field('title')) head.push(appendText(div('hy-proposal-title'), p.field('title')));
    if (p.field('summary')) head.push(appendText(div('hy-proposal-summary'), p.field('summary')));
    const details = p.field('details');
    if (details) {
      const body = appendText(div('hy-proposal-details'), details);
      body.hidden = !open;
      const more = document.createElement('button');
      more.type = 'button';
      more.className = 'hy-proposal-more';
      more.textContent = open ? 'Details ▾' : 'Details ▸';
      more.setAttribute('aria-expanded', String(open));
      more.addEventListener('click', () => {
        open = !open;
        body.hidden = !open;
        more.textContent = open ? 'Details ▾' : 'Details ▸';
        more.setAttribute('aria-expanded', String(open));
      });
      head.push(more, body);
    }
    // The buttons keep their own node so the chat's choice state survives a re-render;
    // they mount in place so they can measure their row before paint.
    if (choices) {
      card.replaceChildren(...head, choices.slot);
      choices.handle.update(options(p));
      return;
    }
    const slot = div('');
    card.replaceChildren(...head, slot);
    choices = { slot, handle: mountChoices(slot, options(p)) };
  }
  render();

  return {
    update(next) {
      source = next;
      render();
    },
    destroy() {
      if (choices) choices.handle.destroy();
    },
  };
}

function div(cls) {
  const d = document.createElement('div');
  if (cls) d.className = cls;
  return d;
}
