import endpoints from './credentialEndpoints.json'
export { endpoints }
export type Endpoint = (typeof endpoints)[number]
export function apiAddress(endpoint: Endpoint, base: string, origin: string) {
  const root = new URL(base || '/', origin).href.replace(/\/+$/, '')
  return root + endpoint.path.replace('{uuid}', 'YOUR_MEDIA_UUID').replace('{category}', 'panorama')
}
export const shellQuote = (s: string) => `'${s.replaceAll("'", "'\\''")}'`
export function curlCommand(
  endpoint: Endpoint,
  base: string,
  origin: string,
  token = 'YOUR_TOKEN',
) {
  const args = [
    'curl --fail-with-body',
    `--request ${endpoint.method}`,
    `--header ${shellQuote(`Authorization: Bearer ${token}`)}`,
  ]
  const form = (name: string, value: string) =>
    args.push(`--form ${shellQuote(`${name}=${value}`)}`)
  const query = (name: string, value: string) => {
    if (!args.includes('--get')) args.push('--get')
    args.push(`--data-urlencode ${shellQuote(`${name}=${value}`)}`)
  }
  switch (endpoint.kind) {
    case 'upload':
      form('file', '@/path/to/photo.jpg')
      form('photo', '{"nsId":YOUR_NAMESPACE_ID,"visibility":0}')
      break
    case 'uploads':
      form('files', '@/path/to/first.jpg')
      form('files', '@/path/to/second.jpg')
      form(
        'photosStr',
        '[{"nsId":YOUR_NAMESPACE_ID,"visibility":0},{"nsId":YOUR_NAMESPACE_ID,"visibility":0}]',
      )
      break
    case 'delete':
      args.push(`--data-urlencode ${shellQuote('uuid=YOUR_MEDIA_UUID')}`)
      break
    case 'original':
      query('uuid', 'YOUR_MEDIA_UUID')
      args.push('--output original-media')
      break
    case 'folders':
      query('parentId', 'YOUR_NAMESPACE_ID')
      break
    case 'legacy-list':
      query('nsId', 'YOUR_NAMESPACE_ID')
      break
    case 'media':
    case 'category-media':
      query('limit', '100')
      break
    case 'thumbnail':
      args.push('--output thumbnail.jpg')
      break
  }
  args.push(shellQuote(apiAddress(endpoint, base, origin)))
  return args.join(' \\\n  ')
}
