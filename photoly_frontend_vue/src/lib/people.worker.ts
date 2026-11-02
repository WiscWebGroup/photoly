/// <reference lib="webworker" />
import * as ort from 'onnxruntime-web/webgpu'
import { sha256 } from '@noble/hashes/sha2.js'
import { ortWasmPaths } from './ortAssets'
import {
  alignedFaceTensor,
  decodeFaces,
  detectorTensor,
  faceOutputs,
  readFaceImage,
} from './peopleInference'

interface ModelFile {
  file: string
  sha256: string
  size: number
}
export interface FaceManifest {
  fingerprint: string
  baseUrl: string
  files: Record<'detector' | 'embedder', ModelFile>
}
let detector: ort.InferenceSession | undefined,
  embedder: ort.InferenceSession | undefined,
  device = 'wasm',
  fingerprint = ''
async function modelBytes(manifest: FaceManifest, kind: 'detector' | 'embedder') {
  const file = manifest.files[kind],
    url = new URL(manifest.baseUrl + file.file, self.location.origin).href
  const cache =
    typeof caches === 'undefined'
      ? null
      : await caches.open('photoly-people-models-v1').catch(() => null)
  let response = await cache?.match(url)
  if (!response) {
    self.postMessage({ type: 'progress', message: `Downloading ${kind} model` })
    response = await fetch(url)
    if (!response.ok) throw Error(`Model download failed (${response.status})`)
  }
  const data = await response.arrayBuffer(),
    digest = Array.from(sha256(new Uint8Array(data)))
      .map((x) => x.toString(16).padStart(2, '0'))
      .join('')
  if (data.byteLength !== file.size || digest !== file.sha256) {
    await cache?.delete(url)
    throw Error('Face model checksum mismatch')
  }
  await cache?.put(url, new Response(data)).catch(() => {})
  return data
}
async function release() {
  await detector?.release().catch(() => {})
  await embedder?.release().catch(() => {})
  detector = embedder = undefined
}
async function load(manifest: FaceManifest, forceWasm = false) {
  if (detector && embedder && fingerprint === manifest.fingerprint) return
  await release()
  ort.env.logLevel = 'error'
  ort.env.wasm.wasmPaths = ortWasmPaths()
  ort.env.wasm.numThreads = self.crossOriginIsolated
    ? Math.min(4, navigator.hardwareConcurrency || 1)
    : 1
  const [det, emb] = await Promise.all([
    modelBytes(manifest, 'detector'),
    modelBytes(manifest, 'embedder'),
  ])
  const create = async (ep: string) => {
    detector = await ort.InferenceSession.create(det, {
      executionProviders: [ep],
      logSeverityLevel: 3,
    })
    embedder = await ort.InferenceSession.create(emb, {
      executionProviders: [ep],
      logSeverityLevel: 3,
    })
    device = ep
  }
  try {
    if (forceWasm || !navigator.gpu || !(await navigator.gpu.requestAdapter()))
      throw Error('WebGPU unavailable')
    await create('webgpu')
  } catch {
    await release()
    await create('wasm')
  }
  fingerprint = manifest.fingerprint
}
self.onmessage = async (
  e: MessageEvent<{ id: number; manifest: FaceManifest; image: ArrayBuffer; forceWasm?: boolean }>,
) => {
  const { id, manifest, image, forceWasm } = e.data
  try {
    await load(manifest, forceWasm)
    const im = readFaceImage(image),
      { pixels, scale } = detectorTensor(im)
    const run = async () => {
      const input = new ort.Tensor('float32', pixels, [1, 3, 640, 640])
      let outputs: ort.InferenceSession.ReturnType
      try {
        outputs = await detector!.run({ input })
      } finally {
        input.dispose()
      }
      const faces = decodeFaces(
        faceOutputs.map((k) => outputs[k]!.data as Float32Array),
        im,
        scale,
      )
      Object.values(outputs).forEach((t) => t.dispose())
      const result = []
      for (const f of faces) {
        const tensor = alignedFaceTensor(im, f.landmarks)
        if (!tensor) continue
        const data = new ort.Tensor('float32', tensor, [1, 3, 112, 112])
        let out: ort.InferenceSession.ReturnType
        try {
          out = await embedder!.run({ data })
        } finally {
          data.dispose()
        }
        const v = Array.from(out.fc1!.data as Float32Array),
          norm = Math.sqrt(v.reduce((s, x) => s + x * x, 0))
        Object.values(out).forEach((t) => t.dispose())
        if (norm > 0) {
          f.vector = v.map((x) => Math.fround(x / norm))
          result.push(f)
        }
      }
      return result
    }
    let faces
    try {
      faces = await run()
    } catch (error) {
      if (device !== 'webgpu') throw error
      await release()
      await load(manifest, true)
      faces = await run()
    }
    self.postMessage({ id, faces, device })
  } catch (error) {
    self.postMessage({ id, error: error instanceof Error ? error.message : String(error) })
  }
}
