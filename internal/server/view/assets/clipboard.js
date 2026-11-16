(function (root) {
  async function writeText(text) {
    if (navigator.clipboard && typeof navigator.clipboard.writeText === 'function') {
      try {
        await navigator.clipboard.writeText(text);
        return;
      } catch (_) {}
    }

    var active = document.activeElement;
    var selection = document.getSelection();
    var ranges = [];
    for (var i = 0; selection && i < selection.rangeCount; i++) ranges.push(selection.getRangeAt(i).cloneRange());
    var start = active && active.selectionStart;
    var end = active && active.selectionEnd;
    var direction = active && active.selectionDirection;
    var field = document.createElement('textarea');
    field.value = text;
    field.readOnly = true;
    field.style.cssText = 'position:fixed;left:-9999px;top:0;opacity:0';
    document.body.appendChild(field);
    try {
      field.focus({ preventScroll: true });
      field.select();
      if (!document.execCommand('copy')) throw new Error('Could not copy to clipboard.');
    } finally {
      field.remove();
      if (active && active.isConnected) active.focus({ preventScroll: true });
      if (selection) {
        selection.removeAllRanges();
        ranges.forEach(function (range) { selection.addRange(range); });
      }
      if (typeof start === 'number' && active.setSelectionRange) active.setSelectionRange(start, end, direction);
    }
  }

  var clipboard = { writeText: writeText };
  if (typeof module !== 'undefined' && module.exports) module.exports = clipboard;
  else root.__hyloClipboard = clipboard;
})(typeof window !== 'undefined' ? window : globalThis);
