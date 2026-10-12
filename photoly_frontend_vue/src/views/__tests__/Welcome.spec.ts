import { describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import naive from 'naive-ui'
import router from '@/router'
import Welcome from '../Welcome.vue'

describe('Welcome', () => {
  it('navigates to login from the hero button', async () => {
    const wrapper = mount(Welcome, { global: { plugins: [router, naive] } })
    expect(wrapper.text()).toContain('PHOTOLY')

    await wrapper.findAll('button').find((b) => b.text() === 'Sign In')!.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/login')
  })
})
