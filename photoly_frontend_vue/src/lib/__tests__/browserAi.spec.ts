import { describe, expect, it, vi } from 'vitest'
import {
  cachedTowers,
  chooseDevice,
  configFromCapabilities,
  createBrowserAi,
  createProgressTracker,
  detectGpu,
  isSoftwareAdapter,
  textDtype,
  threadCount,
  unitVector,
} from '../browserAi'
import { normalize, resizeBilinear, SIZE } from '../browserPreprocess'
import { loadingCopy } from '../browserAiCopy'
import { fakeRuntime } from '@/test/fakeBrowserAi'

const config = {
  fingerprint: 'fp-1',
  modelBaseUrl: '/models/rev1/',
  modelName: 'siglip2',
  dtype: 'fp32',
  textModel: 'small' as const,
}
const make = (over: Parameters<typeof createBrowserAi>[0] = {}) => {
  const fake = fakeRuntime()
  const reload = vi.fn()
  const ai = createBrowserAi({
    loadRuntime: async () => fake.runtime,
    nav: {},
    isolated: () => true,
    cores: () => 8,
    cacheApi: () => undefined,
    prepare: async () => new Float32Array(3 * 224 * 224),
    reload,
    ...over,
  })
  ai.configure(config)
  return { ai, fake, reload }
}

describe('device choice', () => {
  const real = { adapter: { features: new Set(['shader-f16']) }, reason: null }
  it('runs search text on the CPU always, images on a real GPU', () => {
    expect(chooseDevice('text', real, 'fp32').device).toBe('wasm')
    expect(chooseDevice('image', real, 'fp32').device).toBe('webgpu')
  })
  it('never counts a software renderer or a missing GPU as a GPU', () => {
    expect(chooseDevice('image', { adapter: {}, software: true, reason: 'x' }, 'fp32').device).toBe('wasm')
    expect(chooseDevice('image', { adapter: null, reason: 'no GPU' }, 'fp32').reason).toBe('no GPU')
    expect(isSoftwareAdapter({ info: { description: 'Google SwiftShader' } })).toBe(true)
    expect(isSoftwareAdapter({ info: { vendor: 'apple' } })).toBe(false)
  })
  it('needs shader-f16 for the fp16 model', () => {
    const noF16 = { adapter: { features: new Set<string>() }, reason: null }
    expect(chooseDevice('image', noF16, 'fp16').device).toBe('wasm')
    expect(chooseDevice('image', real, 'fp16').device).toBe('webgpu')
  })
  it('detects WebGPU through navigator.gpu', async () => {
    expect((await detectGpu({})).reason).toMatch(/no WebGPU/)
    const gpu = { requestAdapter: async () => ({ info: { vendor: 'apple', architecture: 'm1' }, features: new Set() }) }
    expect((await detectGpu({ gpu })).name).toBe('apple m1')
  })
})

describe('model options', () => {
  it('uses int8 for the small text model on the CPU only', () => {
    expect(textDtype('small', 'wasm', 'fp32')).toBe('int8')
    expect(textDtype('full', 'wasm', 'fp32')).toBe('fp32')
    expect(textDtype('small', 'webgpu', 'fp16')).toBe('fp16')
  })
  it('falls back to one thread without cross-origin isolation', () => {
    expect(threadCount(true, 8)).toBe(8)
    expect(threadCount(false, 8)).toBe(1)
    expect(threadCount(true, undefined)).toBe(1)
  })
  it('normalises vectors to unit length', () => {
    expect(unitVector([3, 4])).toEqual([0.6, 0.8])
    expect(unitVector([0, 0])).toEqual([0, 0])
  })
  it('reads the browser model from capabilities', () => {
    expect(configFromCapabilities(undefined, '/models/')).toBeNull()
    expect(
      configFromCapabilities({ fingerprint: 'f', modelName: 'm', dtype: '' }, undefined),
    ).toMatchObject({ modelBaseUrl: '/models/', dtype: 'fp32', textModel: 'small' })
  })
  it('judges saved models by file name', () => {
    const urls = ['http://x/models/r/siglip2/onnx/text_model_int8.onnx', 'http://x/models/r/siglip2/config.json']
    expect(cachedTowers(urls, 'int8', 'fp32')).toEqual({ text: true, image: false })
    expect(cachedTowers(urls, 'fp32', 'fp32').text).toBe(false)
  })
})

describe('progress', () => {
  it('says Downloading for files missing from the cache, Loading from this device for hits', async () => {
    const tracker = createProgressTracker(async (file) => file === 'cached.onnx')
    const seen: string[] = []
    tracker.subscribe((s) => seen.push(s.source))
    tracker.onProgress({ status: 'progress', file: 'cached.onnx', loaded: 5e6, total: 10e6 })
    expect(tracker.summary().source).toBe('checking')
    await vi.waitFor(() => expect(tracker.summary().source).toBe('device'))
    tracker.onProgress({ status: 'progress', file: 'new.onnx', loaded: 1e6, total: 10e6 })
    await vi.waitFor(() => expect(tracker.summary().source).toBe('download'))
    expect(tracker.summary()).toMatchObject({ loadedMB: 6, totalMB: 20, percent: 30 })
    tracker.onProgress({ status: 'done', file: 'x', loaded: 1, total: 1 })
    expect(tracker.summary().totalMB).toBe(20)
  })
  it('words the two cases as the design does', () => {
    expect(loadingCopy('download', 131, 318).headline).toBe('Downloading the search model')
    expect(loadingCopy('download', 131, 318).amount).toBe('131 of 318 MB')
    expect(loadingCopy('device', 212, 283).headline).toBe('Loading the search model from this device')
    expect(loadingCopy('device', 1, 2, 'image').headline).toContain('image model')
  })
})

