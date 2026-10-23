// End-to-end check of the POC in headless Chromium:
//   0. device choice: a software-emulated GPU must fall back to WebAssembly
//   1. on-device image embedding and upload of the AI fixtures
//   2. the 8 fixture queries (English + Chinese) embedded on-device; top hit must match
//   3. the server rejects malformed vectors and wrong profiles
//   4. after closing and reopening the browser, the model loads with zero model downloads
//
// usage: node e2e/e2e.mjs [--url http://localhost:8084] [--wasm | --force-webgpu] [--chrome /path/to/chrome]
//   default         Chromium offers only its software GPU (SwiftShader); expect WebAssembly
//   --wasm          no WebGPU at all; expect WebAssembly
//   --force-webgpu  software GPU plus ?device=webgpu; exercises the WebGPU path (slow)
//                         [--dump-text text-vectors.json]
// Needs the `playwright` package (global install is fine) and a running `photoly serve`
// against an empty database.
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const args = process.argv.slice(2)
const option = (name, fallback) => (args.includes(name) ? args[args.indexOf(name) + 1] : fallback)
const url = option('--url', 'http://localhost:8084')
const wasm = args.includes('--wasm')
const forceWebgpu = args.includes('--force-webgpu')
const expected = forceWebgpu ? 'webgpu' : 'wasm'
const pageURL = forceWebgpu ? `${url}/?device=webgpu` : url
const executablePath = option('--chrome', process.env.CHROME_PATH)
const dump = option('--dump-text', null) // write query -> browser text vector, for scripts/parity.py

const require = createRequire(import.meta.url)
let chromium
try {
  ({ chromium } = require('playwright'))
} catch {
  ({ chromium } = require(join(process.env.NODE_PATH || '/opt/node22/lib/node_modules', 'playwright')))
}

const here = dirname(fileURLToPath(import.meta.url))
const fixtures = join(here, '../../../photoly_backend/tests/fixtures/ai')
const manifest = JSON.parse(readFileSync(join(fixtures, 'manifest.json'), 'utf8'))

// Software WebGPU (SwiftShader) works in headless Chromium on Linux without a GPU.
const gpuArgs = ['--enable-unsafe-webgpu', '--enable-features=Vulkan', '--use-vulkan=swiftshader',
  '--use-webgpu-adapter=swiftshader', '--disable-vulkan-surface']

const profileDir = mkdtempSync(join(tmpdir(), 'photoly-e2e-'))
const results = { device: null, steps: [] }
const log = (step, detail) => { results.steps.push({ step, ...detail }); console.log(step, JSON.stringify(detail)) }
const check = (ok, message) => { if (!ok) throw new Error(message) }

async function open() {
  const context = await chromium.launchPersistentContext(profileDir, {
    headless: true, executablePath, args: wasm ? [] : gpuArgs,
  })
  const page = context.pages()[0] ?? await context.newPage()
  page.on('console', (m) => { if (m.type() === 'error') console.log('browser:', m.text()) })
  const modelRequests = []
  page.on('request', (r) => { if (r.url().includes('/models/')) modelRequests.push(r.url()) })
  await page.goto(pageURL)
  await page.evaluate(() => window.photoly.ready)
  const runtime = await page.textContent('#runtime')
  return { context, page, modelRequests, runtime }
}

