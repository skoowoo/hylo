
  function confirmDialogCtrl() {
    return window.hyloOverlay('confirm', {
      title: '',
      titleHTML: '',
      message: '',
      confirmLabel: 'Confirm',
      danger: true,
      threeButton: false,
      altLabel: '',
      altDanger: true,
      hasInput: false,
      inputValue: '',
      inputPlaceholder: '',
      _resolve: null,

      init() { window._confirmDialogCtrl = this; },

      show(opts) {
        opts = opts || {};
        this.titleHTML     = opts.titleHTML     || '';
        this.title         = this.titleHTML ? '' : (opts.title || 'Are you sure?');
        this.message       = opts.message       || '';
        this.confirmLabel  = opts.confirmLabel  || 'Confirm';
        this.altLabel      = opts.altLabel      || '';
        this.threeButton   = !!this.altLabel;
        this.altDanger     = opts.altDanger !== false;
        // opts.input {value, placeholder}: confirm resolves with the trimmed text.
        this.hasInput         = !!opts.input;
        this.inputValue       = this.hasInput ? (opts.input.value || '') : '';
        this.inputPlaceholder = this.hasInput ? (opts.input.placeholder || '') : '';
        if (this.threeButton) {
          this.danger = opts.danger === true;
        } else {
          this.danger = opts.danger !== false;
        }
        var _self = this;
        this.openOverlay(function() { _self.cancel(); });
        if (this.hasInput) this.$nextTick(function() { _self.$refs.input.focus(); _self.$refs.input.select(); });
        return new Promise(function(resolve) {
          window._confirmDialogCtrl._resolve = resolve;
        });
      },

      confirm() {
        this.closeOverlay();
        if (this._resolve) {
          this._resolve(this.hasInput ? this.inputValue.trim() : this.threeButton ? 'confirm' : true);
          this._resolve = null;
        }
      },
      chooseAlt() {
        this.closeOverlay();
        if (this._resolve) { this._resolve('alt'); this._resolve = null; }
      },
      cancel() {
        this.closeOverlay();
        if (this._resolve) { this._resolve(false); this._resolve = null; }
      },
    });
  }

  // Global helper — call from any page that includes confirmDialogHTML.
  window.showConfirm = function(opts) {
    if (!window._confirmDialogCtrl) return Promise.resolve(false);
    return window._confirmDialogCtrl.show(opts);
  };
