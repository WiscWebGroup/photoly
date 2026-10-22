import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import router from '@/router'
import Welcome from '../Welcome.vue'

describe('Welcome', () => {
  it('navigates to login from the header', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router], stubs: { ShapeWaves: true } } })
    expect(wrapper.find('img[alt="Photoly"]').exists()).toBe(true)

    await wrapper
      .findAll('a')
      .find((a) => a.text() === 'Sign In')!
      .trigger('click')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })

  it('sends Get Started to sign up', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router], stubs: { ShapeWaves: true } } })
    await wrapper
      .findAll('a')
      .find((a) => a.text() === 'Get Started')!
      .trigger('click')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/signup'))
  })

  it('toggles the phone menu', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router], stubs: { ShapeWaves: true } } })
    const toggle = wrapper.get('button[aria-label="Menu"]')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
  })
})