try {
  // ---- first visit: download model, index, search ----
  let { context, page, modelRequests, runtime } = await open()
  results.device = await page.evaluate(() => window.photoly.state.device)
  const reason = await page.evaluate(() => window.photoly.state.deviceReason)
  log('runtime', { runtime, reason, crossOriginIsolated: await page.evaluate(() => crossOriginIsolated) })
  check(results.device === expected, `expected ${expected}, got ${results.device}`)
  if (!wasm && !forceWebgpu) check(/software-emulated/.test(reason ?? ''), `expected the software GPU to be skipped, got: ${reason}`)

  let started = Date.now()
  await page.evaluate(() => Promise.all([window.photoly.loadVision(), window.photoly.loadText()]))
  log('first model load', { seconds: (Date.now() - started) / 1000, modelRequests: modelRequests.length })

  started = Date.now()
  await page.setInputFiles('#files', manifest.samples.map((s) => join(fixtures, s.file)))
  await page.waitForFunction(() => /Uploaded|error/i.test(document.querySelector('#upload-status').textContent), null, { timeout: 600_000 })
  const uploadStatus = await page.textContent('#upload-status')
  log('upload', { status: uploadStatus, seconds: (Date.now() - started) / 1000 })
  check(uploadStatus.startsWith(`Uploaded ${manifest.samples.length}`), uploadStatus)

  let correct = 0
  const textVectors = {}
  for (const sample of manifest.samples) {
    for (const query of sample.queries) {
      const t = Date.now()
      const hits = await page.evaluate((q) => window.photoly.search(q), query)
      textVectors[query] = await page.evaluate(() => window.photoly.state.lastQueryEmbedding)
      const top = hits[0]
      const ok = top?.name === sample.file
      correct += ok
      log('search', { query, expected: sample.file, top: top?.name, score: top?.score?.toFixed(4), ms: Date.now() - t, ok })
    }
  }
  if (dump) writeFileSync(dump, JSON.stringify(textVectors))
  const total = manifest.samples.reduce((n, s) => n + s.queries.length, 0)
  log('recall@1', { correct, total })
  check(correct === total, `recall@1 ${correct}/${total}`)

  // Similar photos use stored vectors; the server runs no model.
  const photos = await page.evaluate(() => fetch('/api/photos').then((r) => r.json()))
  const similar = await page.evaluate((id) => fetch(`/api/photos/${id}/similar`).then((r) => r.json()), photos[0].id)
  log('similar', { photo: photos[0].name, results: similar.length })
  check(similar.length === photos.length - 1 && similar.every((h) => h.id !== photos[0].id), 'similar results')

  // ---- the server rejects vectors it cannot trust ----
  const rejections = await page.evaluate(async () => {
    const profile = window.photoly.state.profile.fingerprint
    const post = (embedding, p = profile) => fetch('/api/search', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ embedding, profile: p }),
    }).then((r) => r.status)
    const unit = Array(768).fill(1 / Math.sqrt(768))
    return {
      valid: await post(unit),
      notUnit: await post(Array(768).fill(1)),
      wrongLength: await post(unit.slice(1)),
      wrongProfile: await post(unit, '0'.repeat(64)),
    }
  })
  log('validation', rejections)
  check(rejections.valid === 200 && rejections.notUnit === 422 && rejections.wrongLength === 422 && rejections.wrongProfile === 422, 'validation statuses')
  await context.close()

  // ---- browser restart: the model must come from the browser cache ----
  ;({ context, page, modelRequests } = await open())
  started = Date.now()
  await page.evaluate(() => Promise.all([window.photoly.loadVision(), window.photoly.loadText()]))
  const reloadSeconds = (Date.now() - started) / 1000
  const storage = await page.evaluate(async () => ({ ...(await navigator.storage.estimate()), persisted: await navigator.storage.persisted() }))
  log('reopen', { seconds: reloadSeconds, modelRequests: modelRequests.length, usageMB: Math.round(storage.usage / 1e6) })
  check(modelRequests.length === 0, `model downloaded again: ${modelRequests.join(', ')}`)
  const again = await page.evaluate(() => window.photoly.search('a cup of coffee on a table'))
  check(again[0]?.name === 'coffee.png', 'search after reopen')
  await context.close()
  console.log(`\nPASS (${results.device})`)
} catch (error) {
  console.error(`\nFAIL: ${error.message}`)
  process.exitCode = 1
} finally {
  rmSync(profileDir, { recursive: true, force: true })
}
