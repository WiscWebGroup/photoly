<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { toast } from 'vue-sonner'
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Download,
  Folder,
  FolderInput,
  Images,
  Info,
  FolderOpen,
  FolderPlus,
  LayoutGrid,
  List,
  Loader2,
  Pencil,
  Plus,
  SquareCheck,
  Trash2,
  Upload,
  X,
} from '@lucide/vue'
import EmptyState from '@/components/EmptyState.vue'
import MediaList from '@/components/MediaList.vue'
import FolderDetails from '@/components/FolderDetails.vue'
import { DRAG_TYPE, dragging, startDrag, type DragPayload } from '@/composables/dragDrop'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
import PageHeader from '@/components/PageHeader.vue'
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
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { isValidFolderName, useFolders, type Folder as FolderInfo } from '@/composables/useFolders'
import { usePhotos } from '@/composables/usePhotos'
import { useCovers } from '@/composables/useCovers'
import { viewMode } from '@/composables/useViewMode'
import { download } from '@/lib/download'
import { photoUrl } from '@/lib/photoUrl'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { cn } from '@/lib/utils'
import { useRoute } from 'vue-router'
import { useUploadsStore } from '@/stores/uploads'
import { useAuthStore } from '@/stores/auth'

const uploads = useUploadsStore()
const route = useRoute()

const auth = useAuthStore()
const folders = useFolders()
const library = usePhotos()
const mediaList = ref<InstanceType<typeof MediaList>>()
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()
const detailsFolder = ref<FolderInfo | null>(null)
let detailsFocus: HTMLElement | null = null
function showDetails(folder: FolderInfo) {
  detailsFocus = document.querySelector<HTMLElement>(`[data-folder-id="${folder.nsId}"]`)
  detailsFolder.value = folder
}
async function closeDetails() {
  detailsFolder.value = null
  await nextTick()
  detailsFocus?.focus()
}

const nsId = computed(() => folders.current.value?.nsId ?? -1)
const isRoot = computed(() => folders.path.value.length <= 1)
/** The root folder reads as "Library" everywhere it's shown. */
const folderName = (f: FolderInfo, i: number) => (i === 0 ? 'Library' : f.nsName)
const title = computed(() =>
  isRoot.value ? 'Library' : (folders.current.value?.nsName ?? 'Library'),
)
const parent = computed(() => {
  const path = folders.path.value
  return path.length > 1 ? { folder: path[path.length - 2]!, i: path.length - 2 } : null
})

const plural = (n: number, one: string) => `${n} ${one}${n === 1 ? '' : 's'}`
const summary = computed(() => {
  const media = library.photos.value
  const videos = media.filter((p) => mediaKind(p.format) === 'video').length
  return [
    plural(folders.children.value.length, 'folder'),
    plural(media.length - videos, 'photo'),
    videos && plural(videos, 'video'),
  ]
    .filter(Boolean)
    .join(' · ')
})

// Folder cards show a mosaic of what's inside and how many items; refetched when the folder's
// children change or photos move in or out of it.
const { covers, refresh: refreshCovers } = useCovers('folder')
watchDebounced(
  [folders.children, library.photos],
  () => refreshCovers(folders.children.value.map((f) => f.nsId)),
  { debounce: 60 },
)

// --- Navigation ---------------------------------------------------------------------------------
async function queryPhotos() {
  try {
    await library.loadByFolder(nsId.value)
    mediaList.value?.updatePhotoList(library.photos.value)
  } catch {
    toast.error('query photo Response Error!')
  }
}

async function go(id: number, name = '') {
  if (id === nsId.value) return
  selected.value = []
  try {
    const opened = folders.open(id, name)
    mediaList.value?.updateNsId(id)
    await Promise.all([opened, queryPhotos()])
  } catch {
    toast.error('query folder Response Error!')
  }
}

onMounted(async () => {
  try {
    if (route.query.folder && Number(route.query.folder) > 0)
      await folders.open(Number(route.query.folder))
    else await folders.openRoot()
    mediaList.value?.updateNsId(nsId.value)
    await queryPhotos()
  } catch {
    toast.error('get root folder Response Error!')
  }
})

/** Runs a folder mutation, then reports the outcome. Returns whether it succeeded. */
async function mutate(action: () => Promise<unknown>, success: string, failure: string) {
  try {
    await action()
    toast.success(success)
    return true
  } catch {
    toast.warning(failure)
    return false
  }
}

