import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import router from '@/router'
import Login from '../Login.vue'

const demo = reactive({ enabled: true, email: 'demo@a.com', password: 'demo-pass' })
vi.mock('@/composables/useDemo', () => ({ useDemo: () => demo, loadDemo: async () => {} }))

describe('demo mode', () => {
  it('keeps the landing page at /', async () => {
    await router.push('/login')
    await router.push('/')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/')
  })

  it('pre-fills the shared login', () => {
    const wrapper = mount(Login, { global: { plugins: [router] } })
    const inputs = wrapper.findAll('input')
    expect(inputs[0]!.element.value).toBe('demo@a.com')
    expect(inputs[1]!.element.value).toBe('demo-pass')
  })
})
