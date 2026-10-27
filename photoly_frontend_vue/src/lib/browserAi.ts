// Browser side of SigLIP2: transformers.js + ONNX Runtime Web run the image and text towers on
// WebGPU or WebAssembly, and only 768-number vectors go to the server. Model files come from the Photoly server (never a CDN) and are
// kept in the Cache API so a second visit downloads nothing.
import { reactive } from 'vue'
import type { AiModelKind, BrowserModelInfo, BrowserTextModel } from './ai'
import { prepareImage } from './browserPreprocess'
import { ortWasmPaths } from './ortAssets'

export const MAX_TOKENS = 64
export const TEXT_MODEL_KEY = 'photoly.textModel'
export const RELOAD_NOTE_KEY = 'photoly.reloadNote'

export type Device = 'webgpu' | 'wasm'
export type LoadSource = 'download' | 'device' | 'checking'

// ---------- pure helpers (unit-tested) ----------

// Software renderers emulate a GPU on the CPU: slower than WebAssembly, so never a GPU here.
const SOFTWARE_GPU = /swiftshader|llvmpipe|lavapipe|softpipe|microsoft basic render|software/i

interface GpuAdapterLike {
  info?: { vendor?: string; architecture?: string; device?: string; description?: string }
  isFallbackAdapter?: boolean
  features?: { has(name: string): boolean }
}

export function isSoftwareAdapter(adapter: GpuAdapterLike): boolean {
  const info = adapter.info ?? {}
  if ((info as { isFallbackAdapter?: boolean }).isFallbackAdapter || adapter.isFallbackAdapter)
    return true
  return SOFTWARE_GPU.test(
    [info.vendor, info.architecture, info.device, info.description].join(' '),
  )
}

export interface GpuInfo {
  adapter: GpuAdapterLike | null
  software?: boolean
  reason: string | null
  name?: string
}

export async function detectGpu(
  nav: { gpu?: { requestAdapter(o?: unknown): Promise<unknown> } } = navigator as never,
): Promise<GpuInfo> {
  if (!nav.gpu) return { adapter: null, reason: 'no WebGPU in this browser' }
  try {
    const adapter = (await nav.gpu.requestAdapter({
      powerPreference: 'high-performance',
    })) as GpuAdapterLike | null
    if (!adapter) return { adapter: null, reason: 'no GPU available' }
    const info = adapter.info
    const name = info ? `${info.vendor ?? ''} ${info.architecture ?? ''}`.trim() : ''
    if (isSoftwareAdapter(adapter))
      return {
        adapter,
        software: true,
        name,
        reason: 'GPU is software-emulated; the CPU is faster',
      }
    return { adapter, name, reason: null }
  } catch (error) {
    return { adapter: null, reason: String(error) }
  }
}

/**
 * Each tower gets its own device. Images use the GPU when there is a real one; search text uses
 * the CPU, since one 64-token query is too small for a GPU to pay off and the text model would
 * only take GPU memory.
 */
export function chooseDevice(
  kind: AiModelKind,
  gpu: GpuInfo,
  dtype: string,
): { device: Device; reason: string | null } {
  if (kind === 'text') return { device: 'wasm', reason: 'a short query is faster on the CPU' }
  if (!gpu.adapter || gpu.software) return { device: 'wasm', reason: gpu.reason }
  if (dtype === 'fp16' && !gpu.adapter.features?.has('shader-f16'))
    return { device: 'wasm', reason: 'GPU lacks shader-f16, needed by the fp16 model' }
  return { device: 'webgpu', reason: null }
}

/** The small text model is int8 (a quarter of the download and memory); it runs on WebAssembly. */
export function textDtype(textModel: BrowserTextModel, device: Device, fullDtype: string) {
  return textModel === 'small' && device === 'wasm' ? 'int8' : fullDtype
}

/** WebAssembly threads need cross-origin isolation; without it run one thread. */
export function threadCount(isolated: boolean, cores: number | undefined) {
  return isolated ? Math.max(1, cores || 1) : 1
}

export function unitVector(data: ArrayLike<number>): number[] {
  const out = Array.from(data)
  let sum = 0
  for (const x of out) sum += x * x
  const norm = Math.sqrt(sum)
  return norm ? out.map((x) => x / norm) : out
}

/**
 * transformers.js reports reading a cached file the same way as downloading it, so look in the
 * cache once per file to tell "Downloading" from "Loading from this device".
 */
