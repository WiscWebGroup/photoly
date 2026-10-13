import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import AdminDashboard from '../AdminDashboard.vue'
import AdminUserManagement from '../AdminUserManagement.vue'

const get = (path: string, body: object) => http.get(`${API}${path}`, () => json(body))

const user = (userId: number, userName: string) => ({
  userId,
  userName,
  email: `${userName}@example.com`,
  createDate: '2024-01-01',
  role: 'user',
  uuid: `uuid-${userName}`,
})

beforeEach(async () => {
  localStorage.setItem('HRD-Token', 'tok')
  await router.push('/thepersonincharge/dashboard')
})

describe('AdminDashboard', () => {
  it('shows service status, addresses and settings', async () => {
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      get('/admin/pingMySQL', { msgCode: 200 }),
      get('/admin/pingRedis', { msgCode: 500 }),
      get('/admin/getAddress', {
        msgCode: 200,
        t: { mysql: 'db:3306', redis: 'cache:6379', upload: '/up', config: '/cfg' },
      }),
      get('/admin/getSettings', {
        msgCode: 200,
        t: { SignUp: '1', SSafeUUID: '0', TokenDuration: 7 },
      }),
    )
    mountWithApp(AdminDashboard)

    await vi.waitFor(() => expect(pageText()).toContain('db:3306'))
    const rings = [...document.querySelectorAll('svg[role=img]')].map((s) =>
      s.getAttribute('aria-label'),
    )
    expect(rings).toEqual([
      'Total Status: 67%',
      'MySQL Status: 100%',
      'Redis Status: 0%',
      'Backend Status: 100%',
    ])
    expect(document.querySelector('[aria-label=Registry]')?.getAttribute('data-state')).toBe(
      'checked',
    )
    expect(document.querySelector('[aria-label="SSafe UUID"]')?.getAttribute('data-state')).toBe(
      'unchecked',
    )
  })

  it('sends non-admins to the login page', async () => {
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: false }),
      get('/admin/pingMySQL', { msgCode: 403 }),
      get('/admin/pingRedis', { msgCode: 403 }),
      get('/admin/getAddress', { msgCode: 403 }),
      get('/admin/getSettings', { msgCode: 403 }),
    )
    mountWithApp(AdminDashboard)
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })
})

describe('AdminUserManagement', () => {
  it('searches, then pages through users', async () => {
    const calls = recordRequests()
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      get('/admin/getUserPage', { msgCode: 200, t: [user(1, 'alice')], pageNum: 2 }),
      get('/admin/searchUserPage', { msgCode: 200, t: [user(2, 'bob')], pageNum: 1 }),
    )
    mountWithApp(AdminUserManagement)
    await vi.waitFor(() => expect(pageText()).toContain('uuid-alice'))
    expect(pageText()).toContain('Page 1 of 2')

    button('Next').click()
    await vi.waitFor(() => expect(pageText()).toContain('Page 2 of 2'))
    expect(calls.at(-1)).toMatchObject({
      path: '/admin/getUserPage',
      query: { page: '2', rowsPerPage: '10' },
    })

    const box = document.querySelector<HTMLInputElement>('input[aria-label="Search users"]')!
    box.value = 'bob'
    box.dispatchEvent(new Event('input', { bubbles: true }))
    button('Search').click()
    await vi.waitFor(() => expect(pageText()).toContain('uuid-bob'))
    expect(calls.at(-1)).toMatchObject({
      path: '/admin/searchUserPage',
      query: { page: '1', search: 'bob' },
    })
  })

  it('deletes a user after confirmation', async () => {
    const calls = recordRequests()
    server.use(
      get('/admin/isAdmin', { msgCode: 200, t: true }),
      get('/admin/getUserPage', { msgCode: 200, t: [user(1, 'alice')], pageNum: 1 }),
      http.post(`${API}/admin/deleteUser`, () => ok()),
    )
    mountWithApp(AdminUserManagement)
    await vi.waitFor(() => expect(pageText()).toContain('uuid-alice'))

    button('Delete User').click()
    await vi.waitFor(() =>
      expect(pageText()).toContain('Do you really want to Delete this User with ID 1'),
    )
    button('Delete').click()
    await vi.waitFor(() => expect(calls.some((c) => c.path === '/admin/deleteUser')).toBe(true))
    expect(calls.find((c) => c.path === '/admin/deleteUser')).toMatchObject({
      query: { userId: '1' },
    })
  })
})
