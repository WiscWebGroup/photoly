import { expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import EmptyState from '../EmptyState.vue'

it('renders with the default icon', () => {
  const wrapper = mount(EmptyState, { props: { title: 'Nothing here' } })
  expect(wrapper.text()).toContain('Nothing here')
  expect(wrapper.find('svg').exists()).toBe(true)
})
