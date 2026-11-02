import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { apiAddress, curlCommand, endpoints, shellQuote } from '../credentialApi'
describe('scoped API examples', () => {
  it('preserves deployment prefixes and uses header credentials', () => {
    const e = endpoints.find((e) => e.kind === 'detail')!
    expect(apiAddress(e, '/api', 'https://photo.example')).toBe(
      'https://photo.example/api/cred/media/YOUR_MEDIA_UUID',
    )
    expect(apiAddress(e, 'https://backend.example/prefix/', 'https://photo.example')).toBe(
      'https://backend.example/prefix/cred/media/YOUR_MEDIA_UUID',
    )
    const c = curlCommand(e, '/api', 'https://photo.example', "secret'$(echo bad)")
    expect(c).toContain("Authorization: Bearer secret'\\''$(echo bad)")
    expect(c).not.toContain('/secret')
    expect(shellQuote("a'b")).toBe("'a'\\''b'")
  })
  it('uses actual upload parameter names and GET downloads', () => {
    expect(
      curlCommand(
        endpoints.find((e) => e.kind === 'uploads')!,
        '',
        'https://photo.example',
      ),
    ).toContain('photosStr=')
    expect(
      curlCommand(
        endpoints.find((e) => e.kind === 'original')!,
        '',
        'https://photo.example',
      ),
    ).toContain('--request GET')
    expect(
      curlCommand(
        endpoints.find((e) => e.kind === 'original')!,
        '',
        'https://photo.example',
      ),
    ).toContain('--output original-media')
  })
  it('keeps every catalog entry in OpenAPI with its scope and bearer authentication', () => {
    const api = JSON.parse(
      readFileSync('../photoly_server/openapi.json', 'utf8'),
    )
    for (const e of endpoints) {
      const op = api.paths[e.path][e.method.toLowerCase()]
      expect(op.security).toEqual([{ ApiCredential: [] }])
      expect(op.description).toContain(`Requires ${e.scope}`)
    }
  })
})
