import { reactive } from 'vue'
import { call } from '@/lib/http'

export interface Cover {
  /** Photos and videos directly inside. */
  count: number
  /** Up to three still images for the card mosaic, newest first. */
  ids: number[]
}

/**
 * Card covers for folders, galleries or tags: the item count and a few images, in one request.
 * A failure leaves cards without covers rather than breaking the page.
 */
export function useCovers(source: 'folder' | 'gallery' | 'tag') {
  const covers = reactive(new Map<number, Cover>())
  let sequence = 0

  async function refresh(ids: number[]) {
    const id = ++sequence
    if (!ids.length) return covers.clear()
    try {
      const list = await call<{ id: number; count: number; coverIds: number[] }[]>({
        url: '/photo/getCovers',
        params: { source, ids: ids.join(',') },
      })
      if (id !== sequence) return
      covers.clear()
      for (const c of list ?? []) covers.set(c.id, { count: c.count, ids: c.coverIds })
    } catch {
      if (id === sequence) covers.clear()
    }
  }

  return { covers, refresh }
}
