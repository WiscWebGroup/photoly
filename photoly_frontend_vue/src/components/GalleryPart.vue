<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import {
  Album,
  Archive,
  Dog,
  Download,
  Flag,
  Globe,
  Heart,
  Images,
  Luggage,
  Palmtree,
  Pencil,
  Plane,
  Star,
  Trash2,
} from '@lucide/vue'
import MediaList from '@/components/MediaList.vue'
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
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { MediaListApi, Photo } from '@/composables/usePhotos'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'

interface Gallery {
  gaId: number
  gaName: string
  userId?: number
  createDate: string
  coverId: number | null
  coverColor: string
}

/** Indexed by `coverId`. */
const covers = [
  { label: 'Default', icon: Album },
  { label: 'Heart', icon: Heart },
  { label: 'Star', icon: Star },
  { label: 'Airplane', icon: Plane },
  { label: 'Flag', icon: Flag },
  { label: 'Album', icon: Images },
  { label: 'Luggage', icon: Luggage },
  { label: 'Animal', icon: Dog },
  { label: 'Archive', icon: Archive },
  { label: 'Beach', icon: Palmtree },
  { label: 'Globe', icon: Globe },
]

const auth = useAuthStore()
const mediaList = ref<MediaListApi>()
const galleries = ref<Gallery[]>([])
const selectedGaId = ref(-1)

const newName = ref('')
const newCover = ref('0')
const newColor = ref('#000000')

const editing = ref<Gallery | null>(null)
const deleting = ref<Gallery | null>(null)

const loadGalleries = async () =>
  (galleries.value = await call<Gallery[]>({ url: '/gallery/getAll' }))

async function queryPhotos() {
  try {
    mediaList.value?.updatePhotoList(
      await call<Photo[]>({ url: '/photo/getByGallery', params: { gaId: selectedGaId.value } }),
    )
  } catch {
    toast.error('query photo Response Error!')
  }
}

function selectGallery(gallery: Gallery) {
  if (selectedGaId.value === gallery.gaId) {
    selectedGaId.value = -1
    mediaList.value?.updatePhotoList([])
  } else {
    selectedGaId.value = gallery.gaId
    queryPhotos()
  }
}

async function createGallery() {
  if (!newName.value) return toast.warning('Please Input a Valid Gallery Name!')
  try {
    await call({
      method: 'post',
      url: '/gallery/insert',
      data: { gaName: newName.value, coverId: Number(newCover.value), coverColor: newColor.value },
    })
    toast.success('Created a New Gallery!')
    newName.value = ''
    newCover.value = '0'
    newColor.value = '#000000'
    await loadGalleries()
  } catch {
    toast.error('createGa Response Error!')
  }
}

async function saveEdit() {
  const g = editing.value
  if (!g) return
  if (!g.gaName) return toast.warning('Please Input a Valid Gallery Name!')
  try {
    const { gaId, gaName, userId, coverId, coverColor } = g
    await call({
      method: 'post',
      url: '/gallery/update',
      data: { gaId, gaName, userId, coverId, coverColor },
    })
    toast.success('Information Updated!')
    editing.value = null
    await loadGalleries()
  } catch {
    toast.error('Update Response Error!')
  }
}

async function deleteGallery() {
  const g = deleting.value
  if (!g) return
  try {
    await call({ method: 'post', url: '/gallery/delete', params: { gaId: g.gaId } })
    toast.success('Gallery Deleted!')
    deleting.value = null
    if (selectedGaId.value === g.gaId) {
      selectedGaId.value = -1
      mediaList.value?.updatePhotoList([])
    }
    await loadGalleries()
  } catch {
    toast.warning('Update Error!')
  }
}

async function downloadGallery(g: Gallery) {
  try {
    await download(`/gallery/downloadGa/${auth.token}`, `${g.gaName}.zip`, {
      params: { gaId: g.gaId },
    })
  } catch {
    toast.error('Download failed')
  }
}

async function detach(photos: (Photo & { checked?: boolean })[]) {
  try {
    await Promise.all(
      photos
        .filter((p) => p.checked)
        .map((p) =>
          call({
            method: 'post',
            url: '/photo/deleteFromGallery',
            params: { photoId: p.photoId, gaId: selectedGaId.value },
          }),
        ),
    )
    toast.success('Detached from gallery!')
  } catch {
    toast.error('removeGa Response Error!')
  }
  await queryPhotos()
}

