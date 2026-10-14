<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  ChevronRight,
  Download,
  Folder,
  FolderInput,
  FolderPlus,
  Loader2,
  Pencil,
  SquareCheck,
  Trash2,
  Upload,
  X,
} from '@lucide/vue'
import EmptyState from '@/components/EmptyState.vue'
import MediaList from '@/components/MediaList.vue'
import { DRAG_TYPE, dragging, startDrag, type DragPayload } from '@/composables/dragDrop'
import MoveToShowFolder from '@/components/MoveToShowFolder.vue'
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
import { Checkbox } from '@/components/ui/checkbox'
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
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { isValidFolderName, useFolders, type Folder as FolderInfo } from '@/composables/useFolders'
import { usePhotos } from '@/composables/usePhotos'
import { download } from '@/lib/download'
import { call, http } from '@/lib/http'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'

const MAX_UPLOAD_FILES = 15

const auth = useAuthStore()
const folders = useFolders()
const library = usePhotos()
const mediaList = ref<InstanceType<typeof MediaList>>()
const movePicker = ref<InstanceType<typeof MoveToShowFolder>>()

const nsId = computed(() => folders.current.value?.nsId ?? -1)

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
    await folders.openRoot()
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
    ids.length && mutate(() => Promise.all(ids.map(folders.remove)), 'Folder deleted!', 'Delete Error!'),
  ])
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

// --- Upload ---------------------------------------------------------------------------------------
const uploadOpen = ref(false)
const files = ref<File[]>([])
const sending = ref(false)
/** Share of the request body sent so far, 0-100. */
const progress = ref(0)

function startUpload() {
  files.value = []
  progress.value = 0
  uploadOpen.value = true
}

