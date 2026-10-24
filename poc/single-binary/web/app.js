// Browser side of the POC: SigLIP2 runs here (WebGPU or WebAssembly, chosen per model), and
// only 768-number vectors go to the server. Either model can be switched to the server
// (ONNX Runtime), which then embeds the uploaded photo or the query text itself.
import {
  env, AutoTokenizer, SiglipTextModel, SiglipVisionModel, Tensor,
} from '/vendor/transformers.min.js'
import { prepareImage } from '/preprocess.js'

const $ = (id) => document.getElementById(id)
const MAX_TOKENS = 64

const state = {
  profile: null, server: null, devices: null, adapter: null, vision: null, text: null, timings: {},
  where: { image: 'browser', text: 'browser' }, // or 'server', per model
}

// ---------- device choice ----------

// Software renderers emulate a GPU on the CPU. They are far slower than running the model
// directly on the CPU with WebAssembly, so they never count as a GPU here.
const SOFTWARE_GPU = /swiftshader|llvmpipe|lavapipe|softpipe|microsoft basic render|software/i

function isSoftwareAdapter(adapter) {
  const info = adapter.info ?? {}
  if (info.isFallbackAdapter || adapter.isFallbackAdapter) return true
  return SOFTWARE_GPU.test([info.vendor, info.architecture, info.device, info.description].join(' '))
}

async function detectGpu() {
  if (!('gpu' in navigator)) return { adapter: null, reason: 'no WebGPU in this browser' }
  try {
    const adapter = await navigator.gpu.requestAdapter({ powerPreference: 'high-performance' })
    if (!adapter) return { adapter: null, reason: 'no GPU available' }
    if (isSoftwareAdapter(adapter)) return { adapter, software: true, reason: 'GPU is software-emulated; the CPU is faster' }
    return { adapter, reason: null }
  } catch (error) {
    return { adapter: null, reason: String(error) }
  }
}

// Each model gets its own device:
// - images use the GPU when there is a real one (a 224 x 224 image is the bigger job);
// - search text uses the CPU. One 64-token query is too small for a GPU to pay off: fixed
//   costs per GPU step outweigh the maths, and the 1.1 GB text model stays out of GPU memory.
// URL overrides for comparing: ?device= (both), ?imageDevice=, ?textDevice= with webgpu|wasm.
// Forcing webgpu also allows a software-emulated GPU, for testing.
async function chooseDevices() {
  const params = new URL(location.href).searchParams
  const gpu = await detectGpu()
  const choose = (forced, preferGpu, cpuReason) => {
    if (forced === 'wasm') return { device: 'wasm', reason: 'forced in the URL' }
    if (forced !== 'webgpu' && !preferGpu) return { device: 'wasm', reason: cpuReason }
    if (!gpu.adapter) return { device: 'wasm', reason: gpu.reason }
    if (gpu.software && forced !== 'webgpu') return { device: 'wasm', reason: gpu.reason }
    if (state.profile.dtype === 'fp16' && !gpu.adapter.features.has('shader-f16')) {
      return { device: 'wasm', reason: 'GPU lacks shader-f16, needed by the fp16 model' }
    }
    return { device: 'webgpu', reason: forced === 'webgpu' ? 'forced in the URL' : null }
  }
  const both = params.get('device')
  return {
    gpu,
    image: choose(params.get('imageDevice') ?? both, true, null),
    text: choose(params.get('textDevice') ?? both, false, 'a short query is faster on the CPU'),
  }
}

function describe({ device, reason }, kind) {
  if (state.where[kind] === 'server') return 'server (ONNX Runtime, CPU)'
  if (device === 'webgpu') {
    const info = state.adapter?.info
    const gpu = info ? `${info.vendor} ${info.architecture}`.trim() : ''
    return `WebGPU${gpu ? ` (${gpu})` : ''}`
  }
  return `CPU, WebAssembly${reason ? ` (${reason})` : ''}`
}

// ---------- runtime setup ----------

