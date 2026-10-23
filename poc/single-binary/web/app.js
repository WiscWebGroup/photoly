// Browser side of the POC: SigLIP2 runs here (WebGPU, or WebAssembly as a fallback), and
// only 768-number vectors go to the server.
import {
  env, AutoTokenizer, SiglipTextModel, SiglipVisionModel, Tensor,
} from '/vendor/transformers.min.js'
import { prepareImage } from '/preprocess.js'

const $ = (id) => document.getElementById(id)
const MAX_TOKENS = 64

const state = { profile: null, device: 'wasm', adapter: null, vision: null, text: null }

// ---------- runtime setup ----------

async function detectDevice() {
  if (!('gpu' in navigator)) return { device: 'wasm', reason: 'no WebGPU in this browser' }
  try {
    const adapter = await navigator.gpu.requestAdapter({ powerPreference: 'high-performance' })
    if (!adapter) return { device: 'wasm', reason: 'no WebGPU adapter' }
    return { device: 'webgpu', adapter }
  } catch (error) {
    return { device: 'wasm', reason: String(error) }
  }
}

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

  const found = await detectDevice()
  state.device = found.device
  state.adapter = found.adapter ?? null
  const gpu = found.adapter?.info ? `${found.adapter.info.vendor} ${found.adapter.info.architecture}`.trim() : ''
  const f16 = found.adapter?.features.has('shader-f16')
  if (profile.dtype === 'fp16' && state.device === 'webgpu' && !f16) {
    state.device = 'wasm'
    found.reason = 'GPU lacks shader-f16, needed by the fp16 model'
  }
  $('runtime').textContent = [
    state.device === 'webgpu' ? `WebGPU (${gpu || 'adapter found'})` : `WebAssembly (${found.reason})`,
    `model ${profile.dtype}`,
    `threads ${crossOriginIsolated ? navigator.hardwareConcurrency : 1}`,
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

function modelOptions() {
  return { device: state.device, dtype: state.profile.dtype, progress_callback: onProgress }
}

async function loadVision() {
  if (!state.vision) {
    state.vision = (async () => {
      const started = performance.now()
      const model = await SiglipVisionModel.from_pretrained(state.profile.modelName, modelOptions())
      $('load-vision').textContent = `Image model ready (${((performance.now() - started) / 1000).toFixed(1)} s)`
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
        SiglipTextModel.from_pretrained(state.profile.modelName, modelOptions()),
      ])
      $('load-text').textContent = `Text model ready (${((performance.now() - started) / 1000).toFixed(1)} s)`
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
  const text = query.toLowerCase()
  const length = tokenizer(text).input_ids.dims.at(-1)
  if (length > MAX_TOKENS) throw new Error(`Query is ${length} tokens; the limit is ${MAX_TOKENS}.`)
  const inputs = tokenizer([text], { padding: 'max_length', max_length: MAX_TOKENS, truncation: false })
  const { pooler_output } = await model(inputs)
  return unitVector(pooler_output)
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
  status.textContent = `Uploaded ${done} photo(s); on-device embedding took ${(totalMs / done).toFixed(0)} ms per photo (${state.device}).`
  await showAll()
}

async function search(query) {
  const started = performance.now()
  const embedding = await embedText(query)
  const embedMs = performance.now() - started
  state.lastQueryEmbedding = embedding
  const hits = await api('/api/search', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ embedding, profile: state.profile.fingerprint, limit: 12 }),
  })
  $('search-status').textContent = `“${query}”: text embedded in ${embedMs.toFixed(0)} ms (${state.device}), ${hits.length} result(s).`
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
