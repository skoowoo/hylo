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

  function formatStoredMessages(msgs) {
    var out = [];
    for (var i = 0; i < msgs.length; i++) {
      var m = msgs[i];
      if (m.role === 'user') {
        out.push({
          id: m.id,
          role: 'user', content: m.content,
          createdAt: m.createdAt ? new Date(m.createdAt).getTime() : 0,
          rev: 1,
        });
      } else {
        var at = m.updatedAt ? new Date(m.updatedAt).getTime() : (m.createdAt ? new Date(m.createdAt).getTime() : 0);
        var segments = m.content ? [{ type: 'text', content: m.content }] : [];
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
          triggerEvent: m.triggerEvent || '',
          segments: segments,
          status: m.status || 'succeeded',
          startTime: 0, duration: 0,
          createdAt: m.createdAt ? new Date(m.createdAt).getTime() : 0,
          completedAt: at,
          rev: 1,
        });
      }
    }
    return out;
  }

  function closeThinking(segs) {
    var t = segs.length ? segs[segs.length - 1] : null;
    if (t && t.type === 'thinking' && !t.endedAt) t.endedAt = Date.now();
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
    if (data.type !== 'thinking_start' && data.type !== 'thinking_delta') closeThinking(segs);
    var last = segs.length ? segs[segs.length - 1] : null;
    switch (data.type) {
      case 'text_delta': {
        var d = data.delta || ''; if (!d) break;
        if (last && last.type === 'text') { last.content += d; }
        else { segs.push({ type: 'text', content: d }); }
        break;
      }
      case 'text_replace': {
        var newText = data.text || '';
        var lastTi = -1;
        for (var rti = segs.length - 1; rti >= 0; rti--) {
          if (segs[rti].type === 'text') { lastTi = rti; break; }
        }
        if (lastTi >= 0) {
          for (var rti2 = lastTi - 1; rti2 >= 0; rti2--) {
            if (segs[rti2].type === 'text') { segs.splice(rti2, 1); lastTi--; }
          }
          segs[lastTi].content = newText;
        } else if (newText) {
          segs.push({ type: 'text', content: newText });
        }
        break;
      }
      case 'thinking_start': {
        if (!last || last.type !== 'thinking') {
          segs.push({ type: 'thinking', content: '', startedAt: Date.now() });
        }
        break;
      }
      case 'thinking_delta': {
        var td = data.delta || ''; if (!td) break;
        if (last && last.type === 'thinking') { last.content += td; }
        else { segs.push({ type: 'thinking', content: td, startedAt: Date.now() }); }
        break;
      }
      case 'tool_use': {
        // ACP re-sends the same call on every location update.
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
        var call = { id: data.id || '', arg: toolArg(data.input), startedAt: Date.now() };
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
          if (done) { done.endedAt = Date.now(); if (data.isError) done.failed = true; }
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

  // Merge DB rows into the live list. Rich segments (thinking, tools) survive
  // only once a message has already left `running` — the DB stores final text.
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
      if (existingIdx >= 0) {
        var ex = messages[existingIdx];
        var prevStatus = ex.status;
        ex.content = nm.content;
        ex.status = nm.status;
        var hasRichSegs = (ex.segments || []).some(function (s) { return s.type !== 'text'; });
        if (prevStatus !== 'running' && hasRichSegs) {
          // note_access is excluded here on purpose (never on the "keep the
          // live version" side) — it's never built live to begin with (see
          // the note_access case above), so it always has to come from the
          // DB's row, and always goes last, after the DB's own text.
          var richSegs = (ex.segments || []).filter(function (s) { return s.type !== 'text' && s.type !== 'note_access'; });
          var dbTextSegs = (nm.segments || []).filter(function (s) { return s.type === 'text'; });
          var dbNoteAccess = (nm.segments || []).filter(function (s) { return s.type === 'note_access'; });
          ex.segments = richSegs.concat(dbTextSegs).concat(dbNoteAccess);
        } else {
          ex.segments = nm.segments;
        }
        ex.completedAt = nm.completedAt;
        if (nm.triggerEvent) ex.triggerEvent = nm.triggerEvent;
        touch(ex);
      } else {
        messages.push(nm);
      }
      changed = true;
    }
    return changed;
  }

  // Replace text segments with the authoritative DB body; keep everything else.
  function replaceTextFromDB(msg, dbContent) {
    if (!msg || msg.role !== 'assistant') return;
    var nonText = (msg.segments || []).filter(function (s) { return s.type !== 'text'; });
    msg.segments = nonText.concat([{ type: 'text', content: dbContent }]);
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
    toolArg: toolArg,
    applySSE: applySSE,
    mergeSynced: mergeSynced,
    replaceTextFromDB: replaceTextFromDB,
    feedSSE: feedSSE,
    formatTime: formatTime,
    msgAt: msgAt,
  };
});
