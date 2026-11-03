import { describe, it, expect } from 'vitest'
import {
  coordinate,
  groupPhotos,
  hasLocation,
  photoBounds,
  photoCenter,
  sameLocation,
} from '../mapGeometry'
const photo = (id: number, lng: number, lat: number, date = '2024-01-01') => ({
  photo_id: id,
  extra_photo_gps_lat: lat,
  extra_photo_gps_lon: lng,
  extra_photo_taken_date: date,
})
describe('photo map geometry', () => {
  it('accepts zero coordinates and excludes missing, non-finite and invalid GPS', () => {
    expect(hasLocation(photo(1, 0, 0))).toBe(true)
    for (const p of [
      photo(1, 181, 0),
      photo(1, 0, -91),
      photo(1, NaN, 0),
      photo(1, 0, Infinity),
      { photo_id: 1 },
    ])
      expect(hasLocation(p)).toBe(false)
    expect(coordinate(photo(1, 20, 30))).toEqual([20, 30])
  })
  it('fits across the date line instead of through Greenwich, and handles poles', () => {
    expect(
      photoBounds([
        [179, 10],
        [-179, 20],
      ]),
    ).toEqual([
      [179, 10],
      [181, 20],
    ])
    expect(photoBounds([[0, -90]])).toEqual([
      [0, -85.051129],
      [0, -85.051129],
    ])
    expect(photoBounds([])).toBeNull()
  })
  it('compares globe directions across longitude wrap and at poles', () => {
    expect(sameLocation([180, 20], [-180, 20])).toBe(true)
    expect(sameLocation([0, 90], [120, 90])).toBe(true)
    expect(sameLocation([0, 0], [180, 0])).toBe(false)
  })
  it('groups only visible points, choosing the newest real location as anchor', () => {
    const items = [photo(1, 179, 10), photo(2, -179, 10, '2025-01-01'), photo(3, 0, 0)]
    const groups = groupPhotos(items, ([lng]) =>
      lng === 0 ? null : { x: lng < 0 ? 102 : 100, y: 50 },
    )
    expect(groups).toEqual([{ key: '2-1', photos: [items[1], items[0]], lat: 10, lng: -179 }])
    expect(
      groupPhotos([...items].reverse(), ([lng]) =>
        lng === 0 ? null : { x: lng < 0 ? 102 : 100, y: 50 },
      ),
    ).toEqual(groups)
  })
  it('checks adjacent grid cells and bounds DOM groups for dense libraries', () => {
    const photos = Array.from({ length: 10000 }, (_, i) => photo(i, 0, 0))
    expect(groupPhotos(photos, () => ({ x: 55, y: 55 }))).toHaveLength(1)
    expect(groupPhotos([photo(1, 55, 0), photo(2, 57, 0)], ([x]) => ({ x, y: 0 }))).toHaveLength(1)
  })
  it('centers the globe on the most populated region', () => {
    expect(
      photoCenter([
        [100, 20],
        [0, 0],
        [1, 1],
      ]),
    ).toEqual([0, 0])
    expect(photoCenter([])).toEqual([10, 19])
  })
})