onMounted(() => loadGalleries().catch(() => toast.error('getGa Response Error!')))
</script>

<template>
  <div class="flex flex-col gap-6 md:flex-row md:items-start" @contextmenu.prevent.stop>
    <aside class="w-full shrink-0 space-y-3 md:w-64">
      <h2 class="text-2xl font-semibold">Galleries</h2>

      <details class="rounded-lg border">
        <summary class="cursor-pointer px-3 py-2 text-sm font-medium">Add New Gallery</summary>
        <div class="space-y-2 border-t p-3">
          <Input v-model="newName" placeholder="Gallery Name" @keyup.enter="createGallery" />
          <Select v-model="newCover">
            <SelectTrigger class="w-full" aria-label="Cover icon"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="(c, i) in covers" :key="c.label" :value="String(i)">{{
                c.label
              }}</SelectItem>
            </SelectContent>
          </Select>
          <input
            v-model="newColor"
            type="color"
            aria-label="Cover color"
            class="h-9 w-full cursor-pointer rounded-md border"
          />
          <Button variant="secondary" class="w-full" @click="createGallery">Add</Button>
        </div>
      </details>

      <details open class="rounded-lg border">
        <summary class="cursor-pointer px-3 py-2 text-sm font-medium">Galleries</summary>
        <div class="flex flex-col gap-2 border-t p-3">
          <ContextMenu v-for="g in galleries" :key="g.gaId">
            <ContextMenuTrigger as-child>
              <button
                type="button"
                :aria-pressed="selectedGaId === g.gaId"
                :class="
                  cn(
                    'hover:bg-accent flex flex-col gap-1 rounded-lg border p-3 text-left text-sm',
                    selectedGaId === g.gaId && 'bg-accent text-accent-foreground',
                  )
                "
                @click="selectGallery(g)"
              >
                <span class="flex items-center justify-between gap-2">
                  <span class="truncate font-medium" :title="g.gaName">{{ g.gaName }}</span>
                  <component
                    :is="covers[g.coverId ?? 0]?.icon ?? Album"
                    class="size-5 shrink-0"
                    :style="{ color: g.coverColor }"
                  />
                </span>
                <span class="text-muted-foreground text-xs"
                  >Created: {{ g.createDate.split('T')[0] }}</span
                >
              </button>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem @select="editing = { ...g }"><Pencil /> Edit</ContextMenuItem>
              <ContextMenuItem variant="destructive" @select="deleting = g"
                ><Trash2 /> Delete</ContextMenuItem
              >
              <ContextMenuItem @select="downloadGallery(g)"><Download /> Download</ContextMenuItem>
            </ContextMenuContent>
          </ContextMenu>
        </div>
      </details>
    </aside>

    <MediaList
      ref="mediaList"
      class="min-w-0 flex-1"
      :ns-or-not="false"
      :detach-enabled="true"
      detach-str="Gallery"
      @query-photos="queryPhotos"
      @do-detach="detach"
    />

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription>
            Do you really want to DELETE this gallery? *All photos associated with this gallery will
            be detached (photos will not been deleted).
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteGallery">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <Dialog :open="!!editing" @update:open="editing = null">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Gallery</DialogTitle>
          <DialogDescription>Change this gallery's name, icon and color.</DialogDescription>
        </DialogHeader>
        <template v-if="editing">
          <Input v-model="editing.gaName" placeholder="Gallery Name" @keyup.enter="saveEdit" />
          <Select
            :model-value="String(editing.coverId ?? 0)"
            @update:model-value="editing.coverId = Number($event)"
          >
            <SelectTrigger class="w-full" aria-label="Cover icon"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="(c, i) in covers" :key="c.label" :value="String(i)">{{
                c.label
              }}</SelectItem>
            </SelectContent>
          </Select>
          <input
            v-model="editing.coverColor"
            type="color"
            aria-label="Cover color"
            class="h-9 w-full cursor-pointer rounded-md border"
          />
        </template>
        <DialogFooter><Button @click="saveEdit">Submit</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
