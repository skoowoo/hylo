  // Agent write channel: ops pushed by the server (MCP editor_type) are typed into
  // the live editor, then acked so the agent's tool call can return.
  var __hyloAgentQueue = Promise.resolve();
  var __hyloAgentCurrent = null;
  // Op ids this page has accepted; the server replays unacked ops after a reconnect.
  var __hyloAgentSeen = new Set();
  var __hyloAgentQueued = new Set();
  var __hyloAgentCancelled = new Set();
  // Survives SSE reconnects so the server keeps routing this page's ops and acks here.
  var __hyloAgentClient = (window.crypto && crypto.randomUUID) ? crypto.randomUUID() : String(Date.now()) + Math.random().toString(36).slice(2);

  function __hyloAgentPost(url, body) {
    return fetch(url, {method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify(body)});
  }

  // A lost ack leaves the agent's tool call waiting, so retry; a rejected one is resent without its payload.
  async function __hyloAgentAck(res) {
    for (var i = 0; i < 4; i++) {
      try {
        var r = await __hyloAgentPost('/api/editor/agent/ack', res);
        if (r.ok) return;
        if (r.status < 500) {
          if (!res.data) return;
          res = {id: res.id, ok: false, error: 'the editor reply was rejected (' + r.status + ')', chars: 0};
          continue;
        }
      } catch (_) {}
      await new Promise(function(ok) { setTimeout(ok, 500 * (i + 1)); });
    }
  }

  // Shows op.path in the editor, opening it first when needed.
  async function __hyloAgentOpenNote(op) {
    var s = __hyloEditor;
    var pane = window.__hyloContentPane;
    if (!pane) throw new Error('editor is not available on this page');
    var tab = __hyloEditorActiveTab();
    if (!(pane.contentPaneOpen && tab && tab.path === op.path && s.currentPath === op.path)) {
      await pane.openNoteInContentPane(op.path, __hyloStripMdExt(op.path.split('/').pop()));
    }
    await __hyloEnsureContentPaneEditor();
    if (s.currentPath !== op.path) throw new Error('could not open ' + op.path + ' in the editor');
    return s.view;
  }

  // Reading view is not editable, so the user could not type to interrupt; write in live
  // preview without touching the saved preference.
  function __hyloAgentSetReading(on) {
    var s = __hyloEditor;
    s.view.dispatch({effects: s._modeEffects(false, on)});
    __hyloEditorSetModeState(false, on);
  }

  async function __hyloAgentRunOp(op) {
    __hyloAgentQueued.delete(op.id);
    if (__hyloAgentCancelled.delete(op.id)) return;
    var s = __hyloEditor;
    var ctrl = new AbortController();
    var onKey = null;
    var view = null;
    var wasReading = false;
    __hyloAgentCurrent = { id: op.id, ctrl: ctrl };
    try {
      view = await __hyloAgentOpenNote(op);
      if (ctrl.signal.aborted) return void __hyloAgentAck({id: op.id, ok: false, interrupted: true, chars: 0});

      if (op.kind === 'read') {
        await __hyloAgentAck({id: op.id, ok: true, data: __hyloCM.readAgentDoc(view.state)});
        return;
      }

      var plan = op.kind === 'rewrite'
        ? __hyloCM.planAgentRewrite(view.state, {target: op.target, expect: op.expect, fromText: op.from_text, toText: op.to_text}, op.content || '')
        : __hyloCM.planAgentInsert(view.state, {at: op.at, heading: op.heading}, op.content);
      if (plan.error) throw new Error(plan.error);

      wasReading = s.readingActive;
      if (wasReading) __hyloAgentSetReading(false);

      // Typing hands control back to the user. A shortcut only does if it edited the
      // note, so copying or saving mid-typing doesn't stop the agent.
      onKey = function(e) {
        if (e.isComposing || e.key === 'Shift' || e.key === 'Meta' || e.key === 'Control' || e.key === 'Alt') return;
        if (!e.metaKey && !e.ctrlKey) return void ctrl.abort();
        var before = __hyloCM.foreignEditCount(view.state);
        setTimeout(function() { if (__hyloCM.foreignEditCount(view.state) !== before) ctrl.abort(); }, 0);
      };
      view.contentDOM.addEventListener('keydown', onKey, true);

      var res = await __hyloCM.agentType(view, plan, {
        speed: op.speed,
        signal: ctrl.signal,
        isCurrent: function() { return s.view === view && s.currentPath === op.path; },
      });
      await __hyloAgentAck({id: op.id, ok: res.ok, interrupted: res.interrupted, error: res.interrupted ? '' : (res.error || ''), chars: res.chars});
    } catch (e) {
      await __hyloAgentAck({id: op.id, ok: false, error: (e && e.message) || 'editor error', chars: 0});
    } finally {
      if (onKey && view) view.contentDOM.removeEventListener('keydown', onKey, true);
      if (wasReading && s.reading && s.view === view && s.currentPath === op.path && !s.readingActive && !s.inSource) {
        __hyloAgentSetReading(true);
      }
      __hyloAgentCurrent = null;
    }
  }

  function __hyloAgentFocus() {
    if (document.visibilityState !== 'visible') return;
    __hyloAgentPost('/api/editor/agent/focus', {client: __hyloAgentClient}).catch(function() {});
  }

  function __hyloAgentInit() {
    if (typeof EventSource === 'undefined' || window.__hyloAgentES) return;
    var es = new EventSource('/api/editor/agent?client=' + encodeURIComponent(__hyloAgentClient));
    window.__hyloAgentES = es;
    es.addEventListener('open', function() { if (document.hasFocus()) __hyloAgentFocus(); });
    window.addEventListener('focus', __hyloAgentFocus);
    es.addEventListener('op', function(e) {
      var op; try { op = JSON.parse(e.data); } catch (_) { return; }
      if (__hyloAgentSeen.has(op.id)) return;
      __hyloAgentSeen.add(op.id);
      if (__hyloAgentSeen.size > 500) __hyloAgentSeen.delete(__hyloAgentSeen.values().next().value);
      __hyloAgentQueued.add(op.id);
      __hyloAgentQueue = __hyloAgentQueue.then(function() { return __hyloAgentRunOp(op); });
    });
    es.addEventListener('cancel', function(e) {
      var d; try { d = JSON.parse(e.data); } catch (_) { return; }
      if (__hyloAgentCurrent && __hyloAgentCurrent.id === d.id) __hyloAgentCurrent.ctrl.abort();
      else if (__hyloAgentQueued.has(d.id)) __hyloAgentCancelled.add(d.id);
    });
  }
  __hyloAgentInit();
