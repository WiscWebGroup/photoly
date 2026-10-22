import { reactive } from 'vue'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import type { Photo } from '@/composables/usePhotos'

export interface Cover {
  /** Photos and videos directly inside. */
  count: number
  /** Up to three still images for the card mosaic. */
  ids: number[]
}

/**
 * Card covers for folders, galleries or tags: the item count and a few images, fetched per card.
 * Failures leave a card without a cover rather than breaking the page.
 */
export function useCovers(source: 'folder' | 'gallery' | 'tag') {
  const covers = reactive(new Map<number, Cover>())
  const request = (id: number) =>
    source === 'folder'
      ? { url: '/photo/getByNamespace', params: { nsId: id } }
      : source === 'gallery'
        ? { url: '/photo/getByGallery', params: { gaId: id } }
        : { url: '/photo/getByTags', params: { tagIds: String(id) } }

  async function refresh(ids: number[]) {
    await Promise.all(
      ids.map(async (id) => {
        try {
          const list = (await call<Photo[]>(request(id))) ?? []
          const stills = list.filter((p) => mediaKind(p.format) === 'photo')
          covers.set(id, {
            count: list.length,
            ids: stills
              .slice(-3)
              .reverse()
              .map((p) => p.photoId),
          })
        } catch {
          covers.delete(id)
        }
      }),
    )
  }

  return { covers, refresh }
}
