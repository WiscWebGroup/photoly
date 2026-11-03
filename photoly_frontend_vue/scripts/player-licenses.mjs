// ArtPlayer's npm archive omits its LICENSE and bundles screenfull and Hint.css.
// Keep the reviewed upstream notices alongside the exact version; upgrades require review.
import { createRequire } from 'node:module'
import { readFileSync, readdirSync, existsSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'

const root = resolve(import.meta.dirname, '..')
const visited = new Set()
const sections = []
function notice(path) {
  const text = readFileSync(path, 'utf8').replace(/\r\n/g, '\n').trim()
  if (!text.includes('Permission is hereby granted') || !text.includes('THE SOFTWARE IS PROVIDED'))
    throw new Error(`Missing complete MIT license: ${path}`)
  return text
}
function visit(name, from) {
  const require = createRequire(join(from, 'package.json'))
  let dir = dirname(require.resolve(name))
  while (
    !existsSync(join(dir, 'package.json')) ||
    JSON.parse(readFileSync(join(dir, 'package.json'))).name !== name
  ) {
    if (dirname(dir) === dir) throw new Error(`Cannot locate ${name}`)
    dir = dirname(dir)
  }
  const pkg = JSON.parse(readFileSync(join(dir, 'package.json')))
  const key = `${name}@${pkg.version}`
  if (visited.has(key)) return
  visited.add(key)
  let files = readdirSync(dir)
    .filter((file) => /^(licen[cs]e|copying|notice|copyright)([.-]|$)/i.test(file))
    .map((file) => join(dir, file))
  if (name === 'artplayer') {
    if (pkg.version !== '5.4.0')
      throw new Error('Review bundled player licenses before upgrading ArtPlayer')
    files = [join(root, 'licenses/artplayer/LICENSE')]
  }
  if (!files.length) throw new Error(`Missing license files: ${key}`)
  sections.push(`${key} (${pkg.license})\n\n${files.map(notice).join('\n\n')}`)
  for (const dependency of Object.keys(pkg.dependencies ?? {}).sort()) visit(dependency, dir)
}
visit('artplayer', root)
for (const [name, file] of [
  ['screenfull (vendored in ArtPlayer 5.4.0)', 'screenfull-license'],
  ['Hint.css 2.7.0 (vendored in ArtPlayer 5.4.0)', 'hint-css-LICENSE'],
]) {
  sections.push(`${name}\n\n${notice(join(root, 'licenses/artplayer', file))}`)
}
writeFileSync(
  join(root, 'public/player-licenses.txt'),
  'Photoly video player — third-party licenses\nThese components retain their original licenses. Photoly is AGPL-3.0-or-later.\n\n' +
    sections.join('\n\n' + '='.repeat(80) + '\n\n') +
    '\n',
)
console.log(`Wrote notices for ${visited.size} packages and 2 bundled components`)
