<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, useTemplateRef, watch } from 'vue'
import { toast } from 'vue-sonner'
import { RotateCcw, RotateCw } from '@lucide/vue'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import type { Photo } from '@/composables/usePhotos'
import { call, http } from '@/lib/http'
import {
  FULL_CROP,
  chooseEncoding,
  clampCrop,
  cropForRatio,
  cropToCanvas,
  encodeToLimit,
  formatBytes,
  rotateCrop,
  rotateToCanvas,
  rotatedSize,
  type Crop,
  type Encoded,
  type Rotation,
} from '@/lib/imageEdit'
import { photoUrl } from '@/lib/photoUrl'

const props = defineProps<{ photo: Photo }>()
const emit = defineEmits<{
  cancel: []
  /** `overwrite` replaced the stored file (so its format may have changed); `new` added a copy. */
  saved: [mode: 'new' | 'overwrite', format: string]
}>()

const MIN_BOX = 0.03
const RATIOS: Record<string, number | undefined> = {
  free: undefined,
  '1:1': 1,
  '4:3': 4 / 3,
  '3:2': 3 / 2,
  '16:9': 16 / 9,
}

const source = shallowRef<HTMLImageElement>()
const originalBytes = ref(0)
const rotation = ref<Rotation>(0)
const crop = ref<Crop>({ ...FULL_CROP })
const ratio = ref('free')
const limitOn = ref(false)
const limitValue = ref('500')
const limitUnit = ref<'KB' | 'MB'>('KB')
const loadFailed = ref(false)
const saving = ref(false)
const confirmOverwrite = ref(false)
const newName = ref(`${props.photo.photoName}_edited`)
const estimate = ref<Encoded>()
const estimating = ref(false)

const stage = useTemplateRef<HTMLElement>('stage')
const rotated = shallowRef<HTMLCanvasElement>()

const maxBytes = computed(() => {
  const n = Number(limitValue.value)
  return limitOn.value && n > 0
    ? Math.round(n * (limitUnit.value === 'MB' ? 1024 * 1024 : 1024))
    : undefined
})
/** Same rule as the backend: no path separators or control characters. */
const nameInvalid = computed(
  () => !newName.value.trim() || /[/\\\0\r\n]/.test(newName.value) || newName.value.length > 250,
)
const limitInvalid = computed(() => limitOn.value && maxBytes.value === undefined)
const encoding = computed(() => chooseEncoding(props.photo.format, limitOn.value))
const unchanged = computed(
  () =>
    rotation.value === 0 &&
    !limitOn.value &&
    crop.value.x === 0 &&
    crop.value.y === 0 &&
    crop.value.w === 1 &&
    crop.value.h === 1,
)
const canSave = computed(
  () => !!source.value && !unchanged.value && !limitInvalid.value && !saving.value,
)
const outputSize = computed(() => {
  if (!source.value) return undefined
  const { width, height } = rotatedSize(
    source.value.naturalWidth,
    source.value.naturalHeight,
    rotation.value,
  )
  return { width: Math.round(width * crop.value.w), height: Math.round(height * crop.value.h) }
})

onMounted(async () => {
  try {
    const { data } = await http.get<Blob>(photoUrl(props.photo.photoId), { responseType: 'blob' })
    originalBytes.value = data.size
    const url = URL.createObjectURL(data)
    const img = new Image()
    img.src = url
    try {
      await img.decode()
    } finally {
      URL.revokeObjectURL(url)
    }
    source.value = img
  } catch {
    loadFailed.value = true
    toast.error('Could not load the image for editing')
  }
})

// The visible preview is the rotated canvas itself, so crop fractions map straight onto it.
watch(
  [source, rotation],
  () => {
    if (!source.value) return
    rotated.value = rotateToCanvas(
      source.value,
      source.value.naturalWidth,
      source.value.naturalHeight,
      rotation.value,
    )
  },
  { immediate: true },
)
watch([rotated, stage], () => {
  if (!rotated.value || !stage.value) return
  rotated.value.className = 'block max-h-[65vh] max-w-full select-none'
  stage.value.replaceChildren(rotated.value)
})

