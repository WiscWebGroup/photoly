<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { http } from '@/lib/http'
import { avatarCrop, encodeAvatar } from '@/lib/avatarCrop'

const props = defineProps<{ src: string; saving?: boolean }>()
const emit = defineEmits<{ submit: [file: File]; cancel: [] }>()
const source = shallowRef<HTMLImageElement>()
const preview = ref('')
const stage = ref<HTMLElement>()
const zoom = ref(1),
  cx = ref(0),
  cy = ref(0)
const loading = ref(false),
  encoding = ref(false),
  error = ref('')
const locked = computed(() => !!props.saving || encoding.value)
const crop = computed(() =>
  source.value
    ? avatarCrop(
        source.value.naturalWidth,
        source.value.naturalHeight,
        zoom.value,
        cx.value,
        cy.value,
      )
    : null,
)
const imageStyle = computed(() => {
  if (!source.value || !crop.value) return {}
  return {
    width: `${(source.value.naturalWidth / crop.value.size) * 100}%`,
    height: `${(source.value.naturalHeight / crop.value.size) * 100}%`,
    left: `${(-crop.value.x / crop.value.size) * 100}%`,
    top: `${(-crop.value.y / crop.value.size) * 100}%`,
  }
})
let sequence = 0
let controller: AbortController | undefined
let drag: { id: number; x: number; y: number } | undefined
function constrain() {
  if (!crop.value) return
  cx.value = crop.value.x + crop.value.size / 2
  cy.value = crop.value.y + crop.value.size / 2
}
watch(zoom, constrain)
async function load(file?: File) {
  const id = ++sequence
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  error.value = ''
  let url = ''
  try {
    const blob =
      file ??
      (await http.get<Blob>(props.src, { responseType: 'blob', signal: controller.signal })).data
    if (id !== sequence) return
    if (blob.size > 20 * 1024 ** 2) throw new Error('Choose an image smaller than 20 MiB.')
    url = URL.createObjectURL(blob)
    const img = new Image()
    img.src = url
    await img.decode()
    if (id !== sequence) return
    if (img.naturalWidth * img.naturalHeight > 40_000_000)
      throw new Error('Choose an image smaller than 40 megapixels.')
    if (preview.value) URL.revokeObjectURL(preview.value)
    preview.value = url
    url = ''
    source.value = img
    zoom.value = 1
    cx.value = img.naturalWidth / 2
    cy.value = img.naturalHeight / 2
  } catch (e) {
    if (id === sequence)
      error.value =
        e instanceof Error && e.message.startsWith('Choose an image')
          ? e.message
          : 'Could not load this image. Choose another image or retry.'
  } finally {
    if (url) URL.revokeObjectURL(url)
    if (id === sequence) loading.value = false
  }
}
function choose(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) void load(file)
}
function pan(dx: number, dy: number) {
  if (!crop.value || !stage.value || locked.value || loading.value) return
  const ratio = crop.value.size / stage.value.getBoundingClientRect().width
  cx.value -= dx * ratio
  cy.value -= dy * ratio
  constrain()
}
function pointerDown(event: PointerEvent) {
  if (locked.value || loading.value || !source.value || event.button !== 0) return
  stage.value?.focus({ preventScroll: true })
  stage.value?.setPointerCapture(event.pointerId)
  drag = { id: event.pointerId, x: event.clientX, y: event.clientY }
}
function pointerMove(event: PointerEvent) {
  if (!drag || drag.id !== event.pointerId) return
  pan(event.clientX - drag.x, event.clientY - drag.y)
  drag.x = event.clientX
  drag.y = event.clientY
}
function pointerUp(event: PointerEvent) {
  if (drag?.id !== event.pointerId) return
  if (stage.value?.hasPointerCapture(event.pointerId))
    stage.value.releasePointerCapture(event.pointerId)
  drag = undefined
}
function keydown(event: KeyboardEvent) {
  const step = event.shiftKey ? 10 : 2
  const delta: Record<string, [number, number]> = {
    ArrowLeft: [-step, 0],
    ArrowRight: [step, 0],
    ArrowUp: [0, -step],
    ArrowDown: [0, step],
  }
  const movement = delta[event.key]
  if (movement) {
    event.preventDefault()
    pan(...movement)
  }
}
async function submit() {
  if (!source.value || !crop.value || loading.value || locked.value) return
  const id = sequence
  encoding.value = true
  error.value = ''
  try {
    const file = await encodeAvatar(source.value, crop.value)
    if (id === sequence) emit('submit', file)
  } catch {
    if (id === sequence) error.value = 'Could not prepare the avatar. Please try again.'
  } finally {
    if (id === sequence) encoding.value = false
  }
}
watch(
  () => props.src,
  () => void load(),
  { immediate: true },
)
onBeforeUnmount(() => {
  sequence++
  controller?.abort()
  if (preview.value) URL.revokeObjectURL(preview.value)
  drag = undefined
})
</script>

<template>
  <div class="space-y-4">
    <Label for="avatar-file">Upload Avatar</Label>
    <Input
      id="avatar-file"
      type="file"
      accept="image/jpeg,image/png,image/webp,image/gif,image/bmp"
      :disabled="locked"
      @change="choose"
    />
    <p v-if="loading" role="status" class="text-sm">Loading avatar…</p>
    <div
      v-if="source"
      ref="stage"
      role="group"
      aria-label="Avatar crop: drag or use arrow keys to reposition"
      tabindex="0"
      data-testid="avatar-crop"
      class="relative mx-auto aspect-square w-full max-w-72 touch-none overflow-hidden rounded-md bg-white outline-none focus-visible:ring-2 focus-visible:ring-ring"
      @pointerdown.prevent="pointerDown"
      @pointermove="pointerMove"
      @pointerup="pointerUp"
      @pointercancel="pointerUp"
      @lostpointercapture="drag = undefined"
      @keydown="keydown"
    >
      <img
        :src="preview"
        alt="Avatar crop preview"
        draggable="false"
        class="absolute max-w-none select-none"
        :style="imageStyle"
      />
      <div
        aria-hidden="true"
        class="pointer-events-none absolute inset-0 rounded-full border-2 border-white shadow-[0_0_0_999px_rgba(0,0,0,0.55)]"
      />
    </div>
    <div v-if="source" class="space-y-2">
      <Label for="avatar-zoom">Zoom</Label>
      <input
        id="avatar-zoom"
        v-model.number="zoom"
        type="range"
        min="1"
        max="5"
        step="0.01"
        :disabled="locked || loading"
        class="w-full"
      />
      <p class="text-sm text-muted-foreground">
        Drag or use arrow keys to position your avatar. Saved as a square image.
      </p>
    </div>
    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <Button v-if="error && !source" variant="outline" :disabled="loading" @click="load()"
      >Retry loading avatar</Button
    >
    <div class="flex justify-end gap-2">
      <Button variant="outline" :disabled="locked" @click="emit('cancel')">Cancel</Button>
      <Button :disabled="!source || loading || locked" @click="submit">{{
        locked ? 'Saving…' : 'Submit'
      }}</Button>
    </div>
  </div>
</template>
