import { beforeEach, expect, it } from 'vitest'
import { http as mock, HttpResponse } from 'msw'
import { API, server } from '@/test/server'
import { useAuthStore } from '@/stores/auth'
import { aiCall, AiError } from '@/lib/ai'
import router from '@/router'

beforeEach(() => useAuthStore().setToken('ai-test-token'))
const headers = { 'Access-Control-Allow-Origin': '*' }
it('accepts asynchronous 202 without treating it as completed', async () => {
  server.use(
    mock.post(`${API}/admin/ai/operations`, () =>
      HttpResponse.json(
        { msgCode: 202, t: { operationId: 5, status: 'queued' } },
        { status: 202, headers },
      ),
    ),
  )
  expect(
    await aiCall({
      method: 'post',
      url: '/admin/ai/operations',
      headers: { 'Idempotency-Key': 'same-key' },
      data: {},
    }),
  ).toEqual({ operationId: 5, status: 'queued' })
})
it('keeps the session on AI access denial and preserves the business reason', async () => {
  server.use(
    mock.post(`${API}/ai/search/text`, () =>
      HttpResponse.json(
        { msgCode: 403, t: null, error: 'ai_not_enabled' },
        { status: 403, headers },
      ),
    ),
  )
  await expect(
    aiCall({ method: 'post', url: '/ai/search/text', data: { text: 'cat' } }),
  ).rejects.toMatchObject({ status: 403, code: 'ai_not_enabled' })
  expect(useAuthStore().token).toBe('ai-test-token')
})
it('clears an invalid session on HTTP 401', async () => {
  server.use(
    mock.get(`${API}/ai/capabilities`, () =>
      HttpResponse.json(
        { msgCode: 401, t: null, error: 'unauthenticated' },
        { status: 401, headers },
      ),
    ),
  )
  await expect(aiCall({ url: '/ai/capabilities' })).rejects.toBeInstanceOf(AiError)
  expect(useAuthStore().token).toBeNull()
  await router.isReady()
})