async function setup() {
  const response = await fetch('/api/profile')
  const { profile, database, server } = await response.json()
  state.profile = profile
  state.server = server
  state.database = database

  // Load model files from this server only, and cache them in the browser.
  env.allowRemoteModels = false
  env.allowLocalModels = true
  env.localModelPath = profile.modelBaseUrl
  env.useBrowserCache = true
  // Engine files come from the binary too, not a CDN.
  env.backends.onnx.wasm.wasmPaths = {
    mjs: new URL('/vendor/ort-wasm-simd-threaded.asyncify.mjs', location.href).href,
    wasm: new URL('/vendor/ort-wasm-simd-threaded.asyncify.wasm', location.href).href,
  }
  // ONNX Runtime Web defaults to half the cores; using all of them roughly halved image
  // embedding time in testing (846 ms -> 492 ms on 4 cores). Threads need cross-origin
  // isolation, which the server provides.
  env.backends.onnx.wasm.numThreads = crossOriginIsolated ? Math.max(1, navigator.hardwareConcurrency || 1) : 1

  const choice = await chooseDevices()
  state.adapter = choice.gpu.adapter
  state.devices = { image: choice.image, text: choice.text }
  setupTextModel()
  setupSwitch()
  await showStorage()
  try {
    const note = sessionStorage.getItem(RELOAD_NOTE)
    sessionStorage.removeItem(RELOAD_NOTE)
    if (note) $('storage').textContent = note
  } catch { /* no note */ }
}

function showRuntime() {
  $('runtime').textContent = [
    `Images: ${describe(state.devices.image, 'image')}`,
    `Text: ${describe(state.devices.text, 'text')}${state.where.text === 'browser' ? ` (${textDtype()})` : ''}`,
    `model ${state.profile.dtype}`,
    `${env.backends.onnx.wasm.numThreads} CPU threads`,
    `PostgreSQL ${state.database.postgres}, pgvector ${state.database.pgvector}`,
  ].join(' · ')
}

// ---------- browser / server switch ----------

const SWITCH_KEY = 'photoly.where'

function setupSwitch() {
  const { available, reason, models } = state.server
  const free = state.server.availableMB ? ` It has about ${(state.server.availableMB / 1000).toFixed(1)} GB of memory free.` : ''
  $('server-ai').textContent = available
    ? `The server can run the model too (ONNX Runtime on its CPU, in a separate worker process). It loads each model when you switch to it or on first use.${free}`
    : `Server inference is off: ${reason}`
  let saved = {}
  try { saved = JSON.parse(localStorage.getItem(SWITCH_KEY)) ?? {} } catch { /* private mode etc. */ }
  for (const kind of ['image', 'text']) {
    const select = $(`where-${kind}`)
    select.querySelector('option[value=server]').disabled = !available
    select.addEventListener('change', () => switchModel(kind, select.value).catch(showError($('storage'))))
    if (models?.[kind]?.loaded) serverLoaded(kind, models[kind])
    setWhere(kind, available && saved[kind] === 'server' ? 'server' : 'browser', false)
  }
}

// A user moves a model between this browser and the server: start it where it goes, then
// free it where it was.
async function switchModel(kind, where) {
  const from = state.where[kind]
  setWhere(kind, where)
  $('storage').classList.remove('error')
  if (where === 'server') {
    // Load right away, so a memory warning comes now rather than mid-upload. If the user
    // declines it, this throws and the browser copy stays.
    await load(kind)
    if (state.where[kind] === 'server') await freeBrowserModel(kind)
  } else if (from === 'server') {
    await unloadOnServer(kind)
  }
}

// Frees a model loaded in this browser. Disposing it releases its GPU memory, but
// WebAssembly memory never shrinks (3.6 GB stayed after disposing both models in
// testing), so the rest only comes back with a page reload. With no model left in the
// browser, reload now; otherwise offer it.
async function freeBrowserModel(kind) {
  const slot = kind === 'image' ? 'vision' : 'text'
  if (!state[slot]) return
  const loaded = await state[slot].catch(() => null)
  state[slot] = null
  state.ready = { ...state.ready, [kind]: undefined }
  await loaded?.model.dispose()
  if (!state.vision && !state.text) {
    try { sessionStorage.setItem(RELOAD_NOTE, `${modelLabels[kind]} moved to the server; the page reloaded to give its browser memory back.`) } catch { /* no note */ }
    location.reload()
    return
  }
  $('storage').textContent = `${modelLabels[kind]} moved to the server. Its browser memory comes back when the page reloads.`
  $('reload').hidden = false
}

