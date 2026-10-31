import { expect, it } from 'vitest'
import { mountWithApp } from '@/test/utils'
import StorageScanProgress from '../StorageScanProgress.vue'
import type { StorageStatus } from '@/composables/useStorageRevision'

const status: StorageStatus = {
  sourceId: 1,
  enabled: true,
  revision: 1,
  scanStatus: 'scanning',
  scanPhase: 'processing',
  scanError: null,
  scannedAt: null,
  scanCompleted: 3,
  scanTotal: 8,
  scanDiscovered: 12,
  scanRetries: 5,
  scanAutomaticRetries: 2,
  scanBlocked: 3,
  scanWaiting: 1,
  scanUnchanged: 6,
}
it('shows four concise metrics and keeps automatic retries separate from warnings', () => {
  mountWithApp(StorageScanProgress, { status })
  expect([...document.querySelectorAll('dt')].map((el) => el.textContent)).toEqual([
    'Discovered',
    'Processed',
    'Retrying',
    'Warning',
  ])
  expect([...document.querySelectorAll('dd')].map((el) => el.textContent)).toEqual([
    '12',
    '3/8',
    '2',
    '3',
  ])
  expect(document.querySelector('[role="progressbar"]')!.getAttribute('aria-valuenow')).toBe('37.5')
  expect(document.body.textContent).not.toMatch(/unchanged|waiting for stability|need attention/)
})
it('does not claim a percentage before enumeration finishes', () => {
  mountWithApp(StorageScanProgress, { status: { ...status, scanPhase: 'enumerating' } })
  expect(document.querySelector('[role="progressbar"]')!.hasAttribute('aria-valuenow')).toBe(false)
  expect(document.querySelector('[data-slot="progress-indicator"]')!.className).toContain(
    'animate-pulse',
  )
})
it('subtracts warnings when older status responses only supply a combined failure count', () => {
  mountWithApp(StorageScanProgress, { status: { ...status, scanAutomaticRetries: undefined } })
  expect(document.querySelectorAll('dd')[2]!.textContent).toBe('2')
})
