// Chat-only record of what an automation change did, with an undo that stays
// usable on older replies. The chat sets its state through el._hyReceipt.
import { parseFields, injectStyle, appendText } from './fields.js';
import { lineIconNode } from '../../line-icons.js';

const CSS = `
.hy-receipt{display:grid;grid-template-columns:auto 1fr auto;column-gap:var(--space-xs,8px);align-items:start;padding:10px 14px;border-radius:var(--r-lg,12px);background:var(--surface-soft,#f5f5f7);font:var(--text-sm,.8125rem)/1.5 var(--font-ui,system-ui,sans-serif);color:var(--fg,#18181b);transition:opacity var(--motion-fast,100ms)}
.hy-receipt-icon{display:flex;height:1.5em;align-items:center;color:var(--s-ok,#16a34a)}
.hy-receipt-icon .line-icon{width:14px;height:14px}
.hy-receipt-title{font-weight:var(--fw-semibold,600)}
.hy-receipt-lines{grid-column:2;margin:2px 0 0;padding:0;list-style:none;color:var(--muted,#6d7080)}
.hy-receipt-undo{all:unset;box-sizing:border-box;grid-column:3;grid-row:1;padding:2px 10px;border-radius:var(--r-sm,6px);font-size:var(--text-xs,.75rem);color:var(--fg,#18181b);background:var(--btn-bg,rgba(24,24,27,.06));cursor:pointer;white-space:nowrap;transition:background-color var(--motion-fast,100ms)}
.hy-receipt-undo:hover{background:var(--btn-bg-hov,rgba(24,24,27,.1))}
.hy-receipt-undo[disabled]{cursor:default;opacity:.5}
.hy-receipt-undo:focus-visible{outline:2px solid rgba(var(--accent-rgb,94,106,210),.5);outline-offset:1px}
.hy-receipt-done{grid-column:3;grid-row:1;font-size:var(--text-xs,.75rem);color:var(--muted,#6d7080)}
.hy-receipt-undone{opacity:.55}
.hy-receipt-undone .hy-receipt-icon{color:var(--muted,#6d7080)}
`;

export function mount(el, source) {
  injectStyle('hy-receipt-style', CSS);
  el.classList.add('hy-receipt-host');
  let parsed = parseFields(source);
  let mode = 'static';

  function render() {
    const card = document.createElement('div');
    card.className = 'hy-receipt' + (mode === 'done' ? ' hy-receipt-undone' : '');
    const mark = document.createElement('span');
    mark.className = 'hy-receipt-icon';
    mark.append(lineIconNode('check'));
    const title = appendText(document.createElement('div'), parsed.field('title'));
    title.className = 'hy-receipt-title';
    card.append(mark, title);
    const undo = parsed.field('undo');
    if (undo && mode === 'done') {
      const done = document.createElement('span');
      done.className = 'hy-receipt-done';
      done.textContent = 'Undone';
      card.appendChild(done);
    } else if (undo && mode !== 'static') {
      const b = document.createElement('button');
      b.type = 'button';
      b.className = 'hy-receipt-undo';
      b.textContent = undo;
      b.disabled = mode !== 'active';
      b.addEventListener('click', () => el.dispatchEvent(new CustomEvent('hylo:choice', { bubbles: true, detail: { text: undo } })));
      card.appendChild(b);
    }
    if (parsed.items.length) {
      const ul = document.createElement('ul');
      ul.className = 'hy-receipt-lines';
      for (const t of parsed.items) ul.appendChild(appendText(document.createElement('li'), t));
      card.appendChild(ul);
    }
    el.replaceChildren(card);
  }

  el._hyReceipt = {
    undo: () => parsed.field('undo'),
    // mode: 'active' | 'busy' (shown, can't click yet) | 'done' | 'static' (no button)
    setState(next) {
      if (next.mode === mode) return;
      mode = next.mode || 'static';
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
      el.classList.remove('hy-receipt-host');
      delete el._hyReceipt;
    },
  };
}
