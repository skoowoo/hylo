import { describe, it, expect } from 'vitest';
import { clipPageToMarkdown } from './markdown';
import { JSDOM } from 'jsdom';
import { WEBSITE_TEST_CASES } from './test-cases';

describe('Unit Tests', () => {
  it('should escape page hashtags so they do not become vault tags', () => {
    const html = `<!DOCTYPE html><html><head><title>Tags #weekly</title></head><body><article>
      <p>Launch day #buildinpublic and 读书 #读书，C# stays, issue #42 stays. Enough words for Readability to keep this paragraph around.</p>
      <p>Wrapped <span>#wrapped</span> tag and <a href="https://x.com/hashtag/ai">#ai</a> link. More sentences so the article body is long enough.</p>
      <p>Inline <code>#notag</code> code.</p>
      <pre><code>#!/bin/sh
# comment</code></pre>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/tags' });
    const { markdown } = clipPageToMarkdown(dom.window.document, 'https://example.com/tags');
    expect(markdown).toContain('\n# Tags \\#weekly\n');
    expect(markdown).toContain('\\#buildinpublic');
    expect(markdown).toContain('\\#读书，');
    expect(markdown).toContain('C# stays, issue #42 stays');
    expect(markdown).toContain('\\#wrapped');
    expect(markdown).toContain('[\\#ai](https://x.com/hashtag/ai)');
    expect(markdown).toContain('`#notag`');
    expect(markdown).toContain('#!/bin/sh\n# comment');
  });

  it('should promote WeChat-style lazy data-src over data: SVG placeholder', () => {
    const placeholder =
      "data:image/svg+xml,%3Csvg width='1px' height='1px' viewBox='0 0 1 1'%3E%3C/svg%3E";
    const html = `<!DOCTYPE html><html><head><title>WX Article</title></head><body><div id="js_content"><p>Intro sentence one. Intro sentence two. Intro sentence three for Readability.</p><img src="${placeholder}" data-src="https://mmbiz.qpic.cn/sz_mmbiz_png/demo/640.png" alt="重新定义交互。" /><p>More body text here. Second sentence. Third sentence for length.</p></div></body></html>`;
    const dom = new JSDOM(html, { url: 'https://mp.weixin.qq.com/s/test' });
    const result = clipPageToMarkdown(dom.window.document, 'https://mp.weixin.qq.com/s/test');
    expect(result.markdown).toContain('https://mmbiz.qpic.cn/sz_mmbiz_png/demo/640.png');
    expect(result.markdown).not.toContain('data:image/svg+xml');
  });

  it('should emit YAML front matter for clip metadata', () => {
    const html = '<!DOCTYPE html><html><head><title>Test Article</title></head><body><article><h1>Main Title</h1><p>Some introductory text that provides context. Multiple sentences are needed here. This should be sufficient content for the article extraction to work properly.</p></article></body></html>';
    const dom = new JSDOM(html, { url: 'https://example.com/page?q=1' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/page?q=1');
    expect(result.markdown.startsWith('---\n')).toBe(true);
    expect(result.markdown).toMatch(/^---\ntitle: "Test Article"\nsource: "https:\/\/example\.com\/page\?q=1"\nclipped: "/);
    expect(result.markdown).toContain('\n---\n\n# Test Article\n\n');
  });

  it('should emit flattened OG fields in YAML front matter', () => {
    const html = `<!DOCTYPE html><html><head>
      <title>Test Article</title>
      <meta property="og:title" content="OG Title" />
      <meta property="og:description" content="A preview description." />
      <meta property="og:image" content="/preview.jpg" />
      <meta property="og:url" content="https://example.com/page" />
      <meta property="og:type" content="article" />
      <meta property="og:site_name" content="Example" />
    </head><body><article><h1>Main Title</h1><p>Some introductory text that provides context. Multiple sentences are needed here. This should be sufficient content for the article extraction to work properly.</p></article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/page' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/page');
    expect(result.markdown).not.toContain('\nog:\n');
    expect(result.markdown).toContain('description: "A preview description."');
    expect(result.markdown).toContain('site_name: "Example"');
    expect(result.markdown).toContain('image_url: "https://example.com/preview.jpg"');
    expect(result.markdown).not.toContain('og:title');
    expect(result.markdown).not.toMatch(/^type: /m);
  });

  it('should fall back to twitter image when og:image is missing', () => {
    const html = `<!DOCTYPE html><html><head>
      <title>Test Article</title>
      <meta name="twitter:description" content="Tweet desc" />
      <meta name="twitter:image" content="https://cdn.example.com/card.png" />
    </head><body><article><h1>Main Title</h1><p>Some introductory text that provides context. Multiple sentences are needed here. This should be sufficient content for the article extraction to work properly.</p></article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/page' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/page');
    expect(result.markdown).toContain('description: "Tweet desc"');
    expect(result.markdown).toContain('image_url: "https://cdn.example.com/card.png"');
    expect(result.markdown).not.toContain('\nog:\n');
  });

  it('should omit OG fields when no preview tags exist', () => {
    const html = '<!DOCTYPE html><html><head><title>Test Article</title></head><body><article><h1>Main Title</h1><p>Some introductory text that provides context. Multiple sentences are needed here. This should be sufficient content for the article extraction to work properly.</p></article></body></html>';
    const dom = new JSDOM(html, { url: 'https://example.com/page' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/page');
    expect(result.markdown).not.toContain('\nog:\n');
    expect(result.markdown).not.toContain('description:');
    expect(result.markdown).not.toContain('site_name:');
    expect(result.markdown).not.toContain('image_url:');
  });

  it('should convert object SVG embeds', () => {
    const html = '<!DOCTYPE html><html><head><title>Test Article</title></head><body><article><h1>Main Article Title</h1><p>This is a paragraph with enough content to make Readability happy. We need multiple sentences here. This should be sufficient content for the article extraction to work properly.</p><object type="image/svg+xml" data="test.svg" alt="Test SVG"></object><p>More content after the image. Another sentence here. And yet another one to make sure we have enough text.</p></article></body></html>';
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');
    expect(result.markdown).toContain('![');
    expect(result.markdown).toContain('test.svg');
  });

  it('should convert img tags', () => {
    const html = '<!DOCTYPE html><html><head><title>Test Article</title></head><body><article><h1>Article Title</h1><p>Some introductory text that provides context. Multiple sentences are needed here. This ensures Readability processes the content.</p><img src="photo.jpg" alt="Photo"><p>More text after the image with additional context. Another sentence. And one more for good measure.</p></article></body></html>';
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');
    expect(result.markdown).toContain('![Photo](https://example.com/photo.jpg)');
  });

  it('should fix invalid nested lists (ul > ul, Zhihu-style) before Readability', () => {
    // Zhihu emits <ul><li>Parent</li><ul><li>Child</li></ul></ul> — nested <ul>
    // is a direct child of the outer <ul>, not inside a <li>. We must repair
    // this structure so the indentation is preserved in the markdown output.
    const html = `<!DOCTYPE html><html><head><title>Nested List Test</title></head><body><article>
      <h1>Best Practices</h1>
      <p>Some introductory text that provides context. Multiple sentences needed. Third sentence here.</p>
      <ul>
        <li><b>指令要具体，而非模糊</b>:</li>
        <ul>
          <li><b>推荐</b>: <code>使用 Prettier。</code></li>
          <li><b>不推荐</b>: <code>保持整洁。</code></li>
        </ul>
        <br/>
        <li><b>结构化组织</b>: 使用 Markdown 标题。</li>
      </ul>
      <p>Footer text with more content. Another sentence. One more sentence.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    // Sub-items must appear indented under their parent (contain leading spaces)
    expect(result.markdown).toContain('推荐');
    expect(result.markdown).toContain('不推荐');

    const lines = result.markdown.split('\n');
    const tuijianLine = lines.find(l => l.includes('推荐'));
    expect(tuijianLine).toBeDefined();
    // Indented list items start with spaces (turndown uses 4-space indent)
    expect(tuijianLine!.trimStart()).not.toEqual(tuijianLine);
  });

  it('should keep table rows on one line when cells use section/span (WeChat-style)', () => {
    const html = `<!DOCTYPE html><html><head><title>Table WX</title></head><body><article>
      <h1>功能表</h1>
      <p>Some introductory text that provides context. Multiple sentences needed. Third sentence here.</p>
      <table>
        <thead><tr><th>功能</th><th>说明</th></tr></thead>
        <tbody>
          <tr>
            <td><section><span leaf="">🔬 Deep Research</span></section></td>
            <td><section><span leaf="">一键提取思考过程与研究链接，打开黑箱</span></section></td>
          </tr>
          <tr>
            <td><section><span leaf="">🍌 水印去除</span></section></td>
            <td><section><span leaf="">让 AI 生成内容回归纯净</span></section></td>
          </tr>
        </tbody>
      </table>
      <p>Footer text with more content. Another sentence. One more sentence.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://mp.weixin.qq.com/s/test' });
    const result = clipPageToMarkdown(dom.window.document, 'https://mp.weixin.qq.com/s/test');

    const lines = result.markdown.split('\n');
    const tableLines = lines.filter((l) => l.trim().startsWith('|'));
    for (const line of tableLines) {
      expect(line).not.toContain('\n');
    }
    const dataRow = tableLines.find((l) => l.includes('Deep Research'));
    expect(dataRow).toBeDefined();
    expect(dataRow).toContain('一键提取');
    expect(dataRow).not.toMatch(/\|\s*\n/);
  });

  it('should keep table rows on a single line when cells contain <br> or lists', () => {
    const html = `<!DOCTYPE html><html><head><title>Table Test</title></head><body><article>
      <h1>Table Test</h1>
      <p>Some introductory text that provides context. Multiple sentences needed. Third sentence here.</p>
      <table>
        <thead><tr><th>类型</th><th>路径</th><th>场景</th></tr></thead>
        <tbody>
          <tr>
            <td>用户内存</td>
            <td>~/.claude/CLAUDE.md</td>
            <td><ul><li>代码风格</li><li>Git 格式</li><li>快捷指令</li></ul></td>
          </tr>
          <tr>
            <td>项目内存</td>
            <td>./CLAUDE.md</td>
            <td>API 规范<br>部署流程</td>
          </tr>
        </tbody>
      </table>
      <p>Footer text with more content. Another sentence. One more sentence.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    const lines = result.markdown.split('\n');
    const tableLines = lines.filter(l => l.trim().startsWith('|'));

    // Every table row must be a single line (no embedded newlines)
    for (const line of tableLines) {
      expect(line).not.toContain('\n');
    }

    // Cell content from <li> should appear inline with <br>
    const dataRow = tableLines.find(l => l.includes('用户内存'));
    expect(dataRow).toBeDefined();
    expect(dataRow).toContain('代码风格');
    expect(dataRow).toContain('<br>');

    // Cell content from <br> should also be inline
    const brRow = tableLines.find(l => l.includes('项目内存'));
    expect(brRow).toBeDefined();
    expect(brRow).toContain('API 规范<br>部署流程');
  });

  it('should convert 2-column KV card groups to bullet lists', () => {
    // Cards with a label + numeric value div pair should become "- **label**: value"
    // lists rather than raw HTML, so they survive Readability and render cleanly.
    const html = `<!DOCTYPE html><html><head><title>Team Economics</title></head><body>
      <article>
        <h1>Team Economics</h1>
        <p>Understanding team costs is crucial for product decisions. Multiple sentences needed here. Third sentence for Readability threshold.</p>
        <div class="metrics">
          <div class="card"><div class="label">Annual cost</div><div class="value">€1,040,000</div></div>
          <div class="card"><div class="label">Monthly cost</div><div class="value">€86,667</div></div>
          <div class="card"><div class="label">Daily cost</div><div class="value">€4,000</div></div>
        </div>
        <p>More prose follows after the metrics. Another sentence. One more for good measure.</p>
      </article>
    </body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    // Each item must appear as its own bullet line.
    // Turndown uses "- " + 3 spaces internally, so we match "-\s+" rather than "- ".
    expect(result.markdown).toMatch(/^-\s+\*\*Annual cost\*\*: €1,040,000\s*$/m);
    expect(result.markdown).toMatch(/^-\s+\*\*Monthly cost\*\*: €86,667\s*$/m);
    expect(result.markdown).toMatch(/^-\s+\*\*Daily cost\*\*: €4,000\s*$/m);
    // Must not leak raw HTML
    expect(result.markdown).not.toContain('<table');
    expect(result.markdown).not.toContain('<div');
  });

  it('should not convert div groups that lack numeric/currency values', () => {
    // Plain navigation-style divs (no numbers) must not be turned into bullet lists.
    const html = `<!DOCTYPE html><html><head><title>Article</title></head><body>
      <article>
        <h1>Article Title</h1>
        <p>Main article content with enough text for Readability. Second sentence. Third sentence here.</p>
        <div class="nav">
          <div>Introduction</div>
          <div>Background</div>
          <div>Conclusion</div>
        </div>
        <p>More article content follows. Another sentence. One more sentence.</p>
      </article>
    </body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    // Should NOT be treated as a KV card group
    expect(result.markdown).not.toMatch(/\*\*Introduction\*\*:/);
    expect(result.markdown).not.toMatch(/\*\*Background\*\*:/);
  });

  it('should preserve input values in calculator tables', () => {
    // <input value="8"> inside a table must survive as plain text "8",
    // not disappear when Readability strips the form element.
    const html = `<!DOCTYPE html><html><head><title>Calculator</title></head><body>
      <article>
        <h1>Team Cost Calculator</h1>
        <p>Use the calculator below to estimate costs. Multiple sentences for Readability. Third sentence here.</p>
        <table>
          <tr><td>Team size</td><td><input type="number" value="8"></td></tr>
          <tr><td>Annual cost</td><td>€1,040,000</td></tr>
        </table>
        <p>Adjust the values to see different scenarios. Another sentence. One more.</p>
      </article>
    </body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    expect(result.markdown).toContain('€1,040,000');
    expect(result.markdown).not.toContain('<input');
  });

  it('should convert unconverted HTML tables to GFM via second-pass', () => {
    // A table with inconsistent column counts is one case Turndown's GFM plugin
    // silently skips (leaving raw HTML). The second-pass convertRemainingHtmlTables
    // must catch it and emit a padded GFM table instead.
    const html = `<!DOCTYPE html><html><head><title>Stats</title></head><body>
      <article>
        <h1>Performance Stats</h1>
        <p>Here are the performance numbers for this quarter. Multiple sentences needed. Third sentence here.</p>
        <table>
          <tr><td>Metric</td><td>Value</td><td>Change</td></tr>
          <tr><td>Revenue</td><td>€1,000,000</td></tr>
          <tr><td>Costs</td><td>€800,000</td><td>+5%</td></tr>
        </table>
        <p>More analysis follows. Another sentence. One more sentence here.</p>
      </article>
    </body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    // No raw HTML table tags
    expect(result.markdown).not.toContain('<table');
    expect(result.markdown).not.toContain('<tr');
    // GFM table markers present
    expect(result.markdown).toContain('|');
    expect(result.markdown).toContain('---');
    // Data preserved
    expect(result.markdown).toContain('Revenue');
    expect(result.markdown).toContain('€1,000,000');
  });

  it('should convert video with poster to a thumbnail link', () => {
    const html = `<!DOCTYPE html><html><head><title>Video Article</title></head><body><article>
      <h1>Article Title</h1>
      <p>Some introductory text with enough content for Readability. Multiple sentences are needed here. This is the third sentence.</p>
      <figure><video src="https://example.com/demo.mp4" poster="https://example.com/thumb.avif" autoplay muted loop></video></figure>
      <p>More article content after the video. Another sentence here. And one more for good measure.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');
    expect(result.markdown).toContain('[![Video](https://example.com/thumb.avif)](https://example.com/demo.mp4)');
  });

  it('should use fallback <source> when <video> has no src attribute', () => {
    const html = `<!DOCTYPE html><html><head><title>Video Article</title></head><body><article>
      <h1>Article Title</h1>
      <p>Some introductory text with enough content for Readability. Multiple sentences are needed here. This is the third sentence.</p>
      <figure><video poster="https://example.com/thumb.avif" muted loop>
        <source media="(resolution >= 2x)" src="https://example.com/hi.mp4" type="video/mp4">
        <source src="https://example.com/fallback.mp4" type="video/mp4">
      </video></figure>
      <p>More article content after the video. Another sentence here. And one more for good measure.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');
    // Should pick the <source> without a media attribute (universal fallback)
    expect(result.markdown).toContain('[![Video](https://example.com/thumb.avif)](https://example.com/fallback.mp4)');
  });

  it('should convert video without poster to a plain link', () => {
    const html = `<!DOCTYPE html><html><head><title>Video Article</title></head><body><article>
      <h1>Article Title</h1>
      <p>Some introductory text with enough content for Readability. Multiple sentences are needed here. This is the third sentence.</p>
      <figure><video src="https://example.com/demo.mp4" autoplay muted loop></video></figure>
      <p>More article content after the video. Another sentence here. And one more for good measure.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');
    expect(result.markdown).toContain('[Video](https://example.com/demo.mp4)');
  });

  it('should resolve relative video src and poster to absolute URLs', () => {
    const html = `<!DOCTYPE html><html><head><title>Video Article</title></head><body><article>
      <h1>Article Title</h1>
      <p>Some introductory text with enough content for Readability. Multiple sentences are needed here. This is the third sentence.</p>
      <figure><video src="_media/post/2.mp4" poster="_media/post/2-thumbnail.avif" autoplay muted loop></video></figure>
      <p>More article content after the video. Another sentence here. And one more for good measure.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://example.com/article/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/article/');
    expect(result.markdown).toContain('https://example.com/article/_media/post/2.mp4');
    expect(result.markdown).toContain('https://example.com/article/_media/post/2-thumbnail.avif');
  });

  it('should convert inline SVG diagrams to data-URI images', () => {
    const html = `<!DOCTYPE html><html><head><title>Agent Loop</title></head><body><article>
      <h1>Agent Loop</h1>
      <p>The agent cognition loop is fundamental. Multiple sentences for Readability. Third sentence here.</p>
      <div class="diagram-wrap">
        <div class="diagram-label">图 1 · Agent 核心循环</div>
        <svg viewBox="0 0 640 300" xmlns="http://www.w3.org/2000/svg">
          <rect x="240" y="18" width="160" height="56" rx="28" fill="#1a3a5c"/>
          <text x="320" y="50" text-anchor="middle" fill="#fff">Observe</text>
        </svg>
      </div>
      <p>More article content follows. Another sentence. One more sentence.</p>
    </article></body></html>`;
    const dom = new JSDOM(html, { url: 'https://agent-cognition.pages.dev/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://agent-cognition.pages.dev/');

    // SVG should be converted to an image
    expect(result.markdown).toContain('![');
    expect(result.markdown).toContain('data:image/svg+xml;base64,');
    // Alt text should come from the diagram-label sibling
    expect(result.markdown).toContain('图 1 · Agent 核心循环');
    // No raw <svg> tag should leak through
    expect(result.markdown).not.toContain('<svg');
  });

  it('should clean up newlines in link text', () => {
    const html = '<!DOCTYPE html><html><head><title>Test</title></head><body><article><h1>Title</h1><p>Text with enough content to pass Readability. Multiple sentences are important. This is the third sentence.</p><p><a href="https://example.com">Create\nan\nAPI\nKey</a> and start using it.</p><p>More text to ensure extraction. Another sentence. And one more.</p></article></body></html>';
    const dom = new JSDOM(html, { url: 'https://example.com/' });
    const result = clipPageToMarkdown(dom.window.document, 'https://example.com/');

    // Link text should be on one line
    expect(result.markdown).toContain('[Create an API Key](https://example.com/)');
  });
});

describe('Real Website Tests', () => {
  for (const testCase of WEBSITE_TEST_CASES) {
    it(testCase.description, async () => {
      const response = await fetch(testCase.url);
      const html = await response.text();
      const dom = new JSDOM(html, { url: testCase.url });
      const result = clipPageToMarkdown(dom.window.document, testCase.url);

      if (testCase.expectations.titleContains) {
        expect(result.title.toLowerCase()).toContain(testCase.expectations.titleContains.toLowerCase());
      }

      if (testCase.expectations.mustContain) {
        for (const content of testCase.expectations.mustContain) {
          expect(result.markdown).toContain(content);
        }
      }

      if (testCase.expectations.mustNotContain) {
        for (const content of testCase.expectations.mustNotContain) {
          expect(result.markdown).not.toContain(content);
        }
      }

      console.log(`\n=== ${testCase.description} ===`);
      console.log(`URL: ${testCase.url}`);
      console.log(`Title: ${result.title}`);
      console.log(`Length: ${result.markdown.length} chars\n`);
    }, 30000);
  }
});
