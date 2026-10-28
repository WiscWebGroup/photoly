import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import AdminDashboard from '../AdminDashboard.vue'
import AdminUserManagement from '../AdminUserManagement.vue'

const get = (path: string, body: object) => http.get(`${API}${path}`, () => json(body))

const emails = () =>
  [...document.querySelectorAll<HTMLInputElement>('input[aria-label="Email"]')].map((i) => i.value)
const user = (userId: number, userName: string) => ({
  userId,
  userName,
  email: `${userName}@example.com`,
  createDate: '2024-01-01',
  role: 'user',
  uuid: `uuid-${userName}`,
  mediaCount: 1234,
  groupId: 1,
  groupName: 'Default',
  usedBytes: 1000,
  capacityBytes: 50000000000,
  storageOverride: false,
  maxStorageBytes: null,
  overQuota: false,
})

beforeEach(async () => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    get('/admin/isAdmin', { msgCode: 200, t: true }),
    http.get(`${API}/admin/ai/users`, ({ request }) =>
      ok(
        new URL(request.url).searchParams
          .get('ids')!
          .split(',')
          .map((id) => ({ userId: Number(id), enabled: false })),
      ),
    ),
  )
  await router.push('/thepersonincharge/dashboard')
})

describe('AdminDashboard', () => {
  it('shows service status, addresses and settings', async () => {
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      get('/admin/pingDatabase', { msgCode: 500 }),
      get('/admin/databaseInfo', {
        msgCode: 200,
        t: {
          type: 'postgresql',
          address: 'db:5432',
          version: '17.11',
          pgvectorVersion: '0.8.7',
          schemaRevision: '0002_ai_backend',
        },
      }),
      get('/admin/getAddress', {
        msgCode: 200,
        t: { mysql: 'db:5432', upload: '/up', config: '/cfg' },
      }),
      get('/admin/getSettings', {
        msgCode: 200,
        t: { SignUp: '1', SSafeUUID: '0', TokenDuration: 7 },
      }),
    )
    mountWithApp(AdminDashboard)

    await vi.waitFor(() => expect(pageText()).toContain('db:5432'))
    await vi.waitFor(() => expect(pageText()).toContain('PostgreSQL not responding'))
    const status = [...document.querySelectorAll('[role=status]')].map((s) =>
      s.getAttribute('aria-label'),
    )
    expect(status).toEqual([
      'Backend Status: Running',
      'PostgreSQL Status: Down',
    ])
    expect(pageText()).toContain('pgvector 0.8.7')
    expect(pageText()).toContain('/up')
    expect(document.querySelector('[aria-label=Registry]')?.getAttribute('data-state')).toBe(
      'checked',
    )
    expect(document.querySelector('[aria-label="SSafe UUID"]')?.getAttribute('data-state')).toBe(
      'unchecked',
    )
  })

  it('sends non-admins home without clearing their session', async () => {
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: false }),
      get('/admin/pingDatabase', { msgCode: 403 }),
      get('/admin/getAddress', { msgCode: 403 }),
      get('/admin/databaseInfo', { msgCode: 403 }),
      get('/admin/getSettings', { msgCode: 403 }),
    )
    mountWithApp(AdminDashboard)
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/home/HomePart'))
  })
})

describe('AdminUserManagement', () => {
  it('searches, filters by role, then pages through users', async () => {
    const calls = recordRequests()
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      http.get(`${API}/admin/getUsers`, ({ request }) => {
        const q = new URL(request.url).searchParams
        if (q.get('search')) return ok({ users: [user(2, 'bob')], total: 1 })
        const first = (Number(q.get('page')) - 1) * 8 + 1
        return ok({ users: [user(first, `user${first}`)], total: 12 })
      }),
    )
    const lastQuery = () => calls.filter((c) => c.path === '/admin/getUsers').at(-1)?.query
    mountWithApp(AdminUserManagement)
    await vi.waitFor(() => expect(emails()).toEqual(['user1@example.com']))
    expect(pageText()).toContain('1–1 of 12')
    expect(pageText()).toContain('1,234')
    expect(lastQuery()).toEqual({ page: '1', rowsPerPage: '8' })

    button('Next').click()
    await vi.waitFor(() => expect(emails()).toEqual(['user9@example.com']))
    await vi.waitFor(() => expect(pageText()).toContain('9–9 of 12'))
    expect(lastQuery()).toEqual({ page: '2', rowsPerPage: '8' })

    document.querySelector<HTMLButtonElement>('[role="radio"]:nth-child(2)')!.click()
    await vi.waitFor(() =>
      expect(lastQuery()).toEqual({ page: '1', rowsPerPage: '8', role: 'admin' }),
    )

    const box = document.querySelector<HTMLInputElement>('input[aria-label="Search users"]')!
    box.value = 'bob'
    box.dispatchEvent(new Event('input', { bubbles: true }))
    button('Search').click()
    await vi.waitFor(() => expect(emails()).toEqual(['bob@example.com']))
    expect(lastQuery()).toEqual({ page: '1', rowsPerPage: '8', search: 'bob', role: 'admin' })
    expect(pageText()).toContain('1–1 of 1')
  })

  it('deletes a user after confirmation', async () => {
    const calls = recordRequests()
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      get('/admin/getUsers', { msgCode: 200, t: { users: [user(1, 'alice')], total: 1 } }),
      http.post(`${API}/admin/deleteUser`, () => ok()),
    )
    mountWithApp(AdminUserManagement)
    await vi.waitFor(() => expect(emails()).toEqual(['alice@example.com']))

    document.querySelector<HTMLButtonElement>('button[aria-label="Delete User"]')!.click()
    await vi.waitFor(() => expect(pageText()).toContain('Delete alice?'))
    expect(pageText()).toMatch(
      /The account with ID 1 and its media folder\s*uuid-alice\s*are deleted\./,
    )
    button('Delete').click()
    await vi.waitFor(() => expect(calls.some((c) => c.path === '/admin/deleteUser')).toBe(true))
    expect(calls.find((c) => c.path === '/admin/deleteUser')).toMatchObject({
      query: { userId: '1' },
    })
  })
})
