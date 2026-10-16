<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { X } from '@lucide/vue'
import ImageEditor from '@/components/ImageEditor.vue'
import LocationMap from '@/components/LocationMap.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
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
import { download } from '@/lib/download'
import { isEditable } from '@/lib/imageEdit'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { photoUrl } from '@/lib/photoUrl'
import { useAuthStore } from '@/stores/auth'
import SimilarRecommendations, {
  type RecommendationSnapshot,
} from '@/components/ai/SimilarRecommendations.vue'

const props = withDefaults(
  defineProps<{ photo: Photo | null; nsPath: string; allowSimilar?: boolean }>(),
  { allowSimilar: true },
)
const emit = defineEmits<{ close: []; changed: [change?: { type: string; photo: Photo }] }>()

interface Tag {
  tagId: number
  tagName: string
}
interface Gallery {
  gaId: number
  gaName: string
}

const auth = useAuthStore()
const selected = ref<Photo | null>(null)
const currentPhoto = computed(() => selected.value ?? props.photo)
const viewerBody = ref<HTMLElement>()
const scrollRoot = computed(
  () => viewerBody.value?.closest<HTMLElement>('[data-slot="dialog-content"]') ?? null,
)
const recommendations = ref<InstanceType<typeof SimilarRecommendations>>()
const restoredRecommendations = ref<RecommendationSnapshot>()
const currentPath = ref(props.nsPath)
const history = ref<
  { photo: Photo; path: string; scroll: number; recommendations?: RecommendationSnapshot }[]
>([])
let metadataSequence = 0
let restoreScroll: number | null = null
let metadataLoaded = false
watch(
  () => props.photo,
  () => {
    selected.value = null
    history.value = []
    restoredRecommendations.value = undefined
  },
  { immediate: true },
)
watch(
  () => props.nsPath,
  (path) => {
    if (!selected.value) currentPath.value = path
  },
)
watch(
  () => auth.token,
  () => {
    history.value = []
    selected.value = null
    emit('close')
  },
)
function invalidateHistory() {
  for (const item of history.value) item.recommendations = undefined
}
async function navigate(photo: Photo) {
  if (!info.value) return
  history.value.push({
    photo: { ...info.value },
    path: currentPath.value,
    scroll: scrollRoot.value?.scrollTop ?? 0,
    recommendations: recommendations.value?.capture(),
  })
  restoredRecommendations.value = undefined
  restoreScroll = null
  selected.value = { ...photo }
  await nextTick()
  scrollRoot.value?.scrollTo({ top: 0 })
}
async function previous() {
  const item = history.value.pop()
  if (!item) return
  restoreScroll = item.scroll
  restoredRecommendations.value = item.recommendations
  selected.value = { ...item.photo }
  currentPath.value = item.path
  await nextTick()
  scrollRoot.value?.scrollTo({ top: item.scroll })
}

/** A copy, so renames and visibility changes show here without mutating the parent's list. */
const info = ref<Photo>()
const exif = ref<Record<string, string | undefined>>({})
const tags = ref<{ tag_id: number; tag_name: string }[]>([])
const galleries = ref<{ ga_id: number; ga_name: string }[]>([])
const tagOptions = ref<Tag[]>([])
const galleryOptions = ref<Gallery[]>([])
const tagChoice = ref('')
const galleryChoice = ref('')
const newName = ref('')
const loaded = ref(false)
const editing = ref(false)
/** Bumped after an overwrite so the browser refetches the image under the same URL. */
const version = ref(0)

const kind = computed(() => mediaKind(info.value?.format))
const canEdit = computed(() => kind.value === 'photo' && isEditable(info.value?.format))
const imageUrl = computed(
  () => photoUrl(info.value!.photoId) + (version.value ? `&v=${version.value}` : ''),
)
const params = computed(() => ({ photoId: info.value?.photoId }))
const hasGps = computed(() => !!exif.value.GPS_Latitude && !!exif.value.GPS_Longitude)
const qrUrl = computed(
  () =>
    `${import.meta.env.VITE_APP_BASE_URL}/photo/getQR2?token=${auth.token}&photoId=${info.value?.photoId}`,
)

const quiet = (message: string) => () => void toast.error(message)

function currentCall<T>(url: string, apply: (value: T) => void) {
  const seq = metadataSequence
  return call<T>({ url, params: { photoId: info.value?.photoId } }).then((value) => {
    if (seq === metadataSequence) apply(value)
  })
}
const loadTags = () =>
  currentCall<typeof tags.value>('/photo/getTagByPhoto', (value) => {
    tags.value = value
  })
