// Page-lifetime chat session. Session owns the transcript, transport owns
// SSE and the two pollers, and a single phase (idle | sending | streaming |
// stopping) is the only busy flag. The view attaches only while the pane is
// in the DOM; a run keeps going when the pane is swapped away.
(function (root) {
  var R = root.__vaultrChatReduce;

  function rank(mode) {
    return mode === 'jump' ? 2 : mode === 'maybe' ? 1 : 0;
  }

  root.__vaultrChatCreate = function (host) {
    host = host || {};
    var mates = [];
    var eventDefs = [];
    var catalogEpoch = 0;
    var matesReady = Promise.resolve();
    var bootstrapped = false;

    var mateId = '';
    var convType = 'chat';
    var conversationId = '';
    var messages = [];
    var lastMsgMs = 0;
    var draft = '';
    var stick = true;
    // Set when insertPath/insertWikilink land on the draft fallback (no view
    // mounted yet, e.g. a note handed to a bot before its chat panel has
    // loaded) — attach() consumes it to focus the composer once the
    // textarea actually exists, so the reference doesn't just sit there
    // unfocused for the user to go click into themselves.
    var focusAfterAttach = false;

    var phase = 'idle';
    var run = null; // { id, assistantMsgId } while streaming or stopping
    var stoppingMsgId = ''; // stays until that message leaves `running`

    var refreshSeq = 0;
    var runCtrl = { seq: 0, timer: null, interval: 5000, firstDelay: 3000 };
    var syncCtrl = { seq: 0, timer: null, interval: 5000, firstDelay: 5000 };
    var syncingConv = '';
    var pollError = {};

    var view = null;
    var raf = 0;
    var pending = 'none';
    var heldScroll = 'none';

    function mateById(id) {
      for (var i = 0; i < mates.length; i++) if (mates[i].id === id) return mates[i];
      return null;
    }

    function mateName(msg) {
      if (!msg) return 'Agent';
      var m = mateById(msg.agentBotId);
      return m ? m.name : (msg.agentId || 'Agent');
    }

    function triggerLabel(type) {
      for (var i = 0; i < eventDefs.length; i++) {
        if (eventDefs[i].type === type) return eventDefs[i].label;
      }
      return type;
    }

    function findMsg(id) {
      for (var i = 0; i < messages.length; i++) if (messages[i].id === id) return messages[i];
      return null;
    }

    function notify(text, kind) {
      if (host.onToast) host.onToast(text, kind || 'err');
    }

    function reportPoll(key, text) {
      if (pollError[key]) return;
      pollError[key] = true;
      notify(text, 'err');
    }

    function clearPoll(key) { pollError[key] = false; }

    function toastFor(status, msg) {
      var label = status === 'succeeded' ? ' finished' : (status === 'canceled' ? ' stopped' : ' failed');
      notify(mateName(msg) + label, status === 'failed' ? 'err' : 'ok');
    }

    function busy() { return phase !== 'idle'; }
    function liveRun() { return phase === 'streaming' || phase === 'stopping'; }

    function activeStoppingId() {
      if (!stoppingMsgId) return '';
      var msg = findMsg(stoppingMsgId);
      if (!msg || msg.status !== 'running') {
        stoppingMsgId = '';
        return '';
      }
      return stoppingMsgId;
    }

    function requestRender(mode) {
      if (rank(mode) > rank(pending)) pending = mode;
      if (!view) {
        if (rank(pending) > rank(heldScroll)) heldScroll = pending;
        pending = 'none';
        return;
      }
      if (raf) return;
      raf = requestAnimationFrame(flush);
    }

    function flush() {
      raf = 0;
      var mode = pending;
      pending = 'none';
      if (!view) {
        if (rank(mode) > rank(heldScroll)) heldScroll = mode;
        return;
      }
      if (mode === 'jump') stick = true;
      view.render(mode);
    }

    function state() {
      return {
        mateId: mateId,
        mate: mateById(mateId),
        mates: mates,
        convType: convType,
        messages: messages,
        catalogEpoch: catalogEpoch,
        draft: draft,
        phase: phase,
        busy: busy(),
        runId: run ? run.id : null,
        stoppingId: activeStoppingId(),
        stick: stick,
        triggerLabel: triggerLabel,
        mateName: mateName,
      };
    }

    function stopLoop(ctrl) {
      ctrl.seq++;
      if (ctrl.timer) { clearTimeout(ctrl.timer); ctrl.timer = null; }
    }

    // tick(seq) returns false to stop, or a delay in ms. A throw retries
    // after ctrl.interval and surfaces once through ctrl.onError.
    function loop(ctrl, tick) {
      var seq = ctrl.seq;
      function arm(delay) {
        ctrl.timer = setTimeout(function () {
          ctrl.timer = null;
          if (ctrl.seq !== seq) return;
          Promise.resolve()
            .then(function () { return tick(seq); })
            .then(function (next) {
              if (ctrl.seq !== seq || next === false) return;
              arm(typeof next === 'number' ? next : ctrl.interval);
            }, function () {
              if (ctrl.seq !== seq) return;
              if (ctrl.onError) ctrl.onError();
              arm(ctrl.interval);
            });
        }, delay);
      }
      arm(ctrl.firstDelay);
    }

    async function fetchJSON(url) {
      var resp = await fetch(url, { cache: 'no-store' });
      if (!resp.ok) return null;
      return resp.json();
    }

    async function loadMates() {
      try {
        var resp = await fetch('/api/mates');
        if (!resp.ok) { notify("Couldn't load agent bots"); return; }
        mates = ((await resp.json()).mates || []).filter(function (m) { return m.enabled; });
        catalogEpoch++;
        if (host.onMates) host.onMates(mates);
        requestRender('none');
      } catch (e) { notify("Couldn't load agent bots"); }
    }

    async function loadEvents() {
      try {
        var resp = await fetch('/api/mate-events');
        if (!resp.ok) { notify("Couldn't load triggers"); return; }
        eventDefs = (await resp.json()).events || [];
        catalogEpoch++;
        requestRender('none');
      } catch (e) { notify("Couldn't load triggers"); }
    }

    function cancelRunPoller() { stopLoop(runCtrl); }

    function cancelSync() {
      stopLoop(syncCtrl);
      syncingConv = '';
    }

    runCtrl.onError = function () { reportPoll('run', "Couldn't check the run"); };

    function spawnRunPoller(runId, msgId) {
      cancelRunPoller();
      var n = 0;
      loop(runCtrl, async function (seq) {
        if (n++ > 720) return false;
        var info = await fetchJSON('/api/runs/' + runId);
        if (runCtrl.seq !== seq) return false;
        if (!info) {
          reportPoll('run', "Couldn't check the run");
          return 5000;
        }
        clearPoll('run');
        var s = info.status;
        if (s === 'succeeded' || s === 'failed' || s === 'canceled') {
          var msg = findMsg(msgId);
          if (msg && msg.status === 'running') {
            msg.status = s;
            msg.completedAt = info.updatedAt || Date.now();
            R.touch(msg);
            requestRender('maybe');
          }
          toastFor(s, msg);
          return false;
        }
        return 5000;
      });
    }

    function ensureSync(convId) {
      if (!convId || syncingConv === convId) return;
      startSync(convId);
    }

    syncCtrl.onError = function () { reportPoll('sync', "Couldn't refresh the chat"); };

    function startSync(convId) {
      cancelSync();
      syncingConv = convId;
      loop(syncCtrl, async function (seq) {
        if (liveRun() || !convId || document.visibilityState !== 'visible') return 5000;
        var mateNow = mateId;
        var typeNow = convType;
        var data = await fetchJSON('/api/conversations?mateId=' + encodeURIComponent(mateNow) + '&type=' + encodeURIComponent(typeNow));
        if (syncCtrl.seq !== seq) return false;
        var convs = (data && data.conversations) || [];
        if (convs.length > 0 && convs[0].id !== convId) {
          if (liveRun()) return 5000;
          void refresh();
          return false;
        }
        var data2 = await fetchJSON('/api/conversations/' + convId + '?since=' + lastMsgMs);
        if (syncCtrl.seq !== seq) return false;
        if (liveRun() || convId !== conversationId) return 5000;
        if (!data2) {
          reportPoll('sync', "Couldn't refresh the chat");
          return 5000;
        }
        clearPoll('sync');
        var msgs = data2.messages || [];
        if (msgs.length > 0) {
          var changed = R.mergeSynced(messages, msgs);
          var lastTs = 0;
          for (var j = 0; j < msgs.length; j++) {
            var t = msgs[j].updatedAt ? new Date(msgs[j].updatedAt).getTime() : 0;
            if (t > lastTs) lastTs = t;
          }
          if (lastTs > lastMsgMs) lastMsgMs = lastTs;
          if (changed) requestRender('maybe');
        }
        return 5000;
      });
    }

    async function loadMessages(convId, seq) {
      try {
        var resp = await fetch('/api/conversations/' + convId, { cache: 'no-store' });
        if (!resp.ok) { notify("Couldn't load the conversation"); return; }
        if (seq !== refreshSeq || convId !== conversationId) return;
        var msgs = (await resp.json()).messages || [];
        if (seq !== refreshSeq || convId !== conversationId) return;
        messages = R.formatStoredMessages(msgs);
        var lastMs = 0;
        for (var i = 0; i < msgs.length; i++) {
          var t = msgs[i].updatedAt ? new Date(msgs[i].updatedAt).getTime() : 0;
          if (t > lastMs) lastMs = t;
        }
        lastMsgMs = lastMs;
        startSync(convId);
        requestRender('jump');
      } catch (e) { notify("Couldn't load the conversation"); }
    }

    async function refresh() {
      if (busy() || !mateId) return;
      var seq = ++refreshSeq;
      try {
        var resp = await fetch('/api/conversations?mateId=' + encodeURIComponent(mateId) + '&type=' + encodeURIComponent(convType), { cache: 'no-store' });
        if (seq !== refreshSeq) return;
        if (!resp.ok) { notify("Couldn't load the conversation"); return; }
        var convs = (await resp.json()).conversations || [];
        if (seq !== refreshSeq) return;
        conversationId = convs.length > 0 ? convs[0].id : '';
        if (conversationId) {
          await loadMessages(conversationId, seq);
        } else {
          messages = [];
          cancelSync();
          requestRender('none');
        }
      } catch (e) { notify("Couldn't load the conversation"); }
    }

    async function syncLastFromDB(convId, msg) {
      try {
        var resp = await fetch('/api/conversations/' + convId, { cache: 'no-store' });
        if (!resp.ok) { notify("Couldn't sync the reply"); return; }
        var dbMsgs = (await resp.json()).messages || [];
        var dbMsg = null;
        for (var i = dbMsgs.length - 1; i >= 0; i--) {
          if (dbMsgs[i].role === 'assistant' && dbMsgs[i].content) { dbMsg = dbMsgs[i]; break; }
        }
        if (!dbMsg || !msg || msg.role !== 'assistant') return;
        R.replaceTextFromDB(msg, dbMsg.content);
        requestRender('maybe');
      } catch (e) { notify("Couldn't sync the reply"); }
    }

    async function consumeSSE(resp, msg) {
      var reader = resp.body.getReader();
      var decoder = new TextDecoder();
      var buf = '';
      var gotEnd = false;
      try {
        while (true) {
          var chunk = await reader.read();
          if (chunk.done) break;
          buf += decoder.decode(chunk.value, { stream: true });
          var fed = R.feedSSE(buf, function (event, raw) {
            if (event === 'heartbeat') return;
            var data;
            try { data = JSON.parse(raw); } catch (e) { return; }
            var out = R.applySSE(msg, event, data);
            if (out.runId && run) run.id = out.runId;
            if (event === 'start') {
              requestRender('maybe');
              return;
            }
            if (out.ended && document.hidden) toastFor(out.status, msg);
            R.touch(msg);
            requestRender('maybe');
          });
          buf = fed.buf;
          if (fed.gotEnd) gotEnd = true;
        }
      } finally {
        try { reader.releaseLock(); } catch (e) { /* ignore */ }
      }
      return gotEnd;
    }

    // Agents whose streamed text is not the body the DB will keep.
    var finishers = { 'cursor-agent': syncLastFromDB };

    async function runTurn(text, userMsgId, mate) {
      if (!mate) return;
      var assistant = {
        id: R.genId(),
        role: 'assistant', agentId: mate.agentId, agentBotId: mate.id,
        segments: [], status: 'running',
        startTime: Date.now(), duration: 0, completedAt: 0,
        rev: 1,
      };
      messages.push(assistant);
      phase = 'streaming';
      run = { id: null, assistantMsgId: assistant.id };
      requestRender('jump');
      var sseGotEnd = true;
      try {
        var resp = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            mateId: mate.id,
            message: text,
            conversationId: conversationId,
            userMessageId: userMsgId,
            assistantMessageId: assistant.id,
          }),
        });
        if (!resp.ok) {
          var errText = await resp.text();
          assistant.segments.push({ type: 'error', message: errText || 'Request failed' });
          assistant.status = 'failed';
          R.touch(assistant);
          return;
        }
        sseGotEnd = await consumeSSE(resp, assistant);
        var finish = finishers[mate.agentId];
        if (sseGotEnd && conversationId && finish) await finish(conversationId, assistant);
      } catch (e) {
        if (messages.indexOf(assistant) !== -1) {
          assistant.segments.push({ type: 'error', message: (e && e.message) || 'Connection error' });
          assistant.status = 'failed';
          R.touch(assistant);
        }
      } finally {
        var runId = run && run.id;
        phase = 'idle';
        run = null;
        if (!sseGotEnd && runId) spawnRunPoller(runId, assistant.id);
        ensureSync(conversationId);
        requestRender('maybe');
      }
    }

    var facade = {
      state: state,
      busy: busy,
      mateId: function () { return mateId; },
      draft: function () { return draft; },
      stick: function () { return stick; },
      setStick: function (v) { stick = !!v; },
      setDraft: function (v) { draft = v || ''; },
      send: function () { return send(); },
      cancel: function () { return cancel(); },
      retry: function (id) { return retry(id); },
      newChat: function () { return newChat(); },
      setConvType: function (t) { return setConvType(t); },
    };

    function attach(el) {
      detach();
      if (!el || !root.__vaultrChatMount) return;
      view = root.__vaultrChatMount(el, facade);
      var mode = heldScroll;
      heldScroll = 'none';
      if (mode === 'none' && stick && messages.length) mode = 'jump';
      if (mode === 'jump') stick = true;
      view.render(mode);
      if (focusAfterAttach) {
        focusAfterAttach = false;
        if (view.focusComposer) view.focusComposer();
      }
    }

    function detach() {
      if (raf) { cancelAnimationFrame(raf); raf = 0; }
      if (rank(pending) > rank(heldScroll)) heldScroll = pending;
      pending = 'none';
      if (!view) return;
      if (view.draftValue) draft = view.draftValue();
      view.destroy();
      view = null;
    }

    function switchMate(id) {
      if (busy()) return false;
      bootstrapped = true;
      if (id !== mateId) {
        cancelRunPoller();
        cancelSync();
        conversationId = '';
        messages = [];
        stoppingMsgId = '';
        refreshSeq++;
      }
      mateId = id;
      try { sessionStorage.setItem('vaultr_agent_bot_id', id); } catch (e) { /* ignore */ }
      if (host.onMate) host.onMate(id);
      requestRender('none');
      return true;
    }

    function tryOpen(id) {
      if (!switchMate(id)) return false;
      void refresh();
      return true;
    }

    // Same mate switch as tryOpen, but lands on a brand-new conversation
    // instead of resuming the mate's most recent one — used when a note is
    // handed to a bot from outside the chat panel (editor toolbar), where a
    // fresh conversation reads as "here's a new thing to look at" rather
    // than continuing whatever that bot was last doing. Awaits refresh()
    // (unlike tryOpen's fire-and-forget) so newChat()'s `messages.length ===
    // 0` guard sees the switched-to mate's real history before deciding
    // whether there's anything to replace.
    async function openFreshChat(id) {
      if (!switchMate(id)) return false;
      await refresh();
      await newChat();
      return true;
    }

    function setConvType(t) {
      if (t === convType || busy()) return;
      cancelRunPoller();
      cancelSync();
      convType = t;
      conversationId = '';
      messages = [];
      stoppingMsgId = '';
      refreshSeq++;
      requestRender('none');
      if (mateId) void refresh();
    }

    async function newChat() {
      if (busy() || !mateId || messages.length === 0) return;
      phase = 'sending';
      requestRender('none');
      cancelRunPoller();
      var d = new Date();
      var title = d.getFullYear() + '-' +
        String(d.getMonth() + 1).padStart(2, '0') + '-' +
        String(d.getDate()).padStart(2, '0') + ' ' +
        String(d.getHours()).padStart(2, '0') + ':' +
        String(d.getMinutes()).padStart(2, '0');
      try {
        var resp = await fetch('/api/conversations', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ mateId: mateId, title: title }),
        });
        if (!resp.ok) { notify("Couldn't start a new chat"); }
        else {
          var data = await resp.json();
          conversationId = data.conversation.id;
          messages = [];
          stoppingMsgId = '';
          lastMsgMs = 0;
          cancelSync();
          startSync(conversationId);
          requestRender('none');
        }
      } catch (e) { notify("Couldn't start a new chat"); }
      finally {
        if (phase === 'sending') phase = 'idle';
        requestRender('maybe');
        if (view && view.focusComposer) view.focusComposer();
      }
    }

    async function send() {
      var text = (draft || '').trim();
      if (!text || busy() || !mateId) return;
      var mate = mateById(mateId);
      if (!mate) return;
      phase = 'sending';
      requestRender('none');
      try {
        if (!conversationId) {
          try {
            var cresp = await fetch('/api/conversations', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ mateId: mate.id, title: '' }),
            });
            if (cresp.ok) conversationId = (await cresp.json()).conversation.id;
            else notify("Couldn't start the chat");
          } catch (e) { notify("Couldn't start the chat"); }
          if (!conversationId) return;
        }
        draft = '';
        if (view && view.clearComposer) view.clearComposer();
        var userMsgId = R.genId();
        messages.push({
          id: userMsgId, role: 'user', content: text, createdAt: Date.now(), rev: 1,
        });
        await runTurn(text, userMsgId, mate);
      } finally {
        if (phase === 'sending') phase = 'idle';
        requestRender('maybe');
      }
    }

    async function retry(msgId) {
      if (busy()) return;
      var idx = -1;
      for (var i = 0; i < messages.length; i++) if (messages[i].id === msgId) { idx = i; break; }
      if (idx < 0) return;
      var msg = messages[idx];
      if (!msg || msg.role !== 'assistant' || (msg.status !== 'failed' && msg.status !== 'canceled')) return;
      var text = '';
      for (var j = idx - 1; j >= 0; j--) {
        if (messages[j].role === 'user') { text = messages[j].content; break; }
      }
      if (!text) return;
      messages.splice(idx, 1);
      var userMsgId = R.genId();
      messages.push({ id: userMsgId, role: 'user', content: text, createdAt: Date.now(), rev: 1 });
      await runTurn(text, userMsgId, mateById(mateId));
    }

    async function cancel() {
      var id = run && run.id;
      if (phase !== 'streaming' || !id) return;
      phase = 'stopping';
      stoppingMsgId = run.assistantMsgId;
      requestRender('none');
      try {
        var resp = await fetch('/api/runs/' + id + '/cancel', { method: 'POST' });
        if (!resp.ok) notify("Couldn't stop the run");
      } catch (e) { notify("Couldn't stop the run"); }
    }

    function insertPath(path) {
      if (!path) return;
      if (view && view.insertPath) { view.insertPath(path); return; }
      var cur = draft || '';
      var bt = String.fromCharCode(96);
      var leadSp = cur.length > 0 && cur[cur.length - 1] !== ' ' ? ' ' : '';
      draft = cur + leadSp + bt + path + bt;
      focusAfterAttach = true;
    }

    // Same shape as insertPath, but wraps a note's filename stem in
    // `[[ ]]` — the wikilink syntax mdhtml.go's wikilinkRe resolves by
    // stem, not backtick-quoted path — matching the format chatMentionSearch
    // already produces when a "@" mention is picked (path_ac.js).
    function insertWikilink(stem) {
      if (!stem) return;
      if (view && view.insertWikilink) { view.insertWikilink(stem); return; }
      var cur = draft || '';
      var leadSp = cur.length > 0 && cur[cur.length - 1] !== ' ' ? ' ' : '';
      draft = cur + leadSp + '[[' + stem + ']] ';
      focusAfterAttach = true;
    }

    return {
      start: function () { matesReady = Promise.all([loadMates(), loadEvents()]); },
      bootstrap: async function () {
        if (bootstrapped) return;
        bootstrapped = true;
        await matesReady;
        if (mateId) return;
        var last = null;
        try { last = sessionStorage.getItem('vaultr_agent_bot_id'); } catch (e) { /* ignore */ }
        var target = last ? mateById(last) : null;
        var id = (target || mates[0] || {}).id || '';
        if (!id) return;
        mateId = id;
        try { sessionStorage.setItem('vaultr_agent_bot_id', id); } catch (e) { /* ignore */ }
        if (host.onMate) host.onMate(id);
        await refresh();
      },
      attach: attach,
      detach: detach,
      tryOpen: tryOpen,
      openFreshChat: openFreshChat,
      busy: busy,
      mateId: function () { return mateId; },
      insertPath: insertPath,
      insertWikilink: insertWikilink,
    };
  };
})(typeof window !== 'undefined' ? window : global);
