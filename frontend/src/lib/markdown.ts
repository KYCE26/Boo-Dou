import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'

// Anything else (javascript:, data:, vbscript:, relative paths) renders as
// plain text. Both layers below use the same rule.
const SAFE_URL = /^(?:https?:|mailto:)/i

const md = new MarkdownIt({ html: false, linkify: true, breaks: true })

// A remote image would make the reader's browser contact any host the model
// names, leaking their IP; the model cannot show real images anyway.
md.disable('image')
md.validateLink = (url) => SAFE_URL.test(url.trim())

const esc = md.utils.escapeHtml

md.renderer.rules.fence = (tokens, idx) => {
  const token = tokens[idx]!
  const lang = token.info.trim().split(/\s+/)[0] ?? ''
  const label = lang ? `Salin kode ${lang}` : 'Salin kode'
  // tabindex lets keyboard users scroll wide code that overflows its box.
  return (
    `<div class="code-block">` +
    `<div class="code-head"><span class="code-lang">${esc(lang || 'kode')}</span>` +
    `<button type="button" class="copy-btn" data-copy aria-label="${esc(label)}">Salin</button></div>` +
    `<pre tabindex="0"><code${lang ? ` class="language-${esc(lang)}"` : ''}>${esc(token.content)}</code></pre>` +
    `</div>`
  )
}

md.renderer.rules.table_open = () => '<div class="table-wrap" tabindex="0" role="region" aria-label="Tabel"><table>'
md.renderer.rules.table_close = () => '</table></div>'

// A private instance keeps the link hook from leaking into other DOMPurify users.
const purifier = DOMPurify()

purifier.addHook('afterSanitizeAttributes', (node) => {
  if (node.tagName === 'A') {
    node.setAttribute('target', '_blank')
    node.setAttribute('rel', 'noopener noreferrer')
  }
})

// DOMPurify tests every plain attribute value against this regexp, not only
// href/src, so it must also accept scheme-less values like tabindex="0" or
// type="button". Same shape as DOMPurify's default, with only http(s) and
// mailto left as schemes; javascript: and data: still fail.
const PURIFY_URI = /^(?:(?:https?|mailto):|[^a-z]|[a-z+.-]+(?:[^a-z+.\-:]|$))/i

const PURIFY_CONFIG = {
  ALLOWED_URI_REGEXP: PURIFY_URI,
  ADD_ATTR: ['target', 'tabindex'],
  FORBID_TAGS: ['style', 'img', 'svg', 'math', 'form', 'input', 'iframe'],
  FORBID_ATTR: ['style'],
}

export function sanitizeHtml(html: string): string {
  return purifier.sanitize(html, PURIFY_CONFIG) as string
}

// Always sanitised: markdown-it with html:false is the first gate, DOMPurify
// the second, so a future markdown plugin cannot open a hole on its own.
export function renderMarkdown(source: string): string {
  return sanitizeHtml(md.render(source))
}
