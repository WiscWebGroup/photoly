import { beforeEach, describe, expect, it } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { usePhotos } from '../usePhotos'

const list = [
  { photoId: 10, photoName: 'beach.jpg' },
  { photoId: 11, photoName: 'city.jpg' },
]

beforeEach(() => {
  localStorage.setItem('HRD-Token', 'tok')
  server.use(
    http.get(`${API}/photo/getByNamespace`, () => ok(list)),
    http.get(`${API}/photo/getByGallery`, () => ok([list[0]])),
  )
})

describe('usePhotos', () => {
  it('loads by folder and by gallery', async () => {
    const calls = recordRequests()
    const { photos, loadByFolder, loadByGallery, loading } = usePhotos()
    await loadByFolder(1)
    expect(photos.value).toEqual(list)
    expect(loading.value).toBe(false)
    await loadByGallery(5)
    expect(photos.value).toEqual([list[0]])
    expect(calls.map((c) => c.query)).toEqual([{ nsId: '1' }, { gaId: '5' }])
  })

  it('moveTo and remove call the API and drop the photo from the list', async () => {
    const calls = recordRequests()
    server.use(
      http.post(`${API}/photo/changeNamespace`, () => ok()),
      http.post(`${API}/photo/delete`, () => ok()),
    )
    const { photos, loadByFolder, moveTo, remove } = usePhotos()
    await loadByFolder(1)
    await moveTo(10, 2)
    await remove(11)
    expect(photos.value).toEqual([])
    expect(calls.slice(1)).toMatchObject([
      { path: '/photo/changeNamespace', query: { photoId: '10', nsId: '2' } },
      { path: '/photo/delete', query: { photoId: '11' } },
    ])
  })

  it('keeps the list and clears loading when a load fails', async () => {
    const { photos, loadByFolder, loading } = usePhotos()
    await loadByFolder(1)
    server.use(http.get(`${API}/photo/getByNamespace`, () => Response.error()))
    await expect(loadByFolder(1)).rejects.toThrow()
    expect(loading.value).toBe(false)
    expect(photos.value).toEqual(list)
  })
})
