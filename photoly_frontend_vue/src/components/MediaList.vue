<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  Check,
  Download,
  Eye,
  File,
  Film,
  FolderInput,
  Play,
  SquareCheck,
  Trash2,
} from '@lucide/vue'
import EmptyState from '@/components/EmptyState.vue'
import { startDrag } from '@/composables/dragDrop'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
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
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { usePhotos, type Photo } from '@/composables/usePhotos'
import { viewMode } from '@/composables/useViewMode'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { cn } from '@/lib/utils'
import { photoUrl } from '@/lib/photoUrl'
import { useAuthStore } from '@/stores/auth'

/** `dragFolders`: selected folders that travel along when a selected photo is dragged. */
const props = withDefaults(
  defineProps<{
    nsOrNot: boolean
    allowSimilar?: boolean
    detachEnabled?: boolean
    detachStr?: string
    dragFolders?: number[]
    /** The parent renders one shared selection bar instead (see HomePart). */
    hideBar?: boolean
    /** Section heading. */
    heading?: string
    /** Split the grid into days by upload date (the library); search keeps its ranking. */
    groupByDay?: boolean
    /** The parent shows the grid / list switch in its own header instead. */
    hideToggle?: boolean
    emptyTitle?: string
    emptyDescription?: string
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

/** Shared with the parent so one select mode covers folders and photos. Entered via the right-click menu. */
const selectMode = defineModel<boolean>('selectMode', { default: false })
watch(selectMode, (on) => on || setChecked(false))
const viewing = ref<Photo | null>(null)
/** Where the clicked thumbnail sits, so the viewer can grow out of it. */
const origin = ref<DOMRect | null>(null)
const nsPath = ref('')
watch(
  () => auth.token,
  () => {
    viewing.value = null
    photos.value = []
    nsPath.value = ''
  },
)
const confirmDelete = ref(false)
const confirmDetach = ref(false)
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
/** The photos the next folder pick applies to. */
const moving = ref<Photo[]>([])

/** `uploadDate` arrives as an ISO timestamp ("2024-06-01T10:20:30"); show just the day, in the local format. */
function formatDate(iso?: string) {
  const [y, m, d] = (iso ?? '').slice(0, 10).split('-').map(Number)
  if (!y || !m || !d) return ''
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

const checked = computed(() => photos.value.filter((p) => p.checked))

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

const groups = computed(() => {
  if (!props.groupByDay) return [{ key: 'all', label: '', items: photos.value }]
  const byDay = new Map<string, Photo[]>()
  for (const p of photos.value) {
    const key = (p.uploadDate ?? '').slice(0, 10)
    byDay.set(key, [...(byDay.get(key) ?? []), p])
  }
  return [...byDay.entries()]
    .sort(([a], [b]) => b.localeCompare(a))
    .map(([key, items]) => ({ key, label: formatDay(key) || 'Undated', items }))
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

function open(photo: Photo, e?: MouseEvent) {
  if (selectMode.value) {
    photo.checked = !photo.checked
    return
  }
  origin.value =
    (e?.currentTarget as HTMLElement | null)?.querySelector('img')?.getBoundingClientRect() ?? null
  viewing.value = photo
  if (!props.nsOrNot && photo.nsId !== undefined) trace(photo.nsId)
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
  selectMode.value = false
  const targets = [...checked.value]
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
  startDrag(
    e,
    {
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

defineExpose({
  checkedCount: computed(() => checked.value.length),
  setChecked,
  deleteChecked,
  moveChecked(targetNsId: number) {
    moving.value = [...checked.value]
    return moveChosen(targetNsId)
  },
  /** Photos checked in select mode (empty when it's off). */
  checkedIds: () => (selectMode.value ? checked.value.map((p) => p.photoId) : []),
  updateNsId(id: number) {
    if (props.nsOrNot) trace(id)
  },
  updatePhotoList(list: Photo[]) {
    photos.value = list
  },
})
</script>

<template>
  <section class="flex flex-col gap-5" :aria-label="heading">
    <div class="flex flex-wrap items-baseline gap-2">
      <h2 class="text-base font-semibold">{{ heading }}</h2>
      <span v-if="photos.length" class="text-muted-foreground text-sm tabular-nums">{{
        photos.length
      }}</span>
      <div class="ml-auto flex items-center gap-1 self-center">
        <button
          v-if="photos.length && !groupByDay"
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

    <EmptyState
      v-if="!photos.length"
      :title="emptyTitle"
      :description="emptyDescription"
      class="py-10"
    />
    <div v-for="group in groups" :key="group.key" class="flex flex-col gap-2.5">
      <div v-if="groupByDay" class="flex items-baseline gap-2">
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
      <ul
        :class="
          viewMode === 'list'
            ? 'flex flex-col gap-1'
            : '-mx-4 grid grid-cols-3 gap-0.5 sm:mx-0 sm:grid-cols-4 sm:gap-1 lg:grid-cols-5 xl:grid-cols-6'
        "
      >
        <li
          v-for="(photo, i) in group.items"
          :key="photo.photoId"
          class="animate-rise relative [content-visibility:auto]"
          :style="{ animationDelay: `${Math.min(i, 12) * 25}ms` }"
        >
          <ContextMenu>
            <ContextMenuTrigger as-child>
              <button
                type="button"
                :aria-pressed="selectMode ? !!photo.checked : undefined"
                :class="
                  cn(
                    'group relative block w-full min-w-0 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
                    viewMode === 'list'
                      ? 'bg-card hover:bg-accent flex items-center gap-3 rounded-lg border p-2 transition-colors'
                      : 'bg-muted aspect-square overflow-hidden sm:rounded-[4px]',
                    viewMode === 'list' && photo.checked && 'border-primary bg-accent',
                  )
                "
                draggable="true"
                @dragstart="dragPhoto($event, photo)"
                @click="open(photo, $event)"
                @contextmenu.stop
              >
                <img
                  v-if="mediaKind(photo.format) === 'photo'"
                  :src="photoUrl(photo.photoId, 'renderThumbnail')"
                  alt=""
                  loading="lazy"
                  decoding="async"
                  draggable="false"
                  :class="
                    viewMode === 'list'
                      ? 'size-10 shrink-0 rounded-md object-cover'
                      : cn(
                          'size-full object-cover transition-transform duration-350 ease-(--ease-out-quart) group-hover:scale-[1.04]',
                          photo.checked && '!scale-[0.92] sm:rounded-[2px]',
                        )
                  "
                />
                <span
                  v-else
                  :class="
                    cn(
                      'text-muted-foreground grid place-items-center',
                      viewMode === 'list' ? 'bg-muted size-10 shrink-0 rounded-md' : 'size-full',
                    )
                  "
                >
                  <component
                    :is="mediaKind(photo.format) === 'video' ? Film : File"
                    :class="viewMode === 'list' ? 'size-4' : 'size-8'"
                    aria-hidden="true"
                  />
                </span>
                <span
                  v-if="viewMode === 'grid' && mediaKind(photo.format) === 'video'"
                  class="pointer-events-none absolute right-1.5 bottom-1.5 flex items-center gap-1 rounded-full bg-black/60 px-1.5 py-0.5 text-[11px] leading-4 font-medium text-white"
                  ><Play class="size-2.5 fill-current" aria-hidden="true" />Video</span
                >
                <!-- The name rides along on hover in the grid, and is the row label in the list. -->
                <span
                  :class="
                    viewMode === 'list'
                      ? 'min-w-0 flex-1 truncate text-sm'
                      : 'pointer-events-none absolute inset-x-0 bottom-0 truncate bg-gradient-to-t from-black/60 to-transparent px-2 pt-6 pb-1.5 text-xs text-white opacity-0 transition-opacity duration-200 group-hover:opacity-100 group-focus-visible:opacity-100'
                  "
                  :title="`${photo.photoName}.${photo.format}`"
                  >{{ photo.photoName }}.{{ photo.format }}</span
                >
                <span
                  v-if="viewMode === 'list'"
                  class="text-muted-foreground hidden shrink-0 pr-1 text-xs sm:block"
                  >{{ formatDate(photo.uploadDate) }}</span
                >
                <span
                  v-if="selectMode"
                  aria-hidden="true"
                  :class="
                    cn(
                      'animate-in fade-in zoom-in-75 grid size-[22px] shrink-0 place-items-center rounded-full transition-colors',
                      viewMode === 'list' ? 'order-first ml-1' : 'absolute top-2 left-2',
                      photo.checked
                        ? 'bg-primary text-primary-foreground ring-2 ring-white'
                        : 'border-2 border-white bg-black/20 shadow-sm',
                      viewMode === 'list' && !photo.checked && 'border-input bg-card',
                    )
                  "
                  ><Check v-if="photo.checked" class="size-3.5" stroke-width="3" aria-hidden="true"
                /></span>
              </button>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem @select="selectPhoto(photo)">
                <SquareCheck /> {{ photo.checked ? 'Deselect' : 'Select' }}
              </ContextMenuItem>
              <ContextMenuItem @select="open(photo)"><Eye /> View</ContextMenuItem>
              <ContextMenuItem @select="downloadPhoto(photo)"
                ><Download /> Download</ContextMenuItem
              >
              <ContextMenuItem @select="askMove([photo])"><FolderInput /> Move To</ContextMenuItem>
              <ContextMenuItem
                variant="destructive"
                @select="
                  applyToAll(
                    [photo],
                    (p) => remove(p.photoId),
                    'Photo deleted!',
                    'delete photo Response Error!',
                  )
                "
              >
                <Trash2 /> Delete
              </ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        </li>
      </ul>
    </div>

    <SelectionBar
      v-if="!hideBar"
      :open="selectMode"
      :count="checked.length"
      @close="selectMode = false"
    >
      <button type="button" @click="setChecked(true)">Select All</button>
      <button type="button" :disabled="!checked.length" @click="askMove([...checked])">Move</button>
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
        Delete Selected
      </button>
    </SelectionBar>

    <PhotoViewer
      :allow-similar="allowSimilar !== false"
      :photo="viewing"
      :origin="origin"
      :ns-path="nsPath"
      @close="viewing = null"
      @changed="(change) => emit('queryPhotos', change)"
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
      :item-name="moving.length === 1 ? `${moving[0]!.photoName}.${moving[0]!.format}` : undefined"
      @move-to-update="moveChosen"
    />
  </section>
</template>
