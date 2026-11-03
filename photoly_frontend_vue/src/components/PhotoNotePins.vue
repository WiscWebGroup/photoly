<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useMutationObserver, useResizeObserver } from '@vueuse/core'
import { layoutBubbles, type PinPoint, type Rect } from '@/lib/notePins'
const props = defineProps<{
  pins: PinPoint[]
  image?: HTMLElement
  viewport?: HTMLElement
  transform: string
  selectedId: number
  showAll: boolean
}>()
const emit = defineEmits<{ select: [id: number]; fullNote: [id: number]; dismiss: [] }>()
const hoverId = ref(0)
const suppressedId = ref(0)
const geometry = ref<{ image: Rect; bounds: Rect }>()
let timer: ReturnType<typeof setTimeout> | undefined
let frame = 0
function measure() {
  const image = props.image?.getBoundingClientRect()
  const viewport = props.viewport?.getBoundingClientRect()
  if (!image || !viewport || !image.width || !viewport.width) return
  const x = image.left - viewport.left,
    y = image.top - viewport.top
  const left = Math.max(0, x),
    top = Math.max(0, y)
  const width = Math.min(viewport.width, x + image.width) - left
  const height = Math.min(viewport.height, y + image.height) - top
  geometry.value = {
    image: { x, y, width: image.width, height: image.height },
    bounds: {
      x: left + 6,
      y: top + 6,
      width: Math.max(0, width - 12),
      height: Math.max(0, height - 12),
    },
  }
}
function schedule() {
  cancelAnimationFrame(frame)
  frame = requestAnimationFrame(measure)
}
// The viewer morphs the image itself while opening; those transforms do not trigger ResizeObserver.
useMutationObserver(() => props.image, schedule, { attributes: true, attributeFilter: ['style'] })
useResizeObserver(() => props.image, schedule)
useResizeObserver(() => props.viewport, schedule)
watch(
  () => [props.transform, props.image, props.viewport],
  () => void nextTick(measure),
  { flush: 'post' },
)
onMounted(schedule)
onBeforeUnmount(() => {
  clearTimeout(timer)
  cancelAnimationFrame(frame)
})
const points = computed(() => {
  const g = geometry.value
  if (!g) return []
  return props.pins
    .map((p) => ({
      ...p,
      px: g.image.x + p.x * g.image.width,
      py: g.image.y + p.y * g.image.height,
    }))
    .filter(
      (p) =>
        p.px >= g.bounds.x - 7 &&
        p.py >= g.bounds.y - 7 &&
        p.px <= g.bounds.x + g.bounds.width + 7 &&
        p.py <= g.bounds.y + g.bounds.height + 7,
    )
})
const activeId = computed(() => hoverId.value || props.selectedId)
const bubbles = computed(() => {
  const bounds = geometry.value?.bounds
  if (!bounds || bounds.width < 1 || bounds.height < 1) return []
  const visible = points.value.filter((p) => props.showAll || p.id === activeId.value)
  return layoutBubbles(
    visible.map((p) => ({
      id: p.id,
      x: p.px,
      y: p.py,
      height:
        68 +
        16 *
          Math.min(
            3,
            p.body
              .split('\n')
              .reduce(
                (sum, line) =>
                  sum +
                  Math.max(
                    1,
                    Math.ceil(line.length / Math.max(1, (Math.min(240, bounds.width) - 24) / 7)),
                  ),
                0,
              ),
          ),
    })),
    bounds,
  ).map((r) => ({ ...r, pin: visible.find((p) => p.id === r.id)! }))
})
function enter(id: number) {
  clearTimeout(timer)
  if (suppressedId.value !== id) hoverId.value = id
}
function leave() {
  suppressedId.value = 0
  clearTimeout(timer)
  timer = setTimeout(() => {
    hoverId.value = 0
  }, 160)
}
function toggle(id: number) {
  clearTimeout(timer)
  if (props.selectedId === id) {
    suppressedId.value = id
    hoverId.value = 0
    emit('select', 0)
  } else {
    suppressedId.value = 0
    emit('select', id)
  }
}
function dismiss() {
  const wasOpen = bubbles.value.length > 0 || props.selectedId !== 0 || props.showAll
  suppressedId.value = activeId.value
  hoverId.value = 0
  clearTimeout(timer)
  emit('dismiss')
  return wasOpen
}
defineExpose({ dismiss })
</script>
<template>
  <div class="pointer-events-none absolute inset-0 overflow-clip" data-note-overlay>
    <svg class="absolute inset-0 h-full w-full" aria-hidden="true">
      <line
        v-for="bubble in bubbles"
        :key="bubble.id"
        :x1="bubble.anchorX"
        :y1="bubble.anchorY"
        :x2="Math.max(bubble.x, Math.min(bubble.x + bubble.width, bubble.anchorX))"
        :y2="Math.max(bubble.y, Math.min(bubble.y + bubble.height, bubble.anchorY))"
        :stroke="bubble.pin.color"
        stroke-opacity="0.7"
        stroke-width="1"
      />
    </svg>
    <button
      v-for="pin in points"
      :key="pin.id"
      type="button"
      class="note-pin pointer-events-auto absolute flex size-8 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border text-xs font-bold text-white shadow-sm"
      :class="{ 'is-active': activeId === pin.id }"
      :style="{
        left: pin.px + 'px',
        top: pin.py + 'px',
        '--pin-color': pin.color,
        backgroundColor: pin.color + '73',
        borderColor: pin.color,
      }"
      :aria-label="`Point ${pin.number}`"
      :aria-expanded="bubbles.some((b) => b.id === pin.id)"
      :aria-controls="`pin-bubble-${pin.id}`"
      @pointerdown.stop
      @touchstart.stop
      @click.stop="toggle(pin.id)"
      @pointerenter="$event.pointerType !== 'touch' && enter(pin.id)"
      @pointerleave="leave"
      @focus="enter(pin.id)"
      @blur="leave"
    >
      {{ pin.number }}
    </button>
    <article
      v-for="bubble in bubbles"
      :id="`pin-bubble-${bubble.id}`"
      :key="bubble.id"
      :aria-label="`Point ${bubble.pin.number} preview`"
      class="note-bubble pointer-events-auto absolute flex flex-col gap-1.5 overflow-hidden rounded-lg border bg-zinc-950/95 p-3 text-white shadow-lg"
      :style="{
        left: bubble.x + 'px',
        top: bubble.y + 'px',
        width: bubble.width + 'px',
        height: bubble.height + 'px',
        borderColor: bubble.pin.color + '99',
        zIndex: activeId === bubble.id ? 3 : 2,
      }"
      @pointerdown.stop
      @touchstart.stop
      @click.stop
      @wheel.stop
      @pointerenter="$event.pointerType !== 'touch' && enter(bubble.id)"
      @pointerleave="leave"
      @focusin="enter(bubble.id)"
      @focusout="leave"
    >
      <span class="text-xs font-semibold" :style="{ color: bubble.pin.color }"
        >Point {{ bubble.pin.number }}</span
      >
      <p
        class="line-clamp-3 min-h-0 whitespace-pre-wrap text-xs leading-4 [overflow-wrap:anywhere]"
      >
        {{ bubble.pin.body }}
      </p>
      <button
        type="button"
        class="mt-auto shrink-0 self-start rounded text-xs underline underline-offset-2 focus-visible:ring-2"
        @click.stop="emit('fullNote', bubble.id)"
      >
        View full note
      </button>
    </article>
  </div>
</template>
<style scoped>
.note-pin {
  transition:
    scale 160ms,
    box-shadow 160ms;
  text-shadow: 0 1px 3px #000;
}
.note-pin.is-active,
.note-pin:focus-visible {
  scale: 1.12;
  outline: 2px solid white;
  outline-offset: 2px;
  box-shadow: 0 0 0 7px color-mix(in srgb, var(--pin-color) 22%, transparent);
  animation: pin-glow 280ms ease-out;
  z-index: 4;
}
.note-bubble {
  animation: bubble-in 140ms ease-out;
}
@keyframes pin-glow {
  from {
    box-shadow: 0 0 0 0 transparent;
  }
}
@keyframes bubble-in {
  from {
    opacity: 0;
    translate: 0 3px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .note-pin,
  .note-pin.is-active,
  .note-pin:focus-visible,
  .note-bubble {
    animation: none;
    transition: none;
  }
}
</style>
