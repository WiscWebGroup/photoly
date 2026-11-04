import type { Photo } from '@/composables/usePhotos'
import { ApiError } from './http'
export interface DuplicateGroup {
  key: string
  version: string
  count: number
  keptCount: number
  ignored: boolean
}
export interface DuplicateMember extends Photo {
  fileName: string
  bytes: number
  kept: boolean
  folder: string
  sourceName: string
  sourceType: string
}
export interface DuplicatePage {
  items: DuplicateGroup[]
  total: number
}
export interface DuplicateMembers {
  items: DuplicateMember[]
  total: number
  version: string
}
export function duplicateError(e: unknown) {
  if (e instanceof ApiError && e.msgCode === 409)
    return 'These duplicates changed. Refresh the results and review your selection again.'
  if (e instanceof ApiError && e.detail === 'duplicate_keep_required')
    return 'Keep at least one copy. Photos marked Keep cannot be deleted here.'
  if (e instanceof ApiError && e.detail === 'duplicate_file_changed')
    return 'A file is missing, unavailable or changed. Refresh its storage folder before trying again.'
  return 'Could not complete this operation. Please retry.'
}
export function duplicateBytes(bytes: number) {
  if (bytes < 1000) return `${bytes} B`
  if (bytes < 1000000) return `${(bytes / 1000).toFixed(1)} KB`
  return `${(bytes / 1000000).toFixed(1)} MB`
}
