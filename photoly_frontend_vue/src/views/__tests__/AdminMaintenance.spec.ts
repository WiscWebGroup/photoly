import { beforeEach, expect, it, vi } from 'vitest'
import { http } from 'msw'
import router from '@/router'
import { API, ok, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import AdminMaintenance from '../AdminMaintenance.vue'

beforeEach(async () => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/admin/isAdmin`, () => ok(true)),
    http.get(`${API}/admin/backups`, () => ok([])),
  )
  await router.push('/thepersonincharge/maintenance')
})
it('requires maintenance before backup and explains external storage scope', async () => {
  server.use(
    http.get(`${API}/admin/maintenance`, () =>
      ok({ state: { mode: 'off' }, problems: [], backupDirectory: '/backups', rollbacks: [] }),
    ),
  )
  mountWithApp(AdminMaintenance)
  await vi.waitFor(() => expect(pageText()).toContain('Service running'))
  expect(button('Create Backup')?.disabled).toBe(true)
  expect(pageText()).toContain('Originals on external mounts, S3, WebDAV, and SMB are not copied')
})
it('disables backup and exit while a task is running', async () => {
  server.use(
    http.get(`${API}/admin/maintenance`, () =>
      ok({
        state: {
          mode: 'on',
          job: {
            id: 'test',
            kind: 'backup',
            state: 'running',
            stage: 'Archiving media',
            files: 5,
            bytes: 200,
            startedAt: new Date().toISOString(),
          },
        },
        problems: [],
        backupDirectory: '/backups',
        rollbacks: [],
      }),
    ),
  )
  mountWithApp(AdminMaintenance)
  await vi.waitFor(() => expect(pageText()).toContain('Archiving media'))
  expect(button('Create Backup')?.disabled).toBe(true)
  expect(button('Exit Maintenance')?.disabled).toBe(true)
})
