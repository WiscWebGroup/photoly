// Conservative installed dependency closure, including peers and build tools.
// Bundled notices are preserved verbatim; manual supplements are version/hash bound.
import {
  existsSync,
  readFileSync,
  readdirSync,
  realpathSync,
  mkdirSync,
  writeFileSync,
} from 'node:fs'
import { dirname, join, resolve, relative } from 'node:path'
import { createHash } from 'node:crypto'
import { fileURLToPath } from 'node:url'

const home = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const supplements = JSON.parse(readFileSync(join(home, 'licenses/supplements.json'), 'utf8'))
const sha = (s) => createHash('sha256').update(s).digest('hex')
const read = (p) => readFileSync(p, 'utf8')
const json = (p) => JSON.parse(read(p))
const allowed =
  /^(MIT|MIT-0|ISC|Apache-2\.0|BSD-[234]-Clause|0BSD|CC0-1\.0|Unlicense|BlueOak-1\.0\.0|MPL-2\.0|CC-BY-4\.0|Python-2\.0|\(MIT OR CC0-1\.0\)|\(MIT OR Apache-2\.0\)|MIT OR Apache-2\.0|\(WTFPL OR MIT\)|\(BSD-2-Clause OR MIT OR Apache-2\.0\))$/
export function locate(name, from) {
  for (let d = from; ; d = dirname(d)) {
    const p = join(d, 'node_modules', name, 'package.json')
    if (existsSync(p)) return dirname(realpathSync(p))
    if (d === dirname(d)) return null
  }
}
function noticeFiles(dir) {
  const out = []
  function scan(d, depth) {
    for (const e of readdirSync(d, { withFileTypes: true })) {
      if (
        e.isFile() &&
        (/(^|[.-])(licen[cs]e|notice|copying|copyright)([.-]|$)/i.test(e.name) ||
          /^ThirdPartyNotices/i.test(e.name))
      )
        out.push(join(d, e.name))
      else if (e.isDirectory() && depth < 2 && /^(dist|licenses?|third.party)$/i.test(e.name))
        scan(join(d, e.name), depth + 1)
    }
  }
  scan(dir, 0)
  return out.sort()
}
export function inventory(root) {
  const seen = new Map(),
    errors = [],
    skipped = [],
    visited = new Set()
  const rootPkg = json(join(root, 'package.json'))
  function visit(name, from, scope, optional = false) {
    const dir = locate(name, from)
    if (!dir) {
      const parent = json(join(from, 'package.json'))
      const bundled = supplements[`${parent.name}@${parent.version}`]?.bundledDependencies?.[name]
      if (bundled) {
        skipped.push({ name, parent: `${parent.name}@${parent.version}`, reason: bundled })
        return
      }
      if (optional) {
        skipped.push({
          name,
          parent: `${parent.name}@${parent.version}`,
          reason: 'optional dependency not installed on this platform',
        })
        return
      }
      errors.push(`Unresolved ${scope} dependency ${name} from ${parent.name}@${parent.version}`)
      return
    }
    const p = json(join(dir, 'package.json')),
      key = `${p.name}@${p.version}`,
      supplement = supplements[key]
    if (visited.has(`${dir}:${scope}`)) return
    visited.add(`${dir}:${scope}`)
    let license = p.license ?? p.licenses ?? 'UNKNOWN'
    if (typeof license !== 'string')
      license = Array.isArray(license) ? license.map((x) => x.type).join(' OR ') : license.type
    const files = noticeFiles(dir).map((f) => ({
      file: relative(dir, f),
      text: read(f),
      source: `npm:${key}/${relative(dir, f)}`,
    }))
    for (const item of supplement?.files ?? []) {
      const text = read(join(home, 'licenses', item.file))
      if (sha(text) !== item.sha256)
        throw new Error(`Supplement checksum mismatch: ${key}/${item.file}`)
      files.push({ file: item.file, text, source: item.source })
    }
    if (!files.length) {
      // Some upstream packages publish the complete grant in README only.
      const r = readdirSync(dir).find((f) => /^readme(\.md|\.markdown|\.txt)?$/i.test(f))
      if (r) {
        const content = read(join(dir, r))
        if (
          /Permission is hereby granted|Permission to use, copy, modify|Redistribution and use in source/.test(
            content,
          )
        )
          files.push({ file: r, text: content, source: `npm:${key}/${r}` })
      }
    }
    if (supplement?.license) license = supplement.license
    const problems = []
    if (!allowed.test(license)) problems.push(`unreviewed license ${license}`)
    if (
      !files.some(
        (f) =>
          f.text.trim().length > 100 &&
          /Permission is hereby granted|Permission to use, copy|Redistribution and use|TERMS AND CONDITIONS|Mozilla Public License|Creative Commons|CREATIVE COMMONS|Blue Oak|BlueOak|PYTHON SOFTWARE FOUNDATION|This is free and unencumbered|DO WHAT THE FUCK/i.test(
            f.text,
          ),
      )
    )
      problems.push('no complete license text')
    if (scope === 'dependency' && problems.length) errors.push(`${key}: ${problems.join(', ')}`)
    const record = seen.get(key) ?? {
      name: p.name,
      version: p.version,
      license,
      source:
        typeof p.repository === 'string'
          ? p.repository
          : (p.repository?.url ??
            p.homepage ??
            `https://www.npmjs.com/package/${p.name}/v/${p.version}`),
      scopes: [],
      status: problems.length ? 'needs-review' : 'notice-collected',
      issues: problems,
      files: files.map((f) => ({ file: f.file, source: f.source, sha256: sha(f.text) })),
      texts: files,
    }
    if (!record.scopes.includes(scope)) record.scopes.push(scope)
    seen.set(key, record)
    for (const n of Object.keys(p.dependencies ?? {}).sort())
      visit(n, dir, scope, n in (p.optionalDependencies ?? {}))
    for (const n of Object.keys(p.optionalDependencies ?? {}).sort())
      if (!(n in (p.dependencies ?? {}))) visit(n, dir, scope, true)
    for (const n of Object.keys(p.peerDependencies ?? {}).sort())
      visit(n, dir, scope, p.peerDependenciesMeta?.[n]?.optional === true)
  }
  for (const n of Object.keys(rootPkg.dependencies ?? {}).sort()) visit(n, root, 'dependency')
  for (const n of Object.keys(rootPkg.devDependencies ?? {}).sort()) visit(n, root, 'build')
  const packages = [...seen.values()].sort((a, b) =>
    `${a.name}@${a.version}`.localeCompare(`${b.name}@${b.version}`, 'en'),
  )
  for (const p of packages) p.scopes.sort()
  return {
    packages,
    errors: [...new Set(errors)].sort(),
    skipped: [...new Map(skipped.map((x) => [JSON.stringify(x), x])).values()].sort((a, b) =>
      JSON.stringify(a).localeCompare(JSON.stringify(b), 'en'),
    ),
  }
}
export function generate(root, output, check = false) {
  const result = inventory(root)
  const unresolved = result.errors.filter((e) => e.startsWith('Unresolved'))
  if (unresolved.length) throw new Error(unresolved.join('\n'))
  const lock = join(root, 'pnpm-lock.yaml')
  const installed = join(root, 'node_modules/.pnpm/lock.yaml')
  if (!existsSync(installed) || sha(read(lock)) !== sha(read(installed)))
    throw new Error('Install with --frozen-lockfile before generating notices')
  const copiedCode = root === home ? json(join(home, 'licenses/copied-code.json')) : []
  const copiedTexts = copiedCode.map((item) => {
    const text = read(join(home, 'licenses', item.file))
    if (sha(text) !== item.sha256)
      throw new Error(`Copied-source notice checksum mismatch: ${item.name}`)
    return `${item.name}\n${item.note}\n${item.source}\n\n${text}`
  })
  const notices =
    'Photoly third-party software notices\nComponents retain their own licenses. Photoly is AGPL-3.0-or-later.\nThis conservative inventory includes installed build tools and dependencies that may be tree-shaken or Node-only.\n\n' +
    result.packages
      .map(
        (p) =>
          `${p.name}@${p.version} (${p.license})\nSource: ${p.source}\n${p.status}\n\n` +
          p.texts.map((f) => `${f.file}\n${f.text.trim()}\n`).join('\n'),
      )
      .join('\n' + '='.repeat(80) + '\n\n')
  const fullNotices = notices + '\n\nCopied source notices\n\n' + copiedTexts.join('\n\n')
  const manifest = {
    copiedCode,
    lockSHA256: sha(read(lock)),
    coverage:
      'installed dependency closure including peers, optional packages present on this platform, and build tools; platform-specific absent packages are not distributed by this build',
    packages: result.packages.map((record) => {
      const p = { ...record }
      delete p.texts
      return p
    }),
    skipped: result.skipped,
  }
  const outputs = {
    'third-party-notices.txt': fullNotices,
    'dependencies.json': JSON.stringify(manifest, null, 2) + '\n',
    'AGPL-3.0.txt': read(join(home, 'licenses/AGPL-3.0.txt')),
  }
  mkdirSync(output, { recursive: true })
  for (const [f, text] of Object.entries(outputs)) {
    const dest = join(output, f)
    if (check) {
      if (!existsSync(dest) || read(dest) !== text)
        throw new Error(`Stale generated notice: ${dest}`)
    } else writeFileSync(dest, text)
  }
  console.log(
    `${root}: ${result.packages.length} packages; ${result.packages.filter((p) => p.status === 'needs-review').length} dependency records require review (bundle validation is separate)`,
  )
  return manifest
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = resolve(process.argv[2] ?? home)
  const output = resolve(process.argv[3] ?? join(root, 'public/legal'))
  generate(root, output, process.argv.includes('--check'))
}