// --- Folder selection ---------------------------------------------------------------------------
const selectMode = ref(false)
const selected = ref<number[]>([])
watch(selectMode, (on) => on || (selected.value = []))
const confirmDeleteSelected = ref(false)

const toggleSelected = (id: number) =>
  (selected.value = selected.value.includes(id)
    ? selected.value.filter((s) => s !== id)
    : [...selected.value, id])

function openFolder(folder: FolderInfo) {
  if (selectMode.value) toggleSelected(folder.nsId)
  else go(folder.nsId, folder.nsName)
}

function selectFolder(folder: FolderInfo) {
  toggleSelected(folder.nsId)
  selectMode.value = true
}

const selectedCount = computed(() => selected.value.length + (mediaList.value?.checkedCount ?? 0))

function clearSelection() {
  selected.value = []
  mediaList.value?.setChecked(false)
}

function selectAll() {
  selected.value = folders.children.value.map((f) => f.nsId)
  mediaList.value?.setChecked(true)
}

async function deleteSelected() {
  confirmDeleteSelected.value = false
  const ids = selected.value
  // Photos first: leaving select mode clears their checkmarks.
  const photosDeleted = mediaList.value?.checkedCount ? mediaList.value.deleteChecked() : undefined
  selectMode.value = false
  await Promise.all([
    photosDeleted,
    ids.length &&
      mutate(() => Promise.all(ids.map(folders.remove)), 'Folder deleted!', 'Delete Error!'),
  ])
}

/** Galleries a selection can be added to; smart galleries fill themselves. */
const galleries = ref<{ gaId: number; gaName: string }[]>([])
async function loadGalleries() {
  try {
    galleries.value = await call<{ gaId: number; gaName: string }[]>({ url: '/gallery/getAll' })
  } catch {
    toast.error('get galleries Response Error!')
  }
}

async function addSelectedTo(gaId: number, name: string) {
  selected.value = []
  await mediaList.value?.addCheckedToGallery(gaId, name)
}

/** Selected photos download one by one; selected folders download as zips. */
async function downloadSelected() {
  const ids = selected.value
  const photos = mediaList.value?.downloadChecked()
  for (const id of ids) {
    const folder = folders.children.value.find((f) => f.nsId === id)
    if (folder) await downloadFolder(folder)
  }
  await photos
}

// --- Create / rename / delete / move one folder ---------------------------------------------------
const creating = ref(false)
const newName = ref('')
const renaming = ref<FolderInfo | null>(null)
const renameTo = ref('')
const deleting = ref<FolderInfo | null>(null)
const movingFolder = ref<FolderInfo | null>(null)

function startCreate() {
  newName.value = ''
  creating.value = true
}

async function createFolder() {
  if (!isValidFolderName(newName.value)) return toast.warning('Please input a valid folder name!')
  if (await mutate(() => folders.create(newName.value), 'Folder created!', 'Creation Error!'))
    creating.value = false
}

function startRename(folder: FolderInfo) {
  renaming.value = folder
  renameTo.value = folder.nsName
}

async function renameFolder() {
  const folder = renaming.value
  if (!folder) return
  if (!isValidFolderName(renameTo.value)) return toast.warning('Please input a valid folder name!')
  if (
    await mutate(
      () => folders.rename(folder.nsId, renameTo.value),
      'Folder Name Updated!',
      'Update Error!',
    )
  )
    renaming.value = null
}

async function deleteFolder() {
  const folder = deleting.value
  if (
    folder &&
    (await mutate(() => folders.remove(folder.nsId), 'Folder deleted!', 'Delete Error!'))
  )
    deleting.value = null
}

function startMove(folder: FolderInfo) {
  movingFolder.value = folder
  movePicker.value?.openModal()
}

/** Move every selected folder and photo; `movingFolder` is null for this. */
function startMoveSelected() {
  movingFolder.value = null
  movePicker.value?.openModal()
}

async function moveFolder(parentId: number) {
  const folder = movingFolder.value
  if (folder)
    return mutate(
      () => folders.move(folder.nsId, parentId),
      'Folder Location Updated!',
      'Update Error!',
    )
  const ids = selected.value.filter((id) => id !== parentId)
  const photosMoved =
    mediaList.value?.checkedCount && parentId !== nsId.value
      ? mediaList.value.moveChecked(parentId)
      : undefined
  selectMode.value = false
  await Promise.all([
    photosMoved,
    ids.length &&
      mutate(
        () => Promise.all(ids.map((id) => folders.move(id, parentId))),
        'Folder Location Updated!',
        'Update Error!',
      ),
  ])
}

