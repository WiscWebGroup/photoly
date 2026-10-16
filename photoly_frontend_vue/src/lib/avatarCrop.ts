import { toBlob } from './imageEdit'

export const AVATAR_SIZE = 512
export interface AvatarCrop {
  x: number
  y: number
  size: number
}

/** Coordinates in decoded, orientation-corrected source pixels. */
export function avatarCrop(
  width: number,
  height: number,
  zoom: number,
  cx: number,
  cy: number,
): AvatarCrop {
  const size = Math.min(width, height) / Math.max(1, Math.min(5, zoom))
  return {
    x: Math.max(0, Math.min(width - size, cx - size / 2)),
    y: Math.max(0, Math.min(height - size, cy - size / 2)),
    size,
  }
}
export async function encodeAvatar(source: HTMLImageElement, crop: AvatarCrop) {
  const canvas = document.createElement('canvas')
  canvas.width = canvas.height = AVATAR_SIZE
  const context = canvas.getContext('2d')
  if (!context) throw new Error('Image editing is unavailable')
  context.fillStyle = '#fff'
  context.fillRect(0, 0, AVATAR_SIZE, AVATAR_SIZE)
  context.imageSmoothingQuality = 'high'
  context.drawImage(source, crop.x, crop.y, crop.size, crop.size, 0, 0, AVATAR_SIZE, AVATAR_SIZE)
  return new File([await toBlob(canvas, 'image/jpeg', 0.92)], 'avatar.jpg', { type: 'image/jpeg' })
}
