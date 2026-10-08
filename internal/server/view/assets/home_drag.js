// ── Drag a note card onto a sidebar folder to move it ──────────────────────
// Folder-view only — Knowledge/Shorts have their own
// list views and no drop path here, by design (backend Vault.MoveNote has no
// such restriction; this is a UI-only choice). Cards are htmx-swapped in/out
// of #home-list-pane on every section/page change, so this is delegated on
// document rather than bound per-card — a per-card listener would silently
// stop working the moment htmx replaces the card it was attached to.
var __hyloDragSourcePath = null;
var __hyloDragSourceCard = null;
var __hyloDragOverTarget = null;

function __hyloDragNoteEligible(card) {
  return !!(card && card.dataset.notePath &&
    card.dataset.noteIsKnowledge !== 'true' && card.dataset.noteIsIndex !== 'true');
}

// Reflects the current section onto every visible card's draggable property
// (not just gating it at dragstart) so the cursor/affordance is honest —
// hovering a card outside folder view no longer shows a grab cursor for a
// drag that was never going to start. Native `draggable` is a reflected
// attribute, so home.css's [draggable="true"] selectors track this for free.
// Called from __hyloSectionArrived, so after every section arrival.
function __hyloUpdateDraggableCards() {
  var pane = document.getElementById('home-list-pane');
  if (!pane) return;
  var isFolderView = !!(window._homeData && window._homeData.curType() === 'folder');
  var cards = pane.querySelectorAll('.home-note-row');
  for (var i = 0; i < cards.length; i++) {
    cards[i].draggable = isFolderView && __hyloDragNoteEligible(cards[i]);
  }
}

document.addEventListener('dragstart', function (e) {
  var card = e.target.closest ? e.target.closest('.home-note-row') : null;
  if (!card) return;
  // A card mid-move (is-move-pending) is also pointer-events:none (home.css),
  // so it can't be the drag source here — this check is defense in depth.
  if (!card.draggable || card.classList.contains('is-move-pending')) { e.preventDefault(); return; }
  __hyloDragSourcePath = card.dataset.notePath;
  __hyloDragSourceCard = card;
  e.dataTransfer.effectAllowed = 'move';
  try { e.dataTransfer.setData('text/plain', __hyloDragSourcePath); } catch (_) { /* ignore */ }
  card.classList.add('is-dragging');
  // Without this, the drag image defaults to a full-size snapshot of the
  // card (as wide as the whole list) — swap in a small title-only chip that
  // sizes to its own text instead. Has to be in the DOM (even off-screen)
  // for setDragImage to snapshot it, then comes back out next tick.
  var ghost = document.createElement('div');
  ghost.className = 'home-drag-ghost';
  ghost.textContent = card.dataset.noteTitle || 'Note';
  document.body.appendChild(ghost);
  e.dataTransfer.setDragImage(ghost, 12, 14);
  setTimeout(function () { ghost.remove(); }, 0);
});

document.addEventListener('dragend', function () {
  // Only clears the *gesture* state — if drop actually kicked off a move,
  // the card keeps is-move-pending (added there) until the request settles,
  // so a second drag can't fire a second move on the same file mid-flight.
  var card = __hyloDragSourceCard;
  if (card && !card.classList.contains('is-move-pending')) card.classList.remove('is-dragging');
  if (__hyloDragOverTarget) { __hyloDragOverTarget.classList.remove('drag-over'); __hyloDragOverTarget = null; }
  __hyloDragSourcePath = null;
  __hyloDragSourceCard = null;
});

// Per spec, both dragenter and dragover need preventDefault() for an element
// to register as a valid drop target — dragover alone is enough in most
// browsers but not guaranteed, so both get the same handling.
document.addEventListener('dragenter', function (e) {
  if (!__hyloDragSourcePath) return;
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  if (target) e.preventDefault();
});

document.addEventListener('dragover', function (e) {
  if (!__hyloDragSourcePath) return;
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  if (target !== __hyloDragOverTarget) {
    if (__hyloDragOverTarget) __hyloDragOverTarget.classList.remove('drag-over');
    if (target) target.classList.add('drag-over');
    __hyloDragOverTarget = target;
  }
  if (target) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }
});

