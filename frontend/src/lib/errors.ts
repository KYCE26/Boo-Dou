import type { ErrorKind } from '../copy'

// The backend speaks HTTP status codes; the UI needs a cause it can name and a
// next step. A 200 here means the stream started and the provider failed later.
export function errorKind(status: number, online = true): ErrorKind {
  if (!online || status === 0) return 'offline'
  if (status === 429) return 'quota'
  if (status === 401 || status === 403) return 'key'
  if (status === 503) return 'config'
  if (status === 400 || status === 404 || status === 413 || status === 415) return 'request'
  if (status === 200 || status >= 500) return 'provider'
  return 'unknown'
}
