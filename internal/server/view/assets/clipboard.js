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

  async function imageCanvas(src) {
    var response = await fetch(src);
    if (!response.ok) throw new Error('Could not load image (' + response.status + ').');
    var url = URL.createObjectURL(await response.blob());
    try {
      var img = new Image();
      img.src = url;
      await img.decode();
      var canvas = document.createElement('canvas');
      canvas.width = img.naturalWidth;
      canvas.height = img.naturalHeight;
      var context = canvas.getContext('2d');
      if (!context) throw new Error('Could not prepare image for copying.');
      context.drawImage(img, 0, 0);
      return canvas;
    } finally {
      URL.revokeObjectURL(url);
    }
  }

  async function imagePNG(src) {
    var canvas = await imageCanvas(src);
    return new Promise(function (resolve, reject) {
      canvas.toBlob(function (blob) {
        if (blob) resolve(blob);
        else reject(new Error('Could not encode image for copying.'));
      }, 'image/png');
    });
  }

  async function writeImage(src) {
    var native = root.hyloDesktop && root.hyloDesktop.clipboard;
    if (native && native.writeImage) {
      var png = await imagePNG(src);
      return native.writeImage(Array.from(new Uint8Array(await png.arrayBuffer())));
    }
    if (!navigator.clipboard || !navigator.clipboard.write || typeof ClipboardItem === 'undefined') {
      return Promise.reject(new Error('Image copying is unavailable in this browser. Open Hylo in the desktop app or a secure browser context.'));
    }
    // Start the clipboard operation before asynchronous loading consumes the user gesture.
    return navigator.clipboard.write([new ClipboardItem({ 'image/png': imagePNG(src) })]);
  }

  async function copyImage(src, button) {
    var title = button && (button.getAttribute('aria-label') || button.title);
    if (button) {
      clearTimeout(button._copyImageTimer);
      button.disabled = true;
    }
    try {
      await writeImage(src);
      if (button) {
        button.title = 'Copied';
        button._copyImageTimer = setTimeout(function () { button.title = title; }, 1500);
      }
    } catch (err) {
      if (root.showError) root.showError(err.message || String(err), 'Copy image failed');
    } finally {
      if (button) button.disabled = false;
    }
  }

  var clipboard = { writeText: writeText, writeImage: writeImage, copyImage: copyImage };
  if (typeof module !== 'undefined' && module.exports) module.exports = clipboard;
  else root.__hyloClipboard = clipboard;
})(typeof window !== 'undefined' ? window : globalThis);