describe('preprocessing', () => {
  it('resizes a flat image to a flat 224 x 224 and normalises to [-1, 1]', () => {
    const rgb = new Uint8Array(500 * 300 * 3).fill(255)
    const out = resizeBilinear({ rgb, width: 500, height: 300 }, SIZE, SIZE)
    expect(out).toHaveLength(SIZE * SIZE * 3)
    expect(new Set(out)).toEqual(new Set([255]))
    const px = normalize(new Uint8Array(SIZE * SIZE * 3))
    expect(px[0]).toBe(-1)
    expect(normalize(out)[0]).toBe(1)
  })
})

describe('createBrowserAi', () => {
  it('embeds text with the small int8 model from this server and returns a unit vector', async () => {
    const { ai, fake } = make()
    expect(await ai.embedText('Rocket Launch')).toEqual([0.6, 0.8, 0])
    expect(fake.env.localModelPath).toBe('/models/rev1/')
    expect(fake.env.allowRemoteModels).toBe(false)
    expect(fake.env.backends.onnx.wasm.numThreads).toBe(8)
    expect(fake.calls[0]!.options).toMatchObject({ device: 'wasm', dtype: 'int8' })
    expect(fake.tokenizer).toHaveBeenCalledWith('rocket launch')
    expect(ai.state.ready.text).toBe(true)
    expect(ai.state.lastQueryMs).not.toBeNull()
  })
  it('reports download progress while loading', async () => {
    const { ai } = make()
    await ai.loadText()
    expect(ai.state.totalMB).toBe(318)
    expect(ai.state.percent).toBe(31)
  })
  it('runs one thread without cross-origin isolation', async () => {
    const { ai, fake } = make({ isolated: () => false })
    await ai.loadText()
    expect(fake.env.backends.onnx.wasm.numThreads).toBe(1)
  })
  it('rejects queries over the token limit', async () => {
    const { ai } = make()
    await expect(ai.embedText('x'.repeat(400))).rejects.toThrow(/limit is 64/)
  })
  it('loads each tower once however many callers ask', async () => {
    const { ai, fake } = make()
    await Promise.all([ai.embedText('a'), ai.embedText('b'), ai.embedText('c')])
    expect(fake.runtime.SiglipTextModel.from_pretrained).toHaveBeenCalledTimes(1)
  })
  it('embeds photos through preprocessing', async () => {
    const prepare = vi.fn(async () => new Float32Array(3 * 224 * 224))
    const { ai } = make({ prepare })
    expect(await ai.embedImage(new Blob(['x']))).toEqual([0.6, 0.8, 0])
    expect(prepare).toHaveBeenCalled()
    expect(ai.state.ready.image).toBe(true)
  })
  it('swaps the text model on the next search and disposes the old one', async () => {
    const { ai, fake } = make()
    await ai.embedText('a')
    await ai.setTextModel('full')
    expect(fake.disposed).toEqual(['text'])
    expect(ai.state.needsReload).toBe(true)
    await ai.embedText('b')
    expect(fake.calls.at(-1)!.options).toMatchObject({ dtype: 'fp32' })
    expect(localStorage.getItem('photoly.textModel')).toBe('full')
  })
  it('remembers the user text model over the admin default', () => {
    localStorage.setItem('photoly.textModel', 'full')
    const { ai } = make()
    expect(ai.state.textModel).toBe('full')
  })
  it('disposes models on unload and reloads the page with a note on request', async () => {
    const { ai, fake, reload } = make()
    await ai.embedText('a')
    await ai.embedImage(new Blob(['x']))
    await ai.unload()
    expect(fake.disposed.sort()).toEqual(['image', 'text'])
    expect(ai.state.phase).toBe('idle')
    ai.reloadPage('freed')
    expect(reload).toHaveBeenCalled()
    expect(sessionStorage.getItem('photoly.reloadNote')).toBe('freed')
  })
  it('drops loaded models when the model fingerprint changes', async () => {
    const { ai, fake } = make()
    await ai.embedText('a')
    ai.configure({ ...config, fingerprint: 'fp-2' })
    await vi.waitFor(() => expect(fake.disposed).toEqual(['text']))
  })
  it('lists and removes saved models through the Cache API', async () => {
    const entries = new Map([
      ['http://localhost:3000/models/rev1/siglip2/onnx/text_model_int8.onnx', 1],
      ['http://localhost:3000/models/rev1/siglip2/config.json', 1],
      ['http://localhost:3000/models/other/readme', 1],
    ])
    const deleted: string[] = []
    const caches = {
      open: async () => ({ keys: async () => [...entries.keys()].map((url) => ({ url })) }),
      delete: async (name: string) => void deleted.push(name),
    } as unknown as CacheStorage
    const { ai } = make({ cacheApi: () => caches })
    expect(await ai.savedOnDevice()).toEqual({ text: true, image: false })
    await ai.removeFromDevice()
    expect(deleted).toEqual(['transformers-cache'])
  })
  it('keeps the error when a model fails to load, then lets the next try start fresh', async () => {
    const { ai, fake } = make()
    vi.mocked(fake.runtime.SiglipTextModel.from_pretrained).mockRejectedValueOnce(new Error('out of memory'))
    await expect(ai.embedText('a')).rejects.toThrow('out of memory')
    expect(ai.state.phase).toBe('error')
    expect(ai.state.error).toBe('out of memory')
    await expect(ai.embedText('a')).resolves.toHaveLength(3)
  })
})