document.addEventListener('drop', function (e) {
  if (!__hyloDragSourcePath) return;
  // Always suppress the browser's own drop handling while our drag is live —
  // dropping outside a valid target (e.g. onto the open editor) would
  // otherwise insert the dragged path as plain text into it.
  e.preventDefault();
  var target = e.target.closest ? e.target.closest('.home-drop-target') : null;
  var sourcePath = __hyloDragSourcePath;
  var card = __hyloDragSourceCard;
  var hoverTarget = __hyloDragOverTarget;
  __hyloDragOverTarget = null;
  __hyloDragSourcePath = null;
  if (!target) { if (hoverTarget) hoverTarget.classList.remove('drag-over'); __hyloShakeCard(card, false); return; }
  var targetDir = target.dataset.dropDir;
  var slash = sourcePath.lastIndexOf('/');
  var currentDir = slash <= 0 ? '/' : sourcePath.slice(0, slash);
  if (currentDir === targetDir) { target.classList.remove('drag-over'); __hyloShakeCard(card, false); return; }
  if (card) { card.classList.remove('is-dragging'); card.classList.add('is-move-pending'); }
  // .drag-over stays through the request itself (its breathing glow doubles
  // as "still working on it") — __hyloDragMoveNote swaps it for a receipt
  // bounce on success or clears it on failure.
  void __hyloDragMoveNote(sourcePath, targetDir, card, target);
});

// Plain "that didn't do anything" feedback — dropped outside any folder, or
// back into the folder the note is already in. rejected=false, no color.
function __hyloShakeCard(card, rejected) {
  if (!card) return;
  card.classList.remove('is-dragging', 'is-move-pending');
  var cls = rejected ? 'is-move-rejected' : 'is-move-noop';
  card.classList.add(cls);
  // 500ms comfortably outlasts both animations home.css puts on these
  // classes (home-shake: --motion-base*2.5 = 400ms; the rejected variant's
  // extra home-reject-flash: --motion-base*3 = 480ms).
  setTimeout(function () { card.classList.remove(cls); }, 500);
}

// Move actually landed — flies the card toward the drop target and shrinks
// it away, then removes it from the DOM itself rather than waiting for
// reloadActiveSection()'s round trip to do it. Also gives the target a quick
// receipt bounce. Safe to no-op if either element isn't in the document any
// more (e.g. the section changed mid-request).
function __hyloFlyCardToTarget(card, targetEl) {
  if (targetEl && targetEl.isConnected) {
    targetEl.classList.remove('drag-over');
    targetEl.classList.add('just-received');
    // home-drop-received runs --motion-base*2 = 320ms — 340ms clears a beat after.
    setTimeout(function () { targetEl.classList.remove('just-received'); }, 340);
  }
  if (!card || !card.isConnected) return;
  card.classList.remove('is-move-pending');
  if (!targetEl || !targetEl.isConnected) { card.remove(); return; }
  var cardRect = card.getBoundingClientRect();
  var targetRect = targetEl.getBoundingClientRect();
  card.style.setProperty('--fly-dx', ((targetRect.left + targetRect.width / 2) - (cardRect.left + cardRect.width / 2)) + 'px');
  card.style.setProperty('--fly-dy', ((targetRect.top + targetRect.height / 2) - (cardRect.top + cardRect.height / 2)) + 'px');
  card.classList.add('is-move-success');
  var done = false;
  var remove = function () { if (done) return; done = true; card.remove(); };
  card.addEventListener('transitionend', remove, { once: true });
  // .is-move-success's longer transition (transform) runs --motion-base*2 =
  // 320ms — 400ms is the fallback in case transitionend never fires.
  setTimeout(remove, 400);
}

async function __hyloDragMoveNote(sourcePath, targetDir, card, targetEl) {
  if (window.__hyloContentPane) await window.__hyloContentPane.flushPendingSaveFor(sourcePath);
  var resp;
  try {
    resp = await fetch('/api/vault/move', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path: sourcePath, newDir: targetDir }),
    });
  } catch (e) {
    if (targetEl) targetEl.classList.remove('drag-over');
    if (card) { card.classList.remove('is-move-pending'); __hyloShakeCard(card, true); }
    if (window.showError) window.showError((e && e.message) || 'Network error.', 'Cannot move');
    return;
  }
  if (!resp.ok) {
    if (targetEl) targetEl.classList.remove('drag-over');
    if (card) { card.classList.remove('is-move-pending'); __hyloShakeCard(card, true); }
    var msg = (await resp.text()).trim() || 'Move failed.';
    if (window.showError) window.showError(resp.status === 409 ? 'A note already exists at that location.' : msg, 'Cannot move');
    return;
  }
  var data = await resp.json();
  if (window.__hyloContentPane) window.__hyloContentPane.noteMoved(sourcePath, data.path);
  __hyloFlyCardToTarget(card, targetEl);
  // The refetch must not swap the list out from under the fly-out animation.
  setTimeout(function () {
    if (window.__hyloVaultChanged) window.__hyloVaultChanged({ op: 'move' });
  }, 420);
}
