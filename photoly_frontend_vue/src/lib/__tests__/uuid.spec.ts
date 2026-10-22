import { afterEach, expect, it, vi } from 'vitest'
import { webcrypto } from 'node:crypto'
import { newUuid } from '../uuid'

afterEach(() => vi.unstubAllGlobals())

it('uses native randomUUID with its Crypto receiver when available', () => {
  const source = {
    randomUUID: vi.fn(function (this: unknown) {
      expect(this).toBe(source)
      return '4487b991-05a6-4b61-8318-ddfa7feecdc7'
    }),
    getRandomValues: vi.fn(),
  }
  vi.stubGlobal('crypto', source)
  expect(newUuid()).toBe('4487b991-05a6-4b61-8318-ddfa7feecdc7')
  expect(source.randomUUID).toHaveBeenCalledOnce()
  expect(source.getRandomValues).not.toHaveBeenCalled()
})

it('generates distinct standard v4 IDs using secure randomness without randomUUID', () => {
  const random = vi.fn(webcrypto.getRandomValues.bind(webcrypto))
  vi.stubGlobal('crypto', { getRandomValues: random })
  const ids = Array.from({ length: 1000 }, newUuid)
  expect(new Set(ids).size).toBe(1000)
  expect(
    ids.every((id) =>
      /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(id),
    ),
  ).toBe(true)
  expect(random).toHaveBeenCalledTimes(1000)
})

it('reports unavailable secure randomness instead of falling back to Math.random', () => {
  vi.stubGlobal('crypto', undefined)
  expect(newUuid).toThrow('Your browser cannot generate secure upload IDs')
})
