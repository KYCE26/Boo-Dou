export type Role = 'user' | 'assistant' | 'system'

export interface ChatMessage {
  role: Role
  content: string
}

export class ChatError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ChatError'
  }
}

interface StreamOptions {
  signal?: AbortSignal
  onToken: (text: string) => void
}

interface SseEvent {
  event: string
  data: string
}

// The backend sends every outcome, including validation errors, as SSE, so
// the body is parsed even when the status is not 2xx.
export async function streamChat(messages: ChatMessage[], { signal, onToken }: StreamOptions): Promise<void> {
  let res: Response
  try {
    res = await fetch('/api/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      body: JSON.stringify({ messages }),
      signal,
    })
  } catch (err) {
    if (isAbort(err)) throw err
    throw new ChatError('Tidak bisa menghubungi server. Pastikan backend sedang berjalan.')
  }

  if (!res.body || !res.headers.get('Content-Type')?.includes('text/event-stream')) {
    throw new ChatError(`Server membalas dengan format yang tidak dikenal (status ${res.status}).`)
  }

  for await (const evt of readEvents(res.body)) {
    if (evt.event === 'token') {
      onToken(parsePayload(evt.data).content ?? '')
    } else if (evt.event === 'error') {
      throw new ChatError(parsePayload(evt.data).message ?? 'Terjadi kesalahan yang tidak diketahui.')
    } else if (evt.event === 'done') {
      return
    }
  }

  throw new ChatError('Koneksi terputus sebelum jawaban selesai.')
}

export function isAbort(err: unknown): boolean {
  return err instanceof DOMException && err.name === 'AbortError'
}

function parsePayload(data: string): { content?: string; message?: string } {
  try {
    return JSON.parse(data)
  } catch {
    return {}
  }
}

async function* readEvents(body: ReadableStream<Uint8Array>): AsyncGenerator<SseEvent> {
  const reader = body.getReader()
  // stream: true keeps multi-byte characters intact when split across chunks.
  const decoder = new TextDecoder()
  let buffer = ''
  try {
    while (true) {
      const { value, done } = await reader.read()
      if (done) break
      // Normalising CRLF keeps the blank-line split working behind proxies
      // that rewrite line endings. Done on the joined buffer so a pair split
      // across two chunks still collapses to one newline.
      buffer = (buffer + decoder.decode(value, { stream: true })).replace(/\r\n/g, '\n')

      let boundary = buffer.indexOf('\n\n')
      while (boundary !== -1) {
        const evt = parseBlock(buffer.slice(0, boundary))
        buffer = buffer.slice(boundary + 2)
        if (evt) yield evt
        boundary = buffer.indexOf('\n\n')
      }
    }
  } finally {
    // Runs when the caller stops early on done/error; cancelling frees the
    // connection instead of leaving it half-read.
    reader.cancel().catch(() => {})
  }
}

function parseBlock(block: string): SseEvent | null {
  let event = 'message'
  const data: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith('event:')) event = line.slice(6).trim()
    else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
  }
  return data.length ? { event, data: data.join('\n') } : null
}
