import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import router from '@/router'
import Welcome from '../Welcome.vue'

describe('Welcome', () => {
  it('navigates to login from the hero button', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router] } })
    expect(wrapper.text()).toContain('PHOTOLY')

    await wrapper.findAll('button').find((b) => b.text() === 'Sign In')!.trigger('click')
    await vi.waitFor(() => expect(router.currentRoute.value.path).toBe('/login'))
  })
})
