import { expect, test } from '@playwright/test'
import { mockApi, ok, signIn } from './mockApi'

const backup = {
  id: 'a30cd21b-6c3c-49e8-b3b0-ac17fa634a21',
  createdAt: '2026-11-03T14:00:00Z',
  size: 1073741824,
  complete: true,
}
test('maintenance, encrypted backup and restore controls follow task state', async ({ page }) => {
  await signIn(page)
  let mode = 'off'
  let job: object | undefined
  const calls = await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/maintenance': () =>
      ok({ state: { mode, job }, problems: [], backupDirectory: '/backups', rollbacks: [] }),
    '/admin/backups': (request) => {
      if (request.method === 'GET') return ok([backup])
      job = {
        id: backup.id,
        kind: 'backup',
        state: 'running',
        stage: 'Exporting database',
        files: 0,
        bytes: 0,
        startedAt: new Date().toISOString(),
      }
      return ok(job)
    },
    '/admin/maintenance/enter': () => {
      mode = 'on'
      return ok({ accepted: true })
    },
    '/admin/maintenance/exit': () => {
      mode = 'off'
      return ok({ accepted: true })
    },
    [`/admin/backups/${backup.id}/restore`]: () => ok({ accepted: true }),
  })
  await page.goto('/thepersonincharge/maintenance')
  await expect(page.getByRole('heading', { name: 'Maintenance and Backup' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create Backup', exact: true })).toBeDisabled()
  await page.getByRole('button', { name: 'Enter Maintenance', exact: true }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Enter Maintenance' }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Maintenance active' })).toBeVisible()
  await page.getByLabel('Backup password', { exact: true }).fill('a long backup password')
  await page.getByLabel('Confirm password', { exact: true }).fill('different')
  await expect(page.getByRole('button', { name: 'Create Backup', exact: true })).toBeDisabled()
  await page.getByLabel('Confirm password', { exact: true }).fill('a long backup password')
  await page.getByRole('button', { name: 'Create Backup', exact: true }).click()
  await expect(page.getByText('Backup · Exporting database')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create Backup', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Restore', exact: true })).toBeDisabled()
  await expect(page.getByRole('button', { name: 'Exit Maintenance' })).toBeDisabled()
  expect(calls.filter((c) => c.path === '/admin/backups' && c.method === 'POST')).toHaveLength(1)
  job = undefined
  await page.getByRole('button', { name: 'Refresh', exact: true }).click()
  await page.getByRole('button', { name: 'Restore', exact: true }).click()
  const dialog = page.getByRole('alertdialog')
  await expect(dialog).toContainText('replaces all users')
  await expect(dialog.getByRole('button', { name: 'Replace and Restore' })).toBeDisabled()
  await dialog.getByLabel('Backup password').fill('a long backup password')
  await dialog.getByRole('button', { name: 'Replace and Restore' }).click()
  await expect
    .poll(() => calls.find((c) => c.path.endsWith('/restore'))?.body)
    .toEqual({ password: 'a long backup password', confirm: true })
})

test('maintenance layout fits small screens and dark mode', async ({ page }) => {
  await signIn(page)
  await mockApi(page, {
    '/admin/isAdmin': ok(true),
    '/admin/maintenance': ok({
      state: { mode: 'on' },
      problems: [],
      backupDirectory: '/backups',
      rollbacks: [],
    }),
    '/admin/backups': ok([backup]),
  })
  await page.goto('/thepersonincharge/maintenance')
  await expect(page.getByRole('heading', { name: 'Maintenance and Backup' })).toBeVisible()
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  )
  await expect(page.getByRole('button', { name: 'Restore', exact: true })).toBeEnabled()
})
