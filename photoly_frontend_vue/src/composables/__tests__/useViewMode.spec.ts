import { expect, it, vi } from 'vitest'

it.each(['grid', 'list', 'compact'])('retains the saved %s layout', async (saved) => {
  vi.resetModules()
  localStorage.setItem('photoly-view-mode', saved)
  const { viewMode } = await import('../useViewMode')
  expect(viewMode.value).toBe(saved)
})
