/** Versioned YuNet/SFace numerical contract, mirrored by people_preprocess.go. */
export interface FaceDetection {
  box: number[]
  landmarks: number[]
  score: number
  vector: number[]
}
export interface FaceImage {
  rgb: Uint8Array
  width: number
  height: number
}
export const faceOutputs = [
  'cls_8',
  'cls_16',
  'cls_32',
  'obj_8',
  'obj_16',
  'obj_32',
  'bbox_8',
  'bbox_16',
  'bbox_32',
  'kps_8',
  'kps_16',
  'kps_32',
]
export function readFaceImage(buffer: ArrayBuffer): FaceImage {
  const v = new DataView(buffer),
    width = v.getUint32(0, true),
    height = v.getUint32(4, true)
  if (
    !width ||
    !height ||
    width > 1600 ||
    height > 1600 ||
    buffer.byteLength !== 8 + width * height * 3
  )
    throw Error('Invalid face image')
  return { rgb: new Uint8Array(buffer, 8), width, height }
}
function sample(im: FaceImage, x: number, y: number, c: number, edge: boolean) {
  if (edge) {
    x = Math.max(0, Math.min(im.width - 1, x))
    y = Math.max(0, Math.min(im.height - 1, y))
  }
  const x0 = Math.floor(x),
    y0 = Math.floor(y),
    dx = x - x0,
    dy = y - y0
  const at = (x: number, y: number) =>
    x < 0 || y < 0 || x >= im.width || y >= im.height ? 0 : im.rgb[(y * im.width + x) * 3 + c]!
  return Math.floor(
    at(x0, y0) * (1 - dx) * (1 - dy) +
      at(x0 + 1, y0) * dx * (1 - dy) +
      at(x0, y0 + 1) * (1 - dx) * dy +
      at(x0 + 1, y0 + 1) * dx * dy +
      0.5,
  )
}
export function detectorTensor(im: FaceImage) {
  const scale = 640 / Math.max(im.width, im.height),
    pixels = new Float32Array(3 * 640 * 640)
  const w = Math.round(im.width * scale),
    h = Math.round(im.height * scale)
  for (let y = 0; y < h; y++)
    for (let x = 0; x < w; x++)
      for (let c = 0; c < 3; c++)
        pixels[c * 640 * 640 + y * 640 + x] = sample(
          im,
          (x + 0.5) / scale - 0.5,
          (y + 0.5) / scale - 0.5,
          2 - c,
          true,
        )
  return { pixels, scale }
}
export function faceIoU(a: number[], b: number[]) {
  const w = Math.max(0, Math.min(a[0]! + a[2]!, b[0]! + b[2]!) - Math.max(a[0]!, b[0]!)),
    h = Math.max(0, Math.min(a[1]! + a[3]!, b[1]! + b[3]!) - Math.max(a[1]!, b[1]!))
  return (w * h) / Math.max(1e-9, a[2]! * a[3]! + b[2]! * b[3]! - w * h)
}
export function decodeFaces(outputs: Float32Array[], im: FaceImage, scale: number) {
  const candidates: FaceDetection[] = []
  for (const [level, stride] of [8, 16, 32].entries()) {
    const cls = outputs[level]!,
      obj = outputs[level + 3]!,
      box = outputs[level + 6]!,
      kps = outputs[level + 9]!,
      cols = 640 / stride
    for (let i = 0; i < cls.length; i++) {
      const score = Math.fround(
        Math.sqrt(
          Math.fround(Math.max(0, Math.min(1, cls[i]!)) * Math.max(0, Math.min(1, obj[i]!))),
        ),
      )
      if (score < 0.8) continue
      const x = i % cols,
        y = Math.floor(i / cols),
        w = (Math.exp(box[i * 4 + 2]!) * stride) / scale,
        h = (Math.exp(box[i * 4 + 3]!) * stride) / scale
      const cx = ((x + box[i * 4]!) * stride) / scale,
        cy = ((y + box[i * 4 + 1]!) * stride) / scale
      const x1 = Math.max(0, cx - w / 2),
        y1 = Math.max(0, cy - h / 2),
        x2 = Math.min(im.width, cx + w / 2),
        y2 = Math.min(im.height, cy + h / 2)
      if (x2 - x1 < 12 || y2 - y1 < 12) continue
      const landmarks: number[] = []
      for (let j = 0; j < 5; j++)
        landmarks.push(
          Math.fround(((x + kps[i * 10 + j * 2]!) * stride) / scale / im.width),
          Math.fround(((y + kps[i * 10 + j * 2 + 1]!) * stride) / scale / im.height),
        )
      if (landmarks.some((n) => n < 0 || n > 1)) continue
      candidates.push({
        box: [x1 / im.width, y1 / im.height, (x2 - x1) / im.width, (y2 - y1) / im.height].map(
          Math.fround,
        ),
        landmarks,
        score,
        vector: [],
      })
    }
  }
  candidates.sort((a, b) => b.score - a.score)
  const keep: FaceDetection[] = []
  for (const f of candidates) {
    if (!keep.some((k) => faceIoU(f.box, k.box) > 0.3)) keep.push(f)
    if (keep.length === 100) break
  }
  return keep
}
const target = [
  38.2946, 51.6963, 73.5318, 51.5014, 56.0252, 71.7366, 41.5493, 92.3655, 70.7299, 92.2041,
]
export function alignedFaceTensor(im: FaceImage, landmarks: number[]) {
  let sx = 0,
    sy = 0,
    dx = 0,
    dy = 0
  for (let i = 0; i < 5; i++) {
    sx += (landmarks[2 * i]! * im.width) / 5
    sy += (landmarks[2 * i + 1]! * im.height) / 5
    dx += target[2 * i]! / 5
    dy += target[2 * i + 1]! / 5
  }
  let a = 0,
    b = 0,
    den = 0
  for (let i = 0; i < 5; i++) {
    const x = landmarks[2 * i]! * im.width - sx,
      y = landmarks[2 * i + 1]! * im.height - sy,
      u = target[2 * i]! - dx,
      v = target[2 * i + 1]! - dy
    a += x * u + y * v
    b += x * v - y * u
    den += x * x + y * y
  }
  if (den < 1e-9) return null
  a /= den
  b /= den
  const norm = a * a + b * b
  if (norm < 1e-9) return null
  const tx = dx - a * sx + b * sy,
    ty = dy - b * sx - a * sy,
    out = new Float32Array(3 * 112 * 112)
  for (let y = 0; y < 112; y++)
    for (let x = 0; x < 112; x++) {
      const u = x - tx,
        v = y - ty,
        px = (a * u + b * v) / norm,
        py = (-b * u + a * v) / norm
      for (let c = 0; c < 3; c++) out[c * 112 * 112 + y * 112 + x] = sample(im, px, py, c, false)
    }
  return out
}
