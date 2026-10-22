<script setup lang="ts">
import SmartBasis from '@/components/SmartBasis.vue'
import SmartBadge from '@/components/SmartBadge.vue'
import type { SmartSource } from '@/lib/smartOrganization'
import { computed, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  Album,
  Archive,
  Check,
  ChevronDown,
  ChevronRight,
  Dog,
  Download,
  Ellipsis,
  Flag,
  Folder,
  Globe,
  Heart,
  Images,
  Luggage,
  Palmtree,
  Pencil,
  Plane,
  Plus,
  Sparkles,
  Star,
  Trash2,
} from '@lucide/vue'
import AlbumSwitch from '@/components/AlbumSwitch.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Photo } from '@/composables/usePhotos'
import { useCovers } from '@/composables/useCovers'
import { photoUrl } from '@/lib/photoUrl'
import { download } from '@/lib/download'
import { call } from '@/lib/http'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth'

interface Gallery {
  smart?: SmartSource | null
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
const mediaList = ref<InstanceType<typeof MediaList>>()
const galleries = ref<Gallery[]>([])
const selectedGaId = ref(-1)

const colors = ['#16827d', '#d97706', '#7c3aed', '#db2777', '#2563eb', '#404040']

/** The create / edit dialog; `gaId` is set when editing. */
const form = ref<{
  gaId?: number
  userId?: number
  gaName: string
  coverId: number
  coverColor: string
} | null>(null)
const deleting = ref<Gallery | null>(null)

const tab = ref<'all' | 'yours' | 'smart'>('all')
const tabs = computed(() => [
  { value: 'all' as const, label: 'All', count: galleries.value.length },
  {
    value: 'yours' as const,
    label: 'Created by you',
    count: galleries.value.filter((g) => !g.smart).length,
  },
  {
    value: 'smart' as const,
    label: 'Smart trips',
    count: galleries.value.filter((g) => g.smart).length,
  },
])
const sorts = { newest: 'Newest first', oldest: 'Oldest first', name: 'Name' } as const
const sort = ref<keyof typeof sorts>('newest')
const shown = computed(() =>
  galleries.value
    .filter((g) => (tab.value === 'all' ? true : tab.value === 'smart' ? !!g.smart : !g.smart))
    .sort((a, b) =>
      sort.value === 'name'
        ? a.gaName.localeCompare(b.gaName, undefined, { numeric: true })
        : (sort.value === 'oldest' ? 1 : -1) * a.createDate.localeCompare(b.createDate),
    ),
)

// Cards show a mosaic of the gallery's latest photos and how many items it holds.
const { covers: cardCovers, refresh: refreshCovers } = useCovers('gallery')
watch(galleries, (list) => refreshCovers(list.map((g) => g.gaId)))
const cardMeta = (g: Gallery) => {
  const n = cardCovers.get(g.gaId)?.count
  const created = `Created ${formatDate(g.createDate)}`
  return n === undefined ? created : `${n} item${n === 1 ? '' : 's'} · ${created}`
}
const current = computed(() => galleries.value.find((g) => g.gaId === selectedGaId.value))

const iconFor = (g: { coverId: number | null }) => covers[g.coverId ?? 0]?.icon ?? Album
/** A light wash of the gallery's color for its card. */
const tint = (color: string) => `color-mix(in oklab, ${color || '#16827d'} 14%, var(--card))`
const formatDate = (iso: string) => {
  const [y, m, d] = iso.slice(0, 10).split('-').map(Number)
  return y && m && d
    ? new Date(y, m - 1, d).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })
    : ''
}

function startCreate() {
  form.value = { gaName: '', coverId: 0, coverColor: colors[0]! }
}

function startEdit(g: Gallery) {
  form.value = {
    gaId: g.gaId,
    userId: g.userId,
    gaName: g.gaName,
    coverId: g.coverId ?? 0,
    coverColor: g.coverColor || colors[0]!,
  }
}

const loadGalleries = async () =>
  (galleries.value = await call<Gallery[]>({
    url: '/gallery/getAll',
    params: { includeSmart: true },
  }))

