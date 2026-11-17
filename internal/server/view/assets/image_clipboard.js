(function () {
  document.addEventListener('click', function (event) {
    var button = event.target.closest && event.target.closest('[data-copy-image]');
    if (!button || button.disabled) return;
    var src = button.dataset.copyImage;
    if (!src) return;
    event.preventDefault();
    event.stopPropagation();
    void window.__hyloClipboard.copyImage(src, button);
  }, true);
})();
