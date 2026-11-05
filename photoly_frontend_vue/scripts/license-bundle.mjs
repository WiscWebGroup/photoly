import { dirname, join } from 'node:path'
import { createHash } from 'node:crypto'
import { existsSync, readFileSync } from 'node:fs'
import { inventory } from './licenses.mjs'

export function packageForFile(file) {
  if (!file || !file.includes('node_modules')) return null
  file = file.replace(/^\0/, '')
  for (let dir = dirname(file.split('?')[0]); dir !== dirname(dir); dir = dirname(dir)) {
    const path = join(dir, 'package.json')
    if (existsSync(path)) {
      const p = JSON.parse(readFileSync(path, 'utf8'))
      if (p.name && p.version) return `${p.name}@${p.version}`
    }
  }
  throw new Error(`Cannot identify bundled dependency: ${file}`)
}
export function validateBundle(root, files) {
  const records = new Map(inventory(root).packages.map((p) => [`${p.name}@${p.version}`, p]))
  const keys = [...new Set(files.map(packageForFile).filter(Boolean))].sort()
  for (const key of keys) {
    const p = records.get(key)
    if (!p || p.status !== 'notice-collected' || p.license === 'MPL-2.0')
      throw new Error(
        `Bundled package needs license review: ${key} (${p?.license === 'MPL-2.0' ? 'provide corresponding source before bundling MPL code' : (p?.issues.join(', ') ?? 'not inventoried')})`,
      )
  }
  return keys
}
export function licenseBundle(root, kind = 'app') {
  return {
    name: 'photoly-license-bundle',
    generateBundle(_options, bundle) {
      const files = Object.values(bundle).flatMap((x) =>
        x.type === 'chunk' ? Object.keys(x.modules) : [],
      )
      const packages = validateBundle(root, files)
      this.emitFile({
        type: 'asset',
        fileName:
          kind === 'app'
            ? 'legal/bundled-dependencies.json'
            : `legal/worker-${createHash('sha256').update(JSON.stringify(packages)).digest('hex').slice(0, 12)}-dependencies.json`,
        source: JSON.stringify({ packages }, null, 2) + '\n',
      })
    },
  }
}
