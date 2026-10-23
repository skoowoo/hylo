// Transcript rules. No DOM, and no view flags: `open` and `copied` stay in
// the view. The kernel is the only caller that mutates a live message list.
(function (root, factory) {
  var api = factory();
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  if (root) root.__hyloChatReduce = api;
})(typeof window !== 'undefined' ? window : global, function () {
  function genId() {
    return (typeof crypto !== 'undefined' && crypto.randomUUID)
      ? crypto.randomUUID()
      : (Date.now().toString(36) + Math.random().toString(36).slice(2));
  }

  function touch(msg) {
    msg.rev = (msg.rev || 0) + 1;
    return msg;
  }

  // What the DB row looked like, so a reload can tell an unchanged message apart.
  function storedSig(m) {
    return JSON.stringify([m.content || '', m.status || '', m.updatedAt || '', m.noteAccess || null, m.attachments || null]);
  }

  function formatStoredMessages(msgs) {
    var out = [];
    for (var i = 0; i < msgs.length; i++) {
      var m = msgs[i];
      if (m.role === 'user') {
        out.push({
          id: m.id,
          role: 'user', content: m.content,
          attachments: m.attachments || [],
          createdAt: m.createdAt ? new Date(m.createdAt).getTime() : 0,
          stored: storedSig(m),
          rev: 1,
        });
      } else {
        var at = m.updatedAt ? new Date(m.updatedAt).getTime() : (m.createdAt ? new Date(m.createdAt).getTime() : 0);
        var segments = replayTrace(m.trace);
        settleAnswer(segments, m.content);
        if (m.noteAccess && m.noteAccess.length) {
          // See the note_access case in applyAgentSegment above for why
          // glob-pattern paths are dropped here too.
          var naItems = m.noteAccess
            .filter(function (na) { return na.path && !/[*?]/.test(na.path); })
            .map(function (na) { return { path: na.path, action: na.action, tool: na.tool || '' }; });
          if (naItems.length) segments.push({ type: 'note_access', items: naItems });
        }
        out.push({
          id: m.id,
          role: 'assistant', agentId: m.agentId, agentBotId: m.mateId,
          content: m.content || '',
          triggerEvent: m.triggerEvent || '',
          triggerId: m.triggerId || '',
          segments: segments,
          status: m.status || 'succeeded',
          startTime: 0, duration: 0,
          createdAt: m.createdAt ? new Date(m.createdAt).getTime() : 0,
          completedAt: at,
          stored: storedSig(m),
          rev: 1,
        });
      }
    }
    return out;
  }

  function closeThinking(segs, at) {
    var t = segs.length ? segs[segs.length - 1] : null;
    if (t && t.type === 'thinking' && !t.endedAt) t.endedAt = at || Date.now();
  }

  var STEP_TYPES = { thinking: 1, tool_use: 1, tool_result: 1 };

  // The reply is the last text; text followed by more steps was narration.
  // While running, text with a step after it is already known to be narration.
  function answerIndex(segs, running) {
    for (var i = segs.length - 1; i >= 0; i--) {
      if (segs[i].type === 'text') return i;
      if (running && STEP_TYPES[segs[i].type]) return -1;
    }
    return -1;
  }

  // The stored reply is authoritative over streamed text (cursor-agent
  // reformats it at the end).
  function settleAnswer(segs, content) {
    if (!content) return;
    var ai = answerIndex(segs, false);
    if (ai >= 0) segs[ai].content = content;
    else segs.push({ type: 'text', content: content });
  }

  // Rebuilds a finished run's segments from its stored agent events.
  function replayTrace(trace) {
    var segs = [];
    if (!Array.isArray(trace)) return segs;
    for (var i = 0; i < trace.length; i++) applyAgentSegment(segs, trace[i]);
    var last = trace.length ? trace[trace.length - 1] : null;
    closeThinking(segs, last && last.at);
    return segs;
  }

  function shortPath(p) {
    var parts = String(p).split('/').filter(Boolean);
    return parts.slice(-2).join('/');
  }

  // One-line subject of a tool call, so the pill can say what it touched
  // rather than only which tool ran.
  function toolArg(input) {
    if (!input || typeof input !== 'object') return '';
    var p = input.file_path || input.path || input.filePath;
    if (!p && input.__notes && input.__notes[0]) p = input.__notes[0].path;
    if (p) return shortPath(p);
    var v = input.pattern || input.query || input.url || input.command || input.description || input.prompt;
    return typeof v === 'string' ? v.replace(/\s+/g, ' ').trim().slice(0, 120) : '';
  }

  // Applies one `agent` SSE payload to a segment list. `last` is the tail at
  // the start of this event — matching the previous in-controller switch.
  function applyAgentSegment(segs, data) {
    var now = data.at || Date.now();
    if (data.type !== 'thinking_start' && data.type !== 'thinking_delta') closeThinking(segs, now);
    var last = segs.length ? segs[segs.length - 1] : null;
    switch (data.type) {
      case 'text_delta': {
        var d = data.delta || ''; if (!d) break;
        // A status line mid-reply doesn't split it.
        var host = null;
        for (var hi = segs.length - 1; hi >= 0; hi--) {
          if (segs[hi].type === 'status') continue;
          if (segs[hi].type === 'text') host = segs[hi];
          break;
        }
        if (host) { host.content += d; }
        else { segs.push({ type: 'text', content: d }); }
        break;
      }
      case 'thinking_start': {
        if (!last || last.type !== 'thinking') {
          segs.push({ type: 'thinking', content: '', startedAt: now });
        }
        break;
      }
      case 'thinking_delta': {
        var td = data.delta || ''; if (!td) break;
        if (last && last.type === 'thinking') { last.content += td; }
        else { segs.push({ type: 'thinking', content: td, startedAt: now }); }
        break;
      }
      case 'tool_use': {
        // A call re-sent with the same id updates it instead of adding a row.
        if (data.id) {
          var dupCall = null;
          for (var di = segs.length - 1; di >= 0 && !dupCall; di--) {
            if (segs[di].type !== 'tool_use') continue;
            var dcs = segs[di].calls || [];
            for (var dj = 0; dj < dcs.length; dj++) if (dcs[dj].id === data.id) { dupCall = dcs[dj]; break; }
          }
          if (dupCall) { if (!dupCall.arg) dupCall.arg = toolArg(data.input); break; }
        }
        var toolName = data.name || 'tool', mergeTarget = null;
        for (var k = segs.length - 1; k >= 0; k--) {
          var sk = segs[k];
          if (sk.type === 'status') continue;
          if (sk.type === 'tool_use' && sk.name === toolName && sk.results.length >= sk.count) mergeTarget = sk;
          break;
        }
        var call = { id: data.id || '', arg: toolArg(data.input), startedAt: now };
        if (mergeTarget) { mergeTarget.count++; (mergeTarget.calls = mergeTarget.calls || []).push(call); }
        else { segs.push({ type: 'tool_use', name: toolName, count: 1, results: [], calls: [call] }); }
        break;
      }
      // Live events only decorate the tool call that touched the note (as
      // `notes`, for the chip row). The authoritative "Notes touched" cards
      // are still rebuilt whole from the DB once the message is done — see
      // mergeSynced, which keeps this segment type out of the live set.
      case 'note_access': {
        var np = data.path || '';
        if (!np || /[*?]/.test(np)) break;
        var host = null;
        for (var ni = segs.length - 1; ni >= 0; ni--) {
          if (segs[ni].type === 'status') continue;
          if (segs[ni].type === 'tool_use') host = segs[ni];
          break;
        }
        if (!host) break;
        var notes = host.notes = host.notes || [];
        var seen = null;
        for (var nj = 0; nj < notes.length; nj++) if (notes[nj].path === np) { seen = notes[nj]; break; }
        if (!seen) notes.push({ path: np, action: data.action || 'read' });
        else if (data.action === 'write') seen.action = 'write';
        break;
      }
      case 'tool_result': {
        var trContent = data.content || '';
        if (typeof trContent !== 'string') trContent = JSON.stringify(trContent);
        var pending = null;
        for (var ti = segs.length - 1; ti >= 0; ti--) {
          if (segs[ti].type === 'tool_use' && segs[ti].results.length < segs[ti].count) { pending = segs[ti]; break; }
          if (segs[ti].type === 'text' || segs[ti].type === 'error') break;
        }
        if (pending) {
          var done = pending.calls && pending.calls[pending.results.length];
          if (done) { done.endedAt = now; if (data.isError) done.failed = true; }
          pending.results.push(trContent);
        }
        else if (trContent) { segs.push({ type: 'tool_result', content: trContent }); }
        break;
      }
      case 'status': {
        var label = (data.label || '').trim(); if (!label || label === 'running' || label === 'requesting') break;
        if (last && last.type === 'status') { last.label = label; }
        else { segs.push({ type: 'status', label: label }); }
        break;
      }
      case 'error': segs.push({ type: 'error', message: data.message || 'Agent error' }); break;
      case 'raw': {
        var rawLine = (data.line || '').replace(/\n$/, ''); if (!rawLine) break;
        if (last && last.type === 'console') { last.content += '\n' + rawLine; }
        else { segs.push({ type: 'console', content: rawLine }); }
        break;
      }
    }
  }

  // One SSE event against a live assistant message. Returns { runId, ended, status }.
  function applySSE(msg, event, data) {
    var out = {};
    switch (event) {
      case 'start':
        if (data.runId) out.runId = data.runId;
        break;
      case 'heartbeat':
        break;
      case 'agent':
        applyAgentSegment(msg.segments, data);
        break;
      case 'stdout':
      case 'stderr': {
        var chunk = (data.chunk || '').replace(/\n$/, '');
        if (!chunk) break;
        var segs = msg.segments, last = segs.length ? segs[segs.length - 1] : null;
        if (last && last.type === 'console') { last.content += '\n' + chunk; }
        else { segs.push({ type: 'console', content: chunk }); }
        break;
      }
      case 'error':
        msg.segments.push({ type: 'error', message: data.message || 'Error' });
        break;
      case 'end':
        closeThinking(msg.segments);
        msg.status = data.status || 'succeeded';
        msg.duration = Date.now() - (msg.startTime || Date.now());
        msg.completedAt = Date.now();
        out.ended = true;
        out.status = msg.status;
        break;
    }
    return out;
  }

  // Merge DB rows into the live list. A message that streamed here keeps its
  // own segments, so the rows and their open state survive; only the reply
  // and the "Notes touched" cards (never built live) come from the DB.
  function mergeStored(ex, nm) {
    var prevStatus = ex.status;
    ex.content = nm.content;
    ex.status = nm.status;
    var hasRichSegs = (ex.segments || []).some(function (s) { return s.type !== 'text'; });
    if (prevStatus !== 'running' && hasRichSegs) {
      var segs = ex.segments.filter(function (s) { return s.type !== 'note_access'; });
      settleAnswer(segs, nm.content);
      ex.segments = segs.concat((nm.segments || []).filter(function (s) { return s.type === 'note_access'; }));
    } else {
      ex.segments = nm.segments;
    }
    ex.completedAt = nm.completedAt;
    ex.stored = nm.stored;
    if (nm.attachments) ex.attachments = nm.attachments;
    if (nm.triggerEvent) ex.triggerEvent = nm.triggerEvent;
    if (nm.triggerId) ex.triggerId = nm.triggerId;
    touch(ex);
  }

  function mergeSynced(messages, dbMsgs) {
    var incoming = formatStoredMessages(dbMsgs);
    var changed = false;
    for (var i = 0; i < incoming.length; i++) {
      var nm = incoming[i];
      var existingIdx = -1;
      if (nm.id) {
        for (var k = 0; k < messages.length; k++) {
          if (messages[k].id === nm.id) { existingIdx = k; break; }
        }
      }
      if (existingIdx >= 0) mergeStored(messages[existingIdx], nm);
      else messages.push(nm);
      changed = true;
    }
    return changed;
  }

  // A full reload as a new list that keeps each unchanged message as is, so its row survives.
  function reconcileStored(messages, dbMsgs) {
    var byId = Object.create(null);
    for (var i = 0; i < messages.length; i++) if (messages[i].id) byId[messages[i].id] = messages[i];
    return formatStoredMessages(dbMsgs).map(function (nm) {
      var ex = nm.id && byId[nm.id];
      if (!ex) return nm;
      if (ex.stored !== nm.stored) mergeStored(ex, nm);
      return ex;
    });
  }

  function replaceTextFromDB(msg, dbContent) {
    if (!msg || msg.role !== 'assistant') return;
    settleAnswer(msg.segments = msg.segments || [], dbContent);
    touch(msg);
  }

  function parseSSEBlock(block) {
    if (!block || !block.trim()) return null;
    var event = '', rawData = '';
    var lines = block.split('\n');
    for (var li = 0; li < lines.length; li++) {
      var line = lines[li];
      if (line.startsWith('event: ')) event = line.slice(7).trim();
      else if (line.startsWith('data: ')) rawData = line.slice(6);
    }
    return { event: event, rawData: rawData };
  }

  // Split a growing SSE buffer on blank lines. The trailing incomplete block
  // stays in `buf`. `gotEnd` is true if any complete block was an end event,
  // even when its JSON later fails to parse.
  function feedSSE(buf, onEvent) {
    var blocks = buf.split('\n\n');
    var rest = blocks.pop();
    var gotEnd = false;
    for (var bi = 0; bi < blocks.length; bi++) {
      var parsed = parseSSEBlock(blocks[bi]);
      if (!parsed) continue;
      if (parsed.event === 'end') gotEnd = true;
      if (parsed.event && parsed.rawData && onEvent) onEvent(parsed.event, parsed.rawData);
    }
    return { buf: rest, gotEnd: gotEnd };
  }

  function formatClock(d) {
    var h = d.getHours(), m = d.getMinutes();
    var ap = h >= 12 ? 'PM' : 'AM';
    h = h % 12;
    if (h === 0) h = 12;
    return h + ':' + String(m).padStart(2, '0') + ' ' + ap;
  }

  function sameCalendarDay(a, b) {
    return a.getFullYear() === b.getFullYear() &&
      a.getMonth() === b.getMonth() &&
      a.getDate() === b.getDate();
  }

  function formatTime(ts, now) {
    if (!ts) return '';
    if (now == null) now = Date.now();
    var diff = now - ts;
    if (diff < 0) diff = 0;
    var sec = Math.floor(diff / 1000);
    if (sec < 45) return 'now';
    var min = Math.floor(sec / 60);
    if (min < 60) return min + 'm ago';
    var hr = Math.floor(min / 60);
    if (hr < 24) return hr + 'h ago';

    var d = new Date(ts);
    var clock = formatClock(d);
    var today = new Date(now);
    var yesterday = new Date(today.getFullYear(), today.getMonth(), today.getDate() - 1);
    if (sameCalendarDay(d, yesterday)) return 'Yesterday, ' + clock;

    var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    var datePart = months[d.getMonth()] + ' ' + d.getDate();
    if (d.getFullYear() !== today.getFullYear()) datePart += ', ' + d.getFullYear();
    return datePart + ', ' + clock;
  }

  function msgAt(msg) {
    return (msg && (msg.completedAt || msg.createdAt)) || 0;
  }

  return {
    genId: genId,
    touch: touch,
    formatStoredMessages: formatStoredMessages,
    applyAgentSegment: applyAgentSegment,
    answerIndex: answerIndex,
    toolArg: toolArg,
    applySSE: applySSE,
    mergeSynced: mergeSynced,
    reconcileStored: reconcileStored,
    replaceTextFromDB: replaceTextFromDB,
    feedSSE: feedSSE,
    formatTime: formatTime,
    msgAt: msgAt,
  };
});