async function unloadOnServer(kind) {
  const { workerMBBefore, workerMBAfter } = await api(`/api/ai/${kind}/unload`, { method: 'POST' })
  state.serverModels = { ...state.serverModels, [kind]: undefined }
  showLoadButton(kind)
  const stopped = ' and stopped its AI worker, which held no other model, so all of its memory is back'
  $('storage').textContent = `${modelLabels[kind]} moved to this browser. The server unloaded it` +
    (workerMBAfter ? ` (AI worker memory, RAM and swap: ${workerMBBefore} → ${workerMBAfter} MB).` : `${stopped}${workerMBBefore ? ` (${workerMBBefore} MB)` : ''}.`)
}

const RELOAD_NOTE = 'photoly.reloadNote'

function setWhere(kind, where, save = true) {
  if (where === 'server' && !state.server.available) throw new Error(`Server inference is off: ${state.server.reason}`)
  state.where[kind] = where
  $(`where-${kind}`).value = where
  if (save) {
    try { localStorage.setItem(SWITCH_KEY, JSON.stringify(state.where)) } catch { /* not remembered */ }
  }
  showLoadButton(kind)
  showRuntime()
}

const loadButtons = { image: 'load-vision', text: 'load-text' }
const modelLabels = { image: 'Image model', text: 'Text model' }

// The load button acts on wherever the model runs now, and says whether it is ready there.
function showLoadButton(kind) {
  const button = $(loadButtons[kind])
  const ready = state.where[kind] === 'server' ? state.serverModels?.[kind] : state.ready?.[kind]
  button.disabled = Boolean(ready)
  button.textContent = ready ?? `Load ${modelLabels[kind].toLowerCase()}${state.where[kind] === 'server' ? ' on the server' : ''}`
}

function serverLoaded(kind, { loadMs, warmMs, variant }) {
  const smaller = variant === 'int8' ? ', smaller int8 model to save memory: search is slightly less precise' : ''
  state.serverModels = { ...state.serverModels, [kind]: `${modelLabels[kind]} ready on the server (load ${seconds(loadMs)}, warm-up ${seconds(warmMs)}${smaller})` }
  state.timings[`server-${kind}`] = { loadMs, warmMs, variant }
}

async function loadOnServer(kind, force = false) {
  const button = $(loadButtons[kind])
  button.disabled = true
  button.textContent = `Loading ${modelLabels[kind].toLowerCase()} on the server…`
  try {
    serverLoaded(kind, await api(`/api/ai/${kind}/load${force ? '?force=1' : ''}`, { method: 'POST' }))
  } finally {
    showLoadButton(kind)
  }
}

// Runs a server-side request for a model and handles the two ways memory can get in the
// way: the server says the model may not fit (ask the user, then load it anyway or stay
// in the browser), or the AI worker was killed (fall back to the browser).
async function withServerModel(kind, run) {
  try {
    return await run()
  } catch (error) {
    if (error.code !== 'low-memory') throw serverFailure(kind, error)
    const { needMB, availableMB } = error.body
    const ok = confirm(`The server may not have enough memory for the ${modelLabels[kind].toLowerCase()}: ` +
      `it needs about ${needMB} MB and about ${availableMB} MB is free.\n\n` +
      'Load it on the server anyway? If memory runs out, only the AI worker stops (photos and search keep ' +
      `working) and the ${modelLabels[kind].toLowerCase()} moves back to this browser.\n\n` +
      'Cancel keeps it in this browser.')
    if (!ok) {
      setWhere(kind, 'browser')
      throw new Error(`${modelLabels[kind]} stays in this browser: the server is short of memory.`)
    }
    try {
      await loadOnServer(kind, true)
      return await run()
    } catch (forced) {
      throw serverFailure(kind, forced)
    }
  }
}

// If the AI worker died (most likely out of memory), move this model back to the browser.
function serverFailure(kind, error) {
  if (error.code !== 'worker-died') return error
  state.serverModels = null // the worker took both models with it
  for (const k of ['image', 'text']) showLoadButton(k)
  setWhere(kind, 'browser')
  return new Error(`${error.message}. The ${modelLabels[kind].toLowerCase()} now runs in this browser; try again.`)
}

function load(kind) {
  if (state.where[kind] === 'server') return withServerModel(kind, () => loadOnServer(kind))
  return kind === 'image' ? loadVision() : loadText()
}

// ---------- model loading ----------

