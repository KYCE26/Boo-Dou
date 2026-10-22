export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export const OFFLINE_MESSAGE = 'Tidak bisa menghubungi server. Pastikan backend sedang berjalan.'

// The backend answers errors as {"error": "..."} in Indonesian, ready to show.
export async function requestJSON<T>(method: string, url: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(url, {
      method,
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch {
    throw new ApiError(OFFLINE_MESSAGE, 0)
  }
  if (res.status === 204) return undefined as T

  let data: unknown = null
  try {
    data = await res.json()
  } catch {
    // Non-JSON bodies (a proxy error page, for instance) fall through to the status message.
  }
  if (!res.ok) {
    const msg = (data as { error?: unknown } | null)?.error
    throw new ApiError(typeof msg === 'string' ? msg : `Server membalas dengan status ${res.status}.`, res.status)
  }
  return data as T
}
