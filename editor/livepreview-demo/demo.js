import { EditorView, keymap } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands';
import { livePreviewExtensions, listIndentExtension, allowFrontmatterEdit, selectionFormatMenu } from '../src/cm-live/index.js';

const sample = `---
title: Live Preview POC
date: 2026-09-16
source: https://codemirror.net
tags: [codemirror, editor, live-preview, wysiwyg, obsidian, yaml]
status: draft
compile_count: 2
related_notes:
  - /foo/bar-note-one.md
  - /foo/bar-note-two.md
  - /foo/bar-note-three.md
  - /foo/bar-note-four.md
  - /foo/bar-note-five.md
  - /foo/bar-note-six.md
  - /foo/bar-note-seven.md
  - /foo/bar-note-eight.md
  - /foo/bar-note-nine.md
  - /foo/bar-note-ten.md
  - /foo/bar-note-eleven.md
---

# Hylo Live Preview 架构 POC

这段文字包含 **加粗**、*斜体*、~~删除线~~ 和 \`行内代码\`，用来验证光标进入/离开节点时标记符号的隐藏与还原（把光标点进任意一处试试）。

## 二级标题

### 三级标题

#### 四级标题

##### 五级标题

###### 六级标题

> 引用块用于验证多行 block 级装饰是否正确覆盖每一行。
> 第二行。

- 列表项一
- 列表项二
  - 嵌套列表项

- [ ] 未完成任务
- [x] 已完成任务

这是一个 wikilink: [[架构设计|设计文档]]，点击会走 onWikiLinkClick 回调。

这是一张 wikiimage: ![[demo.png]]

也支持标准 markdown 链接: [CodeMirror 官网](https://codemirror.net)

也支持标准 markdown 图片: ![占位图](https://placehold.co/280x120?text=std+image)

---

上面是分割线（---），下面是星号分割线。

***

1. 有序列表第一项
2. 有序列表第二项，这一行故意写得很长很长很长很长很长很长很长很长很长很长很长很长很长很长，用来验证软换行时是否悬挂缩进对齐到文字开头
3. 有序列表第三项

- 无序列表项，这一行同样故意写得很长很长很长很长很长很长很长很长很长很长很长很长很长很长，用来验证软换行时的对齐效果

\`\`\`js
function hello() {
  console.log("fenced code block");
}
\`\`\`

下面是 GFM 表格，用来验证对齐标记（左/中/右）、空单元格补位、以及编辑分隔行时是否正常还原为原始文本。

| 功能 | 状态 | 备注 |
| :--- | :-: | ----: |
| 加粗/斜体 | 已完成 | 与旧版一致 |
| 表格 | 进行中 | 对齐 \`.prose table\` 精美样式 |
| 空单元格 |  |  |
`;

function logEvent(msg) {
  const log = document.getElementById('log');
  const line = document.createElement('div');
  line.className = 'log-line';
  line.textContent = msg;
  log.prepend(line);
}

const view = new EditorView({
  state: EditorState.create({
    doc: sample,
    extensions: [
      history(),
      listIndentExtension,
      selectionFormatMenu(), // right-click a selection for bold/italic/strike/list
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.lineWrapping,
      livePreviewExtensions({
        // Shaped like the real endpoint (internal/storage/image.go via
        // /api/images/serve?name=) so this option reads as production
        // wiring, not a demo stub — Phase 3 swaps the placeholder service
        // for that literal path in content_pane.js, nothing here changes shape.
        resolveImageSrc: (filename) =>
          'https://placehold.co/320x160?text=' + encodeURIComponent(filename), // stand-in for /api/images/serve?name=...
        onWikiLinkClick: (target, alias) => {
          // Phase 3: replace this log with window.__hyloContentPaneOpenWikiLink(target).
          logEvent('wikilink click → target="' + target + '" alias="' + (alias ?? '') + '"');
        },
        // Stand-in for content_pane.js's dialog (frontmatter_dialog.html/js) —
        // window.prompt() has no textarea, but it's enough to exercise the
        // read-only-until-edited round trip end to end in this demo.
        onEditFrontmatter: (v, from, to) => {
          const raw = v.state.doc.sliceString(from, to);
          const lines = raw.split('\n');
          const hasClose = lines.length > 1 && lines[lines.length - 1].trim() === '---';
          const interior = (hasClose ? lines.slice(1, -1) : lines.slice(1)).join('\n');
          const edited = window.prompt('Edit frontmatter YAML (fences added back automatically):', interior);
          if (edited === null) return;
          const body = edited.replace(/\s+$/, '');
          const newBlock = '---\n' + (body ? body + '\n' : '') + '---';
          v.dispatch({ changes: { from, to, insert: newBlock }, annotations: allowFrontmatterEdit.of(true) });
          logEvent('frontmatter saved via dialog stand-in');
        },
      }),
    ],
  }),
  parent: document.getElementById('editor'),
});

window.__cmView = view; // console poking during manual QA
logEvent('editor mounted — click into a bold/heading/wikilink span to see it unhide.');
