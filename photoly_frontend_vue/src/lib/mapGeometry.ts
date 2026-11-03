/** Geographic coordinates are always [longitude, latitude], including on the Leaflet path. */
export type Coordinate = [number, number]
/** EXIF coordinates arrive as strings; empty values must not become zero. */
export function exifCoordinate(latitude: unknown, longitude: unknown): Coordinate | null {
  const parse = (value: unknown) =>
    typeof value === 'number' || (typeof value === 'string' && value.trim() !== '')
      ? Number(value)
      : NaN
  const lat = parse(latitude)
  const lon = parse(longitude)
  return Number.isFinite(lat) && Number.isFinite(lon) && Math.abs(lat) <= 90 && Math.abs(lon) <= 180
    ? [lon, lat]
    : null
}
export interface MapPhotoLocation {
  photo_id: number
  extra_photo_gps_lat?: number
  extra_photo_gps_lon?: number
  extra_photo_taken_date?: string
}
export interface Pixel {
  x: number
  y: number
}
export interface PhotoGroup<T> {
  key: string
  photos: T[]
  lat: number
  lng: number
}
export function hasLocation(p: MapPhotoLocation): boolean {
  return (
    typeof p.extra_photo_gps_lat === 'number' &&
    typeof p.extra_photo_gps_lon === 'number' &&
    Number.isFinite(p.extra_photo_gps_lat) &&
    Number.isFinite(p.extra_photo_gps_lon) &&
    Math.abs(p.extra_photo_gps_lat) <= 90 &&
    Math.abs(p.extra_photo_gps_lon) <= 180
  )
}
export const coordinate = (p: MapPhotoLocation): Coordinate => [
  p.extra_photo_gps_lon!,
  p.extra_photo_gps_lat!,
]
export const wrapLongitude = (n: number) => ((((n + 180) % 360) + 360) % 360) - 180
/** Compare directions, so pole longitudes and wrapped copies are equivalent. */
export function sameLocation(a: Coordinate, b: Coordinate) {
  const radians = Math.PI / 180
  const vector = ([lng, lat]: Coordinate) => [
    Math.cos(lat * radians) * Math.cos(lng * radians),
    Math.cos(lat * radians) * Math.sin(lng * radians),
    Math.sin(lat * radians),
  ]
  const x = vector(a),
    y = vector(b)
  return Math.hypot(...x.map((v, i) => v - y[i]!)) < 1e-5
}
/** Smallest longitude span; never fit Pacific photos through Greenwich. */
export function photoBounds(points: Coordinate[]): [Coordinate, Coordinate] | null {
  if (!points.length) return null
  const longs = points.map(([lng]) => wrapLongitude(lng)).sort((a, b) => a - b)
  let gap = -1,
    start = 0
  for (let i = 0; i < longs.length; i++) {
    const next = i + 1 === longs.length ? longs[0]! + 360 : longs[i + 1]!
    if (next - longs[i]! > gap) {
      gap = next - longs[i]!
      start = (i + 1) % longs.length
    }
  }
  const west = longs[start]!,
    east = west + 360 - gap
  return [
    [west, Math.min(85.051129, Math.max(-85.051129, Math.min(...points.map((p) => p[1]))))],
    [east, Math.max(-85.051129, Math.min(85.051129, Math.max(...points.map((p) => p[1]))))],
  ]
}
/** Densest ten-degree region; deterministic ties follow input order. */
export function photoCenter(points: Coordinate[]): Coordinate {
  const cells = new Map<string, { first: Coordinate; count: number }>()
  let best: { first: Coordinate; count: number } | undefined
  for (const p of points) {
    const key = `${Math.floor((wrapLongitude(p[0]) + 180) / 10)},${Math.floor((p[1] + 90) / 10)}`
    const cell = cells.get(key) ?? { first: p, count: 0 }
    cell.count++
    cells.set(key, cell)
    if (!best || cell.count > best.count) best = cell
  }
  return best?.first ?? [10, 19]
}
/** Screen-space grid keeps work proportional to photos plus nearby groups. */
export function groupPhotos<T extends MapPhotoLocation>(
  photos: T[],
  project: (p: Coordinate) => Pixel | null,
  radius = 56,
): PhotoGroup<T>[] {
  const buckets = new Map<string, { point: Pixel; group: PhotoGroup<T> }[]>()
  const groups: PhotoGroup<T>[] = []
  const ordered = [...photos].sort(
    (a, b) =>
      (b.extra_photo_taken_date ?? '').localeCompare(a.extra_photo_taken_date ?? '') ||
      a.photo_id - b.photo_id,
  )
  for (const photo of ordered) {
    const point = project(coordinate(photo))
    if (!point) continue
    const x = Math.floor(point.x / radius),
      y = Math.floor(point.y / radius)
    let near: PhotoGroup<T> | undefined
    for (let dx = -1; dx <= 1 && !near; dx++)
      for (let dy = -1; dy <= 1 && !near; dy++) {
        near = buckets
          .get(`${x + dx},${y + dy}`)
          ?.find((g) => Math.hypot(g.point.x - point.x, g.point.y - point.y) < radius)?.group
      }
    if (near) near.photos.push(photo)
    else {
      const group = {
        key: String(photo.photo_id),
        photos: [photo],
        lat: photo.extra_photo_gps_lat!,
        lng: photo.extra_photo_gps_lon!,
      }
      groups.push(group)
      const key = `${x},${y}`,
        bucket = buckets.get(key) ?? []
      bucket.push({ point, group })
      buckets.set(key, bucket)
    }
  }
  for (const group of groups) group.key = group.photos.map((p) => p.photo_id).join('-')
  return groups
}
