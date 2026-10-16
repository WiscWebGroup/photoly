import { beforeEach, describe, expect, it, vi } from 'vitest'
import { http, HttpResponse } from 'msw'
import { API, server } from '@/test/server'
import { stubCanvas, stubImage } from '@/test/canvas'
import { button, mountWithApp, pageText } from '@/test/utils'
import AvatarCropper from '../AvatarCropper.vue'

async function open() {
  const wrapper = mountWithApp(AvatarCropper, { src: `${API}/user/getAvatar/test` })
  await vi.waitFor(() => expect(button('Submit').disabled).toBe(false))
  return wrapper
}
describe('AvatarCropper', () => {
  beforeEach(() => {
    stubCanvas()
    stubImage(800, 400)
    server.use(
      http.get(`${API}/user/getAvatar/test`, () =>
        HttpResponse.arrayBuffer(new ArrayBuffer(20), {
          headers: { 'Access-Control-Allow-Origin': '*' },
        }),
      ),
    )
  })
  it('loads the current avatar and submits an encoded square JPEG', async () => {
    const wrapper = await open()
    expect(document.querySelector('[data-testid="avatar-crop"]')).not.toBeNull()
    button('Submit').click()
    await vi.waitFor(() => expect(wrapper.emitted('submit')).toHaveLength(1))
    const file = wrapper.emitted('submit')![0]![0] as File
    expect(file.type).toBe('image/jpeg')
    expect(file.size).toBe(Math.round(512 * 512 * 0.92))
  })
  it('cancels without submitting', async () => {
    const wrapper = await open()
    button('Cancel').click()
    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(wrapper.emitted('submit')).toBeUndefined()
  })
  it('reports decode failures and supports retry', async () => {
    vi.stubGlobal(
      'Image',
      class {
        src = ''
        decode = () => Promise.reject(new Error('decode'))
      },
    )
    mountWithApp(AvatarCropper, { src: `${API}/user/getAvatar/test` })
    await vi.waitFor(() => expect(pageText()).toContain('Could not load this image'))
    expect(button('Submit').disabled).toBe(true)
    stubImage(400, 800)
    button('Retry loading avatar').click()
    await vi.waitFor(() => expect(button('Submit').disabled).toBe(false))
  })
})
