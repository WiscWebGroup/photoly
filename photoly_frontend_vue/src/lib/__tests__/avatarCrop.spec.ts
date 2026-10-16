import { beforeEach, describe, expect, it } from 'vitest'
import { avatarCrop, encodeAvatar } from '../avatarCrop'
import { stubCanvas, stubImage } from '@/test/canvas'

describe('avatar crop', () => {
  beforeEach(() => {
    stubCanvas()
    stubImage()
  })
  it('centres a square crop for landscape and portrait images', () => {
    expect(avatarCrop(800, 400, 1, 400, 200)).toEqual({ x: 200, y: 0, size: 400 })
    expect(avatarCrop(400, 800, 1, 200, 400)).toEqual({ x: 0, y: 200, size: 400 })
  })
  it('constrains pan and zoom to keep the entire square covered', () => {
    expect(avatarCrop(800, 400, 2, -100, 900)).toEqual({ x: 0, y: 200, size: 200 })
    expect(avatarCrop(800, 400, 0.5, 400, 200).size).toBe(400)
    expect(avatarCrop(800, 400, 10, 400, 200).size).toBe(80)
  })
  it('always exports a 512-pixel JPEG regardless of input aspect or zoom', async () => {
    const image = new Image()
    for (const crop of [avatarCrop(800, 400, 1, 400, 200), avatarCrop(400, 800, 3, 200, 400)]) {
      const file = await encodeAvatar(image, crop)
      expect(file.name).toBe('avatar.jpg')
      expect(file.type).toBe('image/jpeg')
      expect(file.size).toBe(Math.round(512 * 512 * 0.92))
    }
  })
})
