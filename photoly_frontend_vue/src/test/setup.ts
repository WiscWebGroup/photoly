import { afterAll, afterEach, beforeAll, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { enableAutoUnmount } from '@vue/test-utils'
import { server } from './server'

// The router guard reads the auth store, even in tests that don't install Pinia.
beforeEach(() => setActivePinia(createPinia()))
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  server.resetHandlers()
  server.events.removeAllListeners()
  localStorage.clear()
  document.body.innerHTML = ''
})
afterAll(() => server.close())

// afterEach hooks run in reverse order: register last so components unmount before the DOM is wiped.
enableAutoUnmount(afterEach)
