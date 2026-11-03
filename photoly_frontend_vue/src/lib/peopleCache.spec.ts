import { describe, expect, it, vi } from 'vitest'
import { AvatarCache } from './peopleCache'
describe('session avatar cache', () => {
  it('deduplicates, expires and evicts least recently used bytes', async () => {
    const cache = new AvatarCache(6, 100)
    const fetch = vi.fn(async () => new Blob(['abc']))
    await Promise.all([cache.get('a', fetch), cache.get('a', fetch)])
    expect(fetch).toHaveBeenCalledTimes(1)
    await cache.get('b', fetch)
    await cache.get('a', fetch)
    await cache.get('c', fetch)
    await cache.get('b', fetch)
    expect(fetch).toHaveBeenCalledTimes(4)
    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 101)
    await cache.get('b', fetch)
    expect(fetch).toHaveBeenCalledTimes(5)
    clock.mockRestore()
    cache.clear()
    await cache.get('b', fetch)
    expect(fetch).toHaveBeenCalledTimes(6)
  })
  it('bounds concurrency and prevents old account requests repopulating cache', async () => {
    const cache = new AvatarCache()
    const release: (() => void)[] = []
    const fetch = vi.fn(
      () => new Promise<Blob>((resolve) => release.push(() => resolve(new Blob(['a'])))),
    )
    const requests = Array.from({ length: 8 }, (_, i) =>
      cache.get(String(i), fetch).catch(() => null),
    )
    expect(fetch).toHaveBeenCalledTimes(4)
    cache.clear()
    release.forEach((r) => r())
    await Promise.all(requests)
    expect(fetch).toHaveBeenCalledTimes(4)
    const fresh = vi.fn(async () => new Blob(['new']))
    expect(await cache.get('0', fresh)).toBeTruthy()
    expect(fresh).toHaveBeenCalledTimes(1)
  })
})
