// A caret that becomes a long selection is rare and not reproducible. The
// transaction that did it is the evidence, so log that one and nothing else.
import { Transaction } from '@codemirror/state';
import { ViewPlugin } from '@codemirror/view';
import { scrollMeasureState } from './scroll.js';

const CHUNK = 120;
const FRESH = 2000;

export function selectionSpan(doc, range) {
  if (range.empty) return { lines: 1, chars: 0 };
  const end = range.to - 1;
  return {
    lines: doc.lineAt(end).number - doc.lineAt(range.from).number + 1,
    chars: range.to - range.from,
  };
}

function isLarge(span) {
  return span.lines > 1 || span.chars >= CHUNK;
}

export function selectionJumped(startDoc, prev, doc, next) {
  return !isLarge(selectionSpan(startDoc, prev)) && isLarge(selectionSpan(doc, next));
}

function samplePointer(event) {
  return {
    type: event.type,
    x: event.clientX,
    y: event.clientY,
    shift: event.shiftKey,
    detail: event.detail,
    buttons: event.buttons,
    at: performance.now(),
  };
}

function fresh(sample) {
  if (!sample) return null;
  const age = Math.round(performance.now() - sample.at);
  if (age > FRESH) return null;
  return {
    type: sample.type,
    x: sample.x,
    y: sample.y,
    shift: sample.shift,
    detail: sample.detail,
    buttons: sample.buttons,
    age,
  };
}

function freshKey(sample) {
  if (!sample) return null;
  const age = Math.round(performance.now() - sample.at);
  if (age > FRESH) return null;
  return { key: sample.key, shift: sample.shift, meta: sample.meta, ctrl: sample.ctrl, alt: sample.alt, age };
}

function place(doc, pos) {
  const line = doc.lineAt(Math.max(0, Math.min(pos, doc.length)));
  const col = pos - line.from;
  return { line: line.number, col, text: line.text.slice(Math.max(0, col - 8), col + 8) };
}

function dragStart(view) {
  const drag = view.inputState && view.inputState.mouseSelection;
  const event = drag && drag.startEvent;
  if (!event) return null;
  return { x: event.clientX, y: event.clientY, shift: event.shiftKey, detail: event.detail };
}

const selectionWatchPlugin = ViewPlugin.fromClass(class SelectionWatch {
  constructor() {
    this.openedAt = performance.now();
    this.pointer = null;
    this.down = null;
    this.key = null;
  }

  notePointer(event) {
    const sample = samplePointer(event);
    this.pointer = sample;
    if (event.type === 'mousedown') this.down = sample;
  }

  noteKey(event) {
    this.key = {
      key: event.key,
      shift: event.shiftKey,
      meta: event.metaKey,
      ctrl: event.ctrlKey,
      alt: event.altKey,
      at: performance.now(),
    };
  }

  update(update) {
    if (!update.selectionSet || update.transactions.some((tr) => tr.isUserEvent('select.agent'))) return;
    const prev = update.startState.selection.main;
    const next = update.state.selection.main;
    if (!selectionJumped(update.startState.doc, prev, update.state.doc, next)) return;
    const doc = update.state.doc;
    const span = selectionSpan(doc, next);
    const pointer = this.pointer;
    const payload = {
      userEvent: update.transactions.filter((tr) => tr.selection).map((tr) => tr.annotation(Transaction.userEvent)),
      before: { anchor: prev.anchor, head: prev.head },
      after: { anchor: next.anchor, head: next.head, lines: span.lines, chars: span.chars, doc: doc.length },
      from: place(doc, next.from),
      to: place(doc, Math.max(next.from, next.to - 1)),
      pointer: fresh(pointer),
      down: fresh(this.down),
      key: freshKey(this.key),
      drag: dragStart(update.view),
      sinceOpen: Math.round(performance.now() - this.openedAt),
      measure: scrollMeasureState(update.view),
    };
    const view = update.view;
    queueMicrotask(() => {
      let hitAtPointer = null;
      try {
        if (pointer && view.dom.isConnected) hitAtPointer = view.posAtCoords({ x: pointer.x, y: pointer.y });
      } catch {
        // The view can be torn down before this runs.
      }
      console.info('[选区] 光标变成了大段选区', { ...payload, hitAtPointer });
    });
  }
}, {
  eventObservers: {
    mousedown(event) { this.notePointer(event); },
    mouseup(event) { this.notePointer(event); },
    mousemove(event) { this.notePointer(event); },
    keydown(event) { this.noteKey(event); },
  },
});

export function selectionWatch() {
  return selectionWatchPlugin;
}
