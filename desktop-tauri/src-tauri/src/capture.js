// Runs before the Go page. That page only talks to window.quickCapture.
(function () {
  var listeners = [];
  var pending = null;

  function invoke(cmd, args) {
    var internals = window.__TAURI_INTERNALS__;
    if (internals && internals.invoke) return internals.invoke(cmd, args || {});
    return Promise.reject(new Error("tauri ipc unavailable"));
  }

  window.quickCapture = {
    onInit: function (cb) {
      if (pending) cb(pending);
      else listeners.push(cb);
    },
    submit: function (message) {
      invoke("quick_capture_submit", { message: message });
    },
    save: function () {
      invoke("quick_capture_save");
    },
    cancel: function () {
      invoke("quick_capture_cancel");
    },
  };

  window.__hyloQuickCaptureInit = function (data) {
    pending = data;
    var queued = listeners;
    listeners = [];
    for (var i = 0; i < queued.length; i++) queued[i](data);
  };

  // Older servers still cancel on a click that misses the composer and buttons.
  document.addEventListener("click", function (e) {
    var el = e.target;
    if (el && el.closest && el.closest("#composer, #save-btn, #send-btn")) return;
    e.stopImmediatePropagation();
  }, true);

  // Fully transparent pixels fall through a non-opaque window and look like
  // a click outside. A hair of alpha keeps the hit inside the panel.
  if (document.documentElement) {
    var hit = document.createElement("style");
    hit.textContent = "html,body,#panel{background-color:rgba(0,0,0,0.01)!important}";
    document.documentElement.appendChild(hit);
  }
})();
