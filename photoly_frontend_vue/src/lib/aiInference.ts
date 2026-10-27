import { AiError, type AiModelKind, type AiStatus, type PlacementSetting } from './ai'

const GIB = 1024 ** 3
/** Server memory each model needs, from the design; a loaded model reports its own. */
export const NEED_GB = { image: 0.9, textSmall: 0.9, textFull: 2.2 }
/** Headroom the server keeps free before it calls a model "fits". */
export const HEADROOM_GB = 0.15

export const placementLabels: Record<PlacementSetting, string> = {
  server: 'Server',
  browser: 'Browser',
  user: 'Let users choose',
}
export const gbLabel = (gb: number) => `${gb.toFixed(1)} GB`

export interface MemoryMeter {
  known: boolean
  totalGB: number
  freeGB: number
  appGB: number
  workerGB: number
  otherGB: number
}
/** Splits server memory into Photoly, the AI worker and everything else. */
export function memoryMeter(status: AiStatus | null, stale: boolean): MemoryMeter {
  const r = status?.services.runtime?.observed.resources
  if (!r?.memory_total || stale)
    return { known: false, totalGB: 0, freeGB: 0, appGB: 0, workerGB: 0, otherGB: 0 }
  const totalGB = r.memory_total / GIB
  const freeGB = r.memory_available / GIB
  const appGB = (status?.worker?.appMemoryMB ?? 0) / 1024
  const workerGB = (status?.worker?.memoryMB ?? 0) / 1024
  return {
    known: true,
    totalGB,
    freeGB,
    appGB,
    workerGB,
    otherGB: Math.max(0, totalGB - freeGB - appGB - workerGB),
  }
}

export type FitState = 'loaded' | 'fits' | 'no'
export function fitState(freeGB: number, needGB: number, loaded: boolean): FitState {
  return loaded ? 'loaded' : freeGB >= needGB + HEADROOM_GB ? 'fits' : 'no'
}

export interface LowMemory {
  needMB: number
  availableMB: number
}
/** The POC's 409 `low-memory` body, wherever the client wrapped it. */
export function lowMemory(error: unknown): LowMemory | null {
  if (!(error instanceof AiError) || error.code !== 'low-memory') return null
  const body = (error.body ?? {}) as Record<string, unknown>
  const t = (body.t ?? body) as Record<string, unknown>
  return { needMB: Number(t.needMB) || 0, availableMB: Number(t.availableMB) || 0 }
}
export const isWorkerDied = (error: unknown) => error instanceof AiError && error.code === 'worker-died'

export const kindName = (kind: AiModelKind) => (kind === 'image' ? 'photo indexing' : 'server search')
