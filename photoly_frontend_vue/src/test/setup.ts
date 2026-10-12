import { afterAll, afterEach, beforeAll } from 'vitest'
import { enableAutoUnmount } from '@vue/test-utils'
import { server } from './server'

enableAutoUnmount(afterEach)

beforeAll(() => server.listen({ onUnhandledFrame: 'error' }))
afterEach(() => {
  server.resetHandlers()
  server.events.removeAllListeners()
  localStorage.clear()
  document.body.innerHTML = ''
})
afterAll(() => server.close())
