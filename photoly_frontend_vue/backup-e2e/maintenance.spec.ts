import { expect, test } from '@playwright/test'
import { execFileSync } from 'node:child_process'

// This config targets only the dedicated localhost:18384 test instance.
test('real AIO backup, restart, changed data, restore and media access', async ({ page }) => {
  const endpoint = 'http://localhost:18384'
  const credentials = { email: 'admin@test.local', password: 'test-password-only' }
  const signIn = async () => {
    const response = await page.request.post(endpoint + '/user/signIn', { data: credentials })
    const data = await response.json()
    expect(data.msgCode).toBe(200)
    return data.token as string
  }
  let token = await signIn()
  const api = async (path: string, method = 'GET', data?: object) => {
    const response = await page.request.fetch(endpoint + path, {
      method,
      headers: { 'HRD-Token': token },
      data,
    })
    const body = await response.json()
    expect(body.msgCode, JSON.stringify(body)).toBe(200)
    return body.t
  }
  const mode = await api('/admin/maintenance')
  if (mode.state.mode !== 'off') await api('/admin/maintenance/exit', 'POST')
  const root = await api('/namespace/getRoot')
  const upload = await page.request.post(endpoint + '/photo/insert', {
    headers: { 'HRD-Token': token },
    multipart: {
      photo: JSON.stringify({ nsId: root.nsId }),
      file: {
        name: 'backup-fixture.png',
        mimeType: 'image/png',
        buffer: Buffer.from(
          'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=',
          'base64',
        ),
      },
    },
  })
  expect((await upload.json()).msgCode).toBe(200)
  await api('/user/updateUsername?username=Before%20backup', 'POST')
  await page.goto('/login')
  await page.evaluate((value) => localStorage.setItem('HRD-Token', value), token)
  await page.goto('/thepersonincharge/maintenance')
  await page.getByRole('button', { name: 'Enter Maintenance', exact: true }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Enter Maintenance' }).click()
  await expect(page.getByRole('status').filter({ hasText: 'Maintenance active' })).toBeVisible({
    timeout: 30000,
  })
  const password = 'browser-test-backup-password'
  await page.getByLabel('Backup password', { exact: true }).fill(password)
  await page.getByLabel('Confirm password', { exact: true }).fill(password)
  await page.getByRole('button', { name: 'Create Backup', exact: true }).click()
  await expect
    .poll(
      async () => {
        const job = (await api('/admin/maintenance')).state.job
        if (job.state === 'failed') throw new Error(job.error)
        return job.state
      },
      { timeout: 90000 },
    )
    .toBe('succeeded')
  const backups = await api('/admin/backups')
  const backup = backups.at(-1)
  expect(backup).toBeTruthy()
  await page.screenshot({ path: '/tmp/photoly-maintenance-desktop.png', fullPage: true })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  await page.screenshot({ path: '/tmp/photoly-maintenance-mobile.png', fullPage: true })
  // A restart must preserve maintenance and must not start background mutation workers.
  execFileSync('docker', ['restart', 'photoly-maintenance-e2e'], { stdio: 'pipe', timeout: 30000 })
  console.info('AIO restarted; checking maintenance')
  await expect
    .poll(
      async () => {
        try {
          return (await (await page.request.get(endpoint + '/maintenance/status')).json()).t.mode
        } catch {
          return ''
        }
      },
      { timeout: 30000 },
    )
    .toBe('on')
  token = await signIn()
  console.info('Signed in after restart')
  await api('/admin/maintenance/exit', 'POST')
  console.info('Exited maintenance after restart')
  await api('/user/updateUsername?username=After%20backup', 'POST')
  await api('/admin/maintenance/enter', 'POST')
  await expect.poll(async () => (await api('/admin/maintenance')).state.mode).toBe('on')
  await page.evaluate((value) => localStorage.setItem('HRD-Token', value), token)
  await page.goto('/thepersonincharge/maintenance')
  await page.getByRole('button', { name: 'Restore', exact: true }).first().click()
  await page.getByRole('alertdialog').getByLabel('Backup password').fill(password)
  await page.getByRole('button', { name: 'Replace and Restore' }).click()
  await expect(page).toHaveURL(/login/, { timeout: 90000 })
  token = await signIn()
  const restored = await api('/admin/maintenance')
  expect(restored.state.job.state, restored.state.job.error).toBe('succeeded')
  expect(restored.state.mode).toBe('on')
  expect(restored.rollbacks.length).toBeGreaterThan(0)
  await api('/admin/maintenance/exit', 'POST')
  expect((await api('/user/getInfo')).userName).toBe('Before backup')
  const archive = await page.request.get(
    endpoint + `/namespace/downloadNs/${token}?nsId=${root.nsId}`,
  )
  expect(archive.status()).toBe(200)
  expect((await archive.body()).length).toBeGreaterThan(100)
})
