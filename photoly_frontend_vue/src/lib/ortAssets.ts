// Same-origin URLs of ONNX Runtime Web's engine files (never a CDN). Vite emits them as hashed
// assets; they load only when browser AI runs and are kept out of the PWA precache.
import mjs from '@ort/ort-wasm-simd-threaded.asyncify.mjs?url'
import wasm from '@ort/ort-wasm-simd-threaded.asyncify.wasm?url'

export const ortWasmPaths = () => ({
  mjs: new URL(mjs, location.href).href,
  wasm: new URL(wasm, location.href).href,
})
