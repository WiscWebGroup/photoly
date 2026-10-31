import { expect, it, vi } from 'vitest'
import { http } from 'msw'
import { API, ok, recordRequests, server } from '@/test/server'
import { button, mountWithApp, pageText } from '@/test/utils'
import StorageActivity from '../StorageActivity.vue'

it('shows source-specific issues and never offers an S3 scan', async () => {
  const requests = recordRequests()
  server.use(
    http.get(`${API}/user/storage/sources`, () =>
      ok([
        {
          sourceId: 2,
          sourceType: 'smb',
          name: 'NAS',
          enabled: true,
          revision: 1,
          scanStatus: 'ready',
          scanPhase: 'complete',
          scanRetries: 1,
          scanAutomaticRetries: 0,
          scanBlocked: 1,
        },
        {
          sourceId: 3,
          sourceType: 's3',
          name: 'Cloud',
          enabled: false,
          revision: 1,
          scanRetries: 0,
        },
      ]),
    ),
    http.get(`${API}/user/storage/sources/2/failures`, () =>
      ok({
        total: 1,
        items: [
          {
            path: 'photos/broken.jpg',
            code: 'invalid_image',
            message: 'This file is not a readable image.',
            retryable: false,
            attemptedAt: '2026-10-30T12:00:00Z',
          },
        ],
      }),
    ),
  )
  mountWithApp(StorageActivity)
  button('Storage activity').click()
  await vi.waitFor(() => expect(pageText()).toContain('Cloud'))
  expect(pageText()).toContain('Bucket contents are not scanned')
  expect(
    [...document.querySelectorAll('button')].filter((b) => b.textContent?.trim() === 'Scan source'),
  ).toHaveLength(1)
  button('View 1 file issues').click()
  await vi.waitFor(() => expect(pageText()).toContain('photos/broken.jpg'))
  expect(pageText()).toContain('Waiting for file changes or forced verification')
  expect(requests.find((r) => r.path.endsWith('/failures'))?.query).toEqual({
    offset: '0',
    limit: '25',
  })
})