/** Renders the current edit to a canvas and encodes it (to the size limit, if one is set). */
async function build() {
  const base = rotated.value!
  const canvas = cropToCanvas(base, crop.value, 1, encoding.value.mime === 'image/jpeg')
  return encodeToLimit(canvas, encoding.value, maxBytes.value)
}

let timer: ReturnType<typeof setTimeout> | undefined
let run = 0
watch([rotated, crop, limitOn, maxBytes, encoding], () => {
  clearTimeout(timer)
  estimate.value = undefined
  if (!rotated.value || unchanged.value || limitInvalid.value) return (estimating.value = false)
  estimating.value = true
  timer = setTimeout(async () => {
    const id = ++run
    try {
      const result = await build()
      if (id === run) estimate.value = result
    } catch {
      // The estimate is a preview only; saving reports encoding errors.
    } finally {
      if (id === run) estimating.value = false
    }
  }, 300)
})
onBeforeUnmount(() => {
  clearTimeout(timer)
  stopDrag?.()
})

function rotate(clockwise: boolean) {
  rotation.value = ((rotation.value + (clockwise ? 90 : 270)) % 360) as Rotation
  crop.value = rotateCrop(crop.value, clockwise)
}

function pickRatio(value: string) {
  ratio.value = value
  const r = RATIOS[value]
  if (r === undefined || !outputSizeOfRotated.value) return
  crop.value = cropForRatio(r, outputSizeOfRotated.value.width, outputSizeOfRotated.value.height)
}
const outputSizeOfRotated = computed(() =>
  source.value
    ? rotatedSize(source.value.naturalWidth, source.value.naturalHeight, rotation.value)
    : undefined,
)

function resetCrop() {
  ratio.value = 'free'
  crop.value = { ...FULL_CROP }
}

type Handle = 'move' | 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw'
// Corners are drawn as L-shaped brackets; edges are invisible grab strips between them (they only
// change the cursor). All sit inside the box so the stage's overflow-hidden never clips them.
const BRACKET = 'size-7 border-white drop-shadow-[0_0_2px_rgba(0,0,0,0.7)]'
const HANDLES: { handle: Handle; class: string }[] = [
  { handle: 'n', class: 'inset-x-7 top-0 h-3 cursor-ns-resize' },
  { handle: 's', class: 'inset-x-7 bottom-0 h-3 cursor-ns-resize' },
  { handle: 'w', class: 'inset-y-7 left-0 w-3 cursor-ew-resize' },
  { handle: 'e', class: 'inset-y-7 right-0 w-3 cursor-ew-resize' },
  {
    handle: 'nw',
    class: `${BRACKET} left-0 top-0 border-l-[3px] border-t-[3px] cursor-nwse-resize`,
  },
  {
    handle: 'ne',
    class: `${BRACKET} right-0 top-0 border-r-[3px] border-t-[3px] cursor-nesw-resize`,
  },
  {
    handle: 'sw',
    class: `${BRACKET} bottom-0 left-0 border-b-[3px] border-l-[3px] cursor-nesw-resize`,
  },
  {
    handle: 'se',
    class: `${BRACKET} bottom-0 right-0 border-b-[3px] border-r-[3px] cursor-nwse-resize`,
  },
]
const GRID = [
  'inset-y-0 left-1/3 w-px',
  'inset-y-0 left-2/3 w-px',
  'inset-x-0 top-1/3 h-px',
  'inset-x-0 top-2/3 h-px',
]

