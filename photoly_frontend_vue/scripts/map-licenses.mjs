// Regenerate after updating either map engine. Include runtime dependency notices, including
// their bundled third-party notices, in a plain-text artifact shipped by Vite and Docker.
import { createRequire } from 'node:module'
import { readFileSync, readdirSync, existsSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
const root = resolve(import.meta.dirname, '..')
const visited = new Set(),
  sections = [],
  missing = []
function visit(name, from) {
  const require = createRequire(join(from, 'package.json'))
  let dir
  try {
    dir = dirname(require.resolve(`${name}/package.json`))
  } catch {
    try {
      dir = dirname(require.resolve(name))
    } catch {
      return
    }
    while (
      !existsSync(join(dir, 'package.json')) ||
      JSON.parse(readFileSync(join(dir, 'package.json'))).name !== name
    ) {
      if (dirname(dir) === dir) throw new Error(`Cannot locate ${name}`)
      dir = dirname(dir)
    }
  }
  const pkg = JSON.parse(readFileSync(join(dir, 'package.json'))),
    key = `${name}@${pkg.version}`
  if (visited.has(key)) return
  visited.add(key)
  const files = readdirSync(dir).filter((f) =>
    /^(licen[cs]e|copying|notice|copyright)([.-]|$)/i.test(f),
  )
  if (
    !files.length &&
    existsSync(join(dir, 'README.md')) &&
    /## License/i.test(readFileSync(join(dir, 'README.md'), 'utf8'))
  )
    files.push('README.md')
  if (!files.length) missing.push(key)
  sections.push(
    `${key} (${pkg.license ?? 'see license below'})\n${files.map((f) => `${f}\n\n${readFileSync(join(dir, f), 'utf8').replace(/\r\n/g, '\n').replace(/[ \t]+$/gm, '')}`).join('\n\n')}`,
  )
  for (const dep of Object.keys(pkg.dependencies ?? {}))
    if (!dep.startsWith('@types/')) visit(dep, dir)
}
visit('maplibre-gl', root)
visit('leaflet', root)
if (missing.length) throw new Error(`Missing license files: ${missing.join(', ')}`)
writeFileSync(
  join(root, 'public/map-licenses.txt'),
  'Photoly map components — third-party licenses\nThese components retain their original licenses. Photoly is AGPL-3.0-or-later.\n\n' +
    sections.join('\n\n' + '='.repeat(80) + '\n\n'),
)
console.log(`Wrote notices for ${visited.size} packages`)
