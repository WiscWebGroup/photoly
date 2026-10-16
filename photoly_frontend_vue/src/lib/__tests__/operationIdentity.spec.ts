import { afterEach, expect, test, vi } from 'vitest'
import { createHash, webcrypto } from 'node:crypto'
import { newOperationKey, operationStorageKey } from '../operationIdentity'

afterEach(() => vi.unstubAllGlobals())

test('HTTP operation keys use secure randomness without secure-context-only APIs', () => {
  vi.stubGlobal('crypto', { getRandomValues: webcrypto.getRandomValues.bind(webcrypto) })
  const keys = Array.from({ length: 100 }, newOperationKey)
  expect(new Set(keys).size).toBe(100)
  expect(keys.every((key) => /^[a-f0-9]{32}$/.test(key))).toBe(true)
})

test('session namespace remains SHA-256 compatible and isolates tokens on HTTP', () => {
  vi.stubGlobal('crypto', { getRandomValues: webcrypto.getRandomValues.bind(webcrypto) })
  for (const token of ['e2e-token', 'another-token', '会话-token']) {
    expect(operationStorageKey(token)).toBe(
      'photoly-ai-operation-' + createHash('sha256').update(token).digest('hex'),
    )
    expect(operationStorageKey(token)).not.toContain(token)
  }
  expect(operationStorageKey('e2e-token')).not.toBe(operationStorageKey('another-token'))
})
