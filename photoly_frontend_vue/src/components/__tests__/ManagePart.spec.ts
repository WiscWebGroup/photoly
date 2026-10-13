import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, byText, input, mountWithApp, pageText, type } from '@/test/utils'
import ManagePart from '../ManagePart.vue'

function mockAccount() {
  server.use(
    http.get(`${API}/user/getInfo/`, () =>
      ok({
        userId: 1,
        userName: 'alice',
        email: 'alice@example.com',
        createDate: '2024-01-01T00:00:00',
        role: 'user',
      }),
    ),
    http.get(`${API}/cred/query`, () => ok([{ credId: 9, authorization: 'CR', token: 'tok-abc' }])),
    http.post(`${API}/user/updatePassword`, () => json({ msgCode: 200 })),
    http.post(`${API}/cred/create`, () => json({ msgCode: 200 })),
  )
}

async function mount() {
  mockAccount()
  mountWithApp(ManagePart)
  await vi.waitFor(() => expect(pageText()).toContain('tok-abc'))
}

describe('ManagePart', () => {
  it('shows the profile and API tokens', async () => {
    await mount()
    expect(pageText()).toContain('email: alice@example.com')
    expect(pageText()).toContain('created: 2024-01-01')
  })

  it('refuses mismatched passwords', async () => {
    const calls = recordRequests()
    await mount()
    button('Edit Info').click()
    await vi.waitFor(() => input('New Password'))
    type(input('New Password'), 'one')
    type(input('Confirm Password'), 'two')
    button('Submit').click()
    await flushPromises()

    expect(calls.map((c) => c.path)).not.toContain('/user/updatePassword')
  })

  it('sends the new password, not the email, as newPass', async () => {
    const calls = recordRequests()
    await mount()
    button('Edit Info').click()
    await vi.waitFor(() => input('New Password'))
    type(input('Current Password'), 'old')
    type(input('New Password'), 'fresh')
    type(input('Confirm Password'), 'fresh')
    button('Submit').click()

    await vi.waitFor(() => expect(calls.map((c) => c.path)).toContain('/user/updatePassword'))
    expect(calls.find((c) => c.path === '/user/updatePassword')?.query).toEqual({
      oldPass: 'old',
      newPass: 'fresh',
    })
  })

  it('creates an API token with the chosen permissions in canonical order', async () => {
    const calls = recordRequests()
    await mount()
    document.querySelector<HTMLButtonElement>('button[aria-label="Add API"]')!.click()
    await vi.waitFor(() => byText('Add API'))
    // Pick Read before Upload; the stored string must still be "CR".
    byText('Read').click()
    byText('Upload').click()
    button('Create').click()

    await vi.waitFor(() => expect(calls.map((c) => c.path)).toContain('/cred/create'))
    expect(calls.find((c) => c.path === '/cred/create')?.body).toEqual({ authorization: 'CR' })
  })
})
