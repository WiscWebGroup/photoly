<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { toast } from 'vue-sonner'
import { ChevronDown } from '@lucide/vue'
import MediaGrid from '@/components/MediaGrid.vue'
import EmptyState from '@/components/EmptyState.vue'
import { startDrag, dragging } from '@/composables/dragDrop'
import { useStorageRevision } from '@/composables/useStorageRevision'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import MediaTaskDialog from '@/components/MediaTaskDialog.vue'
import PhotoViewer from '@/components/PhotoViewer.vue'
import SelectionBar from '@/components/SelectionBar.vue'
import ViewToggle from '@/components/ViewToggle.vue'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { usePhotos, type Photo } from '@/composables/usePhotos'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

/** `dragFolders`: selected folders that travel along when a selected photo is dragged. */
const props = withDefaults(
  defineProps<{
    nsOrNot: boolean
    /** Keep selected records and a stable viewer list when virtual pages leave the cache. */
    retainState?: boolean
    allowSimilar?: boolean
    detachEnabled?: boolean
    detachStr?: string
    dragFolders?: number[]
    /** The parent renders one shared selection bar instead (see HomePart). */
    hideBar?: boolean
    /** Section heading. */
    heading?: string
    totalCount?: number
    /** Split the grid into days by upload date (the library); search keeps its ranking. */
    groupByDay?: boolean
    /** Server-ordered date feed; preserve its order and group by timelineDate. */
    timeline?: boolean
    /** Group by month instead (galleries); day grouping wins when both are set. */
    groupByMonth?: boolean
    /** The parent shows the grid / list switch in its own header instead. */
    hideToggle?: boolean
    emptyTitle?: string
    emptyDescription?: string
    /** What the viewer's back button names ("Library", a gallery…). */
    context?: string
  }>(),
  { allowSimilar: true, heading: 'Media', emptyTitle: 'No photos here' },
)
/** `queryPhotos`: the list changed on the server, reload it. `doDetach`: remove the checked photos from the parent's tag/gallery. */
const emit = defineEmits<{
  queryPhotos: [change?: { type: string; photo: Photo }]
  doDetach: [photos: Photo[]]
}>()

const auth = useAuthStore()
const { photos, moveTo, remove } = usePhotos()
const cachedPhotos = shallowRef<Photo[]>([])

/** Shared with the parent so one select mode covers folders and photos. Entered via the right-click menu. */
const selectMode = defineModel<boolean>('selectMode', { default: false })
watch(
  selectMode,
  (on) => {
    if (!on) {
      setChecked(false)
      if (props.retainState) photos.value = cachedPhotos.value
    }
  },
  { flush: 'sync' },
)
const viewing = ref<Photo | null>(null)
const viewerSnapshot = ref<Photo[]>([])
const viewerPhotos = computed(() =>
  props.retainState && viewing.value ? viewerSnapshot.value : ordered.value,
)
watch(viewing, (value) => {
  if (!value) viewerSnapshot.value = []
})
/** Where the clicked thumbnail sits, so the viewer can grow out of it. */
const origin = ref<DOMRect | null>(null)
const nsPath = ref('')
watch(
  () => auth.token,
  () => {
    viewing.value = null
    photos.value = []
    cachedPhotos.value = []
    viewerSnapshot.value = []
    nsPath.value = ''
  },
)
const confirmDelete = ref(false)
const confirmDetach = ref(false)
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
/** The photos the next folder pick applies to. */
const moving = ref<Photo[]>([])

const checked = computed(() => photos.value.filter((p) => p.checked))
useStorageRevision(
  () => emit('queryPhotos'),
  () =>
    props.nsOrNot ||
    selectMode.value ||
    checked.value.length > 0 ||
    !!viewing.value ||
    dragging.value,
)

