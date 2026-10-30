import { beforeEach, describe, expect, it } from 'vitest'
import { http } from 'msw'
import { ApiError } from '@/lib/http'
import { API, json, ok, recordRequests, server } from '@/test/server'
import { isValidFolderName, useFolders } from '../useFolders'

const root = { nsId: 1, nsName: 'root' }
const trips = { nsId: 2, nsName: 'Trips' }

beforeEach(() => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/namespace/getRoot`, () => ok(root)),
    http.get(`${API}/namespace/getChildren`, () => ok([trips])),
    http.get(`${API}/namespace/trace`, () => ok([root])),
  )
})

describe('isValidFolderName', () => {
  it('allows a real directory named root', () => expect(isValidFolderName('root')).toBe(true))
  it.each([null, '', '.', '..', '/', 'a/b'])('rejects %j', (name) =>
    expect(isValidFolderName(name)).toBe(false),
  )
  it('accepts a normal name', () => expect(isValidFolderName('Trips')).toBe(true))
})

describe('useFolders', () => {
  it('opens the root with its children and path, sending the token', async () => {
    const calls = recordRequests()
    const folders = useFolders()
    await folders.openRoot()
    expect(folders.current.value).toEqual(root)
    expect(folders.children.value).toEqual([trips])
    expect(folders.path.value).toEqual([root])
    expect(calls.map((c) => c.path).sort()).toEqual([
      '/namespace/getChildren',
      '/namespace/getRoot',
      '/namespace/trace',
    ])
  })

  it('create posts under the current folder, then reloads', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/namespace/insert`, () => ok()))
    const folders = useFolders()
    await folders.openRoot()
    calls.length = 0
    await folders.create('Pets')
    expect(calls[0]).toMatchObject({
      method: 'POST',
      path: '/namespace/insert',
      body: { nsParentId: 1, nsName: 'Pets' },
    })
    expect(calls.map((c) => c.path)).toContain('/namespace/getChildren')
  })

  it('remove passes nsId as a query param', async () => {
    const calls = recordRequests()
    server.use(http.post(`${API}/namespace/delete`, () => ok()))
    const folders = useFolders()
    await folders.openRoot()
    await folders.remove(2)
    expect(calls.find((c) => c.path === '/namespace/delete')).toMatchObject({
      query: { nsId: '2' },
    })
  })

  it('throws ApiError when the backend refuses', async () => {
    server.use(http.post(`${API}/namespace/updateName`, () => json({ msgCode: 400, t: null })))
    const folders = useFolders()
    await folders.openRoot()
    await expect(folders.rename(2, 'x')).rejects.toBeInstanceOf(ApiError)
  })
})
