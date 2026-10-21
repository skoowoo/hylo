import { Markmap, childSelector } from 'markmap-view';
import { walkTree } from 'markmap-common';

// renderData is ported from markmap-view 0.18.12 (src/view.ts); keep the version pinned and
// re-diff on upgrade. Changes: elbow links between box centers, no underline,
// one fold toggle at the exit point (minus on hover, hidden count when folded) instead of the circle,
// content wrapped in .mm-text so CSS can clamp it.

const SELECTOR_NODE = 'g.markmap-node';
const SELECTOR_LINK = 'path.markmap-link';
const SELECTOR_HIGHLIGHT = 'g.markmap-highlight';
// The stub clears the toggle so its pill never sits on the elbow's corner.
const STUB = 24;
const RADIUS = 6;
const TOGGLE = 14;
const TOGGLE_GAP = 2;

const stopPropagation = (e) => e.stopPropagation();

// Same command sequence for every input, degenerate ones included, so d3 can tween `d`.
function elbow([x0, y0], [x1, y1], stub = STUB) {
  const tx = x0 + Math.min(stub, Math.max(0, (x1 - x0) / 2));
  const sy = Math.sign(y1 - y0);
  const bend = Math.min(RADIUS, Math.abs(y1 - y0) / 2);
  const r0 = Math.max(0, Math.min(bend, tx - x0));
  const r1 = Math.max(0, Math.min(bend, x1 - tx));
  return `M${x0},${y0}H${tx - r0}Q${tx},${y0} ${tx},${y0 + sy * r0}V${y1 - sy * r1}Q${tx},${y1} ${tx + r1},${y1}H${x1}`;
}

// Memoized: a render asks several times per toggle, and subtrees never change after setData.
const counts = new WeakMap();
function hiddenCount(node) {
  let n = counts.get(node);
  if (n === undefined) {
    n = node.children.reduce((sum, c) => sum + 1 + hiddenCount(c), 0);
    counts.set(node, n);
  }
  return n;
}
const toggleWidth = (node) =>
  node.payload?.fold ? Math.max(TOGGLE, 6 + 6 * String(hiddenCount(node)).length) : TOGGLE;

export class HyloMarkmap extends Markmap {
  static create(svg, opts, data = null) {
    const mm = new HyloMarkmap(svg, opts);
    if (data) mm.setData(data).then(() => mm.fit());
    return mm;
  }

