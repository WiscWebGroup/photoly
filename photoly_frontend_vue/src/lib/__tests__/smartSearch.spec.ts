import { describe, expect, it } from 'vitest'
import { fuseSearchResults } from '../smartSearch'
const photo = (photoId: number) => ({ photoId, photoName: String(photoId) })
const ids = (rows: ReturnType<typeof fuseSearchResults>) => rows.map((p) => p.photoId)

describe('Smart Search rank fusion', () => {
  it('equally weights sources and boosts agreement', () => {
    expect(ids(fuseSearchResults([photo(1), photo(2)], [photo(3), photo(2)]))).toEqual([2, 1, 3])
    expect(ids(fuseSearchResults([photo(3), photo(2)], [photo(1), photo(2)]))).toEqual([2, 1, 3])
  })
  it('deduplicates within a source without inflating scores', () => {
    expect(ids(fuseSearchResults([photo(1), photo(1), photo(2)], [photo(3)]))).toEqual([1, 3, 2])
  })
  it('preserves a single available source and limits the final list to 100', () => {
    const rows = Array.from({ length: 120 }, (_, i) => photo(120 - i))
    expect(ids(fuseSearchResults(rows, []))).toEqual(rows.slice(0, 100).map((p) => p.photoId))
    expect(fuseSearchResults([], [])).toEqual([])
  })
})
