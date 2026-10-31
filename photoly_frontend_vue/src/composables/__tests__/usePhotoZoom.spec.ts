import { effectScope, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import { usePhotoZoom } from '../usePhotoZoom'

vi.mock('@vueuse/core', () => ({ useResizeObserver: vi.fn() }))
function setup() {
  const element = document.createElement('div')
  const img = document.createElement('img')
  element.append(img)
  Object.defineProperties(element, { clientWidth: { value: 800 }, clientHeight: { value: 600 } })
  Object.defineProperties(img, { offsetWidth: { value: 800 }, offsetHeight: { value: 600 } })
  element.getBoundingClientRect = () =>
    ({ left: 100, top: 100, width: 800, height: 600 }) as DOMRect
  element.setPointerCapture = vi.fn()
  const enabled = ref(true)
  const scope = effectScope()
  const zoom = scope.run(() => usePhotoZoom(ref(element), () => enabled.value))!
  const wheel = (deltaY: number, x = 500, y = 400) => {
    const event = new WheelEvent('wheel', { deltaY, clientX: x, clientY: y, cancelable: true })
    zoom.wheel(event)
    return event
  }
  return { element, zoom, enabled, wheel }
}

describe('photo zoom', () => {
  it('keeps the image point below the cursor anchored', () => {
    const { zoom, wheel } = setup()
    wheel(-200, 600, 450)
    const scale = Math.exp(0.4)
    expect(zoom.scale.value).toBeCloseTo(scale)
    expect(zoom.style.value.transform).toBe(
      `translate(${100 - 100 * scale}px, ${50 - 50 * scale}px) scale(${scale})`,
    )
  })
  it('limits zoom to 1–8 and restores the fit transform', () => {
    const { zoom, wheel } = setup()
    for (let i = 0; i < 20; i++) wheel(-300)
    expect(zoom.scale.value).toBe(8)
    for (let i = 0; i < 20; i++) wheel(300)
    expect(zoom.style.value.transform).toBe('translate(0px, 0px) scale(1)')
    wheel(-300)
    zoom.reset()
    expect(zoom.style.value.transform).toBe('translate(0px, 0px) scale(1)')
  })
  it('clamps dragging to the image edges and leaves touch gestures alone', () => {
    const { zoom, wheel, element } = setup()
    wheel(-300)
    const pointer = (x: number, y: number, pointerType = 'mouse') =>
      ({
        button: 0,
        pointerId: 1,
        pointerType,
        clientX: x,
        clientY: y,
        currentTarget: element,
        preventDefault: vi.fn(),
        stopPropagation: vi.fn(),
      }) as unknown as PointerEvent
    zoom.down(pointer(500, 400, 'touch'))
    expect(zoom.dragging.value).toBe(false)
    zoom.down(pointer(500, 400))
    zoom.move(pointer(10000, -10000))
    const scale = zoom.scale.value
    expect(zoom.style.value.transform).toBe(
      `translate(${(800 * scale - 800) / 2}px, ${-(600 * scale - 600) / 2}px) scale(${scale})`,
    )
    zoom.up()
    expect(zoom.dragging.value).toBe(false)
  })
  it('does not intercept wheel events while editing or playing live media', () => {
    const { zoom, enabled, wheel } = setup()
    enabled.value = false
    expect(wheel(-300).defaultPrevented).toBe(false)
    expect(zoom.scale.value).toBe(1)
  })
})
