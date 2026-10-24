// Compares embedding in the browser with embedding on the server: what the user waits for,
// and the memory and CPU each side uses while it happens.
//
// For each configuration it restarts the server container (so no model is loaded), opens a
// fresh browser profile (so nothing is cached), then measures three phases:
//   load    load both models (browser: download + initialise; server: read + initialise)
//   photos  upload the 4 AI fixtures, `--rounds` times (deleting them in between)
//   text    the 8 fixture queries, `--rounds` times
// Browser memory is the summed RSS of every browser process; server memory is the
// container's memory without page cache. CPU is CPU-seconds used during the phase.
//
// usage: node e2e/bench.mjs --url http://localhost:8085 --container NAME --chrome PATH
//            [--rounds 3] [--configs browser,browser-cpu,server] [--out results.json]
// Run it against a server with an empty database that nothing else uses: it deletes
// every photo between rounds.
import { readFileSync, writeFileSync, mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
import { request } from 'node:http'
import { createRequire } from 'node:module'

const args = process.argv.slice(2)
const option = (name, fallback) => (args.includes(name) ? args[args.indexOf(name) + 1] : fallback)
const url = option('--url', 'http://localhost:8085')
const container = option('--container', 'photoly-poc-bench-photoly-1')
const chrome = option('--chrome', process.env.CHROME_PATH)
const rounds = Number(option('--rounds', '3'))
const configs = option('--configs', 'browser,browser-cpu,server').split(',')
const out = option('--out', null)
const dockerSocket = option('--docker-socket', '/var/run/docker.sock')

const { chromium } = createRequire(import.meta.url)('playwright')
const here = dirname(fileURLToPath(import.meta.url))
const fixtures = join(here, '../../../photoly_backend/tests/fixtures/ai')
const manifest = JSON.parse(readFileSync(join(fixtures, 'manifest.json'), 'utf8'))
const files = manifest.samples.map((s) => join(fixtures, s.file))
const queries = manifest.samples.flatMap((s) => s.queries)

// ---------- resource sampling ----------

function dockerGet(path) {
  return new Promise((resolve, reject) => {
    request({ socketPath: dockerSocket, path }, (res) => {
      let body = ''
      res.on('data', (c) => { body += c })
      res.on('end', () => (res.statusCode < 300 ? resolve(JSON.parse(body || 'null')) : reject(new Error(`${path}: ${res.statusCode} ${body}`))))
    }).on('error', reject).end()
  })
}

async function serverSample() {
  const s = await dockerGet(`/containers/${container}/stats?stream=false&one-shot=true`)
  const m = s.memory_stats
  const cache = m.stats?.inactive_file ?? m.stats?.total_inactive_file ?? 0
  return { memMB: (m.usage - cache) / 1e6, cpuS: s.cpu_stats.cpu_usage.total_usage / 1e9 }
}

// Browser processes are the ones started with our unique profile directory.
function cpuSeconds(t) {
  return t.split(':').reduce((acc, part) => acc * 60 + Number(part), 0)
}
function browserSample(profileDir) {
  const lines = execFileSync('ps', ['-A', '-o', 'rss=,time=,args='], { encoding: 'utf8', maxBuffer: 64 << 20 }).split('\n')
  let rss = 0
  let cpu = 0
  for (const line of lines) {
    if (!line.includes(profileDir)) continue
    const [kb, time] = line.trim().split(/\s+/, 2)
    rss += Number(kb) * 1024
    cpu += cpuSeconds(time)
  }
  return { memMB: rss / 1e6, cpuS: cpu }
}

// Runs fn while sampling both sides every 250 ms; returns peak memory and CPU used.
async function measure(profileDir, fn) {
  const startB = browserSample(profileDir)
  const startS = await serverSample()
  const peak = { browser: startB.memMB, server: startS.memMB }
  let running = true
  const sampler = (async () => {
    while (running) {
      const [b, s] = [browserSample(profileDir), await serverSample()]
      peak.browser = Math.max(peak.browser, b.memMB)
      peak.server = Math.max(peak.server, s.memMB)
      await new Promise((r) => setTimeout(r, 250))
    }
  })()
  const started = performance.now()
  const result = await fn()
  const wallMs = performance.now() - started
  running = false
  await sampler
  const endB = browserSample(profileDir)
  const endS = await serverSample()
  return {
    result, wallMs,
    browser: { peakMB: Math.max(peak.browser, endB.memMB), endMB: endB.memMB, cpuS: endB.cpuS - startB.cpuS },
    server: { peakMB: Math.max(peak.server, endS.memMB), endMB: endS.memMB, cpuS: endS.cpuS - startS.cpuS },
  }
}

// ---------- server control ----------

async function restartServer() {
  await new Promise((resolve, reject) => request({ socketPath: dockerSocket, path: `/containers/${container}/restart?t=10`, method: 'POST' },
    (res) => { res.resume(); res.on('end', () => (res.statusCode < 300 ? resolve() : reject(new Error(`restart: ${res.statusCode}`)))) }).on('error', reject).end())
  for (let i = 0; i < 60; i++) {
    try { if ((await fetch(`${url}/healthz`)).ok) return } catch { /* starting */ }
    await new Promise((r) => setTimeout(r, 500))
  }
  throw new Error('server did not come back')
}

async function deleteAllPhotos() {
  for (const p of await (await fetch(`${url}/api/photos`)).json()) await fetch(`${url}/api/photos/${p.id}`, { method: 'DELETE' })
}

// ---------- one configuration ----------

const median = (xs) => { const s = [...xs].sort((a, b) => a - b); return s.length % 2 ? s[(s.length - 1) / 2] : (s[s.length / 2 - 1] + s[s.length / 2]) / 2 }

async function run(config) {
  await deleteAllPhotos()
  await restartServer()
  const profileDir = mkdtempSync(join(tmpdir(), `photoly-bench-${config}-`))
  const context = await chromium.launchPersistentContext(profileDir, { headless: true, executablePath: chrome })
  try {
    const page = context.pages()[0] ?? await context.newPage()
    await page.goto(config === 'browser-cpu' ? `${url}/?device=wasm` : url)
    await page.evaluate(() => window.photoly.ready)
    const where = config === 'server' ? 'server' : 'browser'
    await page.evaluate((w) => { window.photoly.setWhere('image', w); window.photoly.setWhere('text', w) }, where)
    const runtime = await page.textContent('#runtime')
    const idle = { browser: browserSample(profileDir), server: await serverSample() }

    const load = await measure(profileDir, () => page.evaluate(() => Promise.all([window.photoly.load('image'), window.photoly.load('text')])))

    const photoMs = []
    const photos = await measure(profileDir, async () => {
      for (let r = 0; r < rounds; r++) {
        if (r) await deleteAllPhotos()
        await page.evaluate(() => { document.querySelector('#upload-status').textContent = '' })
        const t = performance.now()
        await page.setInputFiles('#files', files)
        await page.waitForFunction(() => /Uploaded|error/i.test(document.querySelector('#upload-status').textContent), null, { timeout: 600_000 })
        photoMs.push((performance.now() - t) / files.length)
      }
    })

    const queryMs = []
    let correct = 0
    const text = await measure(profileDir, async () => {
      for (let r = 0; r < rounds; r++) {
        for (const sample of manifest.samples) {
          for (const q of sample.queries) {
            const { ms, top } = await page.evaluate(async (query) => {
              const t = performance.now()
              const hits = await window.photoly.search(query)
              return { ms: performance.now() - t, top: hits[0]?.name }
            }, q)
            queryMs.push(ms)
            correct += top === sample.file
          }
        }
      }
    })
    const settled = { browser: browserSample(profileDir), server: await serverSample() }
    return {
      config, runtime, idle, settled,
      load: { ms: load.wallMs, browser: load.browser, server: load.server },
      photos: { msPerPhoto: median(photoMs), samples: photoMs, n: files.length * rounds, browser: photos.browser, server: photos.server },
      text: { msPerQuery: median(queryMs), maxMs: Math.max(...queryMs), n: queryMs.length, correct, browser: text.browser, server: text.server },
    }
  } finally {
    await context.close()
    rmSync(profileDir, { recursive: true, force: true })
  }
}

// ---------- report ----------

const fmt = (x, d = 0) => (x ?? 0).toFixed(d)
const results = []
for (const config of configs) {
  console.error(`running ${config}…`)
  const r = await run(config)
  results.push(r)
  console.error(`  ${r.runtime}`)
  console.error(`  load ${fmt(r.load.ms)} ms · photo ${fmt(r.photos.msPerPhoto)} ms · query ${fmt(r.text.msPerQuery)} ms · recall ${r.text.correct}/${r.text.n}`)
}
if (out) writeFileSync(out, JSON.stringify(results, null, 2))

const row = (label, f) => `| ${label} | ${results.map(f).join(' | ')} |`
console.log(`| | ${results.map((r) => r.config).join(' | ')} |`)
console.log(`| --- | ${results.map(() => '---').join(' | ')} |`)
for (const [phase, key, unit] of [['Load models', 'load', 'ms'], ['Photos', 'photos', 'msPerPhoto'], ['Text', 'text', 'msPerQuery']]) {
  console.log(row(`**${phase}**: user waits`, (r) => `${fmt(r[key][unit])} ms`))
  console.log(row(`${phase}: browser peak mem / CPU`, (r) => `${fmt(r[key].browser.peakMB)} MB / ${fmt(r[key].browser.cpuS, 1)} s`))
  console.log(row(`${phase}: server peak mem / CPU`, (r) => `${fmt(r[key].server.peakMB)} MB / ${fmt(r[key].server.cpuS, 1)} s`))
}
console.log(row('Memory held afterwards (browser / server)', (r) => `${fmt(r.settled.browser.memMB)} / ${fmt(r.settled.server.memMB)} MB`))