// --- Drag a folder or photo onto a folder (or breadcrumb) to move it ------------------------------
const dragOver = ref<number | null>(null)

function dragFolder(e: DragEvent, folder: FolderInfo) {
  const multi = selectMode.value && selected.value.includes(folder.nsId)
  startDrag(
    e,
    {
      folders: multi ? selected.value : [folder.nsId],
      photos: multi ? (mediaList.value?.checkedIds() ?? []) : [],
    },
    folder.nsName,
  )
}

function dropOn(target: number) {
  return {
    onDragover(e: DragEvent) {
      if (!e.dataTransfer?.types.includes(DRAG_TYPE)) return
      e.preventDefault()
      dragOver.value = target
    },
    onDragleave() {
      if (dragOver.value === target) dragOver.value = null
    },
    async onDrop(e: DragEvent) {
      dragOver.value = null
      const raw = e.dataTransfer?.getData(DRAG_TYPE)
      if (!raw) return
      e.preventDefault()
      const { folders: dragged, photos }: DragPayload = JSON.parse(raw)
      const folderIds = dragged.filter((id) => id !== target)
      const photoIds = target === nsId.value ? [] : photos
      if (!folderIds.length && !photoIds.length) return
      selectMode.value = false
      const moved = await mutate(
        () =>
          Promise.all([
            ...folderIds.map((id) => folders.move(id, target)),
            ...photoIds.map((id) => library.moveTo(id, target)),
          ]),
        'Location Updated!',
        'Update Error!',
      )
      if (moved && photoIds.length) await queryPhotos()
    },
  }
}

// --- Download a folder ----------------------------------------------------------------------------
const downloading = ref(false)
let abort = new AbortController()

async function downloadFolder(folder: FolderInfo) {
  abort = new AbortController()
  downloading.value = true
  try {
    await download(`/namespace/downloadNs/${auth.token}`, `${folder.nsName}.zip`, {
      params: { nsId: folder.nsId },
      signal: abort.signal,
    })
  } catch (e) {
    if (!abort.signal.aborted) toast.error(String(e))
  } finally {
    downloading.value = false
  }
}

function abortDownload() {
  abort.abort()
  downloading.value = false
  toast.warning('Download Request Canceled')
}

// Upload state belongs to the application, so navigating never cancels a transfer.
function startUpload() {
  uploads.start(nsId.value, title.value)
}
watch(
  () => uploads.revision,
  async () => {
    if (nsId.value > 0) await Promise.all([folders.refresh(), queryPhotos()])
  },
)
watch(
  () => route.query.folder,
  async (value) => {
    if (value && Number(value) > 0) await go(Number(value))
  },
)
</script>