let stopDrag: (() => void) | undefined
function startDrag(handle: Handle, event: PointerEvent) {
  if (!stage.value) return
  event.preventDefault()
  stopDrag?.()
  const rect = stage.value.getBoundingClientRect()
  const start = { ...crop.value }
  const move = (e: PointerEvent) => {
    const dx = (e.clientX - event.clientX) / rect.width
    const dy = (e.clientY - event.clientY) / rect.height
    let { x, y, w, h } = start
    if (handle === 'move') {
      x = Math.min(1 - w, Math.max(0, start.x + dx))
      y = Math.min(1 - h, Math.max(0, start.y + dy))
    } else {
      if (handle.includes('w')) {
        x = Math.min(start.x + start.w - MIN_BOX, Math.max(0, start.x + dx))
        w = start.x + start.w - x
      }
      if (handle.includes('e')) w = Math.min(1 - start.x, Math.max(MIN_BOX, start.w + dx))
      if (handle.includes('n')) {
        y = Math.min(start.y + start.h - MIN_BOX, Math.max(0, start.y + dy))
        h = start.y + start.h - y
      }
      if (handle.includes('s')) h = Math.min(1 - start.y, Math.max(MIN_BOX, start.h + dy))
    }
    ratio.value = 'free'
    crop.value = clampCrop({ x, y, w, h }, MIN_BOX)
  }
  stopDrag = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stopDrag!)
    window.removeEventListener('pointercancel', stopDrag!)
    stopDrag = undefined
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stopDrag)
  window.addEventListener('pointercancel', stopDrag)
}

const pct = (n: number) => `${n * 100}%`
const boxStyle = computed(() => ({
  left: pct(crop.value.x),
  top: pct(crop.value.y),
  width: pct(crop.value.w),
  height: pct(crop.value.h),
}))

async function save(mode: 'new' | 'overwrite') {
  confirmOverwrite.value = false
  if (!canSave.value || (mode === 'new' && nameInvalid.value)) return
  saving.value = true
  try {
    const { blob } = await build()
    const { ext } = encoding.value
    const form = new FormData()
    if (mode === 'new') {
      form.append('file', blob, `${newName.value.trim()}.${ext}`)
      form.append('photo', JSON.stringify({ nsId: props.photo.nsId, visibility: 0 }))
      await call({ method: 'post', url: '/photo/insert', data: form })
      toast.success('Saved as a new image (private)')
    } else {
      form.append('photoId', String(props.photo.photoId))
      form.append('file', blob, `${props.photo.photoName}.${ext}`)
      await call({ method: 'post', url: '/photo/replace', data: form })
      toast.success('Image overwritten')
    }
    emit('saved', mode, ext)
  } catch {
    toast.error(mode === 'new' ? 'Saving the new image failed' : 'Overwriting the image failed')
  } finally {
    saving.value = false
  }
}

const label = 'text-sm font-medium'
</script>

