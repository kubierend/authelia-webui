export function renderMarkdown(markdown) {
  const lines = String(markdown || '').split('\n');
  const html = [];
  let paragraph = [];
  let list = [];
  let code = [];
  let inCode = false;
  let codeLanguage = '';

  function flushParagraph() {
    if (paragraph.length === 0) return;
    html.push(`<p>${renderInline(paragraph.join(' '))}</p>`);
    paragraph = [];
  }

  function flushList() {
    if (list.length === 0) return;
    html.push(`<ul>${list.map((item) => `<li>${renderInline(item)}</li>`).join('')}</ul>`);
    list = [];
  }

  function flushCode() {
    if (code.length === 0 && !inCode) return;
    const languageClass = codeLanguage ? ` class="language-${escapeAttribute(codeLanguage)}"` : '';
    html.push(`<pre><code${languageClass}>${escapeHTML(code.join('\n'))}</code></pre>`);
    code = [];
    codeLanguage = '';
  }

  for (const rawLine of lines) {
    const line = rawLine.replace(/\r$/, '');
    const trimmed = line.trim();

    if (trimmed.startsWith('```')) {
      if (inCode) {
        inCode = false;
        flushCode();
      } else {
        flushParagraph();
        flushList();
        inCode = true;
        codeLanguage = trimmed.slice(3).split(/\s+/)[0] || '';
        code = [];
      }
      continue;
    }

    if (inCode) {
      code.push(line);
      continue;
    }

    if (trimmed === '' || trimmed.startsWith('{{%') || trimmed.startsWith('{{<')) {
      flushParagraph();
      flushList();
      continue;
    }

    const heading = /^(#{2,6})\s+(.+)$/.exec(trimmed);
    if (heading) {
      flushParagraph();
      flushList();
      const level = Math.min(heading[1].length + 1, 6);
      html.push(`<h${level}>${renderInline(heading[2])}</h${level}>`);
      continue;
    }

    const bullet = /^[-*]\s+(.+)$/.exec(trimmed);
    if (bullet) {
      flushParagraph();
      list.push(bullet[1]);
      continue;
    }

    flushList();
    paragraph.push(trimmed);
  }

  flushParagraph();
  flushList();
  if (inCode || code.length > 0) {
    flushCode();
  }

  return html.join('');
}

function renderInline(value) {
  let text = escapeHTML(value);
  text = text.replace(/`([^`]+)`/g, '<code>$1</code>');
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_match, label, href) => {
    return `<a href="${escapeAttribute(href)}" target="_blank" rel="noreferrer">${label}</a>`;
  });
  return text;
}

function escapeHTML(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
}

function escapeAttribute(value) {
  return escapeHTML(value).replaceAll('`', '&#96;');
}
