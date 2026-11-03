import { expect, it, vi } from 'vitest'
it('defaults to stars, shares the ref and persists each supported choice', async () => {
  vi.resetModules()
  const { mapBackground } = await import('../useMapBackground')
  expect(mapBackground.value).toBe('stars')
  expect((await import('../useMapBackground')).mapBackground).toBe(mapBackground)
  for (const value of ['gradient', 'simple', 'stars'] as const) {
    mapBackground.value = value
    await vi.waitFor(() => expect(localStorage.getItem('photoly-map-background')).toBe(value))
  }
})
it('restores the saved background and repairs unsupported preferences', async () => {
  vi.resetModules()
  localStorage.setItem('photoly-map-background', 'simple')
  const { mapBackground } = await import('../useMapBackground')
  expect(mapBackground.value).toBe('simple')
  Reflect.set(mapBackground, 'value', 'invalid')
  expect(mapBackground.value).toBe('stars')
  await vi.waitFor(() => expect(localStorage.getItem('photoly-map-background')).toBe('stars'))
})
it('repairs an invalid stored background on startup', async () => {
  vi.resetModules()
  localStorage.setItem('photoly-map-background', 'old-background')
  const { mapBackground } = await import('../useMapBackground')
  expect(mapBackground.value).toBe('stars')
})