const loadGalleries = () =>
  currentCall<typeof galleries.value>('/photo/getGalleryByPhoto', (value) => {
    galleries.value = value
  })

const loadTagOptions = () =>
  call<Tag[]>({ url: '/tag/getAll' }).then(
    (t) => (tagOptions.value = t),
    quiet('get tags Response Error!'),
  )
const loadGalleryOptions = () =>
  call<Gallery[]>({ url: '/gallery/getAll' }).then(
    (g) => (galleryOptions.value = g),
    quiet('get galleries Response Error!'),
  )

const loadExif = () =>
  currentCall<typeof exif.value>('/photo/getPhotoExif', (value) => {
    exif.value = value
  }).catch(quiet('Exif Info Response Error!'))

watch(
  () => currentPhoto.value,
  (photo) => {
    metadataLoaded = false
    const seq = ++metadataSequence
    if (!photo) return
    info.value = { ...photo }
    newName.value = photo.photoName
    loaded.value = false
    editing.value = false
    version.value = 0
    exif.value = {}
    tags.value = []
    galleries.value = []
    currentPath.value = selected.value ? 'Loading…' : props.nsPath
    if (selected.value)
      void call<{ nsName: string }[]>({ url: '/namespace/trace', params: { nsId: photo.nsId } })
        .then((path) => {
          if (seq === metadataSequence)
            currentPath.value =
              '/root' +
              path
                .slice(1)
                .map((n) => `/${n.nsName}`)
                .join('')
        })
        .catch(() => {
          if (seq === metadataSequence) currentPath.value = 'Unavailable'
        })
    tagChoice.value = galleryChoice.value = ''
    void Promise.allSettled([loadTags(), loadGalleries(), loadExif()]).then(() => {
      if (seq === metadataSequence) {
        metadataLoaded = true
        void restorePosition()
      }
    })
  },
  { immediate: true },
)

async function restorePosition() {
  if (restoreScroll === null || !metadataLoaded || !loaded.value) return
  const top = restoreScroll
  restoreScroll = null
  await nextTick()
  scrollRoot.value?.scrollTo({ top })
}
function imageLoaded() {
  loaded.value = true
  void restorePosition()
}

function edited(mode: 'new' | 'overwrite', format: string) {
  editing.value = false
  if (mode === 'overwrite') {
    // The file (and maybe its format) changed under the same ID: refetch the picture and EXIF.
    info.value!.format = format
    version.value++
    invalidateHistory()
    restoredRecommendations.value = undefined
    recommendations.value?.invalidate()
    loaded.value = false
    exif.value = {}
    loadExif()
  }
  emit('changed', { type: 'content', photo: info.value! })
}

async function run(
  request: Promise<unknown>,
  success: string,
  failure: string,
  then?: () => unknown,
) {
  try {
    await request
    toast.success(success)
    await then?.()
  } catch {
    toast.error(failure)
  }
}

const addTag = () =>
  run(
    call({
      method: 'post',
      url: '/photo/addTags',
      params: { ...params.value, tagIds: tagChoice.value },
    }),
    'added to tag!',
    'add tag Response Error!',
    () => ((tagChoice.value = ''), loadTags()),
  )
const removeTag = (tagId: number) =>
  run(
    call({ method: 'post', url: '/photo/deleteTag', params: { ...params.value, tagId } }),
    'Tag detached!',
    'remove tag Response Error!',
    loadTags,
  )
const addGallery = () =>
  run(
    call({
      method: 'post',
      url: '/photo/addToGallery',
      params: { ...params.value, gaId: galleryChoice.value },
    }),
    'added to gallery!',
    'add gallery Response Error!',
    () => ((galleryChoice.value = ''), loadGalleries()),
  )
const removeGallery = (gaId: number) =>
  run(
    call({ method: 'post', url: '/photo/deleteFromGallery', params: { ...params.value, gaId } }),
    'Gallery detached!',
    'remove gallery Response Error!',
    loadGalleries,
  )

function changeVisibility(on: boolean) {
  const visibility = on ? 1 : 0
  return run(
    call({
      method: 'post',
      url: '/photo/update',
      data: { photoId: info.value!.photoId, visibility },
    }),
    'visibility changed!',
    'change visibility Response Error!',
    () => (info.value!.visibility = visibility),
  )
}

function changeName() {
  if (!newName.value) return toast.warning('Image name cannot be empty!')
  return run(
    call({
      method: 'post',
      url: '/photo/update',
      data: { photoId: info.value!.photoId, photoName: newName.value },
    }),
    'Photo name changed!',
    'change name Response Error!',
    () => (
      (info.value!.photoName = newName.value),
      emit('changed', { type: 'metadata', photo: info.value! })
    ),
  )
}

