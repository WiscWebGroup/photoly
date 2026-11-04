import { expect, it } from 'vitest'
import { globeLightingStrength, solarPosition, surfaceNormal } from '../solarPosition'

it.each([
  ['2026-03-20T14:46:00Z', 0, -39.7],
  ['2026-06-21T08:24:00Z', 23.44, 54.4],
  ['2026-09-23T00:05:00Z', 0, 176.8],
  ['2026-12-21T20:50:00Z', -23.44, -132.9],
])('places the subsolar point within 1° at %s', (date, latitude, longitude) => {
  const position = solarPosition(new Date(date))
  expect(Math.abs(position.latitude - latitude)).toBeLessThan(1)
  expect(Math.abs(position.longitude - longitude)).toBeLessThan(1)
  expect(Math.hypot(...position.direction)).toBeCloseTo(1, 12)
})

it('uses the instant independently of local timezone, including leap day', () => {
  expect(solarPosition(new Date('2024-02-29T12:30:00Z'))).toEqual(
    solarPosition(new Date('2024-02-29T07:30:00-05:00')),
  )
  const before = solarPosition(new Date('2024-02-29T23:59:59Z'))
  const after = solarPosition(new Date('2024-03-01T00:00:01Z'))
  expect(Math.hypot(...before.direction.map((v, i) => v - after.direction[i]!))).toBeLessThan(0.001)
  expect(Math.abs(after.latitude - before.latitude)).toBeLessThan(0.001)
})

it('moves sunlight westward and handles UTC year rollover continuously', () => {
  const a = solarPosition(new Date('2026-03-20T06:00:00Z'))
  const b = solarPosition(new Date('2026-03-20T12:00:00Z'))
  expect(a.longitude - b.longitude).toBeCloseTo(90, 0)
  const before = solarPosition(new Date('2024-12-31T23:59:59Z'))
  const after = solarPosition(new Date('2025-01-01T00:00:01Z'))
  expect(Math.hypot(...before.direction.map((v, i) => v - after.direction[i]!))).toBeLessThan(0.002)
})

it('lights the subsolar point and darkens its antipode, without a date-line discontinuity', () => {
  const { longitude, latitude, direction } = solarPosition(new Date('2026-06-21T12:00:00Z'))
  const dot = (normal: number[]) => normal.reduce((sum, v, i) => sum + v * direction[i]!, 0)
  expect(dot(surfaceNormal(longitude, latitude))).toBeCloseTo(1)
  expect(dot(surfaceNormal(longitude + 180, -latitude))).toBeCloseTo(-1)
  expect(dot(surfaceNormal(180, 0))).toBeCloseTo(dot(surfaceNormal(-180, 0)))
  expect(surfaceNormal(80, 90)[2]).toBe(1)
})

it('fades illumination only between zoom 3 and 6', () => {
  expect([-2, 3, 4.5, 6, 19].map(globeLightingStrength)).toEqual([1, 1, 0.5, 0, 0])
})
