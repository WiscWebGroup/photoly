// End-to-end check of the POC in headless Chromium:
//   0. device choice: a software-emulated GPU must fall back to WebAssembly, and search
//      text uses the CPU unless forced
//   1. on-device image embedding and upload of the AI fixtures
//   2. the 8 fixture queries (English + Chinese) embedded on-device; top hit must match
//   3. the server rejects malformed vectors and wrong profiles
//   4. after closing and reopening the browser, the model loads with zero model downloads
//   With --server, both models are switched to the server instead: the same uploads and
//   queries must pass with no model file ever reaching the browser, and the switch must
//   survive a browser restart.
//
// usage: node e2e/e2e.mjs [--url http://localhost:8084] [--wasm | --force-webgpu | --server] [--chrome /path/to/chrome]
//   default         Chromium offers only its software GPU (SwiftShader); expect WebAssembly
//   --wasm          no WebGPU at all; expect WebAssembly for both models
//   --force-webgpu  software GPU plus ?device=webgpu; both models on WebGPU (slow)
//                         [--dump-text text-vectors.json] [--text-model small|full]
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
const serverMode = args.includes('--server')
const expected = forceWebgpu ? 'webgpu' : 'wasm'
const textModel = option('--text-model', null) // small (int8, the default) or full
const pageURL = new URL(url)
if (forceWebgpu) pageURL.searchParams.set('device', 'webgpu')
if (textModel) pageURL.searchParams.set('textModel', textModel)
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
  await page.goto(pageURL.href)
  await page.evaluate(() => window.photoly.ready)
  const runtime = await page.textContent('#runtime')
  return { context, page, modelRequests, runtime }
}

try {
  // ---- first visit: download model, index, search ----
  let { context, page, modelRequests, runtime } = await open()
  const devices = await page.evaluate(() => window.photoly.state.devices)
  results.device = `image ${devices.image.device}, text ${devices.text.device}`
  if (serverMode) {
    await page.evaluate(() => { window.photoly.setWhere('image', 'server'); window.photoly.setWhere('text', 'server') })
    runtime = await page.textContent('#runtime')
    results.device = 'server'
    check(/Images: server.*Text: server/.test(runtime), `runtime line: ${runtime}`)
  }
  log('runtime', { runtime, devices, crossOriginIsolated: await page.evaluate(() => crossOriginIsolated) })
  if (!serverMode) {
    check(devices.image.device === expected && devices.text.device === expected, `expected ${expected} for both models, got ${results.device}`)
    if (!wasm && !forceWebgpu) check(/software-emulated/.test(devices.image.reason ?? ''), `expected the software GPU to be skipped, got: ${devices.image.reason}`)
    if (!forceWebgpu) check(/short query/.test(devices.text.reason ?? '') || wasm, `expected text on the CPU by choice, got: ${devices.text.reason}`)
  }

  let started = Date.now()
  await page.evaluate(() => Promise.all([window.photoly.load('image'), window.photoly.load('text')]))
  log('first model load', { seconds: (Date.now() - started) / 1000, modelRequests: modelRequests.length,
    timings: await page.evaluate(() => window.photoly.state.timings) })

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
      const timing = await page.evaluate(() => window.photoly.state.timings.lastQuery)
      const top = hits[0]
      const ok = top?.name === sample.file
      correct += ok
      log('search', { query, expected: sample.file, top: top?.name, score: top?.score?.toFixed(4), ms: Date.now() - t,
        modelMs: Math.round(timing.modelMs), tokenizeMs: Math.round(timing.tokenizeMs), ok })
    }
  }
  if (dump && !serverMode) writeFileSync(dump, JSON.stringify(textVectors))
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

  if (serverMode) {
    const status = await page.evaluate(async () => {
      const post = (body) => fetch('/api/search', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }).then((r) => r.status)
      const form = new FormData()
      // A PNG signature followed by junk: passes the type check, fails to decode.
      form.append('file', new Blob([new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 1, 2, 3])]), 'bad.png')
      form.append('embed', 'server')
      return {
        tooLong: await post({ query: 'dog cat '.repeat(40) }),
        notImage: await fetch('/api/photos', { method: 'POST', body: form }).then((r) => r.status),
      }
    })
    log('server validation', status)
    check(status.tooLong === 422 && status.notImage === 422, 'server validation statuses')
    check(modelRequests.length === 0, `server mode downloaded model files: ${modelRequests.join(', ')}`)
  }
  await context.close()

  // ---- browser restart: the model must come from the browser cache ----
  ;({ context, page, modelRequests } = await open())
  if (serverMode) {
    const where = await page.evaluate(() => window.photoly.state.where)
    check(where.image === 'server' && where.text === 'server', `switch not remembered: ${JSON.stringify(where)}`)
  }
  started = Date.now()
  await page.evaluate(() => Promise.all([window.photoly.load('image'), window.photoly.load('text')]))
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