// transformers.js reports reading a cached file the same way as downloading it, so look in
// the cache once per file to say which one is happening.
const progress = new Map()
const source = new Map()
function onProgress(event) {
  if (event.status !== 'progress' || !event.total) return
  const { file } = event
  if (!source.has(file)) {
    source.set(file, 'Loading')
    const url = new URL(`${state.profile.modelBaseUrl}${state.profile.modelName}/${file}`, location.href).href
    caches.open(env.cacheKey).then((cache) => cache.match(url)).then((hit) => {
      source.set(file, hit ? 'Loading from this device' : 'Downloading')
    }).catch(() => {})
  }
  progress.set(file, () => `${source.get(file)} ${file}: ${(event.loaded / 1e6).toFixed(0)} / ${(event.total / 1e6).toFixed(0)} MB`)
  $('progress').replaceChildren(...[...progress.values()].map((text) => {
    text = text()
    const line = document.createElement('div')
    line.textContent = text
    return line
  }))
}

function modelOptions(kind) {
  return { device: state.devices[kind].device, dtype: kind === 'text' ? textDtype() : state.profile.dtype, progress_callback: onProgress }
}

// ---------- browser text model: small (int8) or full ----------
// The int8 text model is a quarter of the download (318 MB vs 1.13 GB) and of the page
// memory (0.9 GB vs 3.2 GB) at the same speed. Its query vectors agree with fp32 at cosine
// 0.94-0.98 and found the right photo for all 8 test queries, with smaller leads over the
// runner-up. Query vectors are never stored, so switching needs no re-indexing.
// ponytail: int8 runs on WebAssembly only; a text model forced onto WebGPU uses full.
const TEXT_MODEL_KEY = 'photoly.textModel'

function textDtype() {
  return state.textModel === 'small' && state.devices.text.device === 'wasm' ? 'int8' : state.profile.dtype
}

function setupTextModel() {
  let saved = null
  try { saved = localStorage.getItem(TEXT_MODEL_KEY) } catch { /* not remembered */ }
  const forced = new URL(location.href).searchParams.get('textModel')
  state.textModel = ['small', 'full'].includes(forced) ? forced : saved === 'full' ? 'full' : 'small'
  $('text-model').value = state.textModel
  $('text-model').addEventListener('change', () => setTextModel($('text-model').value).catch(showError($('storage'))))
}

async function setTextModel(choice) {
  state.textModel = choice
  try { localStorage.setItem(TEXT_MODEL_KEY, choice) } catch { /* not remembered */ }
  showRuntime()
  if (!state.text) return
  // Swap on next use. Disposing frees the sessions, but WebAssembly memory only comes back
  // with a reload.
  const loaded = await state.text.catch(() => null)
  state.text = null
  state.ready = { ...state.ready, text: undefined }
  showLoadButton('text')
  await loaded?.model.dispose()
  $('storage').textContent = `The ${choice} text model loads on the next search. Reload to give back the previous one's browser memory.`
  $('reload').hidden = false
}

// The first run compiles GPU shaders and sets up kernels, which can cost more than the run
// itself. Doing it right after loading keeps that cost off the first real photo or search.
async function warmUp(run) {
  const started = performance.now()
  await run()
  return performance.now() - started
}

function seconds(ms) { return `${(ms / 1000).toFixed(1)} s` }

async function loadVision() {
  if (!state.vision) {
    state.vision = (async () => {
      const started = performance.now()
      const model = await SiglipVisionModel.from_pretrained(state.profile.modelName, modelOptions('image'))
      const loadMs = performance.now() - started
      const warmMs = await warmUp(() => model({ pixel_values: new Tensor('float32', new Float32Array(3 * 224 * 224), [1, 3, 224, 224]) }))
      state.timings.image = { loadMs, warmMs }
      state.ready = { ...state.ready, image: `Image model ready in this browser (load ${seconds(loadMs)}, warm-up ${seconds(warmMs)})` }
      showLoadButton('image')
      showStorage()
      return { model }
    })()
    state.vision.catch(() => { state.vision = null })
  }
  return state.vision
}

