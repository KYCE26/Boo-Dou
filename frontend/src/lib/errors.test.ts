import { describe, expect, it } from 'vitest'
import { errorKind } from './errors'

describe('errorKind', () => {
  it.each([
    [429, 'quota'],
    [401, 'key'],
    [403, 'key'],
    [503, 'config'],
    [0, 'offline'],
    [400, 'request'],
    [415, 'request'],
    [502, 'provider'],
    [504, 'provider'],
    [200, 'provider'],
    [418, 'unknown'],
  ])('status %i -> %s', (status, kind) => {
    expect(errorKind(status)).toBe(kind)
  })

  it('offline menang atas status apa pun', () => {
    expect(errorKind(429, false)).toBe('offline')
  })
})