<template>
  <div class="grid gap-4 md:grid-cols-[1fr_20rem]">
    <div class="flex min-h-64 items-start justify-center">
      <p v-if="loadFailed" class="text-muted-foreground self-center text-sm">
        This image can't be edited.
      </p>
      <Skeleton v-else-if="!rotated" class="h-[60vh] w-full" />
      <div v-show="rotated" class="relative inline-block touch-none overflow-hidden rounded-md">
        <div ref="stage" data-testid="editor-stage" class="relative" />
        <template v-if="rotated">
          <div
            data-testid="crop-box"
            class="absolute cursor-move outline outline-1 outline-white/80 shadow-[0_0_0_9999px_rgba(0,0,0,0.55)]"
            :style="boxStyle"
            @pointerdown="startDrag('move', $event)"
          >
            <!-- Rule-of-thirds guides. -->
            <span
              v-for="g in GRID"
              :key="g"
              :class="['pointer-events-none absolute bg-white/35', g]"
            />
            <span
              v-for="h in HANDLES"
              :key="h.handle"
              :data-handle="h.handle"
              :class="['absolute', h.class]"
              @pointerdown.stop="startDrag(h.handle, $event)"
            />
          </div>
        </template>
      </div>
    </div>

    <div class="space-y-3">
      <h2 class="text-xl font-semibold">Edit Image</h2>

      <section class="space-y-2 rounded-lg border p-3">
        <p :class="label">Rotate</p>
        <div class="flex gap-2">
          <Button variant="secondary" size="sm" :disabled="!rotated" @click="rotate(false)">
            <RotateCcw /> Left
          </Button>
          <Button variant="secondary" size="sm" :disabled="!rotated" @click="rotate(true)">
            <RotateCw /> Right
          </Button>
        </div>
      </section>

      <section class="space-y-2 rounded-lg border p-3">
        <p :class="label">Crop</p>
        <p class="text-muted-foreground text-xs">Drag the box or its handles on the image.</p>
        <div class="flex gap-2">
          <Select :model-value="ratio" @update:model-value="pickRatio(String($event))">
            <SelectTrigger class="flex-1" aria-label="Aspect ratio"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="free">Free</SelectItem>
              <SelectItem v-for="r in Object.keys(RATIOS).slice(1)" :key="r" :value="r">{{
                r
              }}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="secondary" size="sm" :disabled="!rotated" @click="resetCrop"
            >Reset</Button
          >
        </div>
        <p v-if="outputSize" class="text-sm">
          Cropped size: {{ outputSize.width }} × {{ outputSize.height }} px
        </p>
      </section>

      <section class="space-y-2 rounded-lg border p-3">
        <label class="flex items-center justify-between gap-2" :class="label">
          Reduce file size
          <Switch v-model="limitOn" aria-label="Limit file size" />
        </label>
        <template v-if="limitOn">
          <div class="flex gap-2">
            <Input
              v-model="limitValue"
              type="number"
              min="1"
              placeholder="Max size"
              aria-label="Maximum file size"
              class="flex-1"
            />
            <Select v-model="limitUnit">
              <SelectTrigger class="w-24" aria-label="Size unit"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="KB">KB</SelectItem>
                <SelectItem value="MB">MB</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <p v-if="limitInvalid" class="text-destructive text-xs">Enter a size above 0.</p>
          <p v-if="photo.format?.toLowerCase() === 'png'" class="text-muted-foreground text-xs">
            PNG can't be compressed this way; the result is saved as JPEG.
          </p>
        </template>
      </section>

      <p class="text-muted-foreground text-sm" aria-live="polite">
        Original: {{ originalBytes ? formatBytes(originalBytes) : '…' }}
        <template v-if="estimating"> · Calculating…</template>
        <template v-else-if="estimate">
          · Result: {{ formatBytes(estimate.blob.size) }} ({{ estimate.width }} ×
          {{ estimate.height }} px<template v-if="estimate.quality !== undefined"
            >, quality {{ Math.round(estimate.quality * 100) }}%</template
          >)
        </template>
      </p>
      <p v-if="estimate?.scaledDown" class="text-xs text-amber-600">
        Quality alone couldn't reach the limit, so the dimensions were reduced.
      </p>
      <p v-if="estimate?.overLimit" class="text-destructive text-xs">
        Couldn't get under the limit; this is the smallest result possible.
      </p>

      <label class="block space-y-1" :class="label">
        Name for a new image
        <Input v-model="newName" placeholder="New image name" aria-label="New image name" />
      </label>
      <p v-if="nameInvalid" class="text-destructive text-xs">
        Enter a name without slashes or line breaks.
      </p>

      <div class="flex flex-wrap gap-2">
        <Button :disabled="!canSave || nameInvalid" @click="save('new')">Save as new image</Button>
        <Button variant="destructive" :disabled="!canSave" @click="confirmOverwrite = true"
          >Overwrite original</Button
        >
        <Button variant="outline" :disabled="saving" @click="emit('cancel')">Cancel</Button>
      </div>
    </div>

    <AlertDialog v-model:open="confirmOverwrite">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Overwrite the original?</AlertDialogTitle>
          <AlertDialogDescription
            >The stored image will be replaced by your edited version. Its name, tags, galleries and
            share link stay the same, but the original pixels and any embedded EXIF data can't be
            recovered.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="save('overwrite')">Overwrite</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
