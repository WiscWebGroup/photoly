import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync, readFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { inventory } from './licenses.mjs'
import { validateBundle } from './license-bundle.mjs'

test('missing, unknown and upgraded packages cannot enter a bundle', () => {
  const root = mkdtempSync(join(tmpdir(), 'photoly-licenses-'))
  try {
    writeFileSync(
      join(root, 'package.json'),
      JSON.stringify({ name: 'test', dependencies: { fixture: '1.0.0' } }),
    )
    const dir = join(root, 'node_modules/fixture')
    mkdirSync(dir, { recursive: true })
    const pkg = (license, version = '1.0.0') =>
      writeFileSync(
        join(dir, 'package.json'),
        JSON.stringify({ name: 'fixture', version, license }),
      )
    pkg('MIT')
    assert.throws(() => validateBundle(root, [join(dir, 'index.js')]), /license review/)
    writeFileSync(
      join(dir, 'LICENSE'),
      readFileSync(new URL('../licenses/artplayer/LICENSE', import.meta.url), 'utf8'),
    )
    pkg('UNKNOWN')
    assert.throws(() => validateBundle(root, [join(dir, 'index.js')]), /license review/)
    pkg('MIT')
    assert.deepEqual(validateBundle(root, [join(dir, 'index.js')]), ['fixture@1.0.0'])
    rmSync(join(dir, 'LICENSE'))
    pkg('MIT', '2.0.0')
    assert.throws(() => validateBundle(root, [join(dir, 'index.js')]), /fixture@2.0.0/)
    rmSync(dir, { recursive: true })
    assert.match(inventory(root).errors[0], /Unresolved/)
  } finally {
    rmSync(root, { recursive: true, force: true })
  }
})