/** "Saturday, 1 June 2024" for an ISO date, or '' when there isn't one. */
function formatDay(iso?: string) {
  const [y, m, d] = (iso ?? '').slice(0, 10).split('-').map(Number)
  if (!y || !m || !d) return ''
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

/** Day-grouped lists can be sorted newest or oldest first, or flat by name. */
const sorts = { newest: 'Newest first', oldest: 'Oldest first', name: 'Name' } as const
const sort = ref<keyof typeof sorts>('newest')
const grouped = computed(() => props.groupByDay || props.groupByMonth)
const byDay = computed(() => grouped.value && (props.timeline || sort.value !== 'name'))
const keyLength = computed(() => (props.groupByDay ? 10 : 7))
function formatMonth(key: string) {
  const [y, m] = key.split('-').map(Number)
  return y && m
    ? new Date(y, m - 1, 1).toLocaleDateString(undefined, { month: 'long', year: 'numeric' })
    : ''
}

const groups = computed(() => {
  if (!byDay.value) {
    const items = grouped.value
      ? [...photos.value].sort((a, b) =>
          a.photoName.localeCompare(b.photoName, undefined, { numeric: true }),
        )
      : photos.value
    return [{ key: 'all', label: '', items }]
  }
  const days = new Map<string, Photo[]>()
  for (const p of photos.value) {
    const key = ((props.timeline ? p.timelineDate : p.uploadDate) ?? '').slice(0, keyLength.value)
    days.set(key, [...(days.get(key) ?? []), p])
  }
  const dir = sort.value === 'oldest' ? -1 : 1
  return [...days.entries()]
    .sort(([a], [b]) => (props.timeline ? 0 : dir * b.localeCompare(a)))
    .map(([key, items]) => ({
      key,
      label: (props.groupByDay ? formatDay(key) : formatMonth(key)) || 'Undated',
      items,
    }))
})

/** Selects a whole day, or clears it when it's already all selected. */
function selectGroup(items: Photo[]) {
  const all = items.every((p) => p.checked)
  items.forEach((p) => (p.checked = !all))
  selectMode.value = true
}

function setChecked(value: boolean) {
  photos.value.forEach((p) => (p.checked = value))
}

async function trace(id: number) {
  try {
    const path = await call<{ nsName: string }[]>({ url: '/namespace/trace', params: { nsId: id } })
    nsPath.value =
      '/root' +
      path
        .slice(1)
        .map((n) => `/${n.nsName}`)
        .join('')
  } catch {
    toast.error('trace Response Error!')
  }
}

/** The photos in the order they're shown, for the viewer's prev / next. */
const ordered = computed(() => groups.value.flatMap((g) => g.items))
const list = ref<HTMLElement>()

/** Where a photo's thumbnail is now (scrolled into view), or null when it isn't on screen. */
function locate(photo: Photo) {
  const img = list.value?.querySelector<HTMLElement>(`[data-photo-id="${photo.photoId}"] img`)
  if (!img) return null
  img.scrollIntoView?.({ block: 'nearest' })
  const r = img.getBoundingClientRect()
  return r.width && r.bottom > 0 && r.top < innerHeight ? r : null
}

function view(photo: Photo) {
  if (props.retainState && !viewing.value) viewerSnapshot.value = [...ordered.value]
  viewing.value = photo
  if (!props.nsOrNot && photo.nsId !== undefined) trace(photo.nsId)
}

/** Delete from inside the viewer: step to a neighbour, or close when it was the last one. */
async function deleteViewed(photo: Photo) {
  const at = viewerPhotos.value.findIndex((p) => p.photoId === photo.photoId)
  const next = viewerPhotos.value[at + 1] ?? viewerPhotos.value[at - 1]
  try {
    await remove(photo.photoId)
    viewerSnapshot.value = viewerSnapshot.value.filter((p) => p.photoId !== photo.photoId)
    toast.success('Photo deleted!')
    if (next) view(next)
    else viewing.value = null
  } catch {
    toast.error('delete photo Response Error!')
  }
  emit('queryPhotos')
}

function open(photo: Photo, e?: MouseEvent) {
  if (selectMode.value) {
    photo.checked = !photo.checked
    return
  }
  origin.value =
    (e?.currentTarget as HTMLElement | null)?.querySelector('img')?.getBoundingClientRect() ?? null
  view(photo)
}

/** Runs one request per photo, reports the outcome once, and asks the parent to reload. */
async function applyToAll(
  targets: Photo[],
  action: (p: Photo) => Promise<unknown>,
  success: string,
  failure: string,
) {
  try {
    await Promise.all(targets.map(action))
    toast.success(success)
  } catch {
    toast.warning(failure)
  }
  emit('queryPhotos')
}

function deleteChecked() {
  confirmDelete.value = false
  const targets = [...checked.value]
  selectMode.value = false
  setChecked(false)
  return applyToAll(
    targets,
    (p) => remove(p.photoId),
    'Photo deleted!',
    'delete photo Response Error!',
  )
}

function detachChecked() {
  confirmDetach.value = false
  selectMode.value = false
  emit('doDetach', photos.value)
}

function askMove(targets: Photo[]) {
  if (new Set(targets.map((p) => p.sourceId)).size > 1) {
    toast.error('Select media from one storage source to move.')
    return
  }
  moving.value = targets
  movePicker.value?.openModal()
}

function moveChosen(targetNsId: number) {
  const targets = moving.value
  if (targets.length > 1) {
    selectMode.value = false
    setChecked(false)
  }
  return applyToAll(
    targets,
    (p) => moveTo(p.photoId, targetNsId),
    'Media Location Updated!',
    'Update Error!',
  )
}

function selectPhoto(photo: Photo) {
  photo.checked = !photo.checked
  selectMode.value = true
}

function dragPhoto(e: DragEvent, photo: Photo) {
  const multi = selectMode.value && photo.checked
  if (multi && new Set(checked.value.map((p) => p.sourceId)).size > 1) {
    e.preventDefault()
    toast.error('Select media from one storage source to move.')
    return
  }
  startDrag(
    e,
    {
      sourceId: photo.sourceId,
      folders: multi ? (props.dragFolders ?? []) : [],
      photos: multi ? checked.value.map((p) => p.photoId) : [photo.photoId],
    },
    `${photo.photoName}.${photo.format}`,
  )
}

const downloadPhoto = (p: Photo) =>
  download(`/photo/download/${auth.token}`, `${p.photoName}.${p.format}`, {
    params: { photoId: p.photoId },
  }).catch(() => toast.error('Download failed'))

/** Saves each checked photo, one after another so the browser doesn't drop any. */
async function downloadChecked() {
  for (const p of [...checked.value]) await downloadPhoto(p)
}

const converting = ref<Photo | null>(null)
const convertOpen = ref(false)
const gridActions = {
  convertPhoto: (photo: Photo) => {
    converting.value = photo
    convertOpen.value = true
  },
  open,
  dragPhoto,
  selectPhoto,
  downloadPhoto,
  askMove,
  deletePhoto: (photo: Photo) =>
    applyToAll([photo], (p) => remove(p.photoId), 'Photo deleted!', 'delete photo Response Error!'),
}

defineExpose({
  checkedCount: computed(() => checked.value.length),
  setChecked,
  deleteChecked,
  moveChecked(targetNsId: number) {
    if (new Set(checked.value.map((p) => p.sourceId)).size > 1) {
      toast.error('Select media from one storage source to move.')
      return
    }
    moving.value = [...checked.value]
    return moveChosen(targetNsId)
  },
  downloadChecked,
  addCheckedToGallery(gaId: number, name: string) {
    const targets = [...checked.value]
    selectMode.value = false
    setChecked(false)
    return applyToAll(
      targets,
      (p) =>
        call({ method: 'post', url: '/photo/addToGallery', params: { photoId: p.photoId, gaId } }),
      `Added to ${name}`,
      'add gallery Response Error!',
    )
  },
  /** Photos checked in select mode (empty when it's off). */
  checkedIds: () => (selectMode.value ? checked.value.map((p) => p.photoId) : []),
  updateNsId(id: number) {
    if (props.nsOrNot) trace(id)
  },
  updatePhotoList(list: Photo[]) {
    if (props.retainState) cachedPhotos.value = list
    if (!props.retainState) {
      photos.value = list
      return
    }
    const selected = new Map(checked.value.map((p) => [p.photoId, p]))
    const ids = new Set(list.map((p) => p.photoId))
    photos.value = [
      ...list.map((p) => {
        if (selected.has(p.photoId)) p.checked = true
        return p
      }),
      ...[...selected.values()].filter((p) => !ids.has(p.photoId)),
    ]
  },
})
</script>

<template>
  <section ref="list" class="flex flex-col gap-5" :aria-label="heading">
    <div class="flex flex-wrap items-baseline gap-2">
      <h2 class="text-base font-semibold">{{ heading }}</h2>
      <span v-if="totalCount ?? photos.length" class="text-muted-foreground text-sm tabular-nums">{{
        totalCount ?? photos.length
      }}</span>
      <div class="ml-auto flex items-center gap-1 self-center">
        <template v-if="grouped && photos.length && !timeline">
          <span class="text-muted-foreground text-sm">Sort</span>
          <DropdownMenu>
            <DropdownMenuTrigger
              class="hover:bg-accent flex h-8 items-center gap-1 rounded-md px-2 text-sm font-medium transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              >{{ sorts[sort] }}<ChevronDown class="size-3.5" aria-hidden="true"
            /></DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="w-40">
              <DropdownMenuRadioGroup v-model="sort">
                <DropdownMenuRadioItem v-for="(label, key) in sorts" :key="key" :value="key">{{
                  label
                }}</DropdownMenuRadioItem>
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        </template>
        <button
          v-if="photos.length && !byDay"
          type="button"
          class="text-primary hover:bg-accent h-8 rounded-md px-2 text-sm font-medium transition-colors"
          :aria-pressed="selectMode"
          @click="selectMode = !selectMode"
        >
          {{ selectMode ? 'Cancel' : 'Select' }}
        </button>
        <ViewToggle v-if="!hideToggle" />
      </div>
    </div>

    <slot name="content" :actions="gridActions" :select-mode="selectMode">
      <EmptyState
        v-if="!photos.length"
        :title="emptyTitle"
        :description="emptyDescription"
        class="py-10"
      />
      <div v-for="group in groups" :key="group.key" class="flex flex-col gap-2.5">
        <div v-if="byDay" class="flex items-baseline gap-2">
          <h3 class="text-sm font-medium">{{ group.label }}</h3>
          <span class="text-muted-foreground text-xs tabular-nums">{{ group.items.length }}</span>
          <button
            type="button"
            class="text-primary hover:text-accent-foreground ml-auto rounded-sm text-[13px] font-medium transition-colors"
            @click="selectGroup(group.items)"
          >
            {{ selectMode && group.items.every((p) => p.checked) ? 'Deselect' : 'Select' }}
          </button>
        </div>
        <MediaGrid
          :items="group.items"
          :select-mode="selectMode"
          :timeline="timeline"
          :actions="gridActions"
        />
      </div>
    </slot>

    <SelectionBar
      v-if="!hideBar"
      :open="selectMode"
      :count="checked.length"
      @close="selectMode = false"
    >
      <button type="button" @click="setChecked(true)">Select All</button>
      <button type="button" :disabled="!checked.length" @click="askMove([...checked])">Move</button>
      <button type="button" :disabled="!checked.length" @click="downloadChecked">Download</button>
      <button
        v-if="detachEnabled"
        type="button"
        :disabled="!checked.length"
        @click="confirmDetach = true"
      >
        Remove From {{ detachStr }}
      </button>
      <button
        type="button"
        class="text-red-300 dark:text-red-700"
        :disabled="!checked.length"
        @click="confirmDelete = true"
      >
        Delete
      </button>
    </SelectionBar>

    <MediaTaskDialog v-model:open="convertOpen" :photo="converting" />
    <PhotoViewer
      :allow-similar="allowSimilar !== false"
      :photo="viewing"
      :origin="origin"
      :ns-path="nsPath"
      :photos="viewerPhotos"
      :context="context"
      :locate="locate"
      manage
      @close="viewing = null"
      @changed="(change) => emit('queryPhotos', change)"
      @navigate="view"
      @move="(p) => askMove([p])"
      @delete="deleteViewed"
    />

    <AlertDialog v-model:open="confirmDelete">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle
            >Delete {{ checked.length }}
            {{ checked.length === 1 ? 'item' : 'items' }}?</AlertDialogTitle
          >
          <AlertDialogDescription
            >The selected photos and videos are deleted for good. This can't be
            undone.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteChecked">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <AlertDialog v-model:open="confirmDetach">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Remove from this {{ detachStr?.toLowerCase() }}?</AlertDialogTitle>
          <AlertDialogDescription
            >The photos stay in your library; only this {{ detachStr?.toLowerCase() }} lets go of
            them.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button @click="detachChecked">Remove</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <MoveToShowFolder
      ref="movePicker"
      :count="moving.length"
      :source-id="moving[0]?.sourceId"
      :item-name="moving.length === 1 ? `${moving[0]!.photoName}.${moving[0]!.format}` : undefined"
      @move-to-update="moveChosen"
    />
  </section>
</template>