const galleryPhotos = ref<Photo[]>([])
const selecting = ref(false)

async function queryPhotos() {
  try {
    const list = await call<Photo[]>({
      url: '/photo/getByGallery',
      params: { gaId: selectedGaId.value },
    })
    galleryPhotos.value = list
    mediaList.value?.updatePhotoList(list)
  } catch {
    toast.error('query photo Response Error!')
  }
}

// "Pulls from": the folders this gallery's photos live in, named on demand.
const folderNames = ref(new Map<number, string>())
const sourceFolders = computed(() => [
  ...new Set(galleryPhotos.value.map((p) => p.nsId).filter((id): id is number => id !== undefined)),
])
watch(sourceFolders, (ids) =>
  ids
    .filter((id) => !folderNames.value.has(id))
    .forEach((id) =>
      call<string>({ url: '/namespace/getNameById', params: { nsId: id } })
        .then((name) => folderNames.value.set(id, name))
        .catch(() => {}),
    ),
)
const detailMeta = computed(() => {
  const n = galleryPhotos.value.length
  const k = sourceFolders.value.length
  return [
    `${n} item${n === 1 ? '' : 's'}`,
    k && `from ${k} folder${k === 1 ? '' : 's'}`,
    current.value && `Created ${formatDate(current.value.createDate)}`,
  ]
    .filter(Boolean)
    .join(' · ')
})

function openGallery(gallery: Gallery) {
  selectedGaId.value = gallery.gaId
  galleryPhotos.value = []
  selecting.value = false
  mediaList.value?.updatePhotoList([])
  queryPhotos()
}

function closeGallery() {
  selectedGaId.value = -1
}

async function saveForm() {
  const f = form.value
  if (!f) return
  if (!f.gaName.trim()) return toast.warning('Please Input a Valid Gallery Name!')
  const editing = f.gaId !== undefined
  try {
    await call(
      editing
        ? { method: 'post', url: '/gallery/update', data: { ...f } }
        : {
            method: 'post',
            url: '/gallery/insert',
            data: { gaName: f.gaName, coverId: f.coverId, coverColor: f.coverColor },
          },
    )
    toast.success(editing ? 'Information Updated!' : 'Created a New Gallery!')
    form.value = null
    await loadGalleries()
  } catch {
    toast.error(editing ? 'Update Response Error!' : 'createGa Response Error!')
  }
}