  async renderData(originData) {
    const { paddingX, autoFit, maxWidth, lineWidth } = this.options;
    const rootNode = this.state.data;
    if (!rootNode) return;

    // The box drawn by CSS spans the foreignObject, which is inset by paddingX.
    const exitPoint = (r) => [r.x + Math.max(0, r.width - paddingX), r.y + r.height / 2];
    const entryPoint = (r) => [r.x + Math.min(paddingX, r.width), r.y + r.height / 2];
    const linkPath = (source, target) => {
      const [x, y] = exitPoint(source.state.rect);
      // A blank root (several top-level headings) has no box to leave from: start on the trunk.
      return source.state.rect.width
        ? elbow([x, y], entryPoint(target.state.rect))
        : elbow([x + STUB, y], entryPoint(target.state.rect), 0);
    };

    const nodeMap = {};
    const parentMap = {};
    const nodes = [];
    walkTree(rootNode, (item, next, parent) => {
      if (!item.payload?.fold) next();
      nodeMap[item.state.id] = item;
      if (parent) parentMap[item.state.id] = parent.state.id;
      nodes.push(item);
    });

    const originMap = {};
    const sourceRectMap = {};
    const setOriginNode = (originNode) => {
      if (!originNode || originMap[originNode.state.id]) return;
      walkTree(originNode, (item, next) => {
        originMap[item.state.id] = originNode.state.id;
        next();
      });
    };
    const getOriginSourceRect = (node) => sourceRectMap[originMap[node.state.id]] || rootNode.state.rect;
    const getOriginTargetRect = (node) => (nodeMap[originMap[node.state.id]] || rootNode).state.rect;
    sourceRectMap[rootNode.state.id] = rootNode.state.rect;
    if (originData) setOriginNode(originData);

    let { highlight } = this.state;
    if (highlight && !nodeMap[highlight.state.id]) highlight = undefined;
    let highlightNodes = this.g
      .selectAll(childSelector(SELECTOR_HIGHLIGHT))
      .selectAll(childSelector('rect'))
      .data(highlight ? [this._getHighlightRect(highlight)] : [])
      .join('rect')
      .attr('x', (d) => d.x)
      .attr('y', (d) => d.y)
      .attr('width', (d) => d.width)
      .attr('height', (d) => d.height);

    const mmG = this.g
      .selectAll(childSelector(SELECTOR_NODE))
      .each((d) => {
        sourceRectMap[d.state.id] = d.state.rect;
      })
      .data(nodes, (d) => d.state.key);
    const mmGEnter = mmG
      .enter()
      .append('g')
      .attr('data-depth', (d) => d.state.depth)
      .attr('data-path', (d) => d.state.path)
      .each((d) => {
        setOriginNode(nodeMap[parentMap[d.state.id]]);
      });
    const mmGExit = mmG.exit().each((d) => {
      setOriginNode(nodeMap[parentMap[d.state.id]]);
    });
    const mmGMerge = mmG
      .merge(mmGEnter)
      .attr('class', (d) => ['markmap-node', d.payload?.fold && 'markmap-fold'].filter(Boolean).join(' '));

    const mmToggle = mmGMerge
      .selectAll(childSelector('g.mm-toggle'))
      .data(
        (d) => (d.children?.length && d.content ? [d] : []),
        (d) => d.state.key,
      )
      .join((enter) => {
        const g = enter
          .append('g')
          .attr('class', 'mm-toggle')
          .on('click', (e, d) => this.handleClick(e, d))
          .on('mousedown', stopPropagation);
        g.append('rect').attr('y', -TOGGLE / 2).attr('height', TOGGLE).attr('rx', TOGGLE / 2).attr('width', TOGGLE);
        g.append('path').attr('d', `M${TOGGLE / 2 - 3},0H${TOGGLE / 2 + 3}`);
        g.append('text');
        return g;
      });
    mmToggle.select('text').text((d) => (d.payload?.fold ? hiddenCount(d) : ''));

    const observer = this._observer;
    const mmFo = mmGMerge
      .selectAll(childSelector('foreignObject'))
      .data(
        (d) => [d],
        (d) => d.state.key,
      );
    const mmFoEnter = mmFo
      .enter()
      .append('foreignObject')
      .attr('class', 'markmap-foreign')
      .attr('x', paddingX)
      .attr('y', 0)
      .style('opacity', 0)
      .on('mousedown', stopPropagation)
      .on('dblclick', stopPropagation);
    mmFoEnter
      .append('xhtml:div')
      .append('xhtml:div')
      // The clamp sits one level below the measured div, so layout sees the clamped height.
      .html((d) => (d.content ? `<div class="mm-text">${d.content}</div>` : ''))
      .attr('xmlns', 'http://www.w3.org/1999/xhtml');
    mmFoEnter.each(function () {
      observer.observe(this.firstChild?.firstChild);
    });
    const mmFoExit = mmGExit.selectAll(childSelector('foreignObject'));
    mmFoExit.each(function () {
      observer.unobserve(this.firstChild?.firstChild);
    });
    const mmFoMerge = mmFoEnter.merge(mmFo);

    const links = nodes.flatMap((node) =>
      node.payload?.fold ? [] : node.children.map((child) => ({ source: node, target: child })),
    );
    const mmPath = this.g.selectAll(childSelector(SELECTOR_LINK)).data(links, (d) => d.target.state.key);
    const mmPathExit = mmPath.exit();
    const mmPathEnter = mmPath
      .enter()
      .insert('path', 'g')
      .attr('class', 'markmap-link')
      .attr('data-depth', (d) => d.target.state.depth)
      .attr('data-path', (d) => d.target.state.path)
      .attr('d', (d) => {
        const p = exitPoint(getOriginSourceRect(d.target));
        return elbow(p, p);
      })
      .attr('stroke-width', 0);
    const mmPathMerge = mmPathEnter.merge(mmPath);

    this.svg.style('--markmap-max-width', maxWidth ? `${maxWidth}px` : null);
    await new Promise(requestAnimationFrame);
    this._relayout();
    mmFoMerge.each(function () {
      const text = this.querySelector('.mm-text');
      if (!text) return;
      if (text.scrollHeight > text.clientHeight + 1) text.title = text.textContent;
      else text.removeAttribute('title');
    });
    // Folded toggles hang past the box; keep them inside fit().
    for (const d of nodes) {
      if (!d.payload?.fold || !d.children?.length) continue;
      const right = d.state.rect.x + d.state.rect.width - paddingX + TOGGLE_GAP + toggleWidth(d);
      this.state.rect.x2 = Math.max(this.state.rect.x2, right);
    }

    highlightNodes = highlightNodes.data(highlight ? [this._getHighlightRect(highlight)] : []).join('rect');
    this.transition(highlightNodes)
      .attr('x', (d) => d.x)
      .attr('y', (d) => d.y)
      .attr('width', (d) => d.width)
      .attr('height', (d) => d.height);

    // Nodes grow out of, and fold back into, the origin's exit point.
    mmGEnter.attr('transform', (d) => {
      const [x, y] = exitPoint(getOriginSourceRect(d));
      return `translate(${x - d.state.rect.width},${y - d.state.rect.height / 2})`;
    });
    this.transition(mmGExit)
      .attr('transform', (d) => {
        const [x, y] = exitPoint(getOriginTargetRect(d));
        return `translate(${x - d.state.rect.width},${y - d.state.rect.height / 2})`;
      })
      .remove();
    this.transition(mmGMerge).attr('transform', (d) => `translate(${d.state.rect.x},${d.state.rect.y})`);

    mmToggle.attr(
      'transform',
      (d) => `translate(${Math.max(0, d.state.rect.width - paddingX) + TOGGLE_GAP},${d.state.rect.height / 2})`,
    );
    this.transition(mmToggle.select('rect')).attr('width', toggleWidth);
    this.transition(mmToggle.select('text')).attr('x', (d) => toggleWidth(d) / 2);

    this.transition(mmFoExit).style('opacity', 0);
    mmFoMerge
      .attr('width', (d) => Math.max(0, d.state.rect.width - paddingX * 2))
      .attr('height', (d) => d.state.rect.height);
    this.transition(mmFoMerge).style('opacity', 1);

    this.transition(mmPathExit)
      .attr('d', (d) => {
        const p = exitPoint(getOriginTargetRect(d.target));
        return elbow(p, p);
      })
      .attr('stroke-width', 0)
      .remove();

    this.transition(mmPathMerge)
      .attr('stroke-width', (d) => lineWidth(d.target))
      .attr('d', (d) => linkPath(d.source, d.target));

    if (autoFit) this.fit();
  }
}
