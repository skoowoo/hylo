if (typeof marked !== 'undefined') { marked.setOptions({ gfm: true, breaks: true }); }
document.addEventListener('alpine:init', () => {
  Alpine.store('settingsModal', { open: false });
  Alpine.store('servers', hyloServersStore());
});
