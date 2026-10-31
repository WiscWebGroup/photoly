import { computed, ref, type Ref } from 'vue'
import { useResizeObserver } from '@vueuse/core'

/** Zoom lives on a separate layer from the viewer's thumbnail morph animation. */
export function usePhotoZoom(viewport: Ref<HTMLElement | undefined>, enabled: () => boolean) {
  const scale = ref(1)
  const x = ref(0)
  const y = ref(0)
  const dragging = ref(false)
  let drag: { id: number; x: number; y: number; fromX: number; fromY: number } | undefined
  function bounds() {
    const el = viewport.value
    const img = el?.querySelector('img')
    if (!el || !img) return { x: 0, y: 0 }
    return {
      x: Math.max(0, (img.offsetWidth * scale.value - el.clientWidth) / 2),
      y: Math.max(0, (img.offsetHeight * scale.value - el.clientHeight) / 2),
    }
  }
  function clamp() {
    const limit = bounds()
    x.value = Math.min(limit.x, Math.max(-limit.x, x.value))
    y.value = Math.min(limit.y, Math.max(-limit.y, y.value))
  }
  function reset() {
    scale.value = 1
    x.value = y.value = 0
    dragging.value = false
    drag = undefined
  }
  function wheel(event: WheelEvent) {
    if (!enabled() || !viewport.value) return
    event.preventDefault()
    event.stopPropagation()
    const box = viewport.value.getBoundingClientRect()
    const delta =
      event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? box.height : 1)
    const next = Math.min(
      8,
      Math.max(1, scale.value * Math.exp(-Math.max(-300, Math.min(300, delta)) * 0.002)),
    )
    const ratio = next / scale.value
    const px = event.clientX - box.left - box.width / 2
    const py = event.clientY - box.top - box.height / 2
    x.value = px - (px - x.value) * ratio
    y.value = py - (py - y.value) * ratio
    scale.value = next
    clamp()
  }
  function down(event: PointerEvent) {
    if (!enabled() || scale.value === 1 || event.button !== 0 || event.pointerType === 'touch')
      return
    event.preventDefault()
    event.stopPropagation()
    drag = {
      id: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      fromX: x.value,
      fromY: y.value,
    }
    dragging.value = true
    ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
  }
  function move(event: PointerEvent) {
    if (!drag || drag.id !== event.pointerId) return
    x.value = drag.fromX + event.clientX - drag.x
    y.value = drag.fromY + event.clientY - drag.y
    clamp()
  }
  function up() {
    drag = undefined
    dragging.value = false
  }
  useResizeObserver(viewport, clamp)
  return {
    scale,
    dragging,
    reset,
    wheel,
    down,
    move,
    up,
    style: computed(() => ({
      transform: `translate(${x.value}px, ${y.value}px) scale(${scale.value})`,
    })),
  }
}
