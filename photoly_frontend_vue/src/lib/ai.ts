import { isAxiosError, type AxiosRequestConfig } from 'axios'
import { http, type Envelope } from './http'
import type { Photo } from '@/composables/usePhotos'

export interface Coverage {
  eligible: number
  supported: number
  ready: number
  pending: number
  failed: number
  unsupported: number
  ratio: number
}
export interface AiCapabilities {
  similarityThreshold: number | null
  enabled: boolean
  globalEnabled: boolean
  ready: boolean
  reason: string | null
  profileId: number | null
  profile: { provider: string; modelId: string; dimension: number } | null
  coverage: Coverage
  modalities: string[]
  formats: string[]
  videoFormats?: string[]
  videoSamplingVersion?: number
  limits: {
    textCharacters: number
    imageBytes: number
    imagePixels: number
    topK: number
    videoBytes?: number
    videoFrames?: number
    videoExtractionSeconds?: number
  }
}
export interface AiHit {
  photo: Photo
  score: number
  matchedTimestampMs?: number
}
export interface AiSearchResponse {
  similarityThreshold: number | null
  items: AiHit[]
  profileId: number
  coverage: Coverage
  requestId: string
}
export interface AiFilters {
  topK: number
  namespaceId?: number
  galleryId?: number
}
export interface AiSettings {
  similarity_threshold: number | null
  generation: number
  enabled: boolean
  new_user_default: boolean
  smart_tag_enabled: boolean
  maintenance: boolean
  search_profile_id: number | null
  building_profile_id: number | null
  loaded_profile_id: number | null
  quota_bytes: number
  scheduler: string
  runtime: string
  device: string
  precision: string
  cpu_threads: number
  background_interval_seconds: number
  background_max_per_minute: number
  cpu_pause_percent: number
  cpu_resume_percent: number
  gpu_pause_percent: number
  gpu_resume_percent: number
  memory_pause_free_mib: number
  memory_resume_free_mib: number
  vram_pause_free_mib: number
  vram_resume_free_mib: number
  disk_pause_free_mib: number
  disk_resume_free_mib: number
}
export type AiChanges = Partial<
  Omit<
    AiSettings,
    | 'generation'
    | 'maintenance'
    | 'scheduler'
    | 'runtime'
    | 'loaded_profile_id'
    | 'search_profile_id'
    | 'building_profile_id'
  >
>
export interface FieldRule {
  default: number | null
  minimum: number
  maximum: number
  unit: string
  apply: string
  pairedWith: string | null
  relation: 'less' | 'greater' | null
}
export interface Device {
  device: string
  precision: string
  available: boolean | null
  reason: string | null
}
export interface AdminCapabilities {
  fields: Record<string, FieldRule>
  devices: Device[]
  verified: { fingerprint: string; device: string; precision: string; cpu_threads: number }[]
  maxConcurrency: number
  batchSize: number
}
export interface ProfileSpec {
  provider: string
  model_id: string
  revision: string
  dimension: number
  fingerprint: string
  query_instruction: string
  document_instruction: string
}
export interface AiProfile {
  profileId: number
  provider: string
  dimension: number
  fingerprint: string
  spec: ProfileSpec
  indexStatus: string
  createdAt: string
  coverage: Coverage
  activationBlockers: string[]
  purgeBlockers: string[]
}
export interface ProfileList {
  registry: ProfileSpec[]
  profiles: AiProfile[]
}
export interface AiJob {
  operationId: number
  command: string | null
  generation: number | null
  stage: string
  kind: string
  photoId: number | null
  profileId: number | null
  status: string
  attempts: number
  result: { reason?: string; step?: string }
  createdAt: string
  finishedAt: string | null
}
export interface JobList {
  items: AiJob[]
  nextAfter: number | null
  nextBefore: number | null
}
export interface Resources {
  sampled_at: string
  cpu_percent: number | null
  host_cpu_percent?: number | null
  container_cpu_percent?: number | null
  memory_available: number
  memory_total: number
  disk_free: number
  gpu_percent: number | null
  vram_available: number | null
  vram_total: number | null
  missing: string[]
  free_score: number
}
export interface AiService {
  online: boolean
  heartbeatAt: string | null
  observed: {
    state: string
    reason?: string | string[] | null
    config?: { device: string; precision: string; cpu_threads: number }
    fingerprint?: string | null
    resources?: Resources
    queued?: number
  }
}
export interface AiStatus {
  smartTag?: import('./smartOrganization').SmartStatus
  settings: AiSettings
  services: Record<string, AiService>
  quota: { estimated_bytes: number; reserved_bytes: number; physical_bytes: number }
  jobs: Record<string, number>
  coverage: Coverage
  buildingCoverage: Coverage | null
  backgroundLimits: {
    lastStartedAt: string | null
    startsLastMinute: number
    nextAllowedAt: string | null
    reason: string | null
    maxConcurrency: number
    batchSize: number
  }
}

