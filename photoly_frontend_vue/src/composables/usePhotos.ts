import { ref } from 'vue'
import { call } from '@/lib/http'

export interface Photo {
  photoId: number
  photoName: string
  format?: string
  nsId?: number
  sourceId?: number
  uploadDate?: string
  timelineDate?: string | null
  dateSource?: 'taken' | 'upload' | null
  /** 1 when shared. */
  visibility?: number
  token?: string
  photoUuid?: string
  /** Selected in `MediaList`'s select mode; parents read it from the list they receive back. */
  checked?: boolean
}

/** A list of photos from one source (a folder or a gallery), plus the actions shared by every view. */
export function usePhotos() {
  const photos = ref<Photo[]>([])
  const loading = ref(false)

  async function load(request: Parameters<typeof call>[0]) {
    loading.value = true
    try {
      photos.value = await call<Photo[]>(request)
    } finally {
      loading.value = false
    }
  }

  const loadByFolder = (nsId: number) => load({ url: '/photo/getByNamespace', params: { nsId } })
  const loadByGallery = (gaId: number) => load({ url: '/photo/getByGallery', params: { gaId } })

  async function moveTo(photoId: number, nsId: number) {
    await call({ method: 'post', url: '/photo/changeNamespace', params: { photoId, nsId } })
    photos.value = photos.value.filter((p) => p.photoId !== photoId)
  }

  async function remove(photoId: number) {
    await call({ method: 'post', url: '/photo/delete', params: { photoId } })
    photos.value = photos.value.filter((p) => p.photoId !== photoId)
  }

  return { photos, loading, loadByFolder, loadByGallery, moveTo, remove }
}