async function copyShareAddr() {
  try {
    await navigator.clipboard.writeText(
      await call<string>({ url: '/photo/getPath', params: params.value }),
    )
    toast.success('Successful copied to clipboard!')
  } catch {
    toast.error('get share addr Response Error!')
  }
}

const downloadPhoto = () =>
  download(`/photo/download/${auth.token}`, `${info.value!.photoName}.${info.value!.format}`, {
    params: params.value,
  }).catch(quiet('Download failed'))

const section = 'rounded-lg border'
const summary = 'cursor-pointer px-3 py-2 text-sm font-medium'
const body = 'space-y-2 border-t p-3 text-sm'
</script>

<template>
  <Dialog :open="!!photo" @update:open="emit('close')">
    <DialogContent v-if="info" class="max-h-[95dvh] w-full overflow-y-auto sm:max-w-6xl">
      <DialogTitle class="sr-only">{{ info.photoName }}.{{ info.format }}</DialogTitle>
      <DialogDescription class="sr-only">Photo details and similar images</DialogDescription>
      <div ref="viewerBody">
        <Button v-if="history.length" variant="outline" class="mb-4" @click="previous"
          >Back to previous photo</Button
        >
        <ImageEditor v-if="editing" :photo="info" @cancel="editing = false" @saved="edited" />
        <div v-else class="grid min-h-[calc(95dvh-8rem)] gap-4 md:grid-cols-[1fr_22rem]">
          <div class="flex min-h-64 items-start justify-center">
            <template v-if="kind === 'photo'">
              <Skeleton v-show="!loaded" class="h-[60vh] w-full" />
              <img
                v-show="loaded"
                :src="imageUrl"
                :alt="info.photoName"
                class="max-h-[85vh] max-w-full cursor-zoom-out rounded-md object-contain"
                @load="imageLoaded"
                @click="emit('close')"
              />
            </template>
            <video
              v-else-if="kind === 'video'"
              :key="info.photoId"
              controls
              class="max-h-[85vh] w-full rounded-md"
            >
              <source :src="photoUrl(info.photoId, 'renderV')" :type="`video/${info.format}`" />
            </video>
          </div>

          <div class="space-y-2">
            <h2 class="text-xl font-semibold">Metadata</h2>

            <details open :class="section">
              <summary :class="summary">Common Metadata</summary>
              <div :class="body">
                <p class="break-all">Name: {{ info.photoName }}.{{ info.format }}</p>
                <p>Upload Date: {{ info.uploadDate }}</p>
                <p v-if="exif.File_Size">Size: {{ exif.File_Size }}</p>
                <p class="break-all">Path: {{ currentPath }}</p>
                <p>Shared: {{ info.visibility === 0 ? 'No' : 'Yes' }}</p>
                <div class="flex flex-wrap gap-2">
                  <Button v-if="kind === 'photo'" variant="secondary" size="sm" as-child>
                    <a :href="imageUrl" target="_blank" rel="noopener">Full Picture</a>
                  </Button>
                  <Button v-if="canEdit" variant="secondary" size="sm" @click="editing = true"
                    >Edit Image</Button
                  >
                  <Button variant="secondary" size="sm" @click="downloadPhoto">Download</Button>
                </div>
              </div>
            </details>

            <details v-if="hasGps" open :class="section">
              <summary :class="summary">Location</summary>
              <div :class="body">
                <LocationMap
                  :key="info.photoId"
                  :lat="exif.GPS_Latitude!"
                  :lon="exif.GPS_Longitude!"
                />
                <Button variant="secondary" size="sm" as-child>
                  <a
                    :href="`https://maps.google.com/?q=${exif.GPS_Latitude},${exif.GPS_Longitude}`"
                    target="_blank"
                    rel="noopener"
                    >Open in Map</a
                  >
                </Button>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Exif Metadata</summary>
              <div :class="body">
                <p v-if="exif.Time">Time Taken: {{ exif.Time }}</p>
                <p v-if="exif.Image_Height && exif.Image_Width">
                  H/W: {{ exif.Image_Height.split(' ')[0] }} * {{ exif.Image_Width }}
                </p>
                <p v-if="exif.GPS_Latitude">
                  GPS DD: {{ exif.GPS_Latitude }} {{ exif.GPS_Longitude }}
                </p>
                <hr />
                <p v-if="exif.Model">Camera: {{ exif.Make }} {{ exif.Model }}</p>
                <p v-if="exif.Lens_Make || exif.Lens_Model">
                  Lens: {{ exif.Lens_Make }} {{ exif.Lens_Model }}
                </p>
                <p v-if="exif.Focal_Length">Focal: {{ exif.Focal_Length }}</p>
                <p v-if="exif.Aperture_Value">Aperture: {{ exif.Aperture_Value }}</p>
                <p v-if="exif.Shutter_Speed_Value">Shutter: {{ exif.Shutter_Speed_Value }}</p>
                <p v-if="exif.ISO_Speed_Ratings">ISO: {{ exif.ISO_Speed_Ratings }}</p>
                <p v-if="exif.Data_Precision">Data Precision: {{ exif.Data_Precision }}</p>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Secrets</summary>
              <div :class="body">
                <p class="break-all">Token: {{ info.token }}</p>
                <p class="break-all">UUID: {{ info.photoUuid }}</p>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Tags</summary>
              <div :class="body">
                <div class="flex flex-wrap gap-1">
                  <Badge v-for="t in tags" :key="t.tag_id" variant="secondary" class="gap-1">
                    {{ t.tag_name }}
                    <button
                      type="button"
                      :aria-label="`Remove ${t.tag_name}`"
                      @click="removeTag(t.tag_id)"
                    >
                      <X class="size-3" />
                    </button>
                  </Badge>
                </div>
                <p>Add</p>
                <div class="flex gap-2">
                  <!-- Options load on first open so the viewer doesn't fetch every tag up front. -->
                  <Select v-model="tagChoice" @update:open="(o) => o && loadTagOptions()">
                    <SelectTrigger class="flex-1" aria-label="Tag"
                      ><SelectValue placeholder="Choose a tag"
                    /></SelectTrigger>
                    <SelectContent>
                      <SelectItem v-for="t in tagOptions" :key="t.tagId" :value="String(t.tagId)">{{
                        t.tagName
                      }}</SelectItem>
                    </SelectContent>
                  </Select>
                  <Button size="sm" :disabled="!tagChoice" @click="addTag">Submit</Button>
                </div>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Gallery</summary>
              <div :class="body">
                <div class="flex flex-wrap gap-1">
                  <Badge v-for="g in galleries" :key="g.ga_id" variant="secondary" class="gap-1">
                    {{ g.ga_name }}
                    <button
                      type="button"
                      :aria-label="`Remove ${g.ga_name}`"
                      @click="removeGallery(g.ga_id)"
                    >
                      <X class="size-3" />
                    </button>
                  </Badge>
                </div>
                <p>Add</p>
                <div class="flex gap-2">
                  <Select v-model="galleryChoice" @update:open="(o) => o && loadGalleryOptions()">
                    <SelectTrigger class="flex-1" aria-label="Gallery"
                      ><SelectValue placeholder="Choose a gallery"
                    /></SelectTrigger>
                    <SelectContent>
                      <SelectItem
                        v-for="g in galleryOptions"
                        :key="g.gaId"
                        :value="String(g.gaId)"
                        >{{ g.gaName }}</SelectItem
                      >
                    </SelectContent>
                  </Select>
                  <Button size="sm" :disabled="!galleryChoice" @click="addGallery">Submit</Button>
                </div>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Share</summary>
              <div :class="[body, 'flex flex-wrap gap-2']">
                <Button variant="secondary" size="sm" @click="copyShareAddr"
                  >Copy Share Addr</Button
                >
                <Popover>
                  <PopoverTrigger as-child
                    ><Button variant="secondary" size="sm">Get QR Code</Button></PopoverTrigger
                  >
                  <PopoverContent class="w-auto"
                    ><img :src="qrUrl" alt="QR code" class="w-44"
                  /></PopoverContent>
                </Popover>
              </div>
            </details>

            <details :class="section">
              <summary :class="summary">Edit Info</summary>
              <div :class="body">
                <label class="flex items-center gap-2">
                  Visibility
                  <Switch
                    aria-label="Visibility"
                    :model-value="info.visibility === 1"
                    @update:model-value="changeVisibility"
                  />
                </label>
                <p>Change Name</p>
                <div class="flex gap-2">
                  <Input v-model="newName" placeholder="New Name" @keyup.enter="changeName" />
                  <Button size="sm" @click="changeName">Update</Button>
                </div>
              </div>
            </details>
          </div>
        </div>
        <SimilarRecommendations
          v-if="!editing && kind === 'photo' && allowSimilar !== false"
          :key="`${info.photoId}:${version}`"
          ref="recommendations"
          :photo="info"
          :scroll-root="scrollRoot"
          :resume="restoredRecommendations"
          @navigate="navigate"
          @invalidated="invalidateHistory"
        />
      </div>
    </DialogContent>
  </Dialog>
</template>
