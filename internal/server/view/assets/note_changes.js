// What a batch of note changes from the live feed touches. No DOM: the section
// cache acts on the answer. `refetch` types are dropped from the cache and
// reloaded if shown; `park` types are only dropped from the cache.
(function (root, factory) {
  var api = factory();
  if (typeof module !== 'undefined' && module.exports) module.exports = api;
  if (root) root.__hyloNoteChanges = api;
})(typeof window !== 'undefined' ? window : global, function () {
  var NOTE_LISTS = ['folder', 'pinned', 'recent', 'tags'];

  function under(path, dir) {
    return path === dir || (path || '').indexOf(dir + '/') === 0;
  }

  // The list sections a note at path shows up in.
  function listsFor(path) {
    if (under(path, '/_knowledge')) return ['knowledge'];
    if (under(path, '/_memory')) return ['memory'];
    if (under(path, '/_shorts')) return ['shorts', 'recent'];
    return NOTE_LISTS;
  }

  // ctx.editing is the note open in the editor: its writes are the page's own
  // autosave, which must not reload the list being looked at.
  function affect(changes, ctx) {
    var editing = (ctx && ctx.editing) || '';
    var out = { all: false, sidebar: false, refetch: {}, park: {} };
    function refetch(types) { types.forEach(function (t) { out.refetch[t] = true; }); }
    function park(types) { types.forEach(function (t) { out.park[t] = true; }); }
    changes.forEach(function (c) {
      switch (c.op) {
        case 'resync':
          out.all = true;
          out.sidebar = true;
          break;
        case 'create':
        case 'delete':
          out.sidebar = true;
          refetch(c.isDir ? NOTE_LISTS : listsFor(c.path));
          break;
        case 'rename':
          out.sidebar = true;
          refetch(listsFor(c.path));
          refetch(listsFor(c.oldPath));
          break;
        case 'write':
          if (c.path === editing) {
            // A save can't say whether tags or links changed, and it reorders Recent.
            park(['tags', 'knowledge', 'recent']);
          } else {
            refetch(listsFor(c.path));
            // Index notes title the sidebar's knowledge entries.
            if (under(c.path, '/_knowledge/_index')) out.sidebar = true;
          }
          break;
        case 'short':
          refetch(['shorts', 'recent']);
          break;
        case 'pin':
          refetch(['pinned', 'folder', 'recent']);
          break;
        case 'image':
          refetch(['images']);
          break;
      }
    });
    Object.keys(out.refetch).forEach(function (t) { delete out.park[t]; });
    return out;
  }

  return { affect: affect };
});