<template>
  <div class="flex min-h-full flex-col">
    <!-- Where you are, plus a way to search, pinned over the library on wide screens. -->
    <div
      class="bg-card sticky top-0 z-20 hidden h-14 shrink-0 items-center gap-4 border-b px-10 md:flex"
    >
      <nav aria-label="Folder path" class="flex min-w-0 flex-wrap items-center gap-1 text-sm">
        <template v-for="(item, i) in folders.path.value" :key="item.nsId">
          <ChevronRight v-if="i" class="text-muted-foreground/70 size-3.5" aria-hidden="true" />
          <span
            v-if="i === folders.path.value.length - 1"
            aria-current="page"
            class="px-1.5 py-1 font-medium"
            >{{ folderName(item, i) }}</span
          >
          <button
            v-else
            type="button"
            :class="
              cn(
                'text-muted-foreground hover:bg-accent hover:text-accent-foreground rounded-md border border-transparent px-1.5 py-1 transition-colors',
                dragging && 'border-muted-foreground/40 border-dashed',
                dragOver === item.nsId && 'bg-primary/15 border-primary border-solid',
              )
            "
            v-bind="dropOn(item.nsId)"
            @click="go(item.nsId, item.nsName)"
          >
            {{ folderName(item, i) }}
          </button>
        </template>
      </nav>
    </div>

    <div v-if="nsId === -1" class="flex justify-center py-24" role="status" aria-label="Loading">
      <Loader2 class="text-muted-foreground size-8 animate-spin" />
    </div>

    <!-- Right-clicking (or long-pressing) empty space offers upload / new folder. -->
    <ContextMenu>
      <ContextMenuTrigger as-child>
        <div
          v-show="nsId !== -1"
          class="flex w-full max-w-[1280px] flex-1 flex-col gap-8 px-4 pt-4 pb-32 md:gap-10 md:px-10 md:pt-8"
        >
          <div class="flex flex-col gap-1">
            <button
              v-if="parent"
              type="button"
              class="text-muted-foreground -ml-1 flex h-7 items-center gap-0.5 self-start text-sm md:hidden"
              @click="go(parent.folder.nsId, parent.folder.nsName)"
            >
              <ChevronLeft class="size-4" aria-hidden="true" />{{
                folderName(parent.folder, parent.i)
              }}
            </button>
            <PageHeader
              :eyebrow="isRoot ? undefined : 'Folder'"
              :title="title"
              :description="summary"
            >
              <span class="hidden items-center gap-2 md:flex">
                <ViewToggle />
                <Button variant="outline" @click="startCreate"><FolderPlus /> New Folder</Button>
                <Button @click="startUpload"><Upload /> Upload</Button>
              </span>
            </PageHeader>
          </div>

          <section aria-labelledby="folders-h" class="flex flex-col gap-3">
            <div class="flex items-baseline gap-2">
              <h2 id="folders-h" class="text-base font-semibold">Folders</h2>
              <span v-if="folders.children.value.length" class="text-muted-foreground text-sm">{{
                folders.children.value.length
              }}</span>
            </div>

            <ul
              :class="
                viewMode === 'list'
                  ? 'flex flex-col gap-1'
                  : '-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 [scrollbar-width:none] sm:mx-0 sm:grid sm:grid-cols-3 sm:gap-4 sm:overflow-visible sm:px-0 sm:pb-0 lg:grid-cols-4 2xl:grid-cols-6'
              "
            >
              <li
                v-for="(folder, i) in folders.children.value"
                :key="folder.nsId"
                :class="['animate-rise', viewMode === 'grid' && 'w-36 shrink-0 sm:w-auto']"
                :style="{ animationDelay: `${Math.min(i, 12) * 30}ms` }"
              >
                <ContextMenu>
                  <ContextMenuTrigger as-child>
                    <button
                      type="button"
                      :data-folder-id="folder.nsId"
                      :aria-pressed="selectMode ? selected.includes(folder.nsId) : undefined"
                      :class="
                        cn(
                          'group bg-card relative flex w-full text-left text-sm shadow-xs transition-[border-color,box-shadow,translate] duration-200 outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
                          viewMode === 'list'
                            ? 'hover:bg-accent h-11 items-center gap-3 rounded-lg border px-3'
                            : 'hover:border-primary flex-col overflow-hidden rounded-[14px] border hover:-translate-y-0.5 hover:shadow-sm',
                          (selected.includes(folder.nsId) || dragOver === folder.nsId) &&
                            'border-primary ring-primary ring-1',
                        )
                      "
                      draggable="true"
                      v-bind="dropOn(folder.nsId)"
                      @dragstart="dragFolder($event, folder)"
                      @click="openFolder(folder)"
                      @contextmenu.stop
                    >
                      <span
                        v-if="viewMode === 'grid' && covers.get(folder.nsId)?.ids.length"
                        class="bg-muted grid h-24 grid-cols-[2fr_1fr] grid-rows-2 gap-0.5 sm:h-32"
                        aria-hidden="true"
                      >
                        <img
                          v-for="(id, k) in covers.get(folder.nsId)!.ids"
                          :key="id"
                          :src="photoUrl(id, 'renderThumbnail')"
                          alt=""
                          loading="lazy"
                          :class="[
                            'size-full object-cover transition-transform duration-300 group-hover:scale-[1.03]',
                            k === 0 && 'row-span-2',
                          ]"
                        />
                      </span>
                      <span
                        v-else-if="viewMode === 'grid'"
                        class="bg-accent/60 text-primary grid h-24 place-items-center sm:h-32"
                        aria-hidden="true"
                      >
                        <Folder
                          class="size-10 stroke-[1.5] transition-transform duration-300 group-hover:scale-0"
                        />
                        <FolderOpen
                          class="absolute size-10 scale-0 stroke-[1.5] transition-transform duration-300 group-hover:scale-100"
                        />
                      </span>
                      <Folder v-else class="text-primary size-4 shrink-0" aria-hidden="true" />
                      <span
                        :class="
                          viewMode === 'grid'
                            ? 'flex items-center gap-2 px-3.5 py-3'
                            : 'flex min-w-0 flex-1 items-center gap-2'
                        "
                      >
                        <span
                          :class="
                            viewMode === 'grid'
                              ? 'flex min-w-0 flex-col'
                              : 'flex min-w-0 flex-1 items-baseline gap-2'
                          "
                        >
                          <span class="truncate font-medium" :title="folder.nsName">{{
                            folder.nsName
                          }}</span>
                          <span
                            v-if="covers.get(folder.nsId)"
                            class="text-muted-foreground text-xs"
                            >{{ plural(covers.get(folder.nsId)!.count, 'item') }}</span
                          >
                        </span>
                        <span
                          v-if="selectMode"
                          role="checkbox"
                          :aria-checked="selected.includes(folder.nsId)"
                          :aria-label="`Select ${folder.nsName}`"
                          :class="
                            cn(
                              'animate-in fade-in zoom-in-75 ml-auto grid size-5 shrink-0 place-items-center rounded-full border-2 transition-colors',
                              selected.includes(folder.nsId)
                                ? 'bg-primary border-primary text-primary-foreground'
                                : 'border-input',
                            )
                          "
                          @click.stop="toggleSelected(folder.nsId)"
                          ><Check
                            v-if="selected.includes(folder.nsId)"
                            class="size-3"
                            stroke-width="3"
                            aria-hidden="true"
                        /></span>
                      </span>
                    </button>
                  </ContextMenuTrigger>
                  <ContextMenuContent>
                    <ContextMenuItem @select="selectFolder(folder)">
                      <SquareCheck /> {{ selected.includes(folder.nsId) ? 'Deselect' : 'Select' }}
                    </ContextMenuItem>
                    <ContextMenuItem @select="showDetails(folder)"
                      ><Info /> Details</ContextMenuItem
                    >
                    <ContextMenuItem @select="startRename(folder)"
                      ><Pencil /> Rename</ContextMenuItem
                    >
                    <ContextMenuItem @select="downloadFolder(folder)"
                      ><Download /> Download</ContextMenuItem
                    >
                    <ContextMenuItem @select="startMove(folder)"
                      ><FolderInput /> Move To</ContextMenuItem
                    >
                    <ContextMenuItem variant="destructive" @select="deleting = folder"
                      ><Trash2 /> Delete</ContextMenuItem
                    >
                  </ContextMenuContent>
                </ContextMenu>
              </li>
              <li
                v-if="viewMode === 'grid'"
                class="animate-rise w-36 shrink-0 sm:w-auto"
                :style="{ animationDelay: `${Math.min(folders.children.value.length, 12) * 30}ms` }"
              >
                <button
                  type="button"
                  class="text-muted-foreground hover:border-primary hover:text-primary flex size-full min-h-[158px] flex-col items-center justify-center gap-2 rounded-[14px] border border-dashed text-sm transition-colors sm:min-h-[194px]"
                  @click="startCreate"
                >
                  <Plus class="size-5" aria-hidden="true" />
                  New Folder
                </button>
              </li>
            </ul>
            <p
              v-if="viewMode === 'list' && !folders.children.value.length"
              class="text-muted-foreground animate-fade text-sm"
            >
              No folders here yet.
            </p>
          </section>

          <MediaList
            ref="mediaList"
            :context="title"
            :ns-or-not="true"
            :detach-enabled="false"
            v-model:select-mode="selectMode"
            hide-bar
            hide-toggle
            group-by-day
            :drag-folders="selected"
            @query-photos="queryPhotos"
          />
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem @select="startUpload"><Upload /> Upload Photo</ContextMenuItem>
        <ContextMenuItem @select="startCreate"><FolderPlus /> Create Folder</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>

    <!-- Phones add through one floating button instead of the header actions. -->
    <DropdownMenu>
      <DropdownMenuTrigger as-child>
        <button
          v-show="!selectMode"
          type="button"
          aria-label="Add"
          class="bg-primary text-primary-foreground fixed right-4 bottom-[calc(5rem+env(safe-area-inset-bottom))] z-30 grid size-14 place-items-center rounded-2xl shadow-lg transition-transform active:scale-95 data-[state=open]:rotate-45 md:hidden"
        >
          <Plus class="size-6" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" side="top" class="w-48">
        <DropdownMenuItem @select="startUpload"><Upload /> Upload Photos</DropdownMenuItem>
        <DropdownMenuItem @select="startCreate"><FolderPlus /> New Folder</DropdownMenuItem>
        <DropdownMenuItem @select="viewMode = viewMode === 'grid' ? 'list' : 'grid'"
          ><component :is="viewMode === 'grid' ? List : LayoutGrid" />
          {{ viewMode === 'grid' ? 'List View' : 'Grid View' }}</DropdownMenuItem
        >
      </DropdownMenuContent>
    </DropdownMenu>

    <SelectionBar :open="selectMode" :count="selectedCount" @close="selectMode = false">
      <button type="button" @click="selectAll">Select All</button>
      <button type="button" :disabled="!selectedCount" @click="startMoveSelected">Move</button>
      <DropdownMenu @update:open="(o: boolean) => o && loadGalleries()">
        <DropdownMenuTrigger as-child>
          <button type="button" :disabled="!mediaList?.checkedCount">Add to Gallery</button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="center" class="max-h-72 w-56 overflow-y-auto">
          <DropdownMenuItem
            v-for="g in galleries"
            :key="g.gaId"
            @select="addSelectedTo(g.gaId, g.gaName)"
            ><Images /> <span class="truncate">{{ g.gaName }}</span></DropdownMenuItem
          >
          <p v-if="!galleries.length" class="text-muted-foreground px-2 py-1.5 text-sm">
            No galleries yet
          </p>
        </DropdownMenuContent>
      </DropdownMenu>
      <button type="button" :disabled="!selectedCount" @click="downloadSelected">Download</button>
      <button
        type="button"
        class="text-red-300 dark:text-red-700"
        :disabled="!selectedCount"
        @click="confirmDeleteSelected = true"
      >
        Delete
      </button>
    </SelectionBar>

    <Dialog v-model:open="creating">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New Folder</DialogTitle>
          <DialogDescription>It goes inside {{ title }}.</DialogDescription>
        </DialogHeader>
        <form class="flex flex-col gap-2" @submit.prevent="createFolder">
          <Label for="new-folder-name">Name</Label>
          <Input id="new-folder-name" v-model="newName" placeholder="Folder Name" autofocus />
        </form>
        <DialogFooter>
          <Button variant="outline" @click="creating = false">Cancel</Button>
          <Button @click="createFolder">Create</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="!!renaming" @update:open="renaming = null">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Rename Folder</DialogTitle>
          <DialogDescription>Choose a new name for {{ renaming?.nsName }}.</DialogDescription>
        </DialogHeader>
        <form class="flex flex-col gap-2" @submit.prevent="renameFolder">
          <Label for="rename-folder">Name</Label>
          <Input id="rename-folder" v-model="renameTo" placeholder="Folder Name" />
        </form>
        <DialogFooter>
          <Button variant="outline" @click="renaming = null">Cancel</Button>
          <Button @click="renameFolder">Update Name</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {{ deleting?.nsName }}?</AlertDialogTitle>
          <AlertDialogDescription>
            Every photo and folder inside it is deleted too. This can't be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteFolder">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <AlertDialog v-model:open="confirmDeleteSelected">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {{ plural(selectedCount, 'item') }}?</AlertDialogTitle>
          <AlertDialogDescription>
            Selected folders are deleted with everything inside them. This can't be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteSelected">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <Dialog :open="downloading">
      <DialogContent :show-close-button="false" @interact-outside.prevent @escape-key-down.prevent>
        <DialogHeader>
          <DialogTitle>Download Folder</DialogTitle>
          <DialogDescription class="flex items-center gap-2"
            ><Loader2 class="size-4 animate-spin" /> Preparing your download…</DialogDescription
          >
        </DialogHeader>
        <DialogFooter
          ><Button variant="outline" class="w-full" @click="abortDownload"
            >Abort Download</Button
          ></DialogFooter
        >
      </DialogContent>
    </Dialog>

    <MoveToShowFolder
      ref="movePicker"
      :original-ns-id="movingFolder?.nsId"
      :disable-org-ns-id="true"
      :count="movingFolder ? 1 : selectedCount"
      :item-name="movingFolder?.nsName"
      @move-to-update="moveFolder"
    />
    <FolderDetails :folder="detailsFolder" @close="closeDetails" />
  </div>
</template>
