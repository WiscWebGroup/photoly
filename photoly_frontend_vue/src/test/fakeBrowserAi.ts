import { vi } from 'vitest'
import type { Runtime } from '@/lib/browserAi'

/** A stand-in for transformers.js: models return [3, 4, 0...] (unit vector [0.6, 0.8, 0...]). */
export function fakeRuntime() {
  const disposed: string[] = []
  const calls: { kind: string; options: Record<string, unknown> }[] = []
  const model = (kind: string) => {
    const fn = vi.fn(async () => ({
      pooler_output: { type: 'float32', data: [3, 4, 0], to: () => null as never },
    })) as unknown as ReturnType<typeof vi.fn> & { dispose: () => Promise<void> }
    fn.dispose = async () => void disposed.push(kind)
    return fn
  }
  const env = {
    allowRemoteModels: true,
    allowLocalModels: false,
    localModelPath: '',
    useBrowserCache: false,
    cacheKey: 'transformers-cache',
    backends: { onnx: { wasm: {} as { wasmPaths?: unknown; numThreads?: number } } },
  }
  const tokenizer = Object.assign(
    vi.fn((input: unknown) => ({
      input_ids: { dims: [1, typeof input === 'string' ? Math.ceil(input.length / 4) : 64] },
    })),
    {},
  )
  const runtime = {
    env,
    AutoTokenizer: { from_pretrained: vi.fn(async () => tokenizer) },
    SiglipTextModel: {
      from_pretrained: vi.fn(async (_n: string, options: Record<string, unknown>) => {
        calls.push({ kind: 'text', options })
        // transformers.js reports progress while it reads files.
        ;(options.progress_callback as (e: unknown) => void)?.({
          status: 'progress',
          file: 'onnx/text_model_int8.onnx',
          loaded: 100e6,
          total: 318e6,
        })
        return model('text')
      }),
    },
    SiglipVisionModel: {
      from_pretrained: vi.fn(async (_n: string, options: Record<string, unknown>) => {
        calls.push({ kind: 'image', options })
        return model('image')
      }),
    },
    Tensor: class {
      constructor(
        public type: string,
        public data: Float32Array,
        public dims: number[],
      ) {}
    },
  } as unknown as Runtime
  return { runtime, disposed, calls, env, tokenizer }
}

export const browserCapabilities = (over: Record<string, unknown> = {}) => ({
  similarityThreshold: null,
  enabled: true,
  globalEnabled: true,
  ready: true,
  reason: null,
  profileId: 1,
  profile: { provider: 'siglip2', modelId: 'siglip2', dimension: 768 },
  coverage: { eligible: 0, supported: 0, ready: 0, pending: 0, failed: 0, unsupported: 0, ratio: 1 },
  modalities: ['text', 'image'],
  formats: [],
  limits: { textCharacters: 512, imageBytes: 1, imagePixels: 1, topK: 100 },
  placement: { image: 'browser', text: 'browser', choosable: { image: false, text: true } },
  modelBaseUrl: '/models/rev1/',
  browser: { fingerprint: 'fp-1', modelName: 'siglip2', dtype: 'fp32', textModel: 'small' },
  ...over,
})
