import { ref } from 'vue'

/** MIME type for in-app drags: a folder or photos dragged onto a folder to move them. */
export const DRAG_TYPE = 'application/x-photoly'

export interface DragPayload {
  folders: number[]
  photos: number[]
}

/** True while a drag started by `startDrag` is in flight; drop targets use it to show themselves. */
export const dragging = ref(false)

/** Starts a move-drag. `name` labels a single item; several show a count. The label is a small chip instead of the browser's full-size ghost. */
export function startDrag(e: DragEvent, payload: DragPayload, name: string) {
  const dt = e.dataTransfer
  if (!dt) return
  dt.setData(DRAG_TYPE, JSON.stringify(payload))
  dt.effectAllowed = 'move'

  const chip = document.createElement('div')
  const counts = [
    [payload.folders.length, 'folder'],
    [payload.photos.length, 'photo'],
  ].filter(([n]) => n)
  chip.textContent =
    payload.folders.length + payload.photos.length > 1
      ? counts.map(([n, w]) => `${n} ${w}${n === 1 ? '' : 's'}`).join(', ')
      : name
  chip.style.cssText =
    'position:fixed;top:-100px;max-width:180px;padding:4px 10px;border-radius:6px;font:500 12px system-ui;' +
    'white-space:nowrap;overflow:hidden;text-overflow:ellipsis;' +
    'background:var(--card,#fff);color:var(--card-foreground,#111);border:1px solid var(--border,#eaeaea)'
  document.body.append(chip)
  // Negative offset puts the chip below-right of the cursor so it never covers what's under it.
  dt.setDragImage(chip, -12, -12)
  setTimeout(() => chip.remove())

  dragging.value = true
  const stop = () => (dragging.value = false)
  window.addEventListener('dragend', stop, { once: true })
  window.addEventListener('drop', stop, { once: true })
}
