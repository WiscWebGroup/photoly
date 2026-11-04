import { expect, it, vi } from 'vitest'

it('defaults to enabled, shares the ref and persists both choices', async () => {
  vi.resetModules()
  const { mapDayNight } = await import('../useMapDayNight')
  expect(mapDayNight.value).toBe(true)
  expect((await import('../useMapDayNight')).mapDayNight).toBe(mapDayNight)
  for (const value of [false, true]) {
    mapDayNight.value = value
    await vi.waitFor(() =>
      expect(localStorage.getItem('photoly-map-day-night')).toBe(String(value)),
    )
  }
})

it.each(['false', 'true', 'corrupt'])('restores %s using a safe boolean default', async (value) => {
  vi.resetModules()
  localStorage.setItem('photoly-map-day-night', value)
  const { mapDayNight } = await import('../useMapDayNight')
  expect(mapDayNight.value).toBe(value !== 'false')
})
