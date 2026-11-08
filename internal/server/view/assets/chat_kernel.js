// Page-lifetime chat session. Session owns the transcript, transport owns
// SSE, the live feed, and a run poller for a stream that drops. Each bot has
// its own turn, whose phase (idle | sending | streaming | stopping) is that
// bot's only busy flag, so switching bots never waits on one. The view
// attaches only while the pane is in the DOM; a run keeps going when the pane
// is swapped away or another bot is opened.
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
    var conversationId = '';
    var messages = [];
    // False until the open conversation's messages are known, so its empty state never flashes.
    var loaded = false;
    // Each mate's last conversation, shown at once on return while a reload catches up.
    var parked = Object.create(null);
    // Only the newest PAGE messages load on open; scrolling up pages in older
    // ones. Rendering a whole long history made every bot switch stall.
    var PAGE = 50;
    // Rows already reported seen, or on their way. Scrolling over them again sends nothing.
    var reported = Object.create(null);
    var hasOlder = false;
    var loadingOlder = false;
    // The bot's chats with messages, most recently active first: { id, title, at }.
    var chats = [];
    var lastMsgMs = 0;
    var draft = '';
    // Images staged in the composer, kept like the draft across bot and chat switches. Each: {localId, id, mime, name, previewUrl, uploading, failed}.
    var images = [];
    var stick = true;
    // The reader's place while not stuck to the bottom, kept across remounts.
    var anchor = null;

    // Each bot's turn: { phase, run: { id, assistantMsgId } while streaming
    // or stopping, stoppingId (until that message leaves `running`), poll }.
    // A turn writes into its own bot's messages, open or parked.
    var turns = Object.create(null);

    var refreshSeq = 0;
    // A live "chat" change arrived. Pulled once the page is looking and no
    // local turn is in flight; one that lands during a turn waits until it ends.
    var transcriptGen = 0;
    var transcriptEpoch = 0;
    var transcriptPending = false;
    var transcriptFull = false;
    var transcriptFlying = false;
    // Rows this page wrote and saw through to the end of their stream; the
    // store's changes for them say nothing new.
    var mine = {};
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

    function turnOf(id) {
      if (!turns[id]) {
        var poll = { seq: 0, timer: null, interval: 5000, firstDelay: 3000 };
        poll.onError = function () { reportPoll('run', "Couldn't check the run"); };
        turns[id] = { phase: 'idle', run: null, stoppingId: '', poll: poll };
      }
      return turns[id];
    }

    // The open bot's turn.
    function turn() { return turnOf(mateId); }
    function busy() { return turn().phase !== 'idle'; }
    function liveRun() { var p = turn().phase; return p === 'streaming' || p === 'stopping'; }

    // A bot's messages, whether its chat is open or parked.
    function msgsOf(id) {
      if (id === mateId) return messages;
      return parked[id] ? parked[id].messages : [];
    }

    function findIn(id, msgId) {
      var list = msgsOf(id);
      for (var i = 0; i < list.length; i++) if (list[i].id === msgId) return list[i];
      return null;
    }

    // A parked bot's turn changes nothing on screen.
    function renderFor(id) { if (id === mateId) requestRender(); }

    function activeStoppingId() {
      var t = turn();
      if (!t.stoppingId) return '';
      var msg = findMsg(t.stoppingId);
      if (!msg || msg.status !== 'running') {
        t.stoppingId = '';
        return '';
      }
      return t.stoppingId;
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

    // A wake is no row: its reply says what woke the bot. That reply shows once
    // it's done, so one the bot keeps silent never appears and no turn ever has
    // to leave the thread. A retry the user started is theirs to watch.
    function shown() {
      var run = turn().run;
      var live = run && run.assistantMsgId;
      return messages.filter(function (m) {
        if (m.role === 'wake') return false;
        return !(m.role === 'assistant' && m.triggerEvent && m.status === 'running' && m.id !== live);
      });
    }

    function state() {
      return {
        mateId: mateId,
        mate: mateById(mateId),
        mates: mates,
        matesLoaded: matesLoaded,
        messages: shown(),
        chats: chats,
        conversationId: conversationId,
        loaded: loaded,
        catalogEpoch: catalogEpoch,
        draft: draft,
        images: images,
        canSend: canSend(),
        phase: turn().phase,
        busy: busy(),
        runId: turn().run ? turn().run.id : null,
        stoppingId: activeStoppingId(),
        stick: stick,
        hasOlder: hasOlder,
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
        publishMates();
        requestRender();
        // A count that still says unread may have raced the last report: the rows in view go once more.
        var cur = mateById(mateId);
        if (cur && cur.unread) reported = Object.create(null);
        checkSeen();
      } catch (e) { notify("Couldn't load bots"); }
    }

    // A bot is at work while its turn here streams, or while the server holds
    // a reply of its on the way (a wake, a turn from a connected app).
    function publishMates() {
      mates = mates.map(function (m) {
        var p = turns[m.id] ? turns[m.id].phase : 'idle';
        var on = !!m.running || p === 'streaming' || p === 'stopping';
        // A new object: the sidebar's rows are bound to these, and an edit in place goes unseen.
        return !!m.working === on ? m : Object.assign({}, m, { working: on });
      });
      if (host.onMates) host.onMates(mates);
    }

    function windowFocused() {
      return typeof document.hasFocus !== 'function' || document.hasFocus();
    }

    // The chat is up and this window is in front: a visible window behind another app isn't looked at.
    function watching() {
      return !!view && !!mateId && document.visibilityState === 'visible' && windowFocused();
    }

    // A reply, asked for or sent on its own, is read once its row is in view.
    // The server takes the newest stored time among the rows in view, so a
    // reply above or below the fold, or not yet rendered, stays unread.
    function checkSeen() {
      var m = mateById(mateId);
      if (!m || !watching() || !view.visibleIds) return;
      var last = lastReplyId();
      if (!m.unread && (!last || reported[last])) return;
      var ids = view.visibleIds().filter(function (id) {
        var msg = findMsg(id);
        return msg && msg.status !== 'running' && !reported[id];
      });
      if (!ids.length) return;
      ids.forEach(function (id) { reported[id] = true; });
      markSeen(m.id, ids);
    }

    function lastReplyId() {
      for (var i = messages.length - 1; i >= 0; i--) {
        var msg = messages[i];
        if (msg.role === 'assistant' && msg.status !== 'running') return msg.id;
      }
      return '';
    }

    async function markSeen(id, ids) {
      try {
        var resp = await fetch('/api/mates/' + encodeURIComponent(id) + '/seen', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ids: ids }),
        });
        if (!resp.ok) throw new Error(String(resp.status));
        var n = (await resp.json()).unread || 0;
        var m = mateById(id);
        if (!m || m.unread === n) return;
        // A new object: the sidebar's rows are bound to these, and an edit in place goes unseen.
        mates = mates.map(function (x) { return x === m ? Object.assign({}, x, { unread: n }) : x; });
        if (host.onMates) host.onMates(mates);
      } catch (e) {
        // The next look retries.
        ids.forEach(function (x) { delete reported[x]; });
      }
    }

    // A wake's turn is over, or some other writer changed the transcript.
    // A silent wake leaves the chat; any other wake is unread until the user looks.
    function onChat(changes) {
      var reload = false, drop = false, sync = false, full = false;
      changes.forEach(function (c) {
        if (c.op === 'resync') { reload = true; full = true; return; }
        // Someone else's turn started or ended: the list says which bots are at work.
        if (c.op === 'change') { if (!ownChange(c.data)) { sync = true; reload = true; } return; }
        var e = c.data;
        if (c.op !== 'wake' || !e) return;
        if (e.silent) {
          var before = messages.length;
          messages = messages.filter(function (m) { return m.id !== e.wakeId && m.id !== e.replyId; });
          if (messages.length !== before) drop = true;
          return;
        }
        reload = true;
        if (e.mateId === mateId) sync = true;
      });
      if (drop) requestRender();
      if (reload) void loadMates();
      if (full) noteTranscript(true);
      else if (sync) noteTranscript(false);
    }

    function ownChange(ids) {
      return Array.isArray(ids) && ids.length > 0 && ids.every(function (id) { return mine[id]; });
    }

    function noteTranscript(full) {
      transcriptEpoch++;
      transcriptPending = true;
      if (full) transcriptFull = true;
      pumpTranscript();
    }

    function pumpTranscript() {
      if (!transcriptPending || transcriptFlying || !mateId || busy()) return;
      if (document.visibilityState !== 'visible') return;
      void pullTranscript();
    }

    // One pass of what the old timer did: follow a newer chat, or merge rows
    // updated since lastMsgMs. A resync reloads the page, since a gap can hide a delete.
    async function pullTranscript() {
      var full = transcriptFull;
      transcriptPending = false;
      transcriptFull = false;
      transcriptFlying = true;
      var gen = transcriptGen;
      var epoch = transcriptEpoch;
      var convId = conversationId;
      var failed = false;
      try {
        if (gen !== transcriptGen) return;
        if (full) {
          await refresh(true);
          return;
        }
        var data = await fetchJSON('/api/conversations?mateId=' + encodeURIComponent(mateId));
        if (gen !== transcriptGen || liveRun()) return;
        if (!data) { failed = true; reportPoll('sync', "Couldn't refresh the chat"); transcriptPending = true; return; }
        clearPoll('sync');
        chats = listed(data.conversations);
        if ((data.activeId || '') !== convId) {
          await refresh();
          return;
        }
        if (!convId) return;
        var data2 = await fetchJSON('/api/conversations/' + convId + '?since=' + lastMsgMs);
        if (gen !== transcriptGen || convId !== conversationId || liveRun()) return;
        if (!data2) { failed = true; reportPoll('sync', "Couldn't refresh the chat"); transcriptPending = true; return; }
        clearPoll('sync');
        var msgs = data2.messages || [];
        if (hasOlder && messages.length) {
          var floor = messages[0].createdAt || 0;
          msgs = msgs.filter(function (m) {
            return findMsg(m.id) || (m.createdAt ? new Date(m.createdAt).getTime() : 0) > floor;
          });
        }
        // A row still running here has newer tokens than the placeholder in the DB.
        msgs = msgs.filter(function (m) {
          var ex = m.id && findMsg(m.id);
          return !(ex && ex.status === 'running' && m.status === 'running');
        });
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
      } catch (e) {
        failed = true;
        reportPoll('sync', "Couldn't refresh the chat");
        transcriptPending = true;
      } finally {
        transcriptFlying = false;
        // A failure waits for the next change or for the tab to be shown again.
        // Another change that arrived mid-fetch still runs.
        if (!failed && transcriptPending && transcriptEpoch !== epoch) pumpTranscript();
      }
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

    function cancelRunPoller(id) { stopLoop(turnOf(id).poll); }

    function spawnRunPoller(id, runId, msgId) {
      var ctrl = turnOf(id).poll;
      stopLoop(ctrl);
      var n = 0;
      loop(ctrl, async function (seq) {
        if (n++ > 720) return false;
        var info = await fetchJSON('/api/runs/' + runId);
        if (ctrl.seq !== seq) return false;
        if (!info) {
          reportPoll('run', "Couldn't check the run");
          return 5000;
        }
        clearPoll('run');
        var s = info.status;
        if (s === 'succeeded' || s === 'failed' || s === 'canceled') {
          var msg = findIn(id, msgId);
          if (msg && msg.status === 'running') {
            msg.status = s;
            msg.completedAt = info.updatedAt || Date.now();
            R.touch(msg);
            renderFor(id);
          }
          toastFor(s, msg);
          void loadMates();
          return false;
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
        requestRender(!keepPlace);
      } catch (e) { notify("Couldn't load the conversation"); }
    }

    function listed(convs) {
      return (convs || []).map(function (c) {
        return { id: c.id, title: c.title || '', at: c.updatedAt ? new Date(c.updatedAt).getTime() : 0 };
      });
    }

    // keepPlace: a manual refresh, which leaves the reader where they were.
    async function refresh(keepPlace) {
      if (busy() || !mateId) return;
      var seq = ++refreshSeq;
      try {
        var resp = await fetch('/api/conversations?mateId=' + encodeURIComponent(mateId), { cache: 'no-store' });
        if (seq !== refreshSeq) return;
        if (!resp.ok) { notify("Couldn't load the conversation"); return; }
        var data = await resp.json();
        if (seq !== refreshSeq) return;
        chats = listed(data.conversations);
        var active = data.activeId || '';
        if (active !== conversationId) {
          messages = [];
          hasOlder = false;
          lastMsgMs = 0;
          loaded = false;
        }
        conversationId = active;
        if (conversationId) {
          await loadMessages(conversationId, seq, keepPlace);
        } else {
          messages = [];
          loaded = true;
          hasOlder = false;
          requestRender(!keepPlace);
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

    async function syncLastFromDB(convId, msg, id) {
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
        renderFor(id);
      } catch (e) { notify("Couldn't sync the reply"); }
    }

    async function consumeSSE(resp, msg, id) {
      var t = turnOf(id);
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
            if (out.runId && t.run) t.run.id = out.runId;
            if (event === 'start') {
              renderFor(id);
              return;
            }
            if (out.ended && document.hidden) toastFor(out.status, msg);
            R.touch(msg);
            renderFor(id);
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
      var t = turnOf(mate.id);
      var convId = conversationId;
      var assistant = {
        id: R.genId(),
        role: 'assistant', agentId: mate.agentId, agentBotId: mate.id,
        segments: [], status: 'running',
        startTime: Date.now(), duration: 0, completedAt: 0,
        rev: 1,
      };
      messages.push(assistant);
      mine[userMsgId] = mine[assistant.id] = true;
      t.phase = 'streaming';
      t.run = { id: null, assistantMsgId: assistant.id };
      publishMates();
      requestRender(true);
      var sseGotEnd = true;
      var settled = false;
      try {
        var resp = await fetch('/api/chat', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            mateId: mate.id,
            message: text,
            attachments: (extra && extra.attachments) || [],
            conversationId: convId,
            newChat: !chats.some(function (c) { return c.id === convId; }),
            userMessageId: userMsgId,
            assistantMessageId: assistant.id,
            retryOf: (extra && extra.retryOf) || '',
            triggerEvent: (extra && extra.triggerEvent) || '',
            triggerId: (extra && extra.triggerId) || '',
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
        sseGotEnd = await consumeSSE(resp, assistant, mate.id);
        settled = sseGotEnd;
        var finish = finishers[mate.agentId];
        if (sseGotEnd && convId && finish) await finish(convId, assistant, mate.id);
      } catch (e) {
        if (msgsOf(mate.id).indexOf(assistant) !== -1) {
          assistant.segments.push({ type: 'error', message: (e && e.message) || 'Connection error' });
          assistant.status = 'failed';
          R.touch(assistant);
        }
      } finally {
        var runId = t.run && t.run.id;
        t.phase = 'idle';
        t.run = null;
        publishMates();
        if (!sseGotEnd && runId) spawnRunPoller(mate.id, runId, assistant.id);
        // Without its end the reply here isn't final: the stored one is. A
        // parked bot reloads its chat when opened again.
        if (!settled) delete mine[assistant.id];
        if (mate.id === mateId) {
          if (!settled) noteTranscript(false);
          else pumpTranscript();
          requestRender();
        }
        // The reply counts as unread on the server until it's seen in view.
        void loadMates();
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
      openChat: function (id) { return openChat(id); },
      loadOlder: function () { return loadOlder(); },
      noteSeen: function () { checkSeen(); },
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
      bootstrapped = true;
      if (id !== mateId) {
        openConversation(id);
      }
      mateId = id;
      try { sessionStorage.setItem('hylo_agent_bot_id', id); } catch (e) { /* ignore */ }
      if (host.onMate) host.onMate(id);
      if (view) view.focusComposer();
      requestRender();
    }

    // Settings changed the bots: a deleted or missing current bot hands the
    // pane to the first one left, or to the empty state when none are. A run
    // in flight finishes first; its last render settles it.
    async function reloadMates() {
      await loadMates();
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
      switchMate(id);
      void refresh();
    }

    // Parks the open conversation and brings back the one last seen for this mate.
    function openConversation(id) {
      transcriptGen++;
      transcriptPending = false;
      transcriptFull = false;
      if (mateId && loaded) {
        parked[mateId] = { conversationId: conversationId, messages: messages, hasOlder: hasOlder, lastMsgMs: lastMsgMs, chats: chats };
      }
      var back = parked[id];
      conversationId = back ? back.conversationId : '';
      messages = back ? back.messages : [];
      hasOlder = back ? back.hasOlder : false;
      lastMsgMs = back ? back.lastMsgMs : 0;
      chats = back ? back.chats : [];
      loaded = !!back;
      stick = true;
      refreshSeq++;
    }

    // Opens one of the bot's chats, or with '' a new one that starts with its first message.
    async function openChat(id) {
      if (busy() || !mateId || id === conversationId) return;
      var bot = mateId;
      var t = turn();
      t.phase = 'sending';
      requestRender();
      cancelRunPoller(bot);
      var ok = false;
      try {
        var resp = await fetch('/api/mates/' + encodeURIComponent(bot) + '/conversation', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ conversationId: id }),
        });
        ok = resp.ok;
        if (!ok) notify(id ? "Couldn't continue that chat" : "Couldn't start a new chat");
      } catch (e) { notify(id ? "Couldn't continue that chat" : "Couldn't start a new chat"); }
      finally { if (t.phase === 'sending') t.phase = 'idle'; }
      // Another bot opened meanwhile: this one's chat loads when it's opened again.
      if (bot !== mateId) { delete parked[bot]; return; }
      if (ok) {
        t.stoppingId = '';
        await refresh();
      }
      pumpTranscript();
      requestRender();
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
      var t = turn();
      t.phase = 'sending';
      requestRender();
      // A new chat is started by the server with its first turn, under this id.
      if (!conversationId) conversationId = R.genId();
      try {
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
        if (t.phase === 'sending') t.phase = 'idle';
        renderFor(mate.id);
      }
    }

    async function retry(msgId) {
      if (busy()) return;
      var idx = -1;
      for (var i = 0; i < messages.length; i++) if (messages[i].id === msgId) { idx = i; break; }
      if (idx < 0) return;
      var msg = messages[idx];
      if (!msg || msg.role !== 'assistant' || (msg.status !== 'failed' && msg.status !== 'canceled')) return;
      // What a reply answers: the user's message, or the wake the bot started on.
      function opens(m) { return !!m && (m.role === 'user' || m.role === 'wake'); }
      var prompt = idx > 0 ? messages[idx - 1] : await fetchPrompt(msg.id);
      // Only the prompt right before the reply goes with it; an older one stays put.
      var drop = opens(prompt);
      for (var j = idx - 2; !drop && j >= 0; j--) {
        if (opens(messages[j])) { prompt = messages[j]; break; }
      }
      var atts = (prompt && prompt.attachments) || [];
      if (!opens(prompt) || (!prompt.content && !atts.length) || messages.indexOf(msg) === -1 || busy()) return;
      var text = prompt.content || '';
      var mate = mateById(mateId);
      // The server drops the same rows once the retry starts.
      messages.splice(messages.indexOf(msg), 1);
      if (drop && messages.indexOf(prompt) !== -1) messages.splice(messages.indexOf(prompt), 1);
      var userMsgId = R.genId();
      var src = prompt.role === 'wake' ? { triggerEvent: prompt.triggerEvent, triggerId: prompt.triggerId } : {};
      messages.push(Object.assign({ id: userMsgId, role: prompt.role, content: text, attachments: atts, createdAt: Date.now(), rev: 1 }, src));
      await runTurn(text, userMsgId, mate, Object.assign({ retryOf: msg.id, attachments: atts }, src));
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
      var t = turn();
      var id = t.run && t.run.id;
      if (t.phase !== 'streaming' || !id) return;
      t.phase = 'stopping';
      t.stoppingId = t.run.assistantMsgId;
      requestRender();
      try {
        var resp = await fetch('/api/runs/' + id + '/cancel', { method: 'POST' });
        if (!resp.ok) notify("Couldn't stop the run");
      } catch (e) { notify("Couldn't stop the run"); }
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
        if (root.__hyloLive) {
          root.__hyloLive.on('bots', reloadMates);
          root.__hyloLive.on('chat', onChat);
        }
        document.addEventListener('visibilitychange', function () {
          if (document.visibilityState !== 'visible') return;
          checkSeen();
          pumpTranscript();
        });
        // Rows that came into view while the window was behind another app are seen once it's back.
        if (typeof window.addEventListener === 'function') window.addEventListener('focus', checkSeen);
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
      busy: busy,
      mateId: function () { return mateId; },
    };
  };
})(typeof window !== 'undefined' ? window : global);
