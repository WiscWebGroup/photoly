import { test, expect } from '@playwright/test'
import { readFile, writeFile } from 'node:fs/promises'
const root = process.env.PHOTOLY_FACE_MODELS
if (process.env.PHOTOLY_REQUIRE_WEBGPU)
  test.use({ headless: false, launchOptions: { args: ['--enable-unsafe-webgpu'] } })
// Opt-in real models; Go generates canonical RGB inputs and expected vectors.
test('YuNet and SFace browser agree with Go CPU', async ({ page, context }, testInfo) => {
  test.skip(!root, 'Set PHOTOLY_FACE_MODELS to models and Go reference outputs')
  test.setTimeout(180000)
  const files = {
    detector: {
      file: 'face_detection_yunet_2023mar.onnx',
      sha256: '8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4',
      size: 232589,
    },
    embedder: {
      file: 'face_recognition_sface_2021dec.onnx',
      sha256: '0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79',
      size: 38696353,
    },
  }
  await context.route(/\/(models\/people-test|people-test)\//, async (route) => {
    await route.fulfill({
      body: await readFile(`${root}/${new URL(route.request().url()).pathname.split('/').pop()}`),
      contentType: 'application/octet-stream',
    })
  })
  await context.route('**/people-harness', (route) =>
    route.fulfill({
      body: '<!doctype html><title>Face test</title>',
      contentType: 'text/html',
      headers: {
        'Cross-Origin-Opener-Policy': 'same-origin',
        'Cross-Origin-Embedder-Policy': 'credentialless',
      },
    }),
  )
  await context.route('**/src/lib/people.worker.ts?simulateGpuFailure=1', async (route) => {
    const response = await route.fetch()
    await route.fulfill({
      response,
      body:
        `Object.defineProperty(navigator,'gpu',{value:{requestAdapter:async()=>{throw Error('simulated GPU initialization failure')}}});\n` +
        (await response.text()),
    })
  })
  await page.goto('/people-harness')
  const measurements: {
    sample: string
    device: string
    elapsedMs: number
    faces: number
    mode: string
  }[] = []
  for (const mode of ['wasm', 'preferred', 'failed-gpu'])
    for (const name of [
      'astronaut.png',
      'chelsea.png',
      'camera.png',
      'small',
      'tilt10',
      'group',
      'exif-rotated',
      'animated-first-frame',
      'tiff',
    ]) {
      const result = await page.evaluate(
        async ({ name, files, mode }) => {
          const started = performance.now()
          const moduleUrl = new URL('/src/lib/people.worker.ts', location.origin).href
          const failingGPU = mode === 'failed-gpu'
          const source = failingGPU ? moduleUrl + '?simulateGpuFailure=1' : moduleUrl
          const image = await (await fetch('/people-test/' + name + '.rgb')).arrayBuffer()
          const worker = new Worker(source, { type: 'module' })
          return await new Promise<{
            faces: { box: number[]; landmarks: number[]; vector: number[] }[]
            device: string
            elapsedMs: number
          }>((resolve, reject) => {
            const deadline = setTimeout(() => {
              worker.terminate()
              reject(Error(`Timed out ${mode}/${name}`))
            }, 30000)
            worker.onerror = (e) => {
              clearTimeout(deadline)
              worker.terminate()
              reject(Error(e.message))
            }
            const send = () =>
              worker.postMessage(
                {
                  id: 1,
                  forceWasm: mode === 'wasm',
                  image,
                  manifest: { fingerprint: 'test', baseUrl: '/models/people-test/', files },
                },
                [image],
              )
            worker.onmessage = (e) => {
              if (e.data.type === 'ready') {
                send()
                return
              }
              if (e.data.type === 'progress') return
              clearTimeout(deadline)
              worker.terminate()
              if (e.data.error) reject(Error(e.data.error))
              else resolve({ ...e.data, elapsedMs: performance.now() - started })
            }
            send()
          })
        },
        { name, files, mode },
      )
      const ref = JSON.parse(await readFile(`${root}/${name}.json`, 'utf8'))
      expect(result.faces.length).toBe(ref.faces.length)
      if (mode !== 'preferred') expect(result.device).toBe('wasm')
      if (mode === 'preferred' && process.env.PHOTOLY_REQUIRE_WEBGPU)
        expect(result.device).toBe('webgpu')
      for (let i = 0; i < result.faces.length; i++) {
        const f = result.faces[i]!,
          r = ref.faces[i]
        f.box.forEach((v, j) => expect(Math.abs(v - r.box[j])).toBeLessThan(0.002))
        f.landmarks.forEach((v, j) => expect(Math.abs(v - r.landmarks[j])).toBeLessThan(0.002))
        expect(f.vector.reduce((s, v, j) => s + v * r.vector[j], 0)).toBeGreaterThan(0.995)
      }
      measurements.push({
        sample: name,
        device: result.device,
        faces: result.faces.length,
        elapsedMs: result.elapsedMs,
        mode,
      })
      await testInfo.attach(`${name}-${mode}`, {
        body: JSON.stringify({
          device: result.device,
          faces: result.faces.length,
          elapsedMs: result.elapsedMs,
        }),
        contentType: 'application/json',
      })
    }
  await writeFile(
    `${root}/browser-evaluation.json`,
    JSON.stringify(
      {
        browser: testInfo.project.name,
        description:
          'Fresh worker wall time including model fetch/cache and initialization; GPU/driver peak memory not measured.',
        measurements,
      },
      null,
      2,
    ),
  )
})
