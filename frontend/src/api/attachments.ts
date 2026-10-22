import type { Attachment } from './conversations'
import { ApiError, OFFLINE_MESSAGE } from './http'

export interface UploadHandle {
  promise: Promise<Attachment>
  abort: () => void
}

// XMLHttpRequest instead of fetch because only XHR reports upload progress.
export function uploadAttachment(blob: Blob, onProgress: (fraction: number) => void): UploadHandle {
  const xhr = new XMLHttpRequest()
  const promise = new Promise<Attachment>((resolve, reject) => {
    xhr.open('POST', '/api/attachments')
    xhr.responseType = 'json'
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total)
    }
    xhr.onload = () => {
      const body = xhr.response as (Attachment & { error?: string }) | null
      if (xhr.status === 201 && body?.id) {
        onProgress(1)
        resolve(body)
      } else {
        reject(new ApiError(body?.error ?? `Upload gagal (status ${xhr.status}).`, xhr.status))
      }
    }
    xhr.onerror = () => reject(new ApiError(OFFLINE_MESSAGE, 0))
    xhr.onabort = () => reject(new DOMException('aborted', 'AbortError'))

    const form = new FormData()
    form.append('file', blob, 'gambar.jpg')
    xhr.send(form)
  })
  return { promise, abort: () => xhr.abort() }
}