export interface ProgressSummary {
  source: LoadSource
  loadedMB: number
  totalMB: number
  percent: number
}
export function createProgressTracker(inCache: (file: string) => Promise<boolean>) {
  const files = new Map<string, { loaded: number; total: number; source: LoadSource }>()
  let onChange: (summary: ProgressSummary) => void = () => {}
  const summary = (): ProgressSummary => {
    let loaded = 0
    let total = 0
    let download = false
    let checking = false
    for (const f of files.values()) {
      loaded += f.loaded
      total += f.total
      if (f.source === 'download') download = true
      if (f.source === 'checking') checking = true
    }
    return {
      source: download ? 'download' : checking ? 'checking' : 'device',
      loadedMB: loaded / 1e6,
      totalMB: total / 1e6,
      percent: total ? Math.min(100, Math.round((loaded / total) * 100)) : 0,
    }
  }
  return {
    subscribe(fn: (summary: ProgressSummary) => void) {
      onChange = fn
    },
    reset() {
      files.clear()
    },
    summary,
    onProgress(event: { status?: string; file?: string; loaded?: number; total?: number }) {
      if (event.status !== 'progress' || !event.total || !event.file) return
      const { file } = event
      let entry = files.get(file)
      if (!entry) {
        entry = { loaded: 0, total: 0, source: 'checking' }
        files.set(file, entry)
        const created = entry
        inCache(file)
          .then((hit) => {
            created.source = hit ? 'device' : 'download'
          })
          .catch(() => {
            created.source = 'download'
          })
          .finally(() => onChange(summary()))
      }
      entry.loaded = event.loaded ?? 0
      entry.total = event.total
      onChange(summary())
    },
  }
}

/** Which towers have a model file in the cache, judged by file name (`text_model_int8.onnx`). */
export function cachedTowers(urls: string[], textDtypeName: string, imageDtype: string) {
  const has = (tower: string, dtype: string) => {
    const suffix = dtype === 'fp32' ? '' : `_${dtype}`
    return urls.some((u) => u.split('?')[0]!.endsWith(`/${tower}${suffix}.onnx`))
  }
  return { text: has('text_model', textDtypeName), image: has('vision_model', imageDtype) }
}

// ---------- runtime ----------

interface TensorLike {
  type: string
  data: ArrayLike<number>
  to(type: string): TensorLike
}
interface LoadedModel {
  (inputs: unknown): Promise<{ pooler_output: TensorLike }>
  dispose(): Promise<void>
}
type Tokenizer = ((input: unknown, options?: unknown) => { input_ids: { dims: number[] } }) &
  Record<string, unknown>
export interface Runtime {
  env: {
    allowRemoteModels: boolean
    allowLocalModels: boolean
    localModelPath: string
    useBrowserCache: boolean
    cacheKey: string
    backends: { onnx: { wasm: { wasmPaths?: unknown; numThreads?: number } } }
  }
  AutoTokenizer: { from_pretrained(name: string): Promise<Tokenizer> }
  SiglipTextModel: { from_pretrained(name: string, options: unknown): Promise<LoadedModel> }
  SiglipVisionModel: { from_pretrained(name: string, options: unknown): Promise<LoadedModel> }
  Tensor: new (type: string, data: Float32Array, dims: number[]) => unknown
}

export interface BrowserAiConfig {
  fingerprint: string
  modelBaseUrl: string
  modelName: string
  dtype: string
  /** Admin default for the text tower. */
  textModel: BrowserTextModel
}

export function configFromCapabilities(
  browser: BrowserModelInfo | undefined,
  modelBaseUrl: string | undefined,
): BrowserAiConfig | null {
  if (!browser?.fingerprint || !browser.modelName) return null
  return {
    fingerprint: browser.fingerprint,
    modelBaseUrl: modelBaseUrl ?? '/models/',
    modelName: browser.modelName,
    dtype: browser.dtype || 'fp32',
    textModel: browser.textModel ?? 'small',
  }
}

export interface BrowserAiOptions {
  loadRuntime?: () => Promise<Runtime>
  /** Test hooks. */
  now?: () => number
  nav?: Parameters<typeof detectGpu>[0]
  isolated?: () => boolean
  cores?: () => number
  cacheApi?: () => CacheStorage | undefined
  prepare?: (blob: Blob) => Promise<Float32Array>
  reload?: () => void
}

async function defaultRuntime(): Promise<Runtime> {
  const mod = (await import('@huggingface/transformers')) as unknown as Runtime
  return mod
}

