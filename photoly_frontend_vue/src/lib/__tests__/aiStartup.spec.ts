import { expect, it } from 'vitest'
import { aiStartupStage, type AiStartupStatus } from '../aiStartup'

function status(enabled: boolean, stage: string, runtime = 'loaded'): AiStartupStatus {
  return {
    settings: { enabled, runtime },
    startup: { stage },
    services: { runtime: { observed: { state: stage } } },
  }
}
it('never labels an enabled, unloaded runtime as paused', () => {
  expect(aiStartupStage(status(true, 'unloaded'))).toBe('preparing')
  expect(aiStartupStage(status(true, 'paused'))).toBe('preparing')
  expect(aiStartupStage(status(true, 'unloaded', 'unloaded'))).toBe('idle')
})
it('keeps loading, failure, ready and pause transitions distinct', () => {
  for (const stage of ['loading', 'failed', 'ready', 'preparing_index'])
    expect(aiStartupStage(status(true, stage))).toBe(stage)
  expect(aiStartupStage(status(false, 'pausing'))).toBe('pausing')
  expect(aiStartupStage(status(false, 'unloaded'))).toBe('paused')
  expect(aiStartupStage(status(false, 'ready'))).toBe('paused')
})
