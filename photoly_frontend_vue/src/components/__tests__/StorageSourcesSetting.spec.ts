import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, field, mountWithApp, pageText, type } from '@/test/utils'
import StorageSourcesSetting from '../StorageSourcesSetting.vue'

const connection = {
  sourceId: 7,
  name: 'Trips',
  enabled: true,
  hasSecret: true,
  canRemove: true,
  values: { endpoint: 's3.example', bucket: 'photos' },
}
const state = {
  connections: [connection],
  canSave: true,
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
    http.get(`${API}/user/storage/webdav`, () => ok([])),
    http.get(`${API}/user/storage/smb`, () => ok([])),
    http.get(`${API}/user/storage/s3`, () => ok(state)),
    http.get(`${API}/user/storage/sources`, () =>
      ok([
        {
          sourceId: 1,
          sourceType: 'local',
          enabled: true,
          scanStatus: 'ready',
          scannedAt: null,
          scanError: null,
        },
      ]),
    ),
  )
})
async function ready() {
  mountWithApp(StorageSourcesSetting)
  await vi.waitFor(() => expect(button('Edit')).toBeTruthy())
}
async function edit() {
  await ready()
  button('Edit')!.click()
  await vi.waitFor(() => expect(field('Endpoint').value).toBe('s3.example'))
}
describe('personal storage connections', () => {
  it('starts collapsed and masks both credentials when editing', async () => {
    await edit()
    expect(field('Secret key').type).toBe('password')
    expect(field('Access key').value).toBe('')
    expect(pageText()).toContain('Scan complete')
    button('Cancel')!.click()
    await vi.waitFor(() => expect(document.querySelector('#s3-endpoint')).toBeNull())
  })
  it('tests the owned source and collapses after save', async () => {
    const requests = recordRequests()
    server.use(
      http.post(`${API}/user/storage/s3/test`, () => ok(1)),
      http.patch(`${API}/user/storage/s3/7`, () => ok(1)),
    )
    await edit()
    await type(field('Endpoint'), 'new.example')
    button('Test connection')!.click()
    await vi.waitFor(() =>
      expect(requests.find((r) => r.path.endsWith('/test'))?.body).toMatchObject({
        sourceId: 7,
        values: { endpoint: 'new.example' },
      }),
    )
    await vi.waitFor(() => expect(button('Save')!.disabled).toBe(false))
    button('Save')!.click()
    await vi.waitFor(() => expect(document.querySelector('#s3-endpoint')).toBeNull())
    expect(requests.find((r) => r.method === 'PATCH')).toMatchObject({
      path: '/user/storage/s3/7',
      body: { name: 'Trips', values: { endpoint: 'new.example' } },
    })
  })
  it('sends only alias when configuration did not change', async () => {
    const requests = recordRequests()
    server.use(http.patch(`${API}/user/storage/s3/7`, () => ok(1)))
    await edit()
    await type(field('Alias'), 'Holiday')
    button('Save')!.click()
    await vi.waitFor(() =>
      expect(requests.find((r) => r.method === 'PATCH')?.body).toEqual({ name: 'Holiday' }),
    )
  })
  it('keeps failed edits and permits cancellation without saving', async () => {
    server.use(
      http.patch(`${API}/user/storage/s3/7`, () =>
        json({ msgCode: 400, t: 'Connection unavailable' }),
      ),
    )
    await edit()
    await type(field('Endpoint'), 'offline.example')
    button('Save')!.click()
    await vi.waitFor(() => expect(pageText()).toContain('Connection unavailable'))
    expect(field('Endpoint').value).toBe('offline.example')
  })
  it('adds multiple connections without implicitly overwriting the existing one', async () => {
    const requests = recordRequests()
    server.use(http.post(`${API}/user/storage/s3`, () => ok({ sourceId: 8, name: 'S3' })))
    await ready()
    button('Add S3')!.click()
    await vi.waitFor(() => expect(field('Alias').value).toBe('S3'))
    button('Save')!.click()
    await vi.waitFor(() =>
      expect(requests.find((r) => r.method === 'POST')?.path).toBe('/user/storage/s3'),
    )
  })
  it('disables only the chosen connection and rolls back on failure', async () => {
    const requests = recordRequests()
    let fail = false
    server.use(
      http.patch(`${API}/user/storage/s3/7`, () =>
        fail ? json({ msgCode: 400, t: 'Connection unavailable' }) : ok(1),
      ),
    )
    await ready()
    const toggle = () => document.querySelector<HTMLElement>('[aria-label="Enable Trips"]')!
    toggle().click()
    await vi.waitFor(() =>
      expect(requests.find((r) => r.method === 'PATCH')?.body).toEqual({ enabled: false }),
    )
    await vi.waitFor(() => expect(toggle().getAttribute('aria-checked')).toBe('false'))
    fail = true
    toggle().click()
    await vi.waitFor(() => expect(pageText()).toContain('Connection unavailable'))
    expect(toggle().getAttribute('aria-checked')).toBe('false')
  })
  it('requires confirmation before removing an empty connection', async () => {
    const requests = recordRequests()
    server.use(http.delete(`${API}/user/storage/s3/7`, () => ok(1)))
    await ready()
    button('Remove')!.click()
    await vi.waitFor(() => expect(pageText()).toContain('Remote objects will be kept'))
    expect(requests.some((r) => r.method === 'DELETE')).toBe(false)
    button('Confirm removal')!.click()
    await vi.waitFor(() => expect(requests.some((r) => r.method === 'DELETE')).toBe(true))
  })
  it('cannot remove occupied connections or save without encryption', async () => {
    server.use(
      http.get(`${API}/user/storage/s3`, () =>
        ok({ ...state, canSave: false, connections: [{ ...connection, canRemove: false }] }),
      ),
    )
    await ready()
    expect(button('Remove')!.disabled).toBe(true)
    button('Add S3')!.click()
    await vi.waitFor(() => expect(button('Save')!.disabled).toBe(true))
  })
})
