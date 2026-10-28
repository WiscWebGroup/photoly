import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, field, mountWithApp, pageText, type } from '@/test/utils'
import AdminStorage from '../AdminStorage.vue'

const drivers = [
  { type: 'local', label: 'Local disk', fields: [] },
  {
    type: 's3',
    label: 'S3-compatible',
    fields: [
      { name: 'endpoint', label: 'Endpoint', kind: 'text', required: true },
      { name: 'bucket', label: 'Bucket', kind: 'text', required: true },
      { name: 'secret_key', label: 'Secret key', kind: 'text', required: true, secret: true },
      { name: 'use_ssl', label: 'Use HTTPS', kind: 'bool', default: 'true' },
    ],
  },
]
const state = (over: object = {}) => ({
  active: 'local',
  driver: 'local',
  values: {},
  hasSecret: false,
  drivers,
  canSave: true,
  ...over,
})
const serve = (s: object) =>
  server.use(
    http.get(`${API}/admin/isAdmin`, () => json({ msgCode: 200, t: true })),
    http.get(`${API}/admin/storage`, () => ok(s)),
  )
const select = () => document.querySelector<HTMLSelectElement>('#storage-driver')!
async function choose(value: string) {
  select().value = value
  select().dispatchEvent(new Event('change', { bubbles: true }))
}

beforeEach(async () => {
  localStorage.setItem('HRD-Token', 'tok')
  await router.push('/thepersonincharge/storage')
})

describe('AdminStorage', () => {
  it('renders the fields of the selected driver', async () => {
    serve(state())
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(pageText()).toContain('Currently using'))
    expect(pageText()).toContain('Local disk')
    expect(document.getElementById('storage-endpoint')).toBeNull()

    await choose('s3')
    await vi.waitFor(() => expect(field('Endpoint')).toBeTruthy())
    expect(field('Secret key').type).toBe('password')
    expect(document.querySelector('button[role="switch"][aria-label="Use HTTPS"]')).toBeTruthy()
  })

  it('tests a connection and saves, sending the typed values', async () => {
    const calls = recordRequests()
    serve(state())
    server.use(
      http.post(`${API}/admin/storage/test`, () => ok(1)),
      http.put(`${API}/admin/storage`, () => ok(1)),
    )
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(pageText()).toContain('Currently using'))
    await choose('s3')
    await vi.waitFor(() => field('Endpoint'))

    button('Save').click()
    await vi.waitFor(() => expect(pageText()).toContain('Endpoint is required.'))
    expect(calls.some((c) => c.method === 'PUT')).toBe(false)

    type(field('Endpoint'), 'minio:9000')
    type(field('Bucket'), 'photos')
    type(field('Secret key'), 'hunter2')
    button('Test connection').click()
    await vi.waitFor(() => expect(calls.some((c) => c.path === '/admin/storage/test')).toBe(true))
    button('Save').click()
    await vi.waitFor(() => expect(calls.some((c) => c.method === 'PUT')).toBe(true))
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({
      confirm: false,
      driver: 's3',
      values: { endpoint: 'minio:9000', bucket: 'photos', secret_key: 'hunter2', use_ssl: 'true' },
    })
  })

  it('shows the backend failure reason and lets a saved secret stay blank', async () => {
    serve(
      state({
        active: 's3',
        driver: 's3',
        hasSecret: true,
        values: { endpoint: 'minio:9000', bucket: 'photos', use_ssl: 'true' },
        saved: {
          s3: {
            values: { endpoint: 'minio:9000', bucket: 'photos', use_ssl: 'true' },
            hasSecret: true,
          },
        },
      }),
    )
    server.use(
      http.put(`${API}/admin/storage`, () =>
        json({ msgCode: 400, t: 'bucket "photos" does not exist' }),
      ),
    )
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(field('Endpoint').value).toBe('minio:9000'))
    expect(field('Secret key').value).toBe('')
    expect(field('Secret key').placeholder).toContain('leave blank')

    button('Save').click()
    await vi.waitFor(() => expect(pageText()).toContain('bucket "photos" does not exist'))
  })

  it('restores the saved settings when switching back to a driver', async () => {
    serve(
      state({
        saved: {
          s3: { values: { endpoint: 'minio:9000', bucket: 'photos' }, hasSecret: true },
        },
      }),
    )
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(pageText()).toContain('Currently using'))
    await choose('s3')
    await vi.waitFor(() => expect(field('Endpoint').value).toBe('minio:9000'))
    expect(field('Bucket').value).toBe('photos')
    expect(field('Secret key').placeholder).toContain('leave blank')
  })

  it('asks for STORAGE_SECRET_KEY when credentials cannot be stored', async () => {
    serve(state({ canSave: false }))
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(pageText()).toContain('Currently using'))
    await choose('s3')
    await vi.waitFor(() => expect(pageText()).toContain('STORAGE_SECRET_KEY'))
    expect(button('Save').disabled).toBe(true)
  })

  it('asks before switching backends and resends with confirm', async () => {
    const calls = recordRequests()
    serve(state())
    let attempts = 0
    server.use(
      http.put(`${API}/admin/storage`, () =>
        ++attempts === 1 ? json({ msgCode: 409, t: 'storage_switch_confirm' }) : ok(1),
      ),
    )
    mountWithApp(AdminStorage)
    await vi.waitFor(() => expect(pageText()).toContain('Currently using'))
    await choose('s3')
    await vi.waitFor(() => field('Endpoint'))
    type(field('Endpoint'), 'minio:9000')
    type(field('Bucket'), 'photos')
    type(field('Secret key'), 'hunter2')

    button('Save').click()
    await vi.waitFor(() => expect(pageText()).toContain('Switch storage?'))
    expect(pageText()).toContain('will be hidden')
    expect(pageText()).not.toContain('storage_switch_confirm')

    button('Switch storage').click()
    await vi.waitFor(() => expect(attempts).toBe(2))
    const puts = calls.filter((c) => c.method === 'PUT')
    expect(puts[0].body).toMatchObject({ confirm: false })
    expect(puts[1].body).toMatchObject({ confirm: true })
  })
})
