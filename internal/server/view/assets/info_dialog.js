
  function infoDialogCtrl() {
    return window.hyloOverlay('info', {
      title: '',
      bodyHTML: '',
      closeLabel: 'Got it',
      isError: false,

      init() { window._infoDialogCtrl = this; },

      show(opts) {
        opts = opts || {};
        this.title     = opts.title      || '';
        this.bodyHTML   = opts.bodyHTML   || '';
        this.closeLabel = opts.closeLabel || 'Got it';
        this.isError    = !!opts.isError;
        var _self = this;
        this.openOverlay(function() { _self.close(); });
      },

      close() {
        this.closeOverlay();
        this.bodyHTML = '';
      },
    });
  }

  // Global helpers — call from any page that includes infoDialogHTML.
  window.showInfo = function(opts) {
    if (!window._infoDialogCtrl) return;
    window._infoDialogCtrl.show(opts);
  };

  window.showError = function(message, title) {
    window.showInfo({
      title:      title   || 'Error',
      bodyHTML:   '<p>' + (message || 'An unexpected error occurred.') + '</p>',
      closeLabel: 'OK',
      isError:    true,
    });
  };
