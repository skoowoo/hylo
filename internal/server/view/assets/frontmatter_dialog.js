
  function frontmatterDialogCtrl() {
    return window.hyloOverlay('frontmatter-edit', {
      text: '',
      _onSave: null,

      init() { window._frontmatterDialogCtrl = this; },

      show(initialText, onSave) {
        this.text = initialText || '';
        this._onSave = onSave || null;
        var self = this;
        this.openOverlay(function() { self.cancel(); });
        this.$nextTick(function() {
          if (self.$refs.textarea) { self.$refs.textarea.focus(); self.$refs.textarea.setSelectionRange(0, 0); }
        });
      },

      save() {
        var onSave = this._onSave;
        var text = this.text;
        this.closeOverlay();
        this._onSave = null;
        if (onSave) onSave(text);
      },

      cancel() {
        this.closeOverlay();
        this._onSave = null;
      },
    });
  }

  // Global helper — call from content_pane.js (or any page that includes
  // frontmatterDialogHTML). onSave receives the edited text verbatim.
  window.__hyloEditFrontmatter = function(initialText, onSave) {
    if (!window._frontmatterDialogCtrl) return;
    window._frontmatterDialogCtrl.show(initialText, onSave);
  };