export function createBrowserAi(options: BrowserAiOptions = {}) {
  const loadRuntime = options.loadRuntime ?? defaultRuntime
  const now = options.now ?? (() => performance.now())
  const isolated = options.isolated ?? (() => globalThis.crossOriginIsolated === true)
  const cores = options.cores ?? (() => navigator.hardwareConcurrency)
  const cacheApi = options.cacheApi ?? (() => globalThis.caches)
  const prepare = options.prepare ?? prepareImage
  const reload = options.reload ?? (() => location.reload())

  const state = reactive({
    configured: false,
    phase: 'idle' as 'idle' | 'loading' | 'ready' | 'error',
    kind: null as AiModelKind | null,
    source: 'checking' as LoadSource,
    loadedMB: 0,
    totalMB: 0,
    percent: 0,
    error: '',
    ready: { image: false, text: false },
    gpuName: '',
    imageDevice: 'wasm' as Device,
    threads: 1,
    textModel: 'small' as BrowserTextModel,
    lastQueryMs: null as number | null,
    needsReload: false,
  })

  let config: BrowserAiConfig | null = null
  let runtime: Promise<Runtime> | undefined
  let gpu: Promise<GpuInfo> | undefined
  let vision: Promise<{ model: LoadedModel }> | undefined
  let text: Promise<{ model: LoadedModel; tokenizer: Tokenizer }> | undefined
  let loadedTextModel: BrowserTextModel | undefined

  const tracker = createProgressTracker(async (file) => {
    const caches = cacheApi()
    if (!caches || !config) return false
    const rt = await runtime
    const url = new URL(
      `${config.modelBaseUrl.replace(/\/?$/, '/')}${config.modelName}/${file}`,
      location.href,
    ).href
    const cache = await caches.open(rt?.env.cacheKey ?? 'transformers-cache')
    return Boolean(await cache.match(url))
  })
  tracker.subscribe((s) => {
    state.source = s.source
    state.loadedMB = s.loadedMB
    state.totalMB = s.totalMB
    state.percent = s.percent
  })

  function userTextModel(): BrowserTextModel {
    try {
      const saved = localStorage.getItem(TEXT_MODEL_KEY)
      if (saved === 'small' || saved === 'full') return saved
    } catch {
      /* not remembered */
    }
    return config?.textModel ?? 'small'
  }

  /** Applies new capabilities; a changed model fingerprint drops what is loaded. */
  function configure(next: BrowserAiConfig | null) {
    if (config && next && config.fingerprint !== next.fingerprint) void unload()
    config = next
    state.configured = Boolean(next)
    state.textModel = userTextModel()
  }

  function getRuntime() {
    runtime ??= (async () => {
      const rt = await loadRuntime()
      if (!config) throw new Error('Browser AI is not configured')
      // Model files come from this server only and are cached in the browser.
      rt.env.allowRemoteModels = false
      rt.env.allowLocalModels = true
      rt.env.localModelPath = config.modelBaseUrl.replace(/\/?$/, '/')
      rt.env.useBrowserCache = true
      rt.env.backends.onnx.wasm.wasmPaths = ortWasmPaths()
      state.threads = threadCount(isolated(), cores())
      rt.env.backends.onnx.wasm.numThreads = state.threads
      return rt
    })()
    runtime.catch(() => (runtime = undefined))
    return runtime
  }

  async function getGpu() {
    gpu ??= detectGpu(options.nav)
    const info = await gpu
    state.gpuName = info.adapter && !info.software ? (info.name ?? 'WebGPU') : ''
    return info
  }

  function begin(kind: AiModelKind) {
    tracker.reset()
    state.phase = 'loading'
    state.kind = kind
    state.error = ''
    state.source = 'checking'
    state.loadedMB = state.totalMB = state.percent = 0
  }
  function finish(kind: AiModelKind) {
    state.ready[kind] = true
    state.phase = state.ready.image || state.ready.text ? 'ready' : 'idle'
  }
  function fail(error: unknown) {
    state.phase = 'error'
    state.error = error instanceof Error ? error.message : String(error)
  }

  async function loadVision() {
    if (!config) throw new Error('Browser AI is not configured')
    if (!vision) {
      const cfg = config
      begin('image')
      vision = (async () => {
        const rt = await getRuntime()
        const choice = chooseDevice('image', await getGpu(), cfg.dtype)
        state.imageDevice = choice.device
        const model = await rt.SiglipVisionModel.from_pretrained(cfg.modelName, {
          device: choice.device,
          dtype: cfg.dtype,
          progress_callback: tracker.onProgress,
        })
        // The first run compiles shaders; do it now so it is not charged to a real photo.
        await model({
          pixel_values: new rt.Tensor('float32', new Float32Array(3 * 224 * 224), [1, 3, 224, 224]),
        })
        finish('image')
        return { model }
      })()
      vision.catch((e) => {
        vision = undefined
        fail(e)
      })
    }
    return vision
  }

  async function loadText() {
    if (!config) throw new Error('Browser AI is not configured')
    if (text && loadedTextModel !== state.textModel) await disposeText()
    if (!text) {
      const cfg = config
      const size = state.textModel
      loadedTextModel = size
      begin('text')
      text = (async () => {
        const rt = await getRuntime()
        const dtype = textDtype(size, 'wasm', cfg.dtype)
        const [tokenizer, model] = await Promise.all([
          rt.AutoTokenizer.from_pretrained(cfg.modelName),
          rt.SiglipTextModel.from_pretrained(cfg.modelName, {
            device: 'wasm',
            dtype,
            progress_callback: tracker.onProgress,
          }),
        ])
        await model(tokenizer(['warm up'], { padding: 'max_length', max_length: MAX_TOKENS }))
        finish('text')
        return { tokenizer, model }
      })()
      text.catch((e) => {
        text = undefined
        fail(e)
      })
    }
    return text
  }

  async function embedImage(blob: Blob): Promise<number[]> {
    const [{ model }, rt] = await Promise.all([loadVision(), getRuntime()])
    const pixels = await prepare(blob)
    const { pooler_output } = await model({
      pixel_values: new rt.Tensor('float32', pixels, [1, 3, 224, 224]),
    })
    return unitVector(
      (pooler_output.type === 'float32' ? pooler_output : pooler_output.to('float32')).data,
    )
  }

  async function embedText(query: string): Promise<number[]> {
    const { tokenizer, model } = await loadText()
    const started = now()
    const lowered = query.toLowerCase()
    const length = tokenizer(lowered).input_ids.dims.at(-1) ?? 0
    if (length > MAX_TOKENS)
      throw new Error(`Query is ${length} tokens; the limit is ${MAX_TOKENS}.`)
    const inputs = tokenizer([lowered], {
      padding: 'max_length',
      max_length: MAX_TOKENS,
      truncation: false,
    })
    const { pooler_output } = await model(inputs)
    const vector = unitVector(
      (pooler_output.type === 'float32' ? pooler_output : pooler_output.to('float32')).data,
    )
    state.lastQueryMs = now() - started
    return vector
  }

  async function disposeText() {
    const current = text
    text = undefined
    state.ready.text = false
    const loaded = await current?.catch(() => null)
    await loaded?.model.dispose()
    if (loaded) state.needsReload = true
  }
  async function disposeVision() {
    const current = vision
    vision = undefined
    state.ready.image = false
    const loaded = await current?.catch(() => null)
    await loaded?.model.dispose()
    if (loaded) state.needsReload = true
  }

  /**
   * Frees the models. Disposing releases GPU memory, but WebAssembly memory never shrinks, so
   * the rest only comes back with a page reload (`reloadPage`).
   */
  async function unload(kinds: AiModelKind[] = ['image', 'text']) {
    if (kinds.includes('image')) await disposeVision()
    if (kinds.includes('text')) await disposeText()
    state.phase = state.ready.image || state.ready.text ? 'ready' : 'idle'
    state.kind = null
  }

  function reloadPage(note?: string) {
    if (note) {
      try {
        sessionStorage.setItem(RELOAD_NOTE_KEY, note)
      } catch {
        /* no note */
      }
    }
    reload()
  }

  /** Switches the browser search model; the next search loads it. */
  async function setTextModel(size: BrowserTextModel) {
    try {
      localStorage.setItem(TEXT_MODEL_KEY, size)
    } catch {
      /* not remembered */
    }
    state.textModel = size
    if (text && loadedTextModel !== size) await disposeText()
  }

  async function cachedUrls(): Promise<string[]> {
    const caches = cacheApi()
    if (!caches || !config) return []
    const rt = await getRuntime().catch(() => undefined)
    const cache = await caches.open(rt?.env.cacheKey ?? 'transformers-cache')
    const prefix = new URL(
      `${config.modelBaseUrl.replace(/\/?$/, '/')}${config.modelName}/`,
      location.href,
    ).href
    return (await cache.keys()).map((r) => r.url).filter((u) => u.startsWith(prefix))
  }

  /** Which models are saved on this device right now. */
  async function savedOnDevice() {
    if (!config) return { text: false, image: false }
    return cachedTowers(await cachedUrls(), textDtype(state.textModel, 'wasm', config.dtype), config.dtype)
  }

  /** Deletes every model file this browser saved and frees what is loaded. */
  async function removeFromDevice() {
    await unload()
    const caches = cacheApi()
    const rt = await getRuntime().catch(() => undefined)
    await caches?.delete(rt?.env.cacheKey ?? 'transformers-cache')
    tracker.reset()
    state.percent = state.loadedMB = state.totalMB = 0
  }

  /** Whether the browser can run the model at all (OffscreenCanvas/createImageBitmap for photos). */
  function supported(kind: AiModelKind) {
    if (typeof WebAssembly === 'undefined') return false
    return kind === 'text' || (typeof createImageBitmap === 'function' && typeof OffscreenCanvas === 'function')
  }

  return {
    state,
    configure,
    getConfig: () => config,
    loadVision,
    loadText,
    embedImage,
    embedText,
    unload,
    reloadPage,
    setTextModel,
    savedOnDevice,
    removeFromDevice,
    supported,
    detectGpu: getGpu,
  }
}

export type BrowserAi = ReturnType<typeof createBrowserAi>
