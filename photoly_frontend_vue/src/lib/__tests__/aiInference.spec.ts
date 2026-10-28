import { describe, expect, it } from 'vitest'
import { AiError, type AiStatus } from '../ai'
import { fitState, isWorkerDied, lowMemory, memoryMeter } from '../aiInference'

const GIB = 1024 ** 3
const status = (over: Partial<AiStatus> = {}) =>
  ({
    services: {
      runtime: {
        online: true,
        heartbeatAt: null,
        observed: { state: 'ready', resources: { memory_total: 8 * GIB, memory_available: 1.5 * GIB } },
      },
    },
    ...over,
  }) as unknown as AiStatus

describe('server memory', () => {
  it('splits memory into Photoly, the AI worker and other apps', () => {
    const m = memoryMeter(status({ worker: { running: true, memoryMB: 819.2, appMemoryMB: 102.4 } }), false)
    expect(m).toMatchObject({ known: true, totalGB: 8, freeGB: 1.5, workerGB: 0.8, appGB: 0.1 })
    expect(m.otherGB).toBeCloseTo(5.6)
  })
  it('is unknown when the sample is stale or missing', () => {
    expect(memoryMeter(status(), true).known).toBe(false)
    expect(memoryMeter(null, false).known).toBe(false)
  })
  it('says which text model fits', () => {
    expect(fitState(1.5, 0.9, false)).toBe('fits')
    expect(fitState(1.5, 2.2, false)).toBe('no')
    expect(fitState(0.6, 0.9, true)).toBe('loaded')
    expect(fitState(1.05, 0.9, false)).toBe('fits')
    expect(fitState(1.04, 0.9, false)).toBe('no')
  })
})

describe('worker errors', () => {
  it('reads the low-memory 409 body', () => {
    const e = new AiError('low-memory', 409, { code: 'low-memory', needMB: 900, availableMB: 600 })
    expect(lowMemory(e)).toEqual({ needMB: 900, availableMB: 600 })
    expect(lowMemory(new AiError('network_error'))).toBeNull()
  })
  it('recognises a dead worker', () => {
    expect(isWorkerDied(new AiError('worker-died', 503))).toBe(true)
    expect(isWorkerDied(new Error('x'))).toBe(false)
  })
})
