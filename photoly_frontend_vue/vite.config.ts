/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// Browser AI runs WebAssembly threads, which need cross-origin isolation. `credentialless`
// (not `require-corp`) keeps Leaflet's cross-origin map tiles loading. Mirrored in nginx.conf.
const isolation = {
  'Cross-Origin-Opener-Policy': 'same-origin',
  'Cross-Origin-Embedder-Policy': 'credentialless',
}

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    headers: isolation,
    // `pnpm dev` talks to a backend on PHOTOLY_BACKEND_URL (default: the compose.go.yml port),
    // same-origin like nginx does in production, so CORS and cross-origin isolation need no setup.
    proxy: {
      '/api': {
        target: process.env.PHOTOLY_BACKEND_URL ?? 'http://localhost:8384',
        rewrite: (path) => path.replace(/^\/api/, ''),
      },
      '/models': process.env.PHOTOLY_BACKEND_URL ?? 'http://localhost:8384',
    },
  },
  preview: { headers: isolation },
  build: {
    rollupOptions: {
      output: {
        // transformers.js and ONNX Runtime load only when a person opts in to browser AI.
        chunkFileNames: (chunk) =>
          /^(transformers|ort)/i.test(chunk.name)
            ? 'assets/browser-ai-[name]-[hash].js'
            : 'assets/[name]-[hash].js',
      },
    },
  },
  cacheDir: process.env.PHOTOLY_VITE_CACHE_DIR ?? 'node_modules/.vite',
  plugins: [
    vue(),
    tailwindcss(),
    VitePWA({
      registerType: 'autoUpdate',
      manifest: {
        name: 'Photoly',
        short_name: 'Photoly',
        description: 'Self-hosted photo management',
        theme_color: '#2a9d9d',
        background_color: '#ffffff',
        display: 'standalone',
        icons: [
          { src: 'pwa-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'pwa-512.png', sizes: '512x512', type: 'image/png' },
        ],
      },
      workbox: {
        // Browser AI is opt-in: its runtime and models never ride along in the offline precache.
        globIgnores: ['**/browser-ai-*.js', '**/*.wasm', '**/ort-wasm-*'],
        navigateFallbackDenylist: [/^\/api\//, /^\/models\//],
        runtimeCaching: [
          {
            urlPattern: /\/photo\/renderThumbnail\//,
            handler: 'CacheFirst',
            options: {
              cacheName: 'thumbnails',
              expiration: { maxEntries: 500, maxAgeSeconds: 60 * 60 * 24 * 14 },
              cacheableResponse: { statuses: [200] },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      // ONNX Runtime Web's engine files are not exported by the package; they load at runtime.
      '@ort': fileURLToPath(new URL('./node_modules/onnxruntime-web/dist', import.meta.url)),
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.ts'],
    setupFiles: ['src/test/setup.ts'],
  },
})
