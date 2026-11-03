import { beforeEach, expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, json, ok, server } from '@/test/server'
import { useAuthStore } from '@/stores/auth'
import { useFolderPinsStore } from '../folderPins'

const trips = { nsId: 2, nsName: 'Trips', path: 'Library / Trips' }
beforeEach(() => useAuthStore().setToken('alice'))

it('cancels a queued mutation when the account changes before dispatch', async () => {
  const handler = vi.fn(() => ok({ items: [], defaultNsId: null }))
  server.use(http.post(`${API}/namespace/defaultLocation`, handler))
  const pins = useFolderPinsStore()
  const pending = pins.setDefault(null)
  useAuthStore().setToken('bob')
  await pending
  expect(handler).not.toHaveBeenCalled()
  expect(pins.saving).toBe(false)
  expect(pins.loaded).toBe(false)
})

it('loads account preferences and only applies successful mutations', async () => {
  const pins = useFolderPinsStore()
  server.use(
    http.get(`${API}/namespace/pins`, () => ok({ items: [trips], defaultNsId: 2 })),
    http.post(`${API}/namespace/unpin`, () => json({ msgCode: 500, t: null })),
  )
  await pins.load()
  await pins.unpin(2)
  expect(pins.items).toEqual([trips])
  expect(pins.defaultNsId).toBe(2)
  expect(pins.saving).toBe(false)
  server.use(http.post(`${API}/namespace/unpin`, () => ok({ items: [], defaultNsId: null })))
  await pins.unpin(2)
  expect(pins.items).toEqual([])
  expect(pins.defaultNsId).toBeNull()
})

it('discards responses from a previous account, even after switching back', async () => {
  const pins = useFolderPinsStore()
  let finish!: () => void
  const delayed = new Promise<void>((resolve) => {
    finish = resolve
  })
  server.use(
    http.get(`${API}/namespace/pins`, async () => {
      await delayed
      return ok({ items: [trips], defaultNsId: 2 })
    }),
  )
  const request = pins.load()
  const auth = useAuthStore()
  auth.setToken('bob')
  auth.setToken('alice')
  finish()
  await request
  expect(pins.items).toEqual([])
  expect(pins.defaultNsId).toBeNull()
  expect(pins.loaded).toBe(false)
})

it('prevents an older refresh from overwriting a successful change', async () => {
  const pins = useFolderPinsStore()
  let finish!: () => void
  const delayed = new Promise<void>((resolve) => {
    finish = resolve
  })
  server.use(
    http.get(`${API}/namespace/pins`, async () => {
      await delayed
      return ok({ items: [], defaultNsId: null })
    }),
    http.post(`${API}/namespace/pin`, () => ok({ items: [trips], defaultNsId: null })),
  )
  const refresh = pins.load(true)
  await pins.pin(2)
  finish()
  await refresh
  expect(pins.items).toEqual([trips])
})

it('keeps existing settings on refresh failure and retries on the next load', async () => {
  const pins = useFolderPinsStore()
  server.use(http.get(`${API}/namespace/pins`, () => ok({ items: [trips], defaultNsId: 2 })))
  await pins.load()
  server.use(http.get(`${API}/namespace/pins`, () => json({ msgCode: 500, t: null })))
  await expect(pins.load(true)).rejects.toThrow()
  expect(pins.defaultNsId).toBe(2)
  server.use(http.get(`${API}/namespace/pins`, () => ok({ items: [], defaultNsId: null })))
  await pins.load(true)
  expect(pins.items).toEqual([])
})
