// Page-lifetime chat session. Session owns the transcript, transport owns
// SSE and the two pollers, and a single phase (idle | sending | streaming |
// stopping) is the only busy flag. The view attaches only while the pane is
// in the DOM; a run keeps going when the pane is swapped away.
(function (root) {
  var R = root.__hyloChatReduce;

  root.__hyloChatCreate = function (host) {
    host = host || {};
    var mates = [];
    var matesLoaded = false;
    var mateGone = false;
    var eventDefs = [];
    var catalogEpoch = 0;
    var matesSig = '';
    var matesReady = Promise.resolve();
    var bootstrapped = false;

    var mateId = '';
    var convType = 'chat';
    // Remembers each mate's own chat/trigger tab choice (keyed by mateId) —
    // convType itself stays a single live variable (everything below reads
    // it directly), this is just what switchMate restores it from so
    // flipping the tab for one bot doesn't leak into the next bot you open.
    var convTypeByMate = {};
    var conversationId = '';
    var messages = [];
    // False until the open conversation's messages are known, so its empty state never flashes.
    var loaded = false;
    // Each mate and tab's last conversation, shown at once on return while a reload catches up.
    var parked = Object.create(null);
    // The trigger tab's cards and the one it narrows the runs to.
    var triggers = [];
    var triggersFor = '';
    var triggerFilter = '';
    // Only the newest PAGE messages load on open; scrolling up pages in older
    // ones. Rendering a whole long trigger log made every bot switch stall.
    var PAGE = 50;
    var hasOlder = false;
    var loadingOlder = false;
    var lastMsgMs = 0;
    var draft = '';
    // Images staged in the composer, kept like the draft across bot and chat switches. Each: {localId, id, mime, name, previewUrl, uploading, failed}.
    var images = [];
    var stick = true;
    // The reader's place while not stuck to the bottom, kept across remounts.
    var anchor = null;

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
    var pending = false;
    var heldJump = false;

    function mateById(id) {
      for (var i = 0; i < mates.length; i++) if (mates[i].id === id) return mates[i];
      return null;
    }

    function mateName(msg) {
      if (!msg) return 'Agent';
      var m = mateById(msg.agentBotId);
      return m ? m.name : (msg.agentId || 'Agent');
    }

    function eventDef(type) {
      for (var i = 0; i < eventDefs.length; i++) {
        if (eventDefs[i].type === type) return eventDefs[i];
      }
      return null;
    }

    function triggerLabel(type) {
      var d = eventDef(type);
      return d ? d.label : type;
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

    // jump: show the bottom and stick to it, as after a send or opening a chat.
    function requestRender(jump) {
      if (mateGone) settleMate();
      if (jump) pending = true;
      if (!view) {
        if (pending) heldJump = true;
        pending = false;
        return;
      }
      if (raf) return;
      raf = requestAnimationFrame(flush);
    }

    function flush() {
      raf = 0;
      var jump = pending;
      pending = false;
      if (!view) {
        if (jump) heldJump = true;
        return;
      }
      if (jump) stick = true;
      view.render();
    }

    function state() {
      return {
        mateId: mateId,
        mate: mateById(mateId),
        mates: mates,
        matesLoaded: matesLoaded,
        convType: convType,
        messages: messages,
        loaded: loaded,
        catalogEpoch: catalogEpoch,
        draft: draft,
        images: images,
        canSend: canSend(),
        phase: phase,
        busy: busy(),
        runId: run ? run.id : null,
        stoppingId: activeStoppingId(),
        stick: stick,
        hasOlder: hasOlder,
        triggers: triggers,
        triggersLoaded: triggersFor === mateId,
        triggerFilter: triggerFilter,
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
        if (!resp.ok) { notify("Couldn't load bots"); return; }
        mates = ((await resp.json()).mates || []).filter(function (m) { return m.enabled; });
        matesLoaded = true;
        // Rows rebuild only when what they draw changed, not on every trigger edit.
        var sig = JSON.stringify(mates.map(function (m) { return [m.id, m.name, m.color]; }));
        if (sig !== matesSig) { matesSig = sig; catalogEpoch++; }
        if (host.onMates) host.onMates(mates);
        requestRender();
      } catch (e) { notify("Couldn't load bots"); }
    }

    async function loadEvents() {
      try {
        var resp = await fetch('/api/mate-events');
        if (!resp.ok) { notify("Couldn't load triggers"); return; }
        eventDefs = (await resp.json()).events || [];
        catalogEpoch++;
        requestRender();
      } catch (e) { notify("Couldn't load triggers"); }
    }

    async function loadTriggers() {
      var id = mateId;
      if (!id) return;
      try {
        var m = await fetchJSON('/api/mates/' + encodeURIComponent(id));
        if (id !== mateId) return;
        triggers = (m && m.mate && m.mate.triggers) || [];
        if (triggerFilter && !triggers.some(function (t) { return t.id === triggerFilter; })) triggerFilter = '';
      } catch (e) {
        if (id !== mateId) return;
        notify("Couldn't load triggers");
      }
      triggersFor = id;
      requestRender();
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
            requestRender();
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
        // Older rows outside the loaded window would land at the tail.
        if (hasOlder && messages.length) {
          var floor = messages[0].createdAt || 0;
          msgs = msgs.filter(function (m) {
            return findMsg(m.id) || (m.createdAt ? new Date(m.createdAt).getTime() : 0) > floor;
          });
        }
        if (msgs.length > 0) {
          var changed = R.mergeSynced(messages, msgs);
          var lastTs = 0;
          for (var j = 0; j < msgs.length; j++) {
            var t = msgs[j].updatedAt ? new Date(msgs[j].updatedAt).getTime() : 0;
            if (t > lastTs) lastTs = t;
          }
          if (lastTs > lastMsgMs) lastMsgMs = lastTs;
          if (changed) requestRender();
        }
        return 5000;
      });
    }

    async function loadMessages(convId, seq, keepPlace) {
      try {
        var resp = await fetch('/api/conversations/' + convId + '?limit=' + PAGE, { cache: 'no-store' });
        if (!resp.ok) { notify("Couldn't load the conversation"); return; }
        if (seq !== refreshSeq || convId !== conversationId) return;
        var data = await resp.json();
        if (seq !== refreshSeq || convId !== conversationId) return;
        var msgs = data.messages || [];
        messages = R.reconcileStored(messages, msgs);
        loaded = true;
        hasOlder = !!data.hasMore;
        var lastMs = 0;
        for (var i = 0; i < msgs.length; i++) {
          var t = msgs[i].updatedAt ? new Date(msgs[i].updatedAt).getTime() : 0;
          if (t > lastMs) lastMs = t;
        }
        lastMsgMs = lastMs;
        startSync(convId);
        requestRender(!keepPlace);
      } catch (e) { notify("Couldn't load the conversation"); }
    }

    // keepPlace: a manual refresh, which leaves the reader where they were.
    async function refresh(keepPlace) {
      if (busy() || !mateId) return;
      var seq = ++refreshSeq;
      if (convType === 'trigger') void loadTriggers();
      try {
        var resp = await fetch('/api/conversations?mateId=' + encodeURIComponent(mateId) + '&type=' + encodeURIComponent(convType), { cache: 'no-store' });
        if (seq !== refreshSeq) return;
        if (!resp.ok) { notify("Couldn't load the conversation"); return; }
        var convs = (await resp.json()).conversations || [];
        if (seq !== refreshSeq) return;
        conversationId = convs.length > 0 ? convs[0].id : '';
        if (conversationId) {
          await loadMessages(conversationId, seq, keepPlace);
        } else {
          messages = [];
          loaded = true;
          hasOlder = false;
          cancelSync();
          requestRender();
        }
      } catch (e) { notify("Couldn't load the conversation"); }
      finally {
        if (seq === refreshSeq && !loaded) { loaded = true; requestRender(); }
      }
    }

    async function loadOlder() {
      if (!hasOlder || loadingOlder || !conversationId || !messages.length) return;
      var convId = conversationId;
      var seq = refreshSeq;
      loadingOlder = true;
      try {
        var resp = await fetch('/api/conversations/' + convId + '?limit=' + PAGE + '&before=' + encodeURIComponent(messages[0].id), { cache: 'no-store' });
        if (!resp.ok) { notify("Couldn't load earlier messages"); return; }
        var data = await resp.json();
        if (seq !== refreshSeq || convId !== conversationId) return;
        var older = R.formatStoredMessages(data.messages || []).filter(function (m) { return !findMsg(m.id); });
        hasOlder = !!data.hasMore;
        if (!older.length) return;
        messages = older.concat(messages);
        requestRender();
      } catch (e) { notify("Couldn't load earlier messages"); }
      finally { loadingOlder = false; }
    }

    async function syncLastFromDB(convId, msg) {
      try {
        var resp = await fetch('/api/conversations/' + convId + '?limit=' + PAGE, { cache: 'no-store' });
        if (!resp.ok) { notify("Couldn't sync the reply"); return; }
        var dbMsgs = (await resp.json()).messages || [];
        var dbMsg = null;
        for (var i = dbMsgs.length - 1; i >= 0; i--) {
          if (dbMsgs[i].role === 'assistant' && dbMsgs[i].content) { dbMsg = dbMsgs[i]; break; }
        }
        if (!dbMsg || !msg || msg.role !== 'assistant') return;
        R.replaceTextFromDB(msg, dbMsg.content);
        requestRender();
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
              requestRender();
              return;
            }
            if (out.ended && document.hidden) toastFor(out.status, msg);
            R.touch(msg);
            requestRender();
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

    async function runTurn(text, userMsgId, mate, extra) {
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
      requestRender(true);
      var sseGotEnd = true;
      try {
        var resp = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            mateId: mate.id,
            message: text,
            attachments: (extra && extra.attachments) || [],
            conversationId: conversationId,
            userMessageId: userMsgId,
            assistantMessageId: assistant.id,
            retryOf: (extra && extra.retryOf) || '',
            intro: !!(extra && extra.intro),
            introLang: (extra && extra.lang) || '',
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
        requestRender();
      }
    }

    // Re-fires a failed/canceled trigger-originated message through
    // /api/runs/retry-trigger instead of runTurn's /api/chat — that endpoint
    // goes through mate.Runner.RetryRun, so the retried run's completion
    // still reaches the mate runner's onDone hook (inbox message on
    // completion) the way the original automatic trigger fire did. There's
    // no SSE stream for this path (it's a background run, like /api/runs),
    // so it leans on the same runId poller + conversation resync runTurn
    // already uses to recover from a dropped SSE connection.
    async function retryTriggerRun(text, userMsgId, triggerEvent, triggerId, mate, retryOf) {
      if (!mate) return;
      var assistant = {
        id: R.genId(),
        role: 'assistant', agentId: mate.agentId, agentBotId: mate.id,
        triggerEvent: triggerEvent,
        triggerId: triggerId,
        createdAt: Date.now(),
        segments: [], status: 'running',
        startTime: Date.now(), duration: 0, completedAt: 0,
        rev: 1,
      };
      messages.push(assistant);
      phase = 'streaming';
      requestRender(true);
      try {
        var resp = await fetch('/api/runs/retry-trigger', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            mateId: mate.id,
            conversationId: conversationId,
            prompt: text,
            triggerEvent: triggerEvent,
            triggerId: triggerId,
            retryOf: retryOf,
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
        var data = await resp.json();
        if (data && data.runId) spawnRunPoller(data.runId, assistant.id);
      } catch (e) {
        if (messages.indexOf(assistant) !== -1) {
          assistant.segments.push({ type: 'error', message: (e && e.message) || 'Connection error' });
          assistant.status = 'failed';
          R.touch(assistant);
        }
      } finally {
        phase = 'idle';
        ensureSync(conversationId);
        requestRender();
      }
    }

    var facade = {
      state: state,
      busy: busy,
      mateId: function () { return mateId; },
      draft: function () { return draft; },
      stick: function () { return stick; },
      setStick: function (v) { stick = !!v; },
      anchor: function () { return anchor; },
      setAnchor: function (a) { anchor = a; },
      setDraft: function (v) { draft = v || ''; },
      addImages: function (files) { return addImageFiles(files); },
      removeImage: function (id) { removeImage(id); },
      send: function () { return send(); },
      sendChoice: function (text) { return send(null, text); },
      cancel: function () { return cancel(); },
      retry: function (id) { return retry(id); },
      newChat: function () { return newChat(); },
      loadOlder: function () { return loadOlder(); },
      setConvType: function (t) { return setConvType(t); },
      setTriggerFilter: function (id) {
        triggerFilter = triggerFilter === id ? '' : (id || '');
        requestRender();
      },
    };

    function attach(el) {
      detach();
      if (!el || !root.__hyloChatMount) return;
      view = root.__hyloChatMount(el, facade);
      var jump = heldJump;
      heldJump = false;
      if (jump) stick = true;
      view.render();
      view.focusComposer();
    }

    function detach() {
      if (raf) { cancelAnimationFrame(raf); raf = 0; }
      if (pending) heldJump = true;
      pending = false;
      if (!view) return;
      if (view.draftValue) draft = view.draftValue();
      view.destroy();
      view = null;
    }

    function switchMate(id) {
      if (busy()) return false;
      bootstrapped = true;
      if (id !== mateId) {
        openConversation(id, convTypeByMate[id] || 'chat');
        triggers = [];
        triggersFor = '';
        triggerFilter = '';
      }
      mateId = id;
      try { sessionStorage.setItem('hylo_agent_bot_id', id); } catch (e) { /* ignore */ }
      if (host.onMate) host.onMate(id);
      if (view) view.focusComposer();
      requestRender();
      return true;
    }

    // Settings changed the bots: a deleted or missing current bot hands the
    // pane to the first one left, or to the empty state when none are. A run
    // in flight finishes first; its last render settles it.
    async function reloadMates() {
      await loadMates();
      if (convType === 'trigger') void loadTriggers();
      if (!bootstrapped) return;
      mateGone = !mateById(mateId);
      settleMate();
    }

    function settleMate() {
      if (!mateGone || busy()) return;
      mateGone = false;
      var next = (mates[0] || {}).id || '';
      if (next === mateId) return;
      switchMate(next);
      if (next) void refresh();
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

    // Parks the open conversation and brings back the one last seen for this mate and tab.
    function openConversation(id, type) {
      cancelRunPoller();
      cancelSync();
      if (mateId && loaded) {
        parked[mateId + '/' + convType] = { conversationId: conversationId, messages: messages, hasOlder: hasOlder, lastMsgMs: lastMsgMs };
      }
      var back = parked[id + '/' + type];
      conversationId = back ? back.conversationId : '';
      messages = back ? back.messages : [];
      hasOlder = back ? back.hasOlder : false;
      lastMsgMs = back ? back.lastMsgMs : 0;
      loaded = !!back;
      convType = type;
      stoppingMsgId = '';
      stick = true;
      refreshSeq++;
    }

    function setConvType(t) {
      if (t === convType || busy()) return;
      openConversation(mateId, t);
      triggerFilter = '';
      if (mateId) convTypeByMate[mateId] = t;
      if (view) view.focusComposer();
      requestRender();
      if (mateId) void refresh();
    }

    async function newChat() {
      if (busy() || !mateId || messages.length === 0) return;
      phase = 'sending';
      requestRender();
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
          hasOlder = false;
          stoppingMsgId = '';
          lastMsgMs = 0;
          cancelSync();
          startSync(conversationId);
          requestRender();
        }
      } catch (e) { notify("Couldn't start a new chat"); }
      finally {
        if (phase === 'sending') phase = 'idle';
        requestRender();
      }
    }

    function readyImages() {
      return images.filter(function (i) { return i.id && !i.uploading && !i.failed; });
    }

    // A send waits for every upload, so no picture is left out of the turn.
    function canSend() {
      if (busy() || !mateId || images.some(function (i) { return i.uploading; })) return false;
      return !!draft.trim() || readyImages().length > 0;
    }

    function dropImages(gone) {
      images = images.filter(function (i) {
        if (gone.indexOf(i) === -1) return true;
        if (i.previewUrl) URL.revokeObjectURL(i.previewUrl);
        return false;
      });
      requestRender();
    }

    function removeImage(localId) {
      dropImages(images.filter(function (i) { return i.localId === localId; }));
    }

    function addImageFiles(files) {
      var room = Math.max(0, 5 - images.length);
      if (files.length > room) notify('Up to 5 images per message');
      files.slice(0, room).forEach(function (file) {
        var item = {
          localId: R.genId(),
          name: file.name || 'image',
          previewUrl: URL.createObjectURL(file),
          uploading: true,
          failed: false,
          id: '',
          mime: '',
        };
        images.push(item);
        uploadImage(item, file);
      });
      requestRender();
    }

    async function uploadImage(item, file) {
      try {
        var fd = new FormData();
        fd.append('file', file, file.name || 'image');
        var resp = await fetch('/api/chat/uploads', { method: 'POST', body: fd });
        if (!resp.ok) throw new Error((await resp.text()) || 'Upload failed');
        var data = await resp.json();
        item.id = data.id;
        item.mime = data.mime;
        item.name = data.name || item.name;
        item.uploading = false;
      } catch (e) {
        item.uploading = false;
        item.failed = true;
        notify((e && e.message) || 'Image upload failed');
      }
      requestRender();
    }

    function publicImages(list) {
      return list.map(function (i) { return { id: i.id, mime: i.mime, name: i.name }; });
    }

    // A choice is sent as is, leaving whatever the user was typing in the composer.
    async function send(extra, choice) {
      var text = (choice != null ? choice : draft).trim();
      if (choice != null ? (!text || busy() || !mateId) : !canSend()) return;
      var pics = choice == null ? readyImages() : [];
      var mate = mateById(mateId);
      if (!mate) return;
      var sent = publicImages(pics);
      phase = 'sending';
      requestRender();
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
        // The composer stays editable while sending, so only clear what was sent.
        if (choice == null && (draft || '').trim() === text) {
          draft = '';
          if (view && view.clearComposer) view.clearComposer();
        }
        var userMsgId = R.genId();
        messages.push({
          id: userMsgId, role: 'user', content: text, attachments: sent,
          createdAt: Date.now(), rev: 1,
        });
        dropImages(pics);
        await runTurn(text, userMsgId, mate, Object.assign({}, extra, { attachments: sent }));
      } finally {
        if (phase === 'sending') phase = 'idle';
        requestRender();
      }
    }

    async function retry(msgId) {
      if (busy()) return;
      var idx = -1;
      for (var i = 0; i < messages.length; i++) if (messages[i].id === msgId) { idx = i; break; }
      if (idx < 0) return;
      var msg = messages[idx];
      if (!msg || msg.role !== 'assistant' || (msg.status !== 'failed' && msg.status !== 'canceled')) return;
      var prompt = idx > 0 ? messages[idx - 1] : await fetchPrompt(msg.id);
      // Only the prompt right before the reply goes with it; an older one stays put.
      var drop = !!prompt && prompt.role === 'user';
      for (var j = idx - 2; !drop && j >= 0; j--) {
        if (messages[j].role === 'user') { prompt = messages[j]; break; }
      }
      var atts = (prompt && prompt.attachments) || [];
      if (!prompt || prompt.role !== 'user' || (!prompt.content && !atts.length) || messages.indexOf(msg) === -1 || busy()) return;
      var text = prompt.content || '';
      var triggerEvent = msg.triggerEvent || '';
      var mate = mateById(mateId);
      // The server drops the same rows once the retry starts.
      messages.splice(messages.indexOf(msg), 1);
      if (drop && messages.indexOf(prompt) !== -1) messages.splice(messages.indexOf(prompt), 1);
      var userMsgId = R.genId();
      messages.push({ id: userMsgId, role: 'user', content: text, attachments: atts, createdAt: Date.now(), rev: 1 });
      // A trigger run is retried through the trigger pipeline, so its
      // completion still reaches the inbox the way the original fire did.
      if (triggerEvent) {
        await retryTriggerRun(text, userMsgId, triggerEvent, msg.triggerId || '', mate, msg.id);
      } else {
        await runTurn(text, userMsgId, mate, { retryOf: msg.id, attachments: atts });
      }
    }

    // The prompt of the oldest loaded reply sits on the page not loaded yet.
    async function fetchPrompt(msgId) {
      if (!conversationId) return null;
      try {
        var data = await fetchJSON('/api/conversations/' + conversationId + '?limit=1&before=' + encodeURIComponent(msgId));
        var m = data && data.messages && data.messages[0];
        return m ? R.formatStoredMessages([m])[0] : null;
      } catch (e) {
        notify("Couldn't load the prompt to retry");
        return null;
      }
    }

    async function cancel() {
      var id = run && run.id;
      if (phase !== 'streaming' || !id) return;
      phase = 'stopping';
      stoppingMsgId = run.assistantMsgId;
      requestRender();
      try {
        var resp = await fetch('/api/runs/' + id + '/cancel', { method: 'POST' });
        if (!resp.ok) notify("Couldn't stop the run");
      } catch (e) { notify("Couldn't stop the run"); }
    }

    // Wraps a note's filename stem in `[[ ]]` — the wikilink syntax
    // mdhtml.go's wikilinkRe resolves by stem, not a full path — matching
    // the format both note-reference entry points produce: chat's "@"
    // mention (path_ac.js) and global search's Cmd+Enter (search_overlay.js).
    function insertWikilink(stem) {
      if (!stem) return;
      if (view && view.insertWikilink) { view.insertWikilink(stem); return; }
      var cur = draft || '';
      var leadSp = cur.length > 0 && cur[cur.length - 1] !== ' ' ? ' ' : '';
      draft = cur + leadSp + '[[' + stem + ']] ';
    }

    // Onboarding lands with ?intro=1: the new bot's first turn is asked for
    // the user, once, and it answers by writing them a guide in the editor.
    var INTRO_PAUSE_MS = 1000;
    async function maybeIntro(lang) {
      if (messages.some(function (m) { return m.role === 'user'; })) return;
      // Let the hello carried over from onboarding land before asking.
      var bot = mateId;
      await new Promise(function (r) { setTimeout(r, INTRO_PAUSE_MS); });
      // The user may have started typing or switched bots meanwhile.
      if (mateId !== bot || (draft || '').trim() || busy()) return;
      // In the desktop app navigator.language follows the app bundle, not the user; the hello's pick is better.
      var zh = /^zh/i.test(lang || navigator.language || '');
      draft = zh ? '你好 Hylo！你能帮我做些什么？我该从哪里开始？' : 'Hi Hylo! What can you do for me, and where should I start?';
      await send({ intro: true, lang: lang });
    }

    return {
      start: function () {
        matesReady = Promise.all([loadMates(), loadEvents()]);
        if (root.__hyloLive) root.__hyloLive.on('bots', reloadMates);
      },
      bootstrap: async function () {
        if (bootstrapped) return;
        bootstrapped = true;
        await matesReady;
        if (mateId) return;
        // ?bot= picks the bot a link was meant for (onboarding lands on the one it made),
        // once: a reload goes back to the bot last chatted with.
        var wanted = null;
        var intro = false;
        var introLang = '';
        try {
          var url = new URL(window.location.href);
          wanted = url.searchParams.get('bot');
          intro = url.searchParams.get('intro') === '1';
          introLang = url.searchParams.get('lang') || '';
          if (wanted || intro) {
            url.searchParams.delete('bot');
            url.searchParams.delete('intro');
            url.searchParams.delete('lang');
            history.replaceState(history.state, '', url.pathname + url.search + url.hash);
          }
        } catch (e) { /* ignore */ }
        var last = null;
        try { last = sessionStorage.getItem('hylo_agent_bot_id'); } catch (e) { /* ignore */ }
        var target = (wanted && mateById(wanted)) || (last ? mateById(last) : null);
        var id = (target || mates[0] || {}).id || '';
        if (!id) return;
        mateId = id;
        try { sessionStorage.setItem('hylo_agent_bot_id', id); } catch (e) { /* ignore */ }
        if (host.onMate) host.onMate(id);
        await refresh();
        if (intro) await maybeIntro(introLang);
      },
      attach: attach,
      detach: detach,
      tryOpen: tryOpen,
      reload: async function () {
        if (!bootstrapped) return;
        await reloadMates();
        await refresh(true);
      },
      clearFilter: function () {
        triggerFilter = '';
        requestRender();
      },
      openFreshChat: openFreshChat,
      busy: busy,
      mateId: function () { return mateId; },
      insertWikilink: insertWikilink,
    };
  };
})(typeof window !== 'undefined' ? window : global);
