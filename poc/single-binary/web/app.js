// Browser side of the POC: SigLIP2 runs here (WebGPU or WebAssembly, chosen per model), and
// only 768-number vectors go to the server.
import {
  env, AutoTokenizer, SiglipTextModel, SiglipVisionModel, Tensor,
} from '/vendor/transformers.min.js'
import { prepareImage } from '/preprocess.js'

const $ = (id) => document.getElementById(id)
const MAX_TOKENS = 64

const state = { profile: null, devices: null, adapter: null, vision: null, text: null, timings: {} }

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

function describe({ device, reason }) {
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
  const { profile, database } = await response.json()
  state.profile = profile

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
  $('runtime').textContent = [
    `Images: ${describe(state.devices.image)}`,
    `Text: ${describe(state.devices.text)}`,
    `model ${profile.dtype}`,
    `${env.backends.onnx.wasm.numThreads} CPU threads`,
    `PostgreSQL ${database.postgres}, pgvector ${database.pgvector}`,
  ].join(' · ')
  await showStorage()
}

// ---------- model loading ----------

const progress = new Map()
function onProgress(event) {
  if (event.status !== 'progress' || !event.total) return
  progress.set(event.file, `${event.file}: ${(event.loaded / 1e6).toFixed(0)} / ${(event.total / 1e6).toFixed(0)} MB`)
  $('progress').replaceChildren(...[...progress.values()].map((text) => {
    const line = document.createElement('div')
    line.textContent = text
    return line
  }))
}

function modelOptions(kind) {
  return { device: state.devices[kind].device, dtype: state.profile.dtype, progress_callback: onProgress }
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
      $('load-vision').textContent = `Image model ready (load ${seconds(loadMs)}, warm-up ${seconds(warmMs)})`
      $('load-vision').disabled = true
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
      $('load-text').textContent = `Text model ready (load ${seconds(loadMs)}, warm-up ${seconds(warmMs)})`
      $('load-text').disabled = true
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

async function api(path, options) {
  const response = await fetch(path, options)
  const body = response.status === 204 ? null : await response.json()
  if (!response.ok) throw new Error(body?.error ?? `HTTP ${response.status}`)
  return body
}

async function uploadFiles(files) {
  const status = $('upload-status')
  status.classList.remove('error')
  let done = 0
  let totalMs = 0
  for (const file of files) {
    status.textContent = `Embedding ${file.name} (${done + 1} of ${files.length})…`
    const started = performance.now()
    const embedding = await embedImage(file)
    totalMs += performance.now() - started
    const form = new FormData()
    form.append('file', file)
    form.append('embedding', JSON.stringify(embedding))
    form.append('profile', state.profile.fingerprint)
    await api('/api/photos', { method: 'POST', body: form })
    done++
  }
  status.textContent = `Uploaded ${done} photo(s); on-device embedding took ${(totalMs / done).toFixed(0)} ms per photo (${state.devices.image.device}).`
  await showAll()
}

async function search(query) {
  const embedding = await embedText(query)
  state.lastQueryEmbedding = embedding
  const started = performance.now()
  const hits = await api('/api/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ embedding, profile: state.profile.fingerprint, limit: 12 }),
  })
  const { tokenizeMs, modelMs } = state.timings.lastQuery
  const ms = (x) => `${x.toFixed(0)} ms`
  $('search-status').textContent = `“${query}”: ${hits.length} result(s). Tokenize ${ms(tokenizeMs)}, ` +
    `text model ${ms(modelMs)} (${state.devices.text.device}), server search ${ms(performance.now() - started)}.`
  render(hits)
  return hits
}

async function similar(photo) {
  const hits = await api(`/api/photos/${photo.id}/similar?limit=12`)
  $('search-status').textContent = `Similar to ${photo.name} (stored vectors only; no model needed).`
  render(hits)
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
  state.vision = state.text = null
  for (const id of ['load-vision', 'load-text']) $(id).disabled = false
  $('load-vision').textContent = 'Load image model'
  $('load-text').textContent = 'Load text model'
  progress.clear()
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

$('load-vision').addEventListener('click', () => loadVision().catch(showError($('storage'))))
$('load-text').addEventListener('click', () => loadText().catch(showError($('storage'))))
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

// Exposed for the end-to-end and parity scripts.
window.photoly = { ready: null, state, embedImage, embedText, search, loadVision, loadText }
window.photoly.ready = setup().then(showAll).catch(showError($('runtime')))
