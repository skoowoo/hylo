  // Knowledge-graph prep: merges reciprocal links, finds communities, and
  // derives fcose options that keep those communities visibly apart. Pure
  // (no cytoscape) so it can be tested under node.

  // Louvain modularity communities. Input order decides ties, so the same
  // graph always yields the same partition.
  function __hyloLouvain(n, pairs) {
    var adj = [];
    for (var i = 0; i < n; i++) adj.push(new Map());
    pairs.forEach(function (p) {
      if (p[0] === p[1]) return;
      adj[p[0]].set(p[1], (adj[p[0]].get(p[1]) || 0) + 1);
      adj[p[1]].set(p[0], (adj[p[1]].get(p[0]) || 0) + 1);
    });
    var membership = [];
    for (i = 0; i < n; i++) membership.push(i);

    for (var level = 0; level < 20; level++) {
      var size = adj.length;
      var k = new Array(size).fill(0);
      var m2 = 0;
      for (i = 0; i < size; i++) {
        adj[i].forEach(function (w) { k[i] += w; });
        m2 += k[i];
      }
      if (m2 === 0) break;
      var comm = [], tot = [];
      for (i = 0; i < size; i++) { comm.push(i); tot.push(k[i]); }

      var improved = false;
      for (var pass = 0; pass < 50; pass++) {
        var moved = false;
        for (i = 0; i < size; i++) {
          var ci = comm[i];
          var wc = new Map();
          adj[i].forEach(function (w, j) {
            if (j !== i) wc.set(comm[j], (wc.get(comm[j]) || 0) + w);
          });
          tot[ci] -= k[i];
          var best = ci;
          var bestGain = (wc.get(ci) || 0) - tot[ci] * k[i] / m2;
          wc.forEach(function (w, c) {
            var gain = w - tot[c] * k[i] / m2;
            if (gain > bestGain + 1e-12) { bestGain = gain; best = c; }
          });
          tot[best] += k[i];
          if (best !== ci) { comm[i] = best; moved = true; improved = true; }
        }
        if (!moved) break;
      }
      if (!improved) break;

      var renum = new Map();
      comm = comm.map(function (c) {
        if (!renum.has(c)) renum.set(c, renum.size);
        return renum.get(c);
      });
      membership = membership.map(function (c) { return comm[c]; });
      var next = [];
      for (i = 0; i < renum.size; i++) next.push(new Map());
      for (i = 0; i < size; i++) {
        adj[i].forEach(function (w, j) {
          next[comm[i]].set(comm[j], (next[comm[i]].get(comm[j]) || 0) + w);
        });
      }
      adj = next;
    }
    return membership;
  }

  // nodes: [{id}], edges: [{source, target}] (directed, may hold both directions).
  function __hyloGraphPrepare(nodes, edges) {
    var index = new Map();
    nodes.forEach(function (n, i) { index.set(n.id, i); });

    var linkByKey = new Map();
    var links = [];
    edges.forEach(function (e) {
      if (e.source === e.target || !index.has(e.source) || !index.has(e.target)) return;
      var a = e.source < e.target ? e.source : e.target;
      var b = a === e.source ? e.target : e.source;
      var key = a + '\u0000' + b;
      var link = linkByKey.get(key);
      if (link) {
        if (link.source !== e.source) link.bidir = true;
        return;
      }
      link = { source: e.source, target: e.target, bidir: false };
      linkByKey.set(key, link);
      links.push(link);
    });

    var degree = {};
    nodes.forEach(function (n) { degree[n.id] = 0; });
    links.forEach(function (l) { degree[l.source]++; degree[l.target]++; });

    var membership = __hyloLouvain(nodes.length, links.map(function (l) {
      return [index.get(l.source), index.get(l.target)];
    }));
    // Renumber by size so community 0 is the largest — colors then follow
    // cluster size rather than Louvain's arbitrary ids.
    var sizes = {};
    membership.forEach(function (c) { sizes[c] = (sizes[c] || 0) + 1; });
    var rank = {};
    Object.keys(sizes).map(Number)
      .sort(function (a, b) { return sizes[b] - sizes[a] || a - b; })
      .forEach(function (c, i) { rank[c] = i; });
    var community = {};
    nodes.forEach(function (n, i) { community[n.id] = rank[membership[i]]; });

    return { links: links, degree: degree, community: community, positions: __hyloGraphSeedPositions(nodes, degree, community) };
  }

  // Starting each community in its own patch (hubs at its middle) gives
  // fcose a deterministic start that already reflects the clusters, so the
  // same vault lays out the same way every time.
  function __hyloGraphSeedPositions(nodes, degree, community) {
    var GOLDEN = Math.PI * (3 - Math.sqrt(5));
    var SPACING = 40;
    var groups = new Map();
    nodes.forEach(function (n) {
      var c = community[n.id];
      if (!groups.has(c)) groups.set(c, []);
      groups.get(c).push(n.id);
    });
    var ordered = Array.from(groups.values()).sort(function (a, b) { return b.length - a.length; });

    var positions = {};
    var placed = 0;
    ordered.forEach(function (members, gi) {
      members.sort(function (a, b) { return degree[b] - degree[a] || (a < b ? -1 : 1); });
      var r = gi === 0 ? 0 : SPACING * 2.2 * Math.sqrt(placed + members.length / 2);
      var cx = r * Math.cos(gi * GOLDEN), cy = r * Math.sin(gi * GOLDEN);
      members.forEach(function (id, j) {
        var rr = SPACING * Math.sqrt(j);
        positions[id] = { x: cx + rr * Math.cos(j * GOLDEN), y: cy + rr * Math.sin(j * GOLDEN) };
      });
      placed += members.length;
    });
    return positions;
  }

  // Edges inside a community stay short and stiff; edges across communities
  // and edges into hubs are long and slack, so hubs stop collapsing every
  // cluster into one central hairball. Cross-community springs are scaled by
  // how many links the two communities share, so a cluster hanging off a
  // single link is still held close (fcose gravity is a no-op on one
  // component). Run it on the connected nodes only: fcose packs isolated
  // nodes far off, which shrinks everything else on fit.
  function __hyloGraphLayoutOptions(prep, nodeCount) {
    var degs = Object.keys(prep.degree).map(function (id) { return prep.degree[id]; }).sort(function (a, b) { return a - b; });
    var median = Math.max(1, degs[Math.floor(degs.length / 2)] || 1);
    var nc = nodeCount;
    var crossLinks = {};
    prep.links.forEach(function (l) {
      var key = pairKey(prep.community[l.source], prep.community[l.target]);
      if (key) crossLinks[key] = (crossLinks[key] || 0) + 1;
    });

    function pairKey(a, b) {
      if (a === b) return '';
      return a < b ? a + '-' + b : b + '-' + a;
    }

    function edgeEnds(edge) {
      var s = edge.data('source'), t = edge.data('target');
      var key = pairKey(prep.community[s], prep.community[t]);
      return {
        same: !key,
        cross: key ? crossLinks[key] : 0,
        hub: Math.max(prep.degree[s] || 0, prep.degree[t] || 0),
      };
    }

    return {
      name: 'fcose',
      quality: nc <= 80 ? 'proof' : 'default',
      randomize: false,
      animate: false,
      fit: false,
      nodeDimensionsIncludeLabels: false,
      uniformNodeDimensions: false,
      packComponents: true,
      step: 'all',
      numIter: 2500,
      nodeRepulsion: function () { return 10000; },
      idealEdgeLength: function (edge) {
        var e = edgeEnds(edge);
        var hubStretch = 1 + 0.1 * Math.log2(Math.max(1, e.hub / median));
        return (e.same ? 45 : 300) * hubStretch;
      },
      edgeElasticity: function (edge) {
        var e = edgeEnds(edge);
        var hubDamp = Math.sqrt(Math.max(1, e.hub / median));
        var cross = Math.max(0.02, 0.1 / Math.sqrt(e.cross));
        return (e.same ? 0.5 : cross) / hubDamp;
      },
      nestingFactor: 0.1,
    };
  }

  // Candidates for an always-on label: the best-connected nodes, so clusters
  // are identifiable before zooming in.
  function __hyloGraphLabeledHubs(degree, nodeCount) {
    var limit = Math.min(20, Math.max(5, Math.ceil(nodeCount * 0.05)));
    return Object.keys(degree)
      .filter(function (id) { return degree[id] > 0; })
      .sort(function (a, b) { return degree[b] - degree[a] || (a < b ? -1 : 1); })
      .slice(0, limit);
  }

  // Greedy, in the given (priority) order: drops any label whose box would
  // overlap one already kept. boxes: [{id, x1, y1, x2, y2}].
  function __hyloGraphPickLabels(boxes) {
    var kept = [];
    boxes.forEach(function (b) {
      var hit = kept.some(function (k) {
        return b.x1 < k.x2 && k.x1 < b.x2 && b.y1 < k.y2 && k.y1 < b.y2;
      });
      if (!hit) kept.push(b);
    });
    return kept.map(function (b) { return b.id; });
  }

  if (typeof module !== 'undefined') {
    module.exports = {
      __hyloLouvain: __hyloLouvain,
      __hyloGraphPrepare: __hyloGraphPrepare,
      __hyloGraphLayoutOptions: __hyloGraphLayoutOptions,
      __hyloGraphLabeledHubs: __hyloGraphLabeledHubs,
      __hyloGraphPickLabels: __hyloGraphPickLabels,
    };
  }
