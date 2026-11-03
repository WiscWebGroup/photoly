import type { MapProjection } from '@/composables/useMapProjection'
import type { Coordinate, Pixel } from './mapGeometry'
export interface MapPin {
  key: string
  coordinate: Coordinate
  title: string
  element: () => HTMLElement
  onClick: () => void
}
export interface Camera {
  center: Coordinate
  zoom: number
}
export interface MapEngine {
  kind: 'maplibre' | 'leaflet'
  camera(): Camera
  initialize(points: Coordinate[]): void
  projection(value: MapProjection): void
  projectVisible(point: Coordinate): Pixel | null
  flyTo(point: Coordinate, zoom: number, done?: () => void): void
  resize(): void
  syncPins(pins: MapPin[]): void
  selectPin(key?: string): void
  popup(point: Coordinate, content: HTMLElement, onClose?: () => void): void
  closePopup(): void
  heat(points: Coordinate[], visible: boolean): void
  onChange(callback: () => void): () => void
  destroy(): void
}
export const tileURL = 'https://tile.openstreetmap.org/{z}/{x}/{y}.png'
export const attribution =
  '© <a href="https://www.openstreetmap.org/copyright" target="_blank" rel="noopener">OpenStreetMap contributors</a> · <a href="/map-licenses.txt" target="_blank" rel="noopener">Licenses</a>'
export const heatStops: [number, string][] = [
  [0.15, '#5eead4'],
  [0.4, '#16827d'],
  [0.7, '#f59e0b'],
  [1, '#dc2626'],
]
export const heatGradient = `linear-gradient(to right, ${heatStops.map(([at, c]) => `${c} ${at * 100}%`).join(', ')})`
export const reduceMotion = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches
