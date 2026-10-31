import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, field, mountWithApp, pageText, type } from '@/test/utils'
import ExternalStorageSettings from '../ExternalStorageSettings.vue'

beforeEach(() => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/user/storage/smb`, () => ok([])),
    http.get(`${API}/user/storage/webdav`, () =>
      ok([
        {
          sourceId: 8,
          name: 'Cloud photos',
          url: 'https://cloud.example/dav/',
          enabled: true,
          scanInterval: 60,
        },
      ]),
    ),
    http.get(`${API}/user/storage/sources`, () =>
      ok([{ sourceId: 8, scanStatus: 'ready', scanCompleted: 5, scanTotal: 5, scanRetries: 0 }]),
    ),
  )
})
describe('external storage settings', () => {
  it('edits credentials without exposing or requiring the stored password', async () => {
    const requests = recordRequests()
    server.use(http.patch(`${API}/user/storage/webdav/8`, () => ok({ sourceId: 8 })))
    mountWithApp(ExternalStorageSettings)
    await vi.waitFor(() => expect(pageText()).toContain('Cloud photos'))
    button('Edit').click()
    await vi.waitFor(() => expect(field('Username').type).toBe('password'))
    expect(field('Username').value).toBe('')
    expect(field('Password or app password').value).toBe('')
    expect(field('Full directory URL').disabled).toBe(true)
    await type(field('Entry name'), 'Renamed cloud')
    button('Save connection').click()
    await vi.waitFor(() =>
      expect(requests.some((r) => r.method === 'PATCH' && r.path.includes('/webdav/8'))).toBe(true),
    )
  })
  it('requires explicit detach confirmation explaining file retention', async () => {
    const requests = recordRequests()
    server.use(http.delete(`${API}/user/storage/webdav/8`, () => ok(1)))
    mountWithApp(ExternalStorageSettings)
    await vi.waitFor(() => expect(pageText()).toContain('Cloud photos'))
    button('Detach').click()
    await vi.waitFor(() => expect(pageText()).toContain('Actual files will be kept'))
    expect(requests.some((r) => r.method === 'DELETE')).toBe(false)
    button('Detach and keep files').click()
    await vi.waitFor(() => expect(requests.some((r) => r.method === 'DELETE')).toBe(true))
  })
})

it('parses a UNC path and posts SMB settings without exposing credentials', async () => {
  const requests = recordRequests()
  server.use(http.post(`${API}/user/storage/smb`, () => ok({ sourceId: 9 })))
  mountWithApp(ExternalStorageSettings)
  await vi.waitFor(() => expect(pageText()).toContain('Cloud photos'))
  button('Add SMB connection').click()
  await vi.waitFor(() => expect(field('Server or UNC path')).toBeTruthy())
  await type(field('Server or UNC path'), String.raw`\\NASFSTBFD\TestFolder\中文`)
  field('Server or UNC path').dispatchEvent(new Event('change', { bubbles: true }))
  await vi.waitFor(() => expect(field('Share').value).toBe('TestFolder'))
  await type(field('Username'), 'nas-user')
  await type(field('Password or app password'), 'app-secret')
  button('Save connection').click()
  await vi.waitFor(() =>
    expect(requests.some((r) => r.method === 'POST' && r.path.endsWith('/smb'))).toBe(true),
  )
  const request = requests.find((r) => r.method === 'POST' && r.path.endsWith('/smb'))!
  expect(request.body).toMatchObject({
    server: 'NASFSTBFD',
    share: 'TestFolder',
    subdirectory: '中文',
  })
})
