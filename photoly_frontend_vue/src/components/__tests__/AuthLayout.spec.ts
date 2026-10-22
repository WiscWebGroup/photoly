import { describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import router from '@/router'
import { link, mountWithApp, pageText } from '@/test/utils'
import AuthLayout from '../AuthLayout.vue'

describe('AuthLayout', () => {
  it('switches between sign in and sign up', async () => {
    await router.push('/login')
    mountWithApp(AuthLayout)
    await vi.waitFor(() => expect(pageText()).toContain('Welcome back'))
    expect(link('Sign In').getAttribute('aria-current')).toBe('page')

    link('Sign Up').click()
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/signup'))
    await flushPromises()
    await vi.waitFor(() => expect(pageText()).toContain('Create your library'))
    expect(link('Sign Up').getAttribute('aria-current')).toBe('page')
  })
})
