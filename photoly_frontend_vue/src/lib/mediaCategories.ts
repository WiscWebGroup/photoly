export const mediaCategories = [
  { id: 'video', name: 'Videos', description: 'Video files' },
  { id: 'panorama', name: 'Panoramas', description: 'Panoramic and wide-format photos' },
  { id: 'live', name: 'Live Photos', description: 'Photos with a paired motion clip' },
  { id: 'animated', name: 'Animated Images', description: 'Multi-frame GIF, APNG and WebP images' },
  { id: 'screenshot', name: 'Screenshots', description: 'Screen captures' },
  { id: 'portrait', name: 'Portrait Mode', description: 'Photos taken in camera portrait mode' },
] as const
export type MediaCategory = (typeof mediaCategories)[number]['id']
export interface CategoryOverride {
  category: MediaCategory
  automatic: boolean
  override: boolean | null
}