async function loadText() {
  if (!state.text) {
    state.text = (async () => {
      const started = performance.now()
      const [tokenizer, model] = await Promise.all([
        AutoTokenizer.from_pretrained(state.profile.modelName),
        SiglipTextModel.from_pretrained(state.profile.modelName, modelOptions('text')),
      ])
      const loadMs = performance.now() - started
      const warmMs = await warmUp(() => model(tokenizer(['warm up'], { padding: 'max_length', max_length: MAX_TOKENS })))
      state.timings.text = { loadMs, warmMs }
      state.ready = { ...state.ready, text: `Text model ready in this browser (load ${seconds(loadMs)}, warm-up ${seconds(warmMs)})` }
      showLoadButton('text')
      showStorage()
      return { tokenizer, model }
    })()
    state.text.catch(() => { state.text = null })
  }
  return state.text
}

// ---------- embeddings ----------

function unitVector(tensor) {
  const data = Array.from(tensor.type === 'float32' ? tensor.data : tensor.to('float32').data)
  const norm = Math.hypot(...data)
  return data.map((x) => x / norm)
}

async function embedImage(blob) {
  const { model } = await loadVision()
  const pixels = await prepareImage(blob)
  const { pooler_output } = await model({ pixel_values: new Tensor('float32', pixels, [1, 3, 224, 224]) })
  return unitVector(pooler_output)
}

async function embedText(query) {
  const { tokenizer, model } = await loadText()
  const t0 = performance.now()
  const text = query.toLowerCase()
  const length = tokenizer(text).input_ids.dims.at(-1)
  if (length > MAX_TOKENS) throw new Error(`Query is ${length} tokens; the limit is ${MAX_TOKENS}.`)
  const inputs = tokenizer([text], { padding: 'max_length', max_length: MAX_TOKENS, truncation: false })
  const t1 = performance.now()
  const { pooler_output } = await model(inputs)
  const vector = unitVector(pooler_output)
  state.timings.lastQuery = { tokenizeMs: t1 - t0, modelMs: performance.now() - t1 }
  return vector
}

// ---------- API ----------

// api returns the JSON body; apiTimed also returns the server's Server-Timing entries in ms.
async function apiTimed(path, options) {
  const response = await fetch(path, options)
  const body = response.status === 204 ? null : await response.json()
  if (!response.ok) throw Object.assign(new Error(body?.error ?? `HTTP ${response.status}`), { status: response.status, code: body?.code, body })
  const timing = {}
  for (const entry of (response.headers.get('Server-Timing') ?? '').split(',')) {
    const [name, ...params] = entry.trim().split(';')
    const dur = params.find((p) => p.trim().startsWith('dur='))
    if (name && dur) timing[name] = Number(dur.trim().slice(4))
  }
  return { body, timing }
}

async function api(path, options) {
  return (await apiTimed(path, options)).body
}

async function uploadFiles(files) {
  const status = $('upload-status')
  status.classList.remove('error')
  const onServer = state.where.image === 'server'
  let done = 0
  let totalMs = 0
  for (const file of files) {
    status.textContent = `Embedding ${file.name} (${done + 1} of ${files.length})…`
    const form = new FormData()
    form.append('file', file)
    if (onServer) {
      form.append('embed', 'server')
      const { timing } = await withServerModel('image', () => apiTimed('/api/photos', { method: 'POST', body: form }))
      totalMs += timing.embed
    } else {
      const started = performance.now()
      const embedding = await embedImage(file)
      totalMs += performance.now() - started
      form.append('embedding', JSON.stringify(embedding))
      form.append('profile', state.profile.fingerprint)
      await api('/api/photos', { method: 'POST', body: form })
    }
    done++
  }
  const where = onServer ? 'server' : `on-device (${state.devices.image.device})`
  status.textContent = `Uploaded ${done} photo(s); ${where} embedding took ${(totalMs / done).toFixed(0)} ms per photo.`
  await showAll()
}

