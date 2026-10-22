import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { http } from 'msw'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { button, byText, field, input, mountWithApp, pageText, type } from '@/test/utils'
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
    http.post(`${API}/user/updateEmail`, () => json({ msgCode: 200 })),
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
    await vi.waitFor(() => expect(field('Email').value).toBe('alice@example.com'))
    expect(field('Username').value).toBe('alice')
    expect(pageText()).toContain(
      `Member since ${new Date(2024, 0, 1).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })}`,
    )
  })

  it('saves only the profile fields that changed', async () => {
    const calls = recordRequests()
    await mount()
    await vi.waitFor(() => expect(field('Email').value).toBe('alice@example.com'))
    expect(button('Save Changes').disabled).toBe(true)
    type(field('Email'), 'new@example.com')
    await flushPromises()
    button('Save Changes').click()

    await vi.waitFor(() => expect(calls.map((c) => c.path)).toContain('/user/updateEmail'))
    expect(calls.map((c) => c.path)).not.toContain('/user/updateUsername')
    expect(calls.find((c) => c.path === '/user/updateEmail')?.query).toEqual({
      email: 'new@example.com',
    })
  })

  it('refuses mismatched passwords', async () => {
    const calls = recordRequests()
    await mount()
    type(input('New Password'), 'one')
    type(input('Confirm Password'), 'two')
    await vi.waitFor(() => expect(pageText()).toContain("Passwords don't match."))
    button('Update Password').click()
    await flushPromises()

    expect(calls.map((c) => c.path)).not.toContain('/user/updatePassword')
  })

  it('sends the new password, not the email, as newPass', async () => {
    const calls = recordRequests()
    await mount()
    type(input('Current Password'), 'old')
    type(input('New Password'), 'fresh')
    type(input('Confirm Password'), 'fresh')
    await flushPromises()
    button('Update Password').click()

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