async function deleteGallery() {
  const g = deleting.value
  if (!g) return
  try {
    await call({ method: 'post', url: '/gallery/delete', params: { gaId: g.gaId } })
    toast.success('Gallery Deleted!')
    deleting.value = null
    if (selectedGaId.value === g.gaId) selectedGaId.value = -1
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
  <div
    class="flex w-full flex-col gap-6 px-4 pt-5 pb-32 md:px-10 md:pt-10"
    @contextmenu.prevent.stop
  >
    <AlbumSwitch
      class="md:hidden"
      :add-label="current ? undefined : 'New Gallery'"
      @add="startCreate"
    />

    <!-- Overview: every gallery as a card. -->
    <template v-if="!current">
      <PageHeader
        class="max-md:hidden"
        title="Galleries"
        description="Curated sets that can pull from any folder."
      >
        <Button @click="startCreate"><Plus /> New Gallery</Button>
      </PageHeader>

      <div class="-mt-1 flex items-center gap-2 border-b">
        <div
          role="tablist"
          aria-label="Filter galleries"
          class="flex gap-5 overflow-x-auto [scrollbar-width:none]"
        >
          <template v-for="t in tabs" :key="t.value">
            <button
              v-if="t.value !== 'smart' || t.count"
              type="button"
              role="tab"
              :aria-selected="tab === t.value"
              class="text-muted-foreground aria-selected:text-foreground aria-selected:border-foreground -mb-px flex h-10 items-center gap-1.5 border-b-2 border-transparent text-sm font-medium transition-colors hover:text-foreground"
              @click="tab = t.value"
            >
              <Sparkles
                v-if="t.value === 'smart'"
                class="text-primary size-3.5"
                aria-hidden="true"
              />
              {{ t.label }}
              <span class="font-normal opacity-70 tabular-nums">{{ t.count }}</span>
            </button>
          </template>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger
            class="hover:bg-accent ml-auto hidden h-8 shrink-0 items-center gap-1 rounded-md px-2 text-sm font-medium transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 sm:flex"
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
      </div>

      <EmptyState
        v-if="!shown.length"
        :icon="Images"
        title="No galleries yet"
        description="Galleries collect photos from any folder without moving them."
      >
        <Button variant="outline" @click="startCreate"><Plus /> New Gallery</Button>
      </EmptyState>
      <ul
        v-else
        class="grid grid-cols-1 gap-4 sm:grid-cols-2 sm:gap-6 lg:grid-cols-3 2xl:grid-cols-[repeat(auto-fill,minmax(280px,1fr))]"
      >
        <li
          v-for="(g, i) in shown"
          :key="g.gaId"
          class="animate-rise"
          :style="{ animationDelay: `${Math.min(i, 12) * 40}ms` }"
        >
          <ContextMenu>
            <ContextMenuTrigger as-child>
              <article
                class="group bg-card hover:border-primary relative overflow-hidden rounded-[14px] border shadow-xs transition-[border-color,box-shadow,translate] duration-200 hover:-translate-y-0.5 hover:shadow-sm"
              >
                <button
                  type="button"
                  class="block w-full text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:ring-inset"
                  @click="openGallery(g)"
                >
                  <span
                    v-if="cardCovers.get(g.gaId)?.ids.length"
                    class="bg-muted grid aspect-[16/10] grid-cols-[3fr_2fr] grid-rows-2 gap-0.5 overflow-hidden"
                    aria-hidden="true"
                  >
                    <img
                      v-for="(id, k) in cardCovers.get(g.gaId)!.ids"
                      :key="id"
                      :src="photoUrl(id, 'renderThumbnail')"
                      alt=""
                      loading="lazy"
                      :class="[
                        'size-full object-cover transition-transform duration-500 ease-(--ease-out-quart) group-hover:scale-[1.03]',
                        k === 0 && 'row-span-2',
                        cardCovers.get(g.gaId)!.ids.length === 1 && 'col-span-2',
                      ]"
                    />
                  </span>
                  <span
                    v-else
                    class="relative grid aspect-[16/10] place-items-center"
                    :style="{ background: tint(g.coverColor) }"
                    aria-hidden="true"
                  >
                    <component
                      :is="iconFor(g)"
                      class="size-12 stroke-[1.5] transition-transform duration-300 ease-(--ease-out-quart) group-hover:scale-110 group-hover:-rotate-6"
                      :style="{ color: g.coverColor || undefined }"
                    />
                  </span>
                  <span class="flex items-center gap-3 py-3.5 pr-14 pl-4">
                    <span
                      class="grid size-8 shrink-0 place-items-center rounded-lg text-white"
                      :style="{ background: g.coverColor || '#16827d' }"
                      aria-hidden="true"
                    >
                      <component :is="iconFor(g)" class="size-4" />
                    </span>
                    <span class="flex min-w-0 flex-col">
                      <span class="truncate font-semibold" :title="g.gaName">{{ g.gaName }}</span>
                      <span class="text-muted-foreground text-xs">{{ cardMeta(g) }}</span>
                    </span>
                  </span>
                  <SmartBadge
                    :smart="g.smart"
                    class="absolute top-3 left-3 border-0 bg-white/90 text-[#033633] shadow-xs"
                  />
                </button>
                <DropdownMenu>
                  <DropdownMenuTrigger as-child>
                    <button
                      type="button"
                      :aria-label="`More for ${g.gaName}`"
                      class="text-muted-foreground hover:bg-accent hover:text-foreground absolute right-3 bottom-3.5 grid size-8 place-items-center rounded-md transition-colors"
                    >
                      <Ellipsis class="size-4" aria-hidden="true" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem @select="startEdit(g)"><Pencil /> Edit</DropdownMenuItem>
                    <DropdownMenuItem @select="downloadGallery(g)"
                      ><Download /> Download</DropdownMenuItem
                    >
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" @select="deleting = g"
                      ><Trash2 /> Delete</DropdownMenuItem
                    >
                  </DropdownMenuContent>
                </DropdownMenu>
              </article>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem @select="startEdit(g)"><Pencil /> Edit</ContextMenuItem>
              <ContextMenuItem @select="downloadGallery(g)"><Download /> Download</ContextMenuItem>
              <ContextMenuItem variant="destructive" @select="deleting = g"
                ><Trash2 /> Delete</ContextMenuItem
              >
            </ContextMenuContent>
          </ContextMenu>
        </li>
      </ul>
    </template>

    <!-- One gallery open. -->
    <template v-else>
      <nav aria-label="Gallery path" class="-mb-2 flex items-center gap-1 text-sm">
        <button
          type="button"
          class="text-muted-foreground hover:bg-accent hover:text-accent-foreground -ml-1.5 rounded-md px-1.5 py-1 transition-colors"
          @click="closeGallery"
        >
          Galleries
        </button>
        <ChevronRight class="text-muted-foreground/70 size-3.5" aria-hidden="true" />
        <span aria-current="page" class="truncate px-1.5 py-1 font-medium">{{
          current.gaName
        }}</span>
      </nav>
      <div class="flex flex-wrap items-center gap-4">
        <span
          class="animate-in zoom-in-75 grid size-14 shrink-0 place-items-center rounded-xl text-white shadow-sm duration-300"
          :style="{ background: current.coverColor || '#16827d' }"
          aria-hidden="true"
        >
          <component :is="iconFor(current)" class="size-6" />
        </span>
        <PageHeader :title="current.gaName" class="min-w-0 flex-1 !items-center">
          <template #description>
            <span class="flex flex-wrap items-center gap-2">
              {{ detailMeta }}
              <SmartBadge :smart="current.smart" />
            </span>
          </template>
          <Button
            v-if="galleryPhotos.length"
            variant="ghost"
            :aria-pressed="selecting"
            @click="selecting = !selecting"
            >{{ selecting ? 'Cancel' : 'Select' }}</Button
          >
          <Button variant="outline" @click="startEdit(current)"><Pencil /> Edit</Button>
          <Button variant="outline" @click="downloadGallery(current)"><Download /> Download</Button>
          <DropdownMenu>
            <DropdownMenuTrigger as-child>
              <Button variant="outline" size="icon" aria-label="More"><Ellipsis /></Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem variant="destructive" @select="deleting = current"
                ><Trash2 /> Delete Gallery</DropdownMenuItem
              >
            </DropdownMenuContent>
          </DropdownMenu>
        </PageHeader>
      </div>
      <div
        v-if="sourceFolders.length"
        class="bg-card animate-fade flex flex-wrap items-center gap-2 rounded-[10px] border px-3.5 py-2.5 text-sm"
      >
        <span class="text-muted-foreground">Pulls from</span>
        <span
          v-for="id in sourceFolders.slice(0, 3)"
          :key="id"
          class="inline-flex h-[26px] items-center gap-1.5 rounded-full border px-2.5"
        >
          <Folder class="text-muted-foreground size-3.5" aria-hidden="true" />
          {{ folderNames.get(id) === 'root' ? 'Library' : (folderNames.get(id) ?? 'Folder') }}
        </span>
        <span v-if="sourceFolders.length > 3" class="text-muted-foreground"
          >+{{ sourceFolders.length - 3 }} more</span
        >
      </div>
      <SmartBasis
        v-if="current.smart"
        class="bg-card animate-fade rounded-xl border px-4 py-3"
        :smart="current.smart"
      />
    </template>

    <MediaList
      v-show="current"
      ref="mediaList"
      class="min-w-0"
      :context="current?.gaName ?? 'Galleries'"
      :ns-or-not="false"
      :detach-enabled="true"
      detach-str="Gallery"
      group-by-month
      v-model:select-mode="selecting"
      @query-photos="queryPhotos"
      @do-detach="detach"
    />

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {{ deleting?.gaName }}?</AlertDialogTitle>
          <AlertDialogDescription>
            Its photos stay in your library; only the gallery goes away.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteGallery">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <Dialog :open="!!form" @update:open="form = null">
      <DialogContent v-if="form" class="sm:max-w-[640px]">
        <DialogHeader>
          <DialogTitle>{{ form.gaId === undefined ? 'New Gallery' : 'Edit Gallery' }}</DialogTitle>
          <DialogDescription
            >Galleries collect items from any folder without moving them.</DialogDescription
          >
        </DialogHeader>
        <form class="grid gap-6 sm:grid-cols-[220px_1fr]" @submit.prevent="saveForm">
          <!-- Live preview of the card. -->
          <div
            class="bg-card hidden self-start overflow-hidden rounded-xl border shadow-xs sm:block"
            aria-hidden="true"
          >
            <div
              class="grid h-32 place-items-center transition-colors"
              :style="{ background: tint(form.coverColor) }"
            >
              <component
                :is="iconFor(form)"
                :key="form.coverId"
                class="animate-in zoom-in-50 size-10 stroke-[1.5] duration-300"
                :style="{ color: form.coverColor }"
              />
            </div>
            <div class="flex items-center gap-3 p-3">
              <span
                class="grid size-8 shrink-0 place-items-center rounded-lg text-white transition-colors"
                :style="{ background: form.coverColor }"
                ><component :is="iconFor(form)" class="size-4"
              /></span>
              <span class="flex min-w-0 flex-col">
                <span class="truncate font-medium">{{ form.gaName || 'Untitled' }}</span>
                <span class="text-muted-foreground text-xs">{{
                  form.gaId === undefined ? 'Empty until you add photos' : 'Gallery'
                }}</span>
              </span>
            </div>
          </div>

          <div class="flex flex-col gap-5">
            <div class="flex flex-col gap-2">
              <Label for="gallery-name">Name</Label>
              <Input id="gallery-name" v-model="form.gaName" placeholder="Gallery Name" />
            </div>
            <fieldset class="flex flex-col gap-2">
              <legend class="mb-2 text-sm font-medium">Color</legend>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  v-for="c in colors"
                  :key="c"
                  type="button"
                  :aria-label="`Color ${c}`"
                  :aria-pressed="form.coverColor === c"
                  class="grid size-8 place-items-center rounded-full text-white ring-offset-2 ring-offset-card transition-transform hover:scale-110 aria-pressed:ring-2"
                  :style="{ background: c, '--tw-ring-color': c }"
                  @click="form.coverColor = c"
                >
                  <Check v-if="form.coverColor === c" class="size-4" aria-hidden="true" />
                </button>
                <label
                  class="relative grid size-8 cursor-pointer place-items-center rounded-full border border-dashed"
                  title="Custom color"
                >
                  <Plus class="text-muted-foreground size-4" aria-hidden="true" />
                  <input
                    v-model="form.coverColor"
                    type="color"
                    aria-label="Cover color"
                    class="absolute inset-0 cursor-pointer opacity-0"
                  />
                </label>
              </div>
            </fieldset>
            <fieldset class="flex flex-col">
              <legend class="mb-2 text-sm font-medium">Icon</legend>
              <div class="flex flex-wrap gap-2" role="radiogroup" aria-label="Cover icon">
                <button
                  v-for="(c, i) in covers"
                  :key="c.label"
                  type="button"
                  role="radio"
                  :aria-checked="form.coverId === i"
                  :aria-label="c.label"
                  :title="c.label"
                  class="bg-card hover:bg-accent aria-checked:border-primary aria-checked:bg-accent aria-checked:text-accent-foreground grid size-10 place-items-center rounded-lg border shadow-xs transition-colors"
                  @click="form.coverId = i"
                >
                  <component :is="c.icon" class="size-4" aria-hidden="true" />
                </button>
              </div>
            </fieldset>
          </div>
        </form>
        <DialogFooter>
          <Button variant="outline" @click="form = null">Cancel</Button>
          <Button @click="saveForm">{{
            form.gaId === undefined ? 'Create Gallery' : 'Save Changes'
          }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
