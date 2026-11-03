import { expect, it, vi } from 'vitest'
import { copyText, shareBase } from './shareLink'
it('resolves proxy prefixes, domain, LAN IP and explicit API endpoints', () => {
  expect(shareBase('/api', 'http://192.168.1.9:5200')).toBe('http://192.168.1.9:5200/api')
  expect(shareBase('/photoly/api/', 'https://photos.example.com')).toBe(
    'https://photos.example.com/photoly/api',
  )
  expect(shareBase('https://api.example.com/v1', 'https://photos.example.com')).toBe(
    'https://api.example.com/v1',
  )
})
it('falls back when clipboard access is denied and reports total failure', async () => {
  vi.stubGlobal('navigator', {
    clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
  })
  const exec = vi.fn().mockReturnValue(true)
  Object.defineProperty(document, 'execCommand', { value: exec, configurable: true })
  expect(await copyText('https://example.com')).toBe(true)
  expect(exec).toHaveBeenCalledWith('copy')
  exec.mockReturnValue(false)
  expect(await copyText('https://example.com')).toBe(false)
  expect(document.querySelector('textarea')).toBeNull()
  vi.unstubAllGlobals()
})
