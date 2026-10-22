import { describe, expect, it } from 'vitest'
import { fitWithin, MAX_IMAGES, MAX_SOURCE_BYTES, selectImages } from './image'

const file = (name: string, type: string, size = 1000) => {
  const f = new File([new Uint8Array(1)], name, { type })
  Object.defineProperty(f, 'size', { value: size })
  return f
}

describe('selectImages', () => {
  it('menolak file bukan gambar sebelum ada request', () => {
    const { accepted, rejected } = selectImages([file('catatan.txt', 'text/plain'), file('foto.heic', 'image/heic')], 0)
    expect(accepted).toHaveLength(0)
    expect(rejected.map((r) => r.reason)).toEqual(['bukan gambar JPEG, PNG, atau WebP', 'bukan gambar JPEG, PNG, atau WebP'])
  })

  it('menolak file mentah yang terlalu besar', () => {
    const { rejected } = selectImages([file('besar.jpg', 'image/jpeg', MAX_SOURCE_BYTES + 1)], 0)
    expect(rejected[0]!.reason).toMatch(/terlalu besar/)
  })

  it(`membatasi ${MAX_IMAGES} gambar termasuk yang sudah terlampir`, () => {
    const files = ['a', 'b', 'c'].map((n) => file(`${n}.png`, 'image/png'))
    const { accepted, rejected } = selectImages(files, 2)
    expect(accepted.map((f) => f.name)).toEqual(['a.png', 'b.png'])
    expect(rejected).toEqual([{ name: 'c.png', reason: `melebihi batas ${MAX_IMAGES} gambar per pesan` }])
  })

  it('memberi nama untuk gambar tempelan tanpa nama', () => {
    expect(selectImages([file('', 'text/html')], 0).rejected[0]!.name).toBe('Gambar tempelan')
  })
})

describe('fitWithin', () => {
  it('memperkecil sisi terpanjang ke 1568 dan menjaga rasio', () => {
    expect(fitWithin(4032, 3024)).toEqual({ width: 1568, height: 1176 })
    expect(fitWithin(900, 1200)).toEqual({ width: 900, height: 1200 })
    expect(fitWithin(1000, 4000)).toEqual({ width: 392, height: 1568 })
  })
})
