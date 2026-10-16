export interface SmartSource {
  id: number
  source: 'camera' | 'lens' | 'trip'
  name: string
  suppressed: boolean
  basis: {
    kind?: string
    method?: string
    startDate?: string
    endDate?: string
    places?: Record<string, number>
    timezoneSources?: string[]
    mediaCount?: number
    gpsCount?: number
  }
}
export interface SmartStatus {
  processed: number
  total: number
  failed: number
  reason: string | null
  pending?: boolean
  phase?: string | null
  updatedAt?: string | null
}
export function smartReason(reason: string | null | undefined): string {
  const messages: Record<string, string> = {
    ai_disabled: 'An administrator must enable AI first.',
    smart_tag_disabled: 'An administrator must enable Smart Tag first.',
    ai_permission_required: 'Your account needs AI access from an administrator.',
    user_disabled: 'Automatic organization is off. Existing smart items remain editable.',
    maintenance: 'Paused for maintenance.',
    scheduler_stopped: 'Waiting for the administrator to start the scheduler.',
    scheduler_paused: 'The scheduler is paused.',
    waiting_for_vectors: 'Metadata organized. Some media without GPS are waiting for AI indexing.',
    smart_processing_failed: 'Organization failed. Use Refresh to retry.',
    memory_pressure: 'Waiting for available memory.',
    disk_pressure: 'Waiting for available disk space.',
    cpu_busy_or_unknown: 'Waiting for available CPU resources.',
  }
  return reason ? (messages[reason] ?? `Waiting: ${reason.replaceAll('_', ' ')}`) : ''
}
