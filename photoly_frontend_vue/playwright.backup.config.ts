import { defineConfig, devices } from '@playwright/test'
export default defineConfig({
  testDir: './backup-e2e',
  outputDir: './test-results-backup',
  workers: 1,
  timeout: 180000,
  retries: 0,
  use: { baseURL: 'http://localhost:5175', trace: 'retain-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: 'pnpm dev --port 5175 --strictPort',
    url: 'http://localhost:5175',
    reuseExistingServer: false,
    env: {
      VITE_APP_BASE_URL: '/api',
      PHOTOLY_BACKEND_URL: 'http://localhost:18384',
      PHOTOLY_VITE_CACHE_DIR: 'node_modules/.vite-backup',
    },
  },
})
