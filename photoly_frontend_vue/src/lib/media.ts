const PHOTO = [
  'jpg',
  'jpeg',
  'gif',
  'png',
  'webp',
  'ico',
  'bmp',
  'tif',
  'svg',
  'psd',
  'raw',
  'arw',
  'pcd',
  'jfif',
]
const VIDEO = ['mp4', 'avi', 'wmv', 'mpeg', 'm4v', 'mov', 'flv', 'asf', 'f4v', 'rmvb', 'vob', 'rm']

/** What the server can render for a file extension; `other` has no preview. */
export function mediaKind(format = ''): 'photo' | 'video' | 'other' {
  const f = format.toLowerCase()
  return PHOTO.includes(f) ? 'photo' : VIDEO.includes(f) ? 'video' : 'other'
}
