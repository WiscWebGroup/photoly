import type { AiStatus } from './ai'

export interface AiStartupStatus {
  settings: Pick<AiStatus['settings'], 'enabled' | 'runtime'>
  startup?: Pick<NonNullable<AiStatus['startup']>, 'stage'>
  services: Record<string, { observed: { state: string } }>
}

export function aiStartupStage(status: AiStartupStatus | null): string | undefined {
  if (!status) return undefined
  const stage = status.startup?.stage ?? status.services.runtime?.observed.state
  if (!status.settings.enabled) return stage === 'pausing' || stage === 'failed' ? stage : 'paused'
  if (!stage || stage === 'unloaded' || stage === 'paused')
    return status.settings.runtime === 'loaded' ? 'preparing' : 'idle'
  return stage
}
