import { computed, onUnmounted, ref, type Ref } from 'vue'
import type { Folder } from './useFolders'

/** Reordering uses captured pointer events, never the file-move DataTransfer channel. */
export function useFolderReorder(
  items: Ref<Folder[]>,
  save: (id: number, before: number | null) => Promise<void>,
  failed: () => void,
) {
  const moving = ref<number | null>(null)
  const before = ref<number | null | undefined>(undefined)
  const announcement = ref('')
  const saving = ref(false)
  const active = computed(() => moving.value !== null || saving.value)
  let origin = { x: 0, y: 0 }
  let dragged = false
  let handle: HTMLElement | null = null
  let pointer: number | null = null
  function cancel() {
    if (pointer !== null && handle?.hasPointerCapture(pointer))
      handle.releasePointerCapture(pointer)
    moving.value = null
    before.value = undefined
    pointer = null
    handle = null
    dragged = false
  }
  async function commit(id: number, target: number | null) {
    saving.value = true
    try {
      await save(id, target)
      announcement.value = 'Folder order saved.'
    } catch {
      announcement.value = 'Could not save folder order. The list has been refreshed.'
      failed()
    } finally {
      saving.value = false
    }
  }
  function down(event: PointerEvent, id: number) {
    if (event.button !== 0 || saving.value || !event.isPrimary) return
    event.preventDefault()
    event.stopPropagation()
    handle = event.currentTarget as HTMLElement
    handle.focus()
    pointer = event.pointerId
    handle.setPointerCapture(event.pointerId)
    origin = { x: event.clientX, y: event.clientY }
    dragged = false
    moving.value = id
  }
  function move(event: PointerEvent) {
    if (pointer !== event.pointerId || moving.value === null) return
    if (Math.hypot(event.clientX - origin.x, event.clientY - origin.y) < 5 && !dragged) return
    dragged = true
    const grid = handle?.closest<HTMLElement>('[data-folder-order-grid]')
    const node = document
      .elementFromPoint(event.clientX, event.clientY)
      ?.closest<HTMLElement>('[data-order-id]')
    if (!grid || !node || node.parentElement !== grid) return
    const id = Number(node.dataset.orderId)
    if (id === moving.value) return
    const box = node.getBoundingClientRect()
    const after =
      grid.dataset.layout === 'list'
        ? event.clientY > box.y + box.height / 2
        : event.clientX > box.x + box.width / 2
    const others = items.value.filter((f) => f.nsId !== moving.value)
    const at = others.findIndex((f) => f.nsId === id)
    before.value = after ? (others[at + 1]?.nsId ?? null) : id
    const bounds = grid.getBoundingClientRect()
    if (grid.scrollWidth > grid.clientWidth)
      grid.scrollBy({
        left: event.clientX > bounds.right - 32 ? 24 : event.clientX < bounds.left + 32 ? -24 : 0,
      })
    const scroll = grid.closest<HTMLElement>('[data-scroll-region]') ?? grid.closest('main')
    if (scroll) {
      const b = scroll.getBoundingClientRect()
      scroll.scrollBy({
        top: event.clientY > b.bottom - 40 ? 24 : event.clientY < b.top + 40 ? -24 : 0,
      })
    }
  }
  function up(event: PointerEvent) {
    if (pointer !== event.pointerId) return
    const id = moving.value,
      target = before.value,
      changed = dragged
    cancel()
    if (changed && id !== null && target !== undefined) void commit(id, target)
  }
  function key(event: KeyboardEvent, id: number) {
    if (event.key === 'Escape') {
      cancel()
      return
    }
    if (
      saving.value ||
      !['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)
    )
      return
    event.preventDefault()
    event.stopPropagation()
    const index = items.value.findIndex((f) => f.nsId === id)
    let target: number | null | undefined
    if (event.key === 'Home') target = items.value[0]?.nsId
    else if (event.key === 'End') target = null
    else if (event.key === 'ArrowUp' || event.key === 'ArrowLeft')
      target = items.value[index - 1]?.nsId
    else if (index < items.value.length - 1) target = items.value[index + 2]?.nsId ?? null
    if (target !== undefined && target !== id) void commit(id, target)
  }
  onUnmounted(cancel)
  return { moving, before, announcement, active, saving, down, move, up, key, cancel }
}
