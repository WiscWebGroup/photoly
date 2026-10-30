import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, field, mountWithApp, pageText, type } from '@/test/utils'
import StorageSourcesSetting from '../StorageSourcesSetting.vue'

const state = {
  configured: true,
  enabled: true,
  hasSecret: true,
  canSave: true,
  values: { endpoint: 's3.example', bucket: 'photos' },
  fields: [
    { name: 'endpoint', label: 'Endpoint', kind: 'text' },
    { name: 'bucket', label: 'Bucket', kind: 'text' },
    { name: 'access_key', label: 'Access key', kind: 'text' },
    { name: 'secret_key', label: 'Secret key', kind: 'text', secret: true },
  ],
}
beforeEach(() => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/user/storage/s3`, () => ok(state)),
    http.get(`${API}/user/storage/sources`, () =>
      ok([
        { sourceId: 1, sourceType: 'local', scanStatus: 'ready', scannedAt: null, scanError: null },
      ]),
    ),
  )
})
describe('personal storage', () => {
  it('loads personal settings with masked credentials and scanner status', async () => {
    mountWithApp(StorageSourcesSetting)
    await vi.waitFor(() => expect(pageText()).toContain('Your S3 storage'))
    expect(field('Endpoint').value).toBe('s3.example')
    expect(field('Secret key').type).toBe('password')
    expect(field('Access key').value).toBe('')
    expect(pageText()).toContain('Scan: ready')
    expect(pageText()).not.toContain('Show media from other storage')
  })
  it('tests and saves only the signed-in user connection', async () => {
    const requests = recordRequests()
    server.use(
      http.post(`${API}/user/storage/s3/test`, () => ok(1)),
      http.put(`${API}/user/storage/s3`, () => ok(1)),
    )
    mountWithApp(StorageSourcesSetting)
    await vi.waitFor(() => expect(pageText()).toContain('Your S3 storage'))
    await type(field('Endpoint'), 'new.example')
    button('Test connection')!.click()
    await vi.waitFor(() =>
      expect(requests.some((r) => r.path === '/user/storage/s3/test')).toBe(true),
    )
    await vi.waitFor(() => expect(button('Save')!.disabled).toBe(false))
    button('Save')!.click()
    await vi.waitFor(() =>
      expect(requests.some((r) => r.path === '/user/storage/s3' && r.method === 'PUT')).toBe(true),
    )
    expect(requests.some((r) => r.path.startsWith('/admin/storage'))).toBe(false)
  })
  it('disables saving when server credential encryption is unavailable', async () => {
    server.use(http.get(`${API}/user/storage/s3`, () => ok({ ...state, canSave: false })))
    mountWithApp(StorageSourcesSetting)
    await vi.waitFor(() => expect(pageText()).toContain('credential encryption'))
    expect(button('Save')!.disabled).toBe(true)
  })
})
