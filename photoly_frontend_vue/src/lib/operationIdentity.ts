import { sha256 } from '@noble/hashes/sha2.js'

function hex(bytes: Uint8Array) {
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

// LAN HTTP has getRandomValues, but not randomUUID or SubtleCrypto.
// Keep the same SHA-256 namespace so pending HTTPS/localhost operations still recover.
export function operationStorageKey(token: string) {
  return 'photoly-ai-operation-' + hex(sha256(new TextEncoder().encode(token)))
}

export function newOperationKey() {
  return hex(crypto.getRandomValues(new Uint8Array(16)))
}
