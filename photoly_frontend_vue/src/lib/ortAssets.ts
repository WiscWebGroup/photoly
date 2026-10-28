// Same-origin URLs of ONNX Runtime Web's engine files (never a CDN). Vite emits them as hashed
// assets; they load only when browser AI runs and are kept out of the PWA precache.
import builtMjs from '@ort/ort-wasm-simd-threaded.asyncify.mjs?url'
import wasm from '@ort/ort-wasm-simd-threaded.asyncify.wasm?url'

// In dev, `?url` on the .mjs yields its source text; the dev server can serve the file by path.
const mjs = import.meta.env.DEV
  ? new URL('../../node_modules/onnxruntime-web/dist/ort-wasm-simd-threaded.asyncify.mjs', import.meta.url).href
  : builtMjs

export const ortWasmPaths = () => ({
  mjs: new URL(mjs, location.href).href,
  wasm: new URL(wasm, location.href).href,
})
