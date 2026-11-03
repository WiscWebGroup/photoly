import { expect, it, vi } from 'vitest'
it('defaults to globe and persists a valid selection', async () => {
  vi.resetModules()
  const { mapProjection } = await import('../useMapProjection')
  expect(mapProjection.value).toBe('globe')
  mapProjection.value = 'mercator'
  await vi.waitFor(() => expect(localStorage.getItem('photoly-map-projection')).toBe('mercator'))
})
it('restores a saved choice and repairs an invalid choice', async () => {
  vi.resetModules()
  localStorage.setItem('photoly-map-projection', 'mercator')
  const { mapProjection } = await import('../useMapProjection')
  expect(mapProjection.value).toBe('mercator')
  // Simulate an old or corrupt preference, not a supported UI action.
  Reflect.set(mapProjection, 'value', 'invalid')
  expect(mapProjection.value).toBe('globe')
  await vi.waitFor(() => expect(localStorage.getItem('photoly-map-projection')).toBe('globe'))
})
