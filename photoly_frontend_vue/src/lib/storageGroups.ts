export interface StorageGroup {
  id: number
  name: string
  maxStorageBytes: number | null
  isDefault: boolean
  memberCount: number
}