function addFiles(list: FileList | null) {
  const accepted = [...(list ?? [])].filter((f) => /^(image|video)\//.test(f.type))
  files.value = [...files.value, ...accepted].slice(0, MAX_UPLOAD_FILES)
}

async function upload() {
  if (!files.value.length) return toast.warning('No Photo selected!')
  const body = new FormData()
  files.value.forEach((f) => body.append('files', f))
  body.append('photosStr', JSON.stringify(files.value.map(() => ({ nsId: nsId.value }))))
  progress.value = 0
  sending.value = true
  try {
    const { data } = await http.post('/photo/inserts', body, {
      onUploadProgress: (e) => {
        if (e.progress !== undefined) progress.value = Math.round(e.progress * 100)
      },
    })
    if (data.msgCode !== 200) throw new Error()
    toast.success('Upload successful!')
    uploadOpen.value = false
    await queryPhotos()
  } catch {
    toast.warning('Submit upload error!')
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="flex min-h-full flex-col gap-4">
    <nav aria-label="Folder path" class="flex flex-wrap items-center gap-1 text-sm">
      <template v-for="(item, i) in folders.path.value" :key="item.nsId">
        <ChevronRight v-if="i" class="text-muted-foreground size-4" aria-hidden="true" />
        <span
          v-if="i === folders.path.value.length - 1"
          aria-current="page"
          class="px-3 py-1.5 font-medium"
          >{{ item.nsName }}</span
        >
        <button
          v-else
          type="button"
          :class="
            cn(
              'hover:bg-accent min-h-9 rounded-md border border-transparent px-3 py-1.5 transition-colors',
              dragging && 'border-muted-foreground/40 border-dashed',
              dragOver === item.nsId && 'bg-primary/15 border-primary border-solid',
            )
          "
          v-bind="dropOn(item.nsId)"
          @click="go(item.nsId, item.nsName)"
        >
          {{ item.nsName }}
        </button>
      </template>
    </nav>
    <hr />

    <div v-if="nsId === -1" class="flex justify-center py-16" role="status" aria-label="Loading">
      <Loader2 class="text-muted-foreground size-8 animate-spin" />
    </div>

    <!-- Right-clicking (or long-pressing) empty space offers upload / new folder. -->
    <ContextMenu>
      <ContextMenuTrigger as-child>
        <div v-show="nsId !== -1" class="flex-1 space-y-6">
          <div
            v-if="selectMode"
            class="bg-background/95 sticky top-0 z-10 flex flex-wrap items-center gap-2 border-b py-2 backdrop-blur"
          >
            <span class="text-muted-foreground mr-2 text-sm">{{ selectedCount }} selected</span>
            <Button variant="outline" size="sm" @click="selectMode = false">Done</Button>
            <Button variant="outline" size="sm" @click="clearSelection">Clear</Button>
            <Button variant="outline" size="sm" @click="selectAll">Select All</Button>
            <Button variant="secondary" size="sm" :disabled="!selectedCount" @click="startMoveSelected"
              >Move</Button
            >
            <Button
              variant="destructive"
              size="sm"
              :disabled="!selectedCount"
              @click="confirmDeleteSelected = true"
              >Delete Selected</Button
            >
          </div>
          <section>
            <div class="mb-3 flex flex-wrap items-center gap-3">
              <h2 class="text-2xl font-semibold">Folders</h2>
              <span class="ml-auto flex gap-2">
                <Button variant="outline" size="sm" @click="startUpload"><Upload /> Upload</Button>
                <Button variant="outline" size="sm" @click="startCreate"
                  ><FolderPlus /> New Folder</Button
                >
              </span>
            </div>

            <EmptyState
              v-if="!folders.children.value.length"
              title="Nothing here yet"
              :icon="Folder"
              class="py-8"
            />
            <ul v-else class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4 2xl:grid-cols-6">
              <li v-for="folder in folders.children.value" :key="folder.nsId">
                <ContextMenu>
                  <ContextMenuTrigger as-child>
                    <Button
                      variant="secondary"
                      :class="
                        cn(
                          'h-11 w-full justify-start',
                          (selected.includes(folder.nsId) || dragOver === folder.nsId) &&
                            'ring-primary ring-2',
                        )
                      "
                      draggable="true"
                      v-bind="dropOn(folder.nsId)"
                      @dragstart="dragFolder($event, folder)"
                      @click="openFolder(folder)"
                      @contextmenu.stop
                    >
                      <Folder />
                      <span class="truncate" :title="folder.nsName">{{ folder.nsName }}</span>
                      <Checkbox
                        v-if="selectMode"
                        class="ml-auto"
                        :aria-label="`Select ${folder.nsName}`"
                        :model-value="selected.includes(folder.nsId)"
                        @click.stop
                        @update:model-value="toggleSelected(folder.nsId)"
                      />
                    </Button>
                  </ContextMenuTrigger>
                  <ContextMenuContent>
                    <ContextMenuItem @select="selectFolder(folder)">
                      <SquareCheck /> {{ selected.includes(folder.nsId) ? 'Deselect' : 'Select' }}
                    </ContextMenuItem>
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
            </ul>
          </section>

          <MediaList
            ref="mediaList"
            :ns-or-not="true"
            :detach-enabled="false"
            v-model:select-mode="selectMode"
            hide-bar
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

    <Dialog v-model:open="uploadOpen">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Upload Photo or Video</DialogTitle>
          <DialogDescription
            >Up to {{ MAX_UPLOAD_FILES }} files into the current folder.</DialogDescription
          >
        </DialogHeader>
        <label
          class="hover:bg-accent flex cursor-pointer flex-col items-center gap-2 rounded-lg border-2 border-dashed p-8 text-center"
          @dragover.prevent
          @drop.prevent="addFiles($event.dataTransfer?.files ?? null)"
        >
          <Upload class="text-muted-foreground size-10" aria-hidden="true" />
          <span>Click or Drag File to This Area</span>
          <input
            type="file"
            multiple
            accept="image/*, video/*"
            class="sr-only"
            @change="addFiles(($event.target as HTMLInputElement).files)"
          />
        </label>
        <ul class="max-h-40 space-y-1 overflow-auto text-sm">
          <li
            v-for="(f, i) in files"
            :key="`${f.name}-${i}`"
            class="flex items-center justify-between gap-2"
          >
            <span class="truncate">{{ f.name }}</span>
            <button type="button" :aria-label="`Remove ${f.name}`" @click="files.splice(i, 1)">
              <X class="size-4" />
            </button>
          </li>
        </ul>
        <div v-if="sending" class="space-y-1">
          <Progress :model-value="progress" aria-label="Upload progress" />
          <p class="text-muted-foreground text-right text-xs">
            {{ progress < 100 ? `Uploading… ${progress}%` : 'Processing…' }}
          </p>
        </div>
        <DialogFooter>
          <Button class="w-full" :disabled="sending" @click="upload">Upload</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="creating">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create a New Folder</DialogTitle>
          <DialogDescription>It will be created inside the current folder.</DialogDescription>
        </DialogHeader>
        <Input v-model="newName" placeholder="Folder Name" @keyup.enter="createFolder" />
        <DialogFooter><Button @click="createFolder">Create</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog :open="!!renaming" @update:open="renaming = null">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Folder's Name</DialogTitle>
          <DialogDescription>Choose a new name for this folder.</DialogDescription>
        </DialogHeader>
        <Input v-model="renameTo" placeholder="Folder Name" @keyup.enter="renameFolder" />
        <DialogFooter><Button @click="renameFolder">Update Name</Button></DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription>
            Do you really want to DELETE this folder? *All photos and folders within this folder
            will be destroied.
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
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription>
            Do you really want to DELETE the selected items? *All photos and folders within selected
            folders will be destroied.
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
            ><Loader2 class="size-4 animate-spin" /> Preparing Your Download
            Request...</DialogDescription
          >
        </DialogHeader>
        <DialogFooter
          ><Button variant="destructive" class="w-full" @click="abortDownload"
            >Abort Download</Button
          ></DialogFooter
        >
      </DialogContent>
    </Dialog>

    <MoveToShowFolder
      ref="movePicker"
      :original-ns-id="movingFolder?.nsId"
      :disable-org-ns-id="true"
      @move-to-update="moveFolder"
    />
  </div>
</template>
