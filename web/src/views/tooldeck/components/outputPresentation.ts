export const outputFormats = [
  { value: 'json', label: 'JSON · application/json' },
  { value: 'text', label: '纯文本 · text/plain' },
  { value: 'html', label: 'HTML · text/html' },
  { value: 'markdown', label: 'Markdown · text/markdown' },
  { value: 'stream', label: '流式文本 · text/event-stream' },
  { value: 'csv', label: 'CSV · text/csv' },
  { value: 'xml', label: 'XML · application/xml' },
  { value: 'image-gallery', label: '图片与文件产物' }
]

export function normalizeOutputType(type?: string): string {
  const aliases: Record<string, string> = {
    'application/json': 'json',
    'text/plain': 'text',
    'text/html': 'html',
    'text/csv': 'csv',
    'text/xml': 'xml',
    'application/xml': 'xml',
    md: 'markdown',
    'text/markdown': 'markdown',
    'text/event-stream': 'stream'
  }
  const value = aliases[type || ''] || type
  return outputFormats.some((item) => item.value === value) ? value! : 'json'
}

// Keep Markdown preview inert; expose only explicit HTTPS Markdown links separately.
export function markdownExternalLinks(source: string): { label: string; href: string }[] {
  const links: { label: string; href: string }[] = []
  for (const match of source.matchAll(/\[([^\]\r\n]{1,120})\]\((https:\/\/[^\s)]+)\)/gi)) {
    try {
      const url = new URL(match[2])
      if (url.protocol !== 'https:' || !url.hostname || url.username || url.password) continue
      if (!links.some((link) => link.href === url.href)) links.push({ label: match[1], href: url.href })
    } catch { /* Invalid links remain inert in the preview. */ }
    if (links.length >= 20) break
  }
  return links
}

// Limit preview size; the full source is always available in the other tab.
export function csvPreview(source: string): string[][] | null {
  if (source.length > 100000) return null
  const rows: string[][] = [],
    row: string[] = []
  let field = '',
    quoted = false,
    closed = false
  function cell() {
    row.push(field)
    field = ''
    closed = false
  }
  function line() {
    cell()
    rows.push(row.splice(0))
  }
  for (let i = 0; i < source.length; i++) {
    const char = source[i]
    if (quoted) {
      if (char === '"') {
        if (source[i + 1] === '"') {
          field += '"'
          i++
        } else {
          quoted = false
          closed = true
        }
      } else field += char
    } else if (char === ',') cell()
    else if (char === '\n' || char === '\r') {
      if (char === '\r' && source[i + 1] === '\n') i++
      line()
    } else if (char === '"' && !field && !closed) quoted = true
    else if (closed || char === '"') return null
    else field += char
    if (rows.length > 200 || row.length > 40) return null
  }
  if (quoted) return null
  if (field || row.length || closed || source.endsWith(',')) line()
  if (!rows.length || rows.length > 200 || rows.some((row) => row.length > 40)) return null
  return rows
}

// An inert template avoids activating tool HTML in the host document.
export function sanitizeOutputHtml(source: string, allowStyles = false): string {
  const template = document.createElement('template')
  template.innerHTML = source
  const tags = new Set(
    'a abbr article aside b blockquote br caption code col colgroup dd del details div dl dt em figcaption figure footer h1 h2 h3 h4 h5 h6 header hr i img li main mark nav ol p pre s section small span strong sub summary sup table tbody td th thead tfoot time tr u ul'.split(
      ' '
    )
  )
  if (allowStyles) tags.add('style')
  const attrs = new Set('alt colspan height lang rowspan title width dir'.split(' '))
  if (allowStyles) for (const attr of ['class', 'id', 'style']) attrs.add(attr)
  for (const element of Array.from(template.content.querySelectorAll('*'))) {
    if (element.namespaceURI !== 'http://www.w3.org/1999/xhtml' || !tags.has(element.localName)) {
      element.remove()
      continue
    }
    for (const attr of Array.from(element.attributes)) {
      if (
        element.localName === 'img' &&
        attr.name === 'src' &&
        /^data:image\/(png|jpeg|gif|webp);base64,[a-z0-9+/=]+$/i.test(attr.value)
      )
        continue
      if (!attrs.has(attr.name)) element.removeAttribute(attr.name)
    }
  }
  return template.innerHTML
}

export function htmlPreview(source: string): string {
  return (
    "<!doctype html><html><head><meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'; img-src data:; base-uri 'none'; form-action 'none'\"><style>body{font:14px/1.6 system-ui,sans-serif;padding:16px;overflow-wrap:anywhere;color:#222;background:#fff}img{max-width:100%;height:auto}table{border-collapse:collapse}td,th{padding:8px;border:1px solid #ddd}</style></head><body>" +
    sanitizeOutputHtml(source, true) +
    '</body></html>'
  )
}
