import { useAuthStore } from '@/stores/auth'

/** Image endpoints take the session token in the path, since `<img>` can't send headers. */
export function photoUrl(
  photoId: number,
  kind: 'render' | 'renderThumbnail' | 'renderV' = 'render',
) {
  return `${import.meta.env.VITE_APP_BASE_URL}/photo/${kind}/${useAuthStore().token}?photoId=${photoId}`
}
