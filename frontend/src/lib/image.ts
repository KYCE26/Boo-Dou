export const MAX_IMAGES = 4
export const ACCEPTED_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const
// Matches the server limit; checked again after compression.
export const MAX_UPLOAD_BYTES = 5 * 1024 * 1024
// Raw files above this are refused before decoding, to keep a phone from
// running out of memory on a huge panorama.
export const MAX_SOURCE_BYTES = 25 * 1024 * 1024
export const MAX_SIDE = 1568
export const JPEG_QUALITY = 0.85

export interface Rejection {
  name: string
  reason: string
}

export interface Selection {
  accepted: File[]
  rejected: Rejection[]
}

const formatMB = (bytes: number) => `${Math.round(bytes / (1024 * 1024))} MB`

// Pure so it can be unit tested; decides which files may even be decoded.
export function selectImages(files: File[], alreadyAttached: number): Selection {
  const accepted: File[] = []
  const rejected: Rejection[] = []
  for (const file of files) {
    const name = file.name || 'Gambar tempelan'
    if (!(ACCEPTED_TYPES as readonly string[]).includes(file.type)) {
      rejected.push({ name, reason: 'bukan gambar JPEG, PNG, atau WebP' })
    } else if (file.size > MAX_SOURCE_BYTES) {
      rejected.push({ name, reason: `terlalu besar (lebih dari ${formatMB(MAX_SOURCE_BYTES)})` })
    } else if (alreadyAttached + accepted.length >= MAX_IMAGES) {
      rejected.push({ name, reason: `melebihi batas ${MAX_IMAGES} gambar per pesan` })
    } else {
      accepted.push(file)
    }
  }
  return { accepted, rejected }
}

export function fitWithin(width: number, height: number, maxSide = MAX_SIDE) {
  const scale = Math.min(1, maxSide / Math.max(width, height))
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) }
}

export class CompressError extends Error {}

// Redrawing through a canvas keeps only pixels, so EXIF (including GPS from
// phone photos) never leaves the device. imageOrientation applies the EXIF
// rotation first, so the stripped photo is still upright.
export async function compressImage(file: Blob): Promise<Blob> {
  let bitmap: ImageBitmap
  try {
    bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
  } catch {
    throw new CompressError('gambar tidak bisa dibaca browser')
  }
  try {
    const { width, height } = fitWithin(bitmap.width, bitmap.height)
    const canvas = document.createElement('canvas')
    canvas.width = width
    canvas.height = height
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new CompressError('canvas tidak tersedia')
    // JPEG has no alpha; without a fill, transparent PNG areas turn black.
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, width, height)
    ctx.drawImage(bitmap, 0, 0, width, height)
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY))
    if (!blob) throw new CompressError('gagal membuat JPEG')
    return blob
  } finally {
    bitmap.close()
  }
}