async function search(query) {
  const onServer = state.where.text === 'server'
  let request = { query, limit: 12 }
  state.lastQueryEmbedding = null
  if (!onServer) {
    const embedding = await embedText(query)
    state.lastQueryEmbedding = embedding
    request = { embedding, profile: state.profile.fingerprint, limit: 12 }
  }
  const started = performance.now()
  const post = () => apiTimed('/api/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  })
  const { body: hits, timing } = onServer ? await withServerModel('text', post) : await post()
  if (onServer) state.timings.lastQuery = { tokenizeMs: timing.tokenize, modelMs: timing.model }
  const { tokenizeMs, modelMs } = state.timings.lastQuery
  const ms = (x) => `${x.toFixed(0)} ms`
  $('search-status').textContent = `“${query}”: ${hits.length} result(s). Tokenize ${ms(tokenizeMs)}, ` +
    `text model ${ms(modelMs)} (${onServer ? 'server' : state.devices.text.device}), ` +
    `search ${ms(timing.search)} (round trip ${ms(performance.now() - started)}).`
  render(hits)
  return hits
}

async function similar(photo) {
  const hits = await api(`/api/photos/${photo.id}/similar?limit=12`)
  $('search-status').textContent = `Similar to ${photo.name} (stored vectors only; no model needed).`
  render(hits)
}

async function deleteAll() {
  const photos = await api('/api/photos')
  if (!photos.length || !confirm(`Delete all ${photos.length} photo(s) and their vectors? You can then upload them again.`)) return
  const { deleted } = await api('/api/photos', { method: 'DELETE' })
  await showAll()
  $('upload-status').classList.remove('error')
  $('upload-status').textContent = `Deleted ${deleted} photo(s).`
}

async function showAll() {
  const photos = await api('/api/photos')
  $('search-status').textContent = `${photos.length} photo(s).`
  render(photos)
}

function render(items) {
  $('grid').replaceChildren(...items.map((photo) => {
    const card = document.createElement('article')
    card.className = 'card'
    card.title = 'Show similar photos'
    const img = document.createElement('img')
    img.src = `/api/photos/${photo.id}/file`
    img.alt = photo.name
    img.loading = 'lazy'
    const caption = document.createElement('div')
    caption.textContent = photo.name
    if (photo.score !== undefined) {
      const score = document.createElement('span')
      score.className = 'score'
      score.textContent = ` ${photo.score.toFixed(3)}`
      caption.append(score)
    } else if (!photo.indexed) {
      caption.append(' (not indexed)')
    }
    card.append(img, caption)
    card.addEventListener('click', () => similar(photo).catch(showError($('search-status'))))
    return card
  }))
}

// ---------- storage ----------

async function showStorage() {
  if (!navigator.storage?.estimate) return
  const [{ usage, quota }, persisted] = await Promise.all([navigator.storage.estimate(), navigator.storage.persisted()])
  $('storage').textContent = `Browser storage for this site: ${(usage / 1e6).toFixed(0)} MB used of ${(quota / 1e9).toFixed(1)} GB. ` +
    (persisted ? 'Kept on this device (persistent).' : 'The browser may clear it when space runs low.')
}

async function persist() {
  const granted = await navigator.storage?.persist?.()
  await showStorage()
  if (!granted) $('storage').textContent += ' Persistence was not granted; installing the app usually allows it.'
}

async function clearCache() {
  await caches.delete(env.cacheKey)
  state.vision = state.text = state.ready = null
  showLoadButton('image')
  showLoadButton('text')
  progress.clear()
  source.clear()
  $('progress').replaceChildren()
  await showStorage()
}

// ---------- wiring ----------

function showError(target) {
  return (error) => {
    console.error(error)
    target.textContent = error.message
    target.classList.add('error')
  }
}

$('load-vision').addEventListener('click', () => load('image').catch(showError($('storage'))))
$('load-text').addEventListener('click', () => load('text').catch(showError($('storage'))))
$('persist').addEventListener('click', () => persist().catch(showError($('storage'))))
$('clear-cache').addEventListener('click', () => clearCache().catch(showError($('storage'))))
$('files').addEventListener('change', (event) => {
  const files = [...event.target.files]
  event.target.value = ''
  if (files.length) uploadFiles(files).catch(showError($('upload-status')))
})
$('search-form').addEventListener('submit', (event) => {
  event.preventDefault()
  $('search-status').classList.remove('error')
  search($('query').value.trim()).catch(showError($('search-status')))
})
$('show-all').addEventListener('click', () => showAll().catch(showError($('search-status'))))
$('reload').addEventListener('click', () => location.reload())
$('delete-all').addEventListener('click', () => deleteAll().catch(showError($('upload-status'))))

// Exposed for the end-to-end and parity scripts.
window.photoly = { ready: null, state, embedImage, embedText, search, loadVision, loadText, load, setWhere }
window.photoly.ready = setup().then(showAll).catch(showError($('runtime')))
