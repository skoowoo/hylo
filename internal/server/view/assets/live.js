// The page's one stream of server-side changes (internal/live), batched by topic.
// Nothing is replayed: after a dropped connection subscribers get [{ op: 'resync' }].
(function (root) {
  var BATCH_MS = 150;
  var subs = {};
  var queue = [];
  var timer = 0;
  var down = false;

  function deliver(topic, batch) {
    (subs[topic] || []).forEach(function (fn) {
      try { fn(batch); } catch (e) { console.error(e); }
    });
  }

  function flush() {
    timer = 0;
    var byTopic = {};
    queue.splice(0).forEach(function (c) { (byTopic[c.topic] = byTopic[c.topic] || []).push(c); });
    Object.keys(byTopic).forEach(function (t) { deliver(t, byTopic[t]); });
  }

  function resync() {
    queue = [];
    Object.keys(subs).forEach(function (t) { deliver(t, [{ topic: t, op: 'resync' }]); });
  }

  root.__hyloLive = {
    on: function (topic, fn) { (subs[topic] = subs[topic] || []).push(fn); },
  };

  if (typeof EventSource === 'undefined') return;
  var es = new EventSource('/api/live');
  es.addEventListener('change', function (e) {
    try { queue.push(JSON.parse(e.data)); } catch (_) { return; }
    if (!timer) timer = setTimeout(flush, BATCH_MS);
  });
  es.addEventListener('resync', resync);
  es.onopen = function () {
    if (!down) return;
    down = false;
    resync();
  };
  es.onerror = function () { down = true; };
})(window);
