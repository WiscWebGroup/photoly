import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import router from '@/router'
import Welcome from '../Welcome.vue'

describe('Welcome', () => {
  it('offers only the demo and deploy buttons', () => {
    const wrapper = mount(Welcome, { global: { plugins: [router], stubs: { ShapeWaves: true } } })
    expect(wrapper.find('img[alt="Photoly"]').exists()).toBe(true)
    const links = wrapper.findAll('a').map((a) => a.text())
    expect(links).toContain('Try the demo')
    expect(links).toContain('Deploy your own')
    for (const gone of ['Sign In', 'Sign Up', 'Get Started']) expect(links).not.toContain(gone)
  })

  it('sends Try the demo to the sign-in page', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router], stubs: { ShapeWaves: true } } })
    await wrapper
      .findAll('a')
      .find((a) => a.text() === 'Try the demo')!
      .trigger('click')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })

  it('skips the landing page on a regular deployment', async () => {
    await router.push('/signup')
    await router.push('/')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/login')
  })
})
