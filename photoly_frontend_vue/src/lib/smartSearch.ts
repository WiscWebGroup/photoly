import type { Photo } from '@/composables/usePhotos'

export const SMART_SEARCH_LIMIT = 100

/** Equal-weight reciprocal rank fusion; raw cosine and text scores are not comparable. */
export function fuseSearchResults(text: Photo[], semantic: Photo[]): Photo[] {
  const hits = new Map<number, { photo: Photo; score: number }>()
  for (const source of [text, semantic]) {
    const seen = new Set<number>()
    let rank = 0
    for (const photo of source) {
      if (seen.has(photo.photoId)) continue
      seen.add(photo.photoId)
      rank++
      const hit = hits.get(photo.photoId) ?? { photo, score: 0 }
      hit.score += 1 / (60 + rank)
      hits.set(photo.photoId, hit)
    }
  }
  return [...hits.values()]
    .sort((a, b) => b.score - a.score || a.photo.photoId - b.photo.photoId)
    .slice(0, SMART_SEARCH_LIMIT)
    .map(({ photo }) => photo)
}