export class AiError extends Error {
  constructor(
    public code: string,
    public status = 0,
  ) {
    super(code)
  }
}
export async function aiCall<T>(config: AxiosRequestConfig): Promise<T> {
  try {
    const { data } = await http.request<Envelope<T> & { error?: string }>({
      timeout: 65000,
      ...config,
    })
    if (![200, 202].includes(data.msgCode))
      throw new AiError(data.error ?? 'request_failed', data.msgCode)
    return data.t
  } catch (e) {
    if (isAxiosError(e) && e.code !== 'ERR_CANCELED')
      throw new AiError(
        e.response?.data?.error ?? (e.code === 'ECONNABORTED' ? 'query_timeout' : 'network_error'),
        e.response?.status ?? 0,
      )
    throw e
  }
}
const messages: Record<string, string> = {
  invalid_similarity_threshold: 'Enter a similarity threshold between -1 and 1, or leave it blank.',
  search_settings_changed: 'The similarity threshold changed. Reload recommendations.',
  cursor_expired: 'These recommendations expired. Reload to continue.',
  invalid_cursor: 'These recommendations are no longer available. Reload to continue.',
  disabled: 'AI search is disabled by the administrator.',
  not_enabled_for_user: 'AI search is not enabled for your account.',
  ai_not_enabled: 'AI search is not enabled for this account.',
  maintenance: 'AI search is temporarily unavailable during model maintenance.',
  no_active_profile: 'No search index has been activated yet.',
  profile_not_ready: 'The search index is not ready.',
  model_not_ready: 'The model is not ready yet.',
  runtime_offline: 'The AI runtime is offline.',
  runtime_unconfigured: 'The AI runtime is not configured.',
  model_restarting: 'The model is restarting.',
  user_query_busy: 'Your previous query is still running. Please wait before trying again.',
  runtime_busy: 'The AI service is busy. Please try again shortly.',
  query_timeout: 'The query timed out. Please try again.',
  inference_timeout: 'Inference timed out. Please try again.',
  device_unavailable: 'The selected inference device is unavailable.',
  unsupported_operator: 'This model uses an operation unsupported by the selected device.',
  out_of_memory: 'The model has insufficient memory. Contact an administrator.',
  invalid_image: 'This image cannot be decoded.',
  unsupported_image: 'This image format is not supported.',
  empty_image: 'The image is empty.',
  image_too_large: 'The image exceeds the file size limit.',
  image_too_many_pixels: 'The image exceeds the pixel limit.',
  text_token_limit: 'The text is too long for this model. Please shorten it.',
  generation_conflict:
    'Settings changed in another session. Review the latest values before saving again.',
  unload_before_thread_change: 'Unload the model before changing CPU threads.',
  unload_before_device_change: 'Unload the model before changing device or precision.',
  profile_changed: 'The active model changed. Please search again.',
  stale_content: 'The source image changed. Please search again.',
  photo_not_found: 'The source photo is no longer available.',
  filter_not_found: 'The selected folder or album is no longer available.',
  admin_required: 'Administrator access is required.',
  unauthenticated: 'Please sign in again.',
  invalid_video: 'This video is damaged or cannot be decoded.',
  video_stream_missing: 'This file has no video stream.',
  video_frame_missing: 'No usable video frames could be decoded.',
  video_decode_timeout: 'Video decoding timed out. Please retry.',
  video_decoder_unavailable: 'Video decoding is unavailable on the server.',
  video_too_large: 'This video exceeds the upload size limit.',
  video_frame_too_large: 'This video exceeds the supported frame dimensions.',
  network_error: 'The service could not be reached. Please retry.',
  activation_not_ready: 'The model, index or coverage is not ready for activation.',
  maintenance_required: 'Enter maintenance before switching or rebuilding profiles.',
  profile_in_use: 'This profile is still in use.',
}
export function aiMessage(error: unknown): string {
  const code =
    error instanceof AiError ? error.code : typeof error === 'string' ? error : 'network_error'
  return messages[code] ?? code.replaceAll('_', ' ')
}
export const activeJob = (job: AiJob) => ['queued', 'running'].includes(job.status)
export const bytesLabel = (bytes: number | null | undefined) =>
  bytes == null ? 'Unknown' : `${(bytes / 1024 ** 3).toFixed(2)} GiB`
