  // The note in front of the user, reported as a visit once it was read for
  // 15s or edited. Sampled rather than hooked: the open note changes in many
  // places. Every 5s is enough against a 15s threshold; leaving the window is
  // caught at once by its own events.
  var __HYLO_VISIT_MIN_MS = 15000;
  var __HYLO_VISIT_SAMPLE_MS = 5000;
  // No input for this long is someone away from a window left open: the visit ends.
  var __HYLO_VISIT_IDLE_MS = 5 * 60 * 1000;

  function __hyloVisitReport(v) {
    return {path: v.path, action: v.edited ? 'write' : 'read', at: v.last};
  }

  // Moves visit v to what is in front of the user at `now` ('' for nothing)
  // and returns the reports that are due. Reported once it counts, so a crash
  // or quit still leaves that touch, and again when it ends.
  function __hyloVisitStep(v, path, now, edited) {
    var out = [];
    if (path !== v.path) {
      if (v.path && (v.edited || v.last - v.since >= __HYLO_VISIT_MIN_MS)) out.push(__hyloVisitReport(v));
      v.path = path; v.since = now; v.edited = false; v.sent = false;
    }
    if (!path) return out;
    v.last = now;
    if (edited && !v.edited) { v.edited = true; v.sent = false; }
    if (!v.sent && (v.edited || now - v.since >= __HYLO_VISIT_MIN_MS)) { v.sent = true; out.push(__hyloVisitReport(v)); }
    return out;
  }

  var __hyloVisit = { path: '', since: 0, last: 0, edited: false, sent: false };
  var __hyloVisitInputAt = Date.now();

  // Only a focused window someone is using counts; the agent's turns in the
  // editor are its own activity, not the user's.
  function __hyloVisitViewing() {
    var pane = window.__hyloContentPane;
    if (document.visibilityState !== 'visible' || !document.hasFocus() || Date.now() - __hyloVisitInputAt > __HYLO_VISIT_IDLE_MS) return '';
    if (!pane || !pane.contentPaneOpen || pane.agentWriting) return '';
    var tab = pane.tabs[pane.activeTab];
    var path = __hyloEditor.currentPath;
    return tab && tab.path === path && /\.md$/i.test(path) ? path : '';
  }

  function __hyloVisitPost(r) {
    fetch('/api/activity/visit', {
      method: 'POST', headers: {'Content-Type': 'application/json'}, keepalive: true, body: JSON.stringify(r),
    }).catch(function() {});
  }

  function __hyloVisitTick(edited, leaving) {
    var path = leaving === true ? '' : __hyloVisitViewing();
    __hyloVisitStep(__hyloVisit, path, Date.now(), edited === true).forEach(__hyloVisitPost);
  }

  function __hyloVisitEdited() { __hyloVisitInputAt = Date.now(); __hyloVisitTick(true); }

  // A write the user made outside the open note's edits: a short, or a new
  // note's first save, which lands before the note has a path to visit.
  function __hyloVisitWrote(path) {
    if (!path) return;
    if (path === __hyloVisit.path) return __hyloVisitEdited();
    __hyloVisitPost({path: path, action: 'write', at: Date.now()});
  }

  // The server moves the visit's rows to the new path; later reports follow it.
  function __hyloVisitRenamed(oldPath, newPath) {
    if (__hyloVisit.path === oldPath) __hyloVisit.path = newPath;
  }

  if (typeof document !== 'undefined') {
    setInterval(__hyloVisitTick, __HYLO_VISIT_SAMPLE_MS);
    document.addEventListener('visibilitychange', __hyloVisitTick);
    window.addEventListener('focus', __hyloVisitTick);
    window.addEventListener('blur', __hyloVisitTick);
    ['pointerdown', 'pointermove', 'keydown', 'wheel'].forEach(function(type) {
      document.addEventListener(type, function() { __hyloVisitInputAt = Date.now(); }, {capture: true, passive: true});
    });
    // The page can still read as visible while it unloads.
    window.addEventListener('pagehide', function() { __hyloVisitTick(false, true); });
  }
  if (typeof module !== 'undefined') {
    module.exports = { __hyloVisitStep };
  }
