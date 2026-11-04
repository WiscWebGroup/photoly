export type Direction = [number, number, number]
const radians = Math.PI / 180

/** Earth-fixed unit vector: X at Greenwich, Y at 90° east, Z at the north pole. */
export function surfaceNormal(longitude: number, latitude: number): Direction {
  const lat = latitude * radians
  const lon = longitude * radians
  return [Math.cos(lat) * Math.cos(lon), Math.cos(lat) * Math.sin(lon), Math.sin(lat)]
}

/** NOAA fractional-year approximation, evaluated entirely in UTC.
 * https://gml.noaa.gov/grad/solcalc/solareqns.PDF
 * Intended for visual illumination, not navigation or sunrise predictions.
 */
export function solarPosition(date: Date): {
  longitude: number
  latitude: number
  direction: Direction
} {
  const year = date.getUTCFullYear()
  const start = Date.UTC(year, 0, 1)
  const days = (Date.UTC(year + 1, 0, 1) - start) / 86400000
  const elapsedDays = (date.getTime() - start) / 86400000
  const gamma = ((2 * Math.PI) / days) * (elapsedDays - 0.5)
  const equation =
    229.18 *
    (0.000075 +
      0.001868 * Math.cos(gamma) -
      0.032077 * Math.sin(gamma) -
      0.014615 * Math.cos(2 * gamma) -
      0.040849 * Math.sin(2 * gamma))
  const declination =
    0.006918 -
    0.399912 * Math.cos(gamma) +
    0.070257 * Math.sin(gamma) -
    0.006758 * Math.cos(2 * gamma) +
    0.000907 * Math.sin(2 * gamma) -
    0.002697 * Math.cos(3 * gamma) +
    0.00148 * Math.sin(3 * gamma)
  const minutes =
    date.getUTCHours() * 60 +
    date.getUTCMinutes() +
    date.getUTCSeconds() / 60 +
    date.getUTCMilliseconds() / 60000
  const longitude = ((180 - (minutes + equation) / 4 + 540) % 360) - 180
  const latitude = declination / radians
  return { longitude, latitude, direction: surfaceNormal(longitude, latitude) }
}

export const globeLightingStrength = (zoom: number) => Math.max(0, Math.min(1, (6 - zoom) / 3))
