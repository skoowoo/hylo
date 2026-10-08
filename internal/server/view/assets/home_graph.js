function homeGraphMixin() {
  return {
    loading: false,
    empty: false,
    cy: null,
    _graphTooltip: null,
    _focusedPath: '',
    nodePanel: null,
    graphColorMode: (function () {
      try { return localStorage.getItem('hylo-graph-color') === 'type' ? 'type' : 'cluster'; } catch (_) { return 'cluster'; }
    })(),
    initGraph() {
      if (window.cytoscapeFcose && window.cytoscape) {
        try { cytoscape.use(cytoscapeFcose); } catch (_) { /* already registered */ }
      }
      this._graphTooltip = document.getElementById('graph-tooltip');
      window.addEventListener('hylo:accent', () => {
        if (!this.cy) return;
        var c = getComputedStyle(document.documentElement).getPropertyValue('--accent').trim();
        if (c) this.cy.style().selector('node:selected').style({ 'border-color': c }).update();
      });
      // #home-list-pane's flex-grow (and so #graph-canvas's box) changes
      // with this class, but cytoscape caches its container size and never
      // notices — same fixup as clicking "Fit all nodes" by hand, just
      // automatic. Opening applies flex-grow with 0 delay (truly instant,
      // no transition event fires for it), so rAF is enough to let that
      // land. Closing holds flex-grow behind --content-pane-exit-ms (home.css)
      // until the panel's own slide-out finishes — a real, if zero-duration,
      // transition — so wait for its transitionend rather than guessing the
      // delay in JS; a timeout is only the fallback if that event never comes.
      this.$watch('contentPaneOpen', (isOpen) => {
        if (!this.cy) return;
        var cy = this.cy;
        var run = () => {
          if (cy !== this.cy) return;
          cy.resize();
          var node = this._focusedPath ? cy.getElementById(this._focusedPath) : null;
          if (node && !node.empty()) cy.animate({ center: { eles: node } }, { duration: 150 });
          else cy.fit(undefined, 48);
        };
        if (isOpen) { requestAnimationFrame(run); return; }
        var listEl = document.getElementById('home-list-pane');
        if (!listEl) { setTimeout(run, 220); return; }
        var done = false;
        var finish = () => { if (done) return; done = true; listEl.removeEventListener('transitionend', onEnd); run(); };
        var onEnd = (e) => { if (e.target === listEl && e.propertyName === 'flex-grow') finish(); };
        listEl.addEventListener('transitionend', onEnd);
        setTimeout(finish, 260); // covers the rare case flex-grow was already at rest and no transition ran
      });
    },
    async loadGraph() {
      this.loading = true;
      this.empty = false;
      this._focusedPath = '';
      this.nodePanel = null;
      var url = '/api/graph/data';
      var index = __hyloSectionParams(__hyloSectionShownURL).get('index');
      if (index) url += '?index=' + encodeURIComponent(index);
      try {
        var resp = await fetch(url);
        if (!resp.ok) { this.loading = false; return; }
        var data = await resp.json();
        this._renderGraph(data);
      } catch (e) {
        console.error('graph load:', e);
      }
      this.loading = false;
    },

    _applyFocus(node) {
      this._focusedPath = node.data('path');
      this.cy.stop();
      this.cy.elements().unselect();
      node.select();
      this.cy.elements().addClass('faded');
      node.removeClass('faded');
      var connected = node.connectedEdges();
      connected.removeClass('faded');
      connected.connectedNodes().removeClass('faded');
      this.cy.animate({ center: { eles: node } }, { duration: 200 });

      var neighborNodes = connected.connectedNodes().filter(function (n) {
        return n.id() !== node.id();
      });
      var connectedData = [];
      neighborNodes.forEach(function (n) {
        connectedData.push({
          path: n.data('path'),
          label: n.data('label'),
          entityType: n.data('entityType') || '',
        });
      });
      this.nodePanel = {
        path: node.data('path'),
        label: node.data('label'),
        entityType: node.data('entityType') || '',
        edgeCount: connected.length,
        connected: connectedData,
      };
      // Selecting a node now drives the editor directly — the card stays,
      // but "Open" is no longer required to see the note.
      this.openNodeInContentPane(this.nodePanel.path, this.nodePanel.label);
    },

    _clearFocus() {
      this._focusedPath = '';
      if (this.cy) {
        this.cy.stop();
        this.cy.elements().unselect();
        this.cy.elements().removeClass('faded');
      }
      this.nodePanel = null;
    },

    openNodeInContentPane(path, label) {
      var pane = window.__hyloContentPane;
      if (!pane) return;
      pane.openNoteInContentPane(path, label || path, true, false, false, false);
    },

    _hexToRgba(hex, alpha) {
      hex = hex.replace(/^#/, '');
      if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
      var r = parseInt(hex.slice(0, 2), 16);
      var g = parseInt(hex.slice(2, 4), 16);
      var b = parseInt(hex.slice(4, 6), 16);
      return 'rgba(' + r + ',' + g + ',' + b + ',' + alpha + ')';
    },

    _entityTypeColors: {
      'concept': '#facc15', 'person': '#60a5fa', 'product': '#34d399', 'company': '#fb923c',
      'project': '#a78bfa', 'topic': '#f472b6', 'brand': '#f87171', 'business-model': '#6366f1',
      'book': '#38bdf8', 'tool': '#14b8a6', 'framework': '#06b6d4', 'technique': '#a855f7',
      'strategy': '#ef4444', 'protocol': '#22d3ee', 'product-platform': '#34d399', 'startup': '#f59e0b',
      'role': '#84cc16', 'market': '#fdba74', 'opensource-project': '#22c55e', 'service': '#2dd4bf',
      'event': '#f43f5e', 'disease': '#dc2626', 'community': '#60a5fa',
    },

    _tagPaletteColor(tag) {
      if (this._entityTypeColors[tag]) return this._entityTypeColors[tag];
      var palette = ['#60a5fa', '#f472b6', '#a78bfa', '#34d399', '#facc15', '#fb923c', '#22d3ee', '#f87171'];
      var h = 0;
      for (var i = 0; i < tag.length; i++) h = (Math.imul(31, h) + tag.charCodeAt(i)) | 0;
      return palette[Math.abs(h) % palette.length];
    },

    _tagColor(tag) {
      if (!tag) {
        var s = getComputedStyle(document.documentElement);
        return s.getPropertyValue('--bg').trim() || '#1a1a1a';
      }
      return this._hexToRgba(this._tagPaletteColor(tag), 0.30);
    },

    _tagBorder(tag) {
      if (!tag) {
        var s = getComputedStyle(document.documentElement);
        return s.getPropertyValue('--line-edge').trim() || 'rgba(244,244,245,0.09)';
      }
      return this._tagPaletteColor(tag);
    },

    _clusterPalette: ['#60a5fa', '#f472b6', '#34d399', '#fb923c', '#a78bfa', '#facc15', '#22d3ee', '#f87171', '#84cc16', '#e879f9'],

    // Communities past the palette (and singletons) share one neutral color —
    // they're the small fringe, and reusing a hue would imply a relation.
    _nodeColors(entityType, community, communitySize) {
      if (this.graphColorMode === 'type') {
        return { fill: this._tagColor(entityType), stroke: this._tagBorder(entityType) };
      }
      var hex = communitySize > 1 ? this._clusterPalette[community] || '#a1a1aa' : '#a1a1aa';
      return { fill: this._hexToRgba(hex, 0.30), stroke: hex };
    },

    setGraphColorMode(mode) {
      this.graphColorMode = mode;
      try { localStorage.setItem('hylo-graph-color', mode); } catch (_) { /* ignore */ }
      if (!this.cy) return;
      var self = this;
      this.cy.batch(function () {
        self.cy.nodes().forEach(function (n) {
          n.data(self._nodeColors(n.data('entityType'), n.data('community'), n.data('communitySize')));
        });
      });
    },

    _renderGraph(data) {
      var container = document.getElementById('graph-canvas');
      if (!container) return;

      var oldCy = this.cy; this.cy = null; if (oldCy) oldCy.destroy();
      this._focusedPath = '';

      if (!data.nodes || data.nodes.length === 0) {
        this.empty = true;
        return;
      }
      this.empty = false;

      var self = this;

      var css = getComputedStyle(document.documentElement);
      var nodeLabelColor = css.getPropertyValue('--fg').trim() || '#f4f4f5';
      var bgColor = css.getPropertyValue('--bg').trim() || '#0f0f0f';
      var accentColor = css.getPropertyValue('--accent').trim() || '#cc785c';
      var mutedHex = css.getPropertyValue('--muted').trim() || '#71717a';
      var edgeColor = /^#[0-9a-f]{6}$/i.test(mutedHex)
        ? self._hexToRgba(mutedHex, 0.32)
        : 'rgba(113,113,122,0.32)';

      var prep = __hyloGraphPrepare(data.nodes, data.edges || []);
      var hubs = new Set(__hyloGraphLabeledHubs(prep.degree, data.nodes.length));
      var communitySize = {};
      data.nodes.forEach(function (n) {
        var c = prep.community[n.id];
        communitySize[c] = (communitySize[c] || 0) + 1;
      });
      function nodeSize(id) {
        return Math.round(Math.min(64, 16 + Math.log2(prep.degree[id] + 1) * 8));
      }

      var elements = [];
      data.nodes.forEach(function (n) {
        var c = prep.community[n.id];
        var et = n.entity_type || '';
        elements.push({
          data: Object.assign({
            id: n.id, label: n.label, path: n.path,
            entityType: et, tags: n.tags || [],
            degree: prep.degree[n.id], nodeSize: nodeSize(n.id),
            community: c, communitySize: communitySize[c],
          }, self._nodeColors(et, c, communitySize[c])),
          position: prep.positions[n.id],
          classes: hubs.has(n.id) ? 'hub' : '',
        });
      });
      prep.links.forEach(function (l) {
        elements.push({ data: { source: l.source, target: l.target }, classes: l.bidir ? 'bidir' : '' });
      });

      this.cy = cytoscape({
        container: container,
        elements: elements,
        style: [
          {
            selector: 'node',
            style: {
              'background-color': 'data(fill)',
              'border-color': 'data(stroke)',
              'border-width': 1.5,
              'label': 'data(label)',
              'color': nodeLabelColor,
              'font-size': function (ele) {
                return Math.max(10, Math.min(13, 10 + ele.data('degree') * 0.2)) + 'px';
              },
              'font-family': 'Inter,-apple-system,sans-serif',
              'text-valign': 'bottom',
              'text-halign': 'center',
              'text-margin-y': 5,
              'text-max-width': '120px',
              'text-wrap': 'ellipsis',
              'min-zoomed-font-size': 12,
              'text-background-opacity': 0,
              'text-shadow-blur': 6,
              'text-shadow-color': bgColor,
              'text-shadow-opacity': 0.9,
              'text-shadow-offset-x': 0,
              'text-shadow-offset-y': 0,
              'width': 'data(nodeSize)',
              'height': 'data(nodeSize)',
              'z-index': 10,
              'cursor': 'pointer',
            }
          },
          {
            selector: 'node.hub',
            style: {
              'min-zoomed-font-size': 0,
              'font-size': function (ele) { return Math.round(11 + Math.log2(ele.data('degree') + 1)) + 'px'; },
              'font-weight': 600,
              'z-index': 15,
            }
          },
          { selector: 'node:selected', style: { 'border-color': accentColor, 'border-width': 3.5, 'z-index': 20 } },
          { selector: 'node.faded', style: { 'opacity': 0.18 } },
          {
            selector: 'edge',
            style: {
              'width': 1,
              'line-color': edgeColor,
              'target-arrow-color': edgeColor,
              'target-arrow-shape': 'triangle',
              'curve-style': 'straight',
              'arrow-scale': 0.6,
              'z-index': 1,
            }
          },
          { selector: 'edge.bidir', style: { 'target-arrow-shape': 'none' } },
          { selector: 'edge.faded', style: { 'opacity': 0.05 } },
        ],
        layout: { name: 'preset', fit: false },
        wheelSensitivity: 0.3,
        minZoom: 0.05,
        maxZoom: 4,
      });

      var cy = this.cy;
      var isolated = cy.nodes().filter(function (n) { return prep.degree[n.id()] === 0; });
      var connected = cy.elements().not(isolated);
      if (connected.nonempty()) connected.layout(__hyloGraphLayoutOptions(prep, data.nodes.length)).run();
      var bb = connected.nonempty() ? connected.boundingBox() : { x1: 0, y2: 0 };
      isolated.forEach(function (n, i) { n.position({ x: bb.x1 + i * 40, y: bb.y2 + 60 }); });

      var hubNodes = cy.nodes('.hub').sort(function (a, b) { return b.data('degree') - a.data('degree'); });
      var keep = new Set(__hyloGraphPickLabels(hubNodes.map(function (n) {
        var b = n.boundingBox({ includeNodes: false, includeEdges: false, includeLabels: true });
        return { id: n.id(), x1: b.x1, y1: b.y1, x2: b.x2, y2: b.y2 };
      })));
      hubNodes.forEach(function (n) { if (!keep.has(n.id())) n.removeClass('hub'); });
      cy.fit(undefined, 48);

      this.cy.on('tap', 'node', function (evt) {
        var node = evt.target;
        var path = node.data('path');
        if (!path) return;
        if (self._focusedPath === path) {
          self._clearFocus();
          setTimeout(function () { node.unselect(); }, 0);
        } else {
          self._applyFocus(node);
        }
      });

      this.cy.on('tap', function (evt) {
        if (evt.target === self.cy) self._clearFocus();
      });

      var tooltip = this._graphTooltip;
      if (tooltip) {
        this.cy.on('mouseover', 'node', function (evt) {
          tooltip.textContent = evt.target.data('label') || '';
          tooltip.classList.add('visible');
        });
        this.cy.on('mousemove', 'node', function (evt) {
          tooltip.style.left = (evt.originalEvent.clientX + 14) + 'px';
          tooltip.style.top = (evt.originalEvent.clientY - 8) + 'px';
        });
        this.cy.on('mouseout', 'node', function () {
          tooltip.classList.remove('visible');
        });
      }
    },

    zoomIn() {
      if (!this.cy) return;
      var cx = this.cy.width() / 2, cy = this.cy.height() / 2;
      this.cy.zoom({ level: this.cy.zoom() * 1.3, renderedPosition: { x: cx, y: cy } });
    },
    zoomOut() {
      if (!this.cy) return;
      var cx = this.cy.width() / 2, cy = this.cy.height() / 2;
      this.cy.zoom({ level: this.cy.zoom() / 1.3, renderedPosition: { x: cx, y: cy } });
    },
    zoomFit() {
      if (!this.cy) return;
      this.cy.fit(undefined, 48);
    }
  };
}
