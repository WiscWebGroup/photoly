import { h, type Component } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { Toaster } from '@/components/ui/sonner'
import router from '@/router'

/**
 * Mounts inside the app-level providers (Pinia, real router).
 * `Api` types the component's exposed methods, since legacy components are untyped JS.
 */
export function mountWithApp<Api = object>(
  component: Component,
  props: Record<string, unknown> = {},
) {
  const wrapper = mount(() => [h(component, props), h(Toaster)], {
    // Real transitions: Reka only shows expanded tree nodes / closes modals after they finish.
    global: {
      plugins: [createPinia(), router],
      stubs: { transition: false, 'transition-group': false },
    },
    attachTo: document.body,
  })
  return wrapper.findComponent(component) as unknown as VueWrapper & { vm: Api }
}

/** Visible text anywhere on the page, including teleported modals/messages. */
export const pageText = () => document.body.textContent ?? ''

export function button(name: string) {
  const match = [...document.querySelectorAll('button')].find((b) => b.textContent?.trim() === name)
  if (!match) throw new Error(`No button named "${name}"`)
  return match
}

export function input(placeholder: string) {
  const match = document.querySelector<HTMLInputElement>(`input[placeholder="${placeholder}"]`)
  if (!match) throw new Error(`No input with placeholder "${placeholder}"`)
  return match
}

export function type(el: HTMLInputElement, value: string) {
  el.value = value
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

/** Innermost element whose whole text is `text` (e.g. a tree node or menu label). */
export function byText(text: string) {
  const match = [...document.body.querySelectorAll<HTMLElement>('*')]
    .reverse()
    .find((el) => el.textContent?.trim() === text)
  if (!match) throw new Error(`No element with text "${text}"`)
  return match
}
