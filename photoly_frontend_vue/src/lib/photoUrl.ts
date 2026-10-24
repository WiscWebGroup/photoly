import { useAuthStore } from '@/stores/auth'

/** Image endpoints take the session token in the path, since `<img>` can't send headers. */
export function photoUrl(
  photoId: number,
  kind: 'render' | 'renderThumbnail' | 'renderV' | 'renderLive' = 'render',
) {
  // Bypass cached legacy thumbnails that ignored EXIF orientation.
  const revision = kind === 'renderThumbnail' ? '&thumbnailVersion=2' : ''
  return `${import.meta.env.VITE_APP_BASE_URL}/photo/${kind}/${useAuthStore().token}?photoId=${photoId}${revision}`
}
