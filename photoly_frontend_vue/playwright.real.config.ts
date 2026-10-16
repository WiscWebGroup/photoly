import { defineConfig, devices } from '@playwright/test'
export default defineConfig({
  testDir: './real-e2e',
  outputDir: './test-results-real',
  timeout: 900000,
  workers: 1,
  retries: 0,
  use: { baseURL: 'http://localhost:5174', trace: 'retain-on-failure', actionTimeout: 20000 },
  projects: [{ name: 'cuda-chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'pnpm dev --port 5174 --strictPort',
    url: 'http://localhost:5174',
    reuseExistingServer: false,
    env: {
      VITE_APP_BASE_URL: 'http://localhost:18384',
      PHOTOLY_VITE_CACHE_DIR: 'node_modules/.vite-real',
    },
  },
})
