// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'
import { renderMarkdown, sanitizeHtml } from './markdown'

function mount(html: string) {
  const el = document.createElement('div')
  el.innerHTML = html
  document.body.append(el)
  return el
}

function assertInert(el: HTMLElement) {
  expect(el.querySelector('script, img, iframe, object, embed, svg')).toBeNull()
  for (const node of el.querySelectorAll('*')) {
    for (const attr of node.attributes) {
      expect(attr.name.startsWith('on'), `${node.tagName} punya ${attr.name}`).toBe(false)
    }
  }
  // An <a> whose href was stripped is inert; one that keeps an href must be safe.
  for (const a of el.querySelectorAll('a[href]')) {
    expect(a.getAttribute('href')).toMatch(/^(https?:|mailto:)/i)
  }
}

const payloads = [
  '<img src=x onerror=alert(1)>',
  '<script>alert(1)</script>',
  '[klik](javascript:alert(1))',
  '[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)',
  '<a href="javascript:alert(1)">x</a>',
]

describe('renderMarkdown sanitasi', () => {
  it.each(payloads)('payload %s keluar sebagai teks, tidak tereksekusi', (payload) => {
    const alert = vi.fn()
    window.alert = alert
    const el = mount(renderMarkdown(payload))
    assertInert(el)
    expect(el.textContent).toContain(payload)
    expect(alert).not.toHaveBeenCalled()
  })

  it('payload di tengah teks biasa tetap jadi teks', () => {
    const el = mount(renderMarkdown(`Coba ini: ${payloads.join(' dan ')} selesai.`))
    assertInert(el)
    expect(el.querySelector('a')).toBeNull()
  })

  it('variasi skema berbahaya tidak jadi link', () => {
    for (const href of ['JaVaScRiPt:alert(1)', ' javascript:alert(1)', 'vbscript:msgbox(1)', 'data:image/png;base64,AAAA']) {
      const el = mount(renderMarkdown(`[x](${href})`))
      expect(el.querySelector('a'), href).toBeNull()
    }
  })
})

describe('sanitizeHtml sebagai lapis kedua', () => {
  it.each([
    '<img src=x onerror=alert(1)>',
    '<script>alert(1)</script>',
    '<a href="javascript:alert(1)">x</a>',
    '<a href="data:text/html;base64,PHNjcmlwdD4=">x</a>',
    '<svg onload=alert(1)></svg>',
    '<p style="position:fixed" onclick="alert(1)">x</p>',
  ])('HTML mentah %s dibersihkan', (html) => {
    assertInert(mount(sanitizeHtml(html)))
  })
})

describe('renderMarkdown output', () => {
  it('link aman dapat target dan rel', () => {
    const a = mount(renderMarkdown('[doc](https://example.com)')).querySelector('a')!
    expect(a.getAttribute('href')).toBe('https://example.com')
    expect(a.getAttribute('target')).toBe('_blank')
    expect(a.getAttribute('rel')).toBe('noopener noreferrer')
  })

  it('linkify juga dapat target dan rel', () => {
    const a = mount(renderMarkdown('lihat https://example.com/a')).querySelector('a')!
    expect(a.getAttribute('rel')).toBe('noopener noreferrer')
  })

  it('code block punya tombol salin dengan aria-label', () => {
    const el = mount(renderMarkdown('```go\nfmt.Println("<b>hi</b>")\n```'))
    const btn = el.querySelector('button[data-copy]')!
    expect(btn.getAttribute('aria-label')).toBe('Salin kode go')
    // type="button" must survive sanitising, or the button would submit forms.
    expect(btn.getAttribute('type')).toBe('button')
    expect(el.querySelector('pre')!.getAttribute('tabindex')).toBe('0')
    expect(el.querySelector('pre code')!.textContent).toBe('fmt.Println("<b>hi</b>")\n')
    expect(el.querySelector('b')).toBeNull()
  })

  it('code fence yang belum tertutup tetap jadi satu code block utuh', () => {
    const el = mount(renderMarkdown('Contoh:\n\n```go\nfunc main() {\n  x := 1'))
    expect(el.querySelectorAll('.code-block')).toHaveLength(1)
    expect(el.querySelector('pre code')!.textContent).toContain('x := 1')
    expect(el.lastElementChild!.className).toBe('code-block')
  })

  it('tabel dibungkus wadah scroll yang bisa difokus', () => {
    const el = mount(renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |'))
    const wrap = el.querySelector('.table-wrap')!
    expect(wrap.getAttribute('tabindex')).toBe('0')
    expect(wrap.querySelector('table')).not.toBeNull()
  })

  it('gambar markdown tidak dirender', () => {
    expect(mount(renderMarkdown('![x](https://example.com/a.png)')).querySelector('img')).toBeNull()
  })
})
