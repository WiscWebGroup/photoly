<script setup lang="ts">
import SmartBadge from '@/components/SmartBadge.vue'
import type { SmartSource } from '@/lib/smartOrganization'
import { animate, type AnimationPlaybackControls } from 'motion'
import { onKeyStroke, useFullscreen, useMediaQuery, useSwipe } from '@vueuse/core'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  ArrowUp,
  Aperture,
  CalendarDays,
  ChevronLeft,
  ChevronRight,
  Copy,
  Download,
  Ellipsis,
  ExternalLink,
  File as FileIcon,
  Film,
  Folder,
  FolderInput,
  Info,
  MapPin,
  Maximize,
  Minimize,
  Pencil,
  Plus,
  Share2,
  Sparkles,
  Trash2,
  Undo2,
  X,
} from '@lucide/vue'
import ImageEditor from '@/components/ImageEditor.vue'
import PhotoLabelPicker from '@/components/PhotoLabelPicker.vue'
import PhotoText from '@/components/PhotoText.vue'
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
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import type { Photo } from '@/composables/usePhotos'
import { vCollapse } from '@/lib/collapse'
import { download } from '@/lib/download'
import { isEditable } from '@/lib/imageEdit'
import { call } from '@/lib/http'
import { mediaKind } from '@/lib/media'
import { photoUrl } from '@/lib/photoUrl'
import { useAuthStore } from '@/stores/auth'
import SimilarRecommendations, {
  type RecommendationSnapshot,
} from '@/components/ai/SimilarRecommendations.vue'

/**
 * `photos`: the list the photo was opened from, for prev/next and the filmstrip.
 * `context`: what the back button returns to ("Library", a gallery name…).
 * `locate`: where a photo's thumbnail sits now, so closing can shrink back into it.
 * `manage`: offer Move and Delete (the parent handles both).
 */
const props = withDefaults(
  defineProps<{
    photo: Photo | null
    nsPath: string
    allowSimilar?: boolean
    origin?: DOMRect | null
    photos?: Photo[]
    context?: string
    locate?: (photo: Photo) => DOMRect | null
    manage?: boolean
  }>(),
  { allowSimilar: true, photos: () => [], manage: false },
)
const emit = defineEmits<{
  close: []
  changed: [change?: { type: string; photo: Photo }]
  navigate: [photo: Photo]
  move: [photo: Photo]
  delete: [photo: Photo]
}>()

interface Tag {
  smart?: SmartSource | null
  tagId: number
  tagName: string
}
interface Gallery {
  smart?: SmartSource | null
  gaId: number
  gaName: string
}

const auth = useAuthStore()
const desktop = useMediaQuery('(min-width: 768px)')
const selected = ref<Photo | null>(null)
const acceptedPhoto = ref<Photo | null>(props.photo)
const photoText = ref<InstanceType<typeof PhotoText>>()
const currentPhoto = computed(() =>
  acceptedPhoto.value ? (selected.value ?? acceptedPhoto.value) : null,
)
async function leaveText() {
  return (await photoText.value?.requestLeave()) ?? true
}
let closing = false
async function closeViewer() {
  if (closing) return
  closing = true
  try {
    if (!(await leaveText())) return
    recommendations.value?.cancel() // a query landing mid-animation must not be retained
    if (isFullscreen.value) await exitFullscreen().catch(() => {})
    await playClose()
    acceptedPhoto.value = null
    emit('close')
  } finally {
    closing = false
  }
}
async function editImage() {
  if (await leaveText()) editing.value = true
}

// --- Layout refs -------------------------------------------------------------------------------
const root = ref<HTMLElement>()
const panelScroll = ref<HTMLElement>()
const stage = ref<HTMLElement>()
const mediaEl = ref<HTMLElement>()
const backdrop = ref<HTMLElement>()
/** The panel scrolls on its own beside the photo; on phones the whole viewer scrolls. */
const scrollRoot = computed(() => (desktop.value ? panelScroll.value : root.value) ?? null)
const content = computed(
  () => root.value?.closest<HTMLElement>('[data-slot=dialog-content]') ?? null,
)
const {
  isSupported: canFullscreen,
  isFullscreen,
  toggle: toggleFullscreen,
  exit: exitFullscreen,
} = useFullscreen(content)

// --- Similar photos, and the trail of photos opened from them -------------------------------------
const recommendations = ref<InstanceType<typeof SimilarRecommendations>>()
const restoredRecommendations = ref<RecommendationSnapshot>()
const currentPath = ref(props.nsPath)
const history = ref<{ photo: Photo; path: string }[]>([])
const RECOMMENDATION_RETENTION_MS = 30_000
const recommendationCache = new Map<
  number,
  {
    recommendations: RecommendationSnapshot
    scroll: number
    leftAt: number
    timer: ReturnType<typeof setTimeout>
  }
>()
let recommendationSession = 0
function forgetRecommendations(photoId: number) {
  const cached = recommendationCache.get(photoId)
  if (cached) clearTimeout(cached.timer)
  recommendationCache.delete(photoId)
}
function invalidateHistory() {
  for (const photoId of recommendationCache.keys()) forgetRecommendations(photoId)
  restoredRecommendations.value = undefined
  restoreScroll = null
}
function rememberRecommendations(photoId: number) {
  forgetRecommendations(photoId)
  const snapshot = recommendations.value?.capture()
  if (!snapshot?.result || snapshot.expiresAt <= Date.now()) return
  recommendationCache.set(photoId, {
    recommendations: snapshot,
    scroll: scrollRoot.value?.scrollTop ?? 0,
    leftAt: Date.now(),
    timer: setTimeout(() => forgetRecommendations(photoId), RECOMMENDATION_RETENTION_MS),
  })
}
function resumeRecommendations(photoId: number) {
  const cached = recommendationCache.get(photoId)
  forgetRecommendations(photoId)
  const valid =
    cached &&
    Date.now() - cached.leftAt < RECOMMENDATION_RETENTION_MS &&
    cached.recommendations.expiresAt > Date.now()
  restoredRecommendations.value = valid ? cached.recommendations : undefined
  restoreScroll = valid ? cached.scroll : null
}
function backToTop() {
  if (info.value) forgetRecommendations(info.value.photoId)
  restoredRecommendations.value = undefined
  restoreScroll = null
  recommendations.value?.collapse()
  scrollRoot.value?.focus({ preventScroll: true })
  scrollRoot.value?.scrollTo({ top: 0, behavior: 'instant' })
}
onBeforeUnmount(invalidateHistory)
let metadataSequence = 0
let restoreScroll: number | null = null
let metadataLoaded = false
let photoChange = 0
watch(
  () => props.photo,
  async (photo) => {
    const seq = ++photoChange
    if (!(await leaveText()) || seq !== photoChange) return
    acceptedPhoto.value = photo
    selected.value = null
    history.value = []
    restoredRecommendations.value = undefined
  },
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
    photoChange++
    photoText.value?.reset()
    acceptedPhoto.value = null
    recommendations.value?.collapse()
    invalidateHistory()
    history.value = []
    selected.value = null
    emit('close')
  },
)
async function navigate(photo: Photo) {
  if (!info.value || photo.photoId === info.value.photoId || !(await leaveText())) return
  direction.value = 'fade'
  history.value.push({ photo: { ...info.value }, path: currentPath.value })
  selected.value = { ...photo }
}
async function previous() {
  if (!(await leaveText())) return
  const item = history.value.pop()
  if (!item) return
  direction.value = 'fade'
  selected.value = { ...item.photo }
  currentPath.value = item.path
}

// --- Prev / next through the list the photo came from -------------------------------------------
const direction = ref<'next' | 'prev' | 'fade'>('fade')
const index = computed(() =>
  selected.value || !acceptedPhoto.value
    ? -1
    : props.photos.findIndex((p) => p.photoId === acceptedPhoto.value!.photoId),
)
const prevPhoto = computed(() => (index.value > 0 ? props.photos[index.value - 1] : undefined))
const nextPhoto = computed(() => (index.value >= 0 ? props.photos[index.value + 1] : undefined))
async function step(delta: 1 | -1) {
  const target = delta > 0 ? nextPhoto.value : prevPhoto.value
  if (!target || editing.value || !(await leaveText())) return
  direction.value = delta > 0 ? 'next' : 'prev'
  emit('navigate', target)
}
/** Seven thumbnails around the current photo. */
const strip = computed(() => {
  const n = props.photos.length
  if (index.value < 0 || n < 2) return []
  const start = Math.max(0, Math.min(index.value - 3, n - 7))
  return props.photos.slice(start, start + 7).map((p, i) => ({ photo: p, n: start + i + 1 }))
})
function jump(target: Photo) {
  const to = props.photos.indexOf(target)
  if (to === index.value) return
  direction.value = to > index.value ? 'next' : 'prev'
  void leaveText().then((ok) => ok && emit('navigate', target))
}

const isTyping = (el: EventTarget | null) =>
  el instanceof HTMLElement &&
  !!el.closest('input, textarea, select, video, [contenteditable="true"]')
/** Only when the viewer is the top layer: no menu, popover or confirmation open over it. */
const onTop = () =>
  document.querySelectorAll('[role=dialog], [role=alertdialog], [role=menu]').length <= 1
onKeyStroke(['ArrowLeft', 'ArrowRight', 'i'], (e) => {
  if (!acceptedPhoto.value || editing.value || isTyping(e.target) || !onTop()) return
  if (e.metaKey || e.ctrlKey || e.altKey) return
  e.preventDefault()
  if (e.key === 'i') showDetails.value = !showDetails.value
  else void step(e.key === 'ArrowRight' ? 1 : -1)
})
useSwipe(stage, {
  threshold: 48,
  onSwipeEnd(_, dir) {
    if (dir === 'left') void step(1)
    else if (dir === 'right') void step(-1)
    // Phones: swipe up on the photo to bring the sheet back, down (from the top) to hide it.
    else if (dir === 'up' && sheetHidden.value) showDetails.value = true
    else if (dir === 'down' && !desktop.value && !sheetHidden.value && atTop())
      showDetails.value = false
  },
})

// --- Photo metadata ------------------------------------------------------------------------------
/** A copy, so renames and visibility changes show here without mutating the parent's list. */
const info = ref<Photo>()
const exif = ref<Record<string, string | undefined>>({})
const tags = ref<{ tag_id: number; tag_name: string; smart?: SmartSource | null }[]>([])
const galleries = ref<{ ga_id: number; ga_name: string; smart?: SmartSource | null }[]>([])
const tagOptions = ref<Tag[]>([])
const galleryOptions = ref<Gallery[]>([])
const tagOpen = ref(false)
const galleryOpen = ref(false)
const loaded = ref(false)
const editing = ref(false)
/**
 * Beside the photo on desktop, a sheet under it on phones; it stays as the user left it from
 * photo to photo.
 */
const showDetails = ref(true)
const sharing = ref(false)
const confirmingDelete = ref(false)
const shareLink = ref('')
/** Bumped after an overwrite so the browser refetches the image under the same URL. */
const version = ref(0)

// --- Phone details sheet: pull it down to give the photo the screen, pull it back up -------------
/** On phones the details sheet sits under the photo; hidden, only its title peeks out. */
const sheetHidden = computed(() => !desktop.value && !showDetails.value)
/** How far the sheet is being dragged, in px (down is positive). */
const sheetDrag = ref(0)
const draggingSheet = ref(false)
const SHEET_DISMISS = 56
const atTop = () => (root.value?.scrollTop ?? 0) <= 0
let dragFrom: { x: number; y: number } | null = null
function sheetTouchStart(e: TouchEvent) {
  const t = e.touches[0]
  dragFrom = !desktop.value && e.touches.length === 1 && t ? { x: t.clientX, y: t.clientY } : null
}
function sheetTouchMove(e: TouchEvent) {
  const t = e.touches[0]
  if (!dragFrom || !t) return
  const dy = t.clientY - dragFrom.y
  if (!draggingSheet.value) {
    // Shown, only a pull down from the very top drags the sheet; anything else scrolls as usual.
    const claim =
      e.cancelable &&
      Math.abs(dy) >= Math.abs(t.clientX - dragFrom.x) &&
      (sheetHidden.value ? dy < 0 : dy > 0 && atTop())
    if (!claim) return void (dragFrom = null)
    draggingSheet.value = true
  }
  e.preventDefault()
  sheetDrag.value = sheetHidden.value ? Math.min(0, dy) : Math.max(0, dy)
}
function sheetTouchEnd() {
  if (draggingSheet.value && Math.abs(sheetDrag.value) >= SHEET_DISMISS)
    showDetails.value = sheetHidden.value
  draggingSheet.value = false
  sheetDrag.value = 0
  dragFrom = null
}
watch(sheetHidden, (hidden) => {
  if (hidden) root.value?.scrollTo({ top: 0, behavior: 'instant' })
})

const kind = computed(() => mediaKind(info.value?.format))
const canSimilar = computed(
  () =>
    !editing.value &&
    (kind.value === 'photo' || kind.value === 'video') &&
    props.allowSimilar !== false,
)
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

const fileName = computed(() => (info.value ? `${info.value.photoName}.${info.value.format}` : ''))
const dimensions = computed(() => {
  const w = exif.value.Image_Width?.split(' ')[0]
  const h = exif.value.Image_Height?.split(' ')[0]
  return w && h ? `${w} × ${h}` : ''
})
const fileMeta = computed(() =>
  [fileName.value, exif.value.File_Size, dimensions.value].filter(Boolean).join(' · '),
)

/** EXIF stamps read "2026:06:14 21:48:00"; upload dates are ISO. */
function parseStamp(value?: string) {
  const m = value?.match(/^(\d{4})[:-](\d{2})[:-](\d{2})(?:[ T](\d{2}):(\d{2}))?/)
  if (!m) return null
  const date = new Date(+m[1]!, +m[2]! - 1, +m[3]!, +(m[4] ?? 0), +(m[5] ?? 0))
  return { date, hasTime: m[4] !== undefined }
}
const when = computed(() => {
  const taken = parseStamp(exif.value.Time)
  const stamp = taken ?? parseStamp(info.value?.uploadDate)
  if (!stamp) return null
  const day = stamp.date.toLocaleDateString(undefined, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
  const time = stamp.hasTime
    ? stamp.date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
    : ''
  const short = stamp.date.toLocaleDateString(undefined, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    year: 'numeric',
  })
  return {
    day,
    detail: [taken ? 'Taken' : 'Uploaded', time].filter(Boolean).join(' · '),
    short: [short, time].filter(Boolean).join(', '),
  }
})
/** "Model" usually repeats the make ("Sony" + "Sony α7 IV"), so show it once. */
function joinOnce(first?: string, second?: string) {
  const a = first?.trim() ?? ''
  const b = second?.trim() ?? ''
  if (a && b && b.toLowerCase().startsWith(a.toLowerCase())) return b
  return [a, b].filter(Boolean).join(' ')
}
const camera = computed(() => joinOnce(exif.value.Make, exif.value.Model))
const lens = computed(() => exif.value.Lens_Model?.trim() || exif.value.Lens_Make?.trim() || '')
const exposure = computed(() => {
  const shutter = exif.value.Shutter_Speed_Value?.replace(/\s*sec$/, '')
  return [
    { label: 'Aperture', value: exif.value.Aperture_Value },
    { label: 'Shutter', value: shutter && (shutter.includes('/') ? shutter : `${shutter}s`) },
    { label: 'ISO', value: exif.value.ISO_Speed_Ratings },
    { label: 'Focal', value: exif.value.Focal_Length?.replace(/\s+mm$/, 'mm') },
  ].filter((x) => x.value)
})
/** "/root/Trips/2024" reads as "Library / Trips / 2024". */
const folder = computed(() => {
  const path = currentPath.value
  if (!path) return ''
  if (!path.startsWith('/root')) return path
  return ['Library', ...path.slice(5).split('/').filter(Boolean)].join(' / ')
})

const quiet = (message: string) => () => void toast.error(message)

function currentCall<T>(url: string, apply: (value: T) => void) {
  const seq = metadataSequence
  return call<T>({
    url,
    params: {
      photoId: info.value?.photoId,
      ...(/get(Tag|Gallery)ByPhoto$/.test(url) ? { includeSmart: true } : {}),
    },
  }).then((value) => {
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
  call<Tag[]>({ url: '/tag/getAll', params: { includeSmart: true } }).then(
    (t) => (tagOptions.value = t),
    quiet('get tags Response Error!'),
  )
const loadGalleryOptions = () =>
  call<Gallery[]>({ url: '/gallery/getAll', params: { includeSmart: true } }).then(
    (g) => (galleryOptions.value = g),
    quiet('get galleries Response Error!'),
  )
/** Options load on first open so the viewer doesn't fetch everything up front. */
function openPicker(open: boolean) {
  if (open) void Promise.all([loadTagOptions(), loadGalleryOptions()])
}

const loadExif = () =>
  currentCall<typeof exif.value>('/photo/getPhotoExif', (value) => {
    exif.value = value
  }).catch(quiet('Exif Info Response Error!'))

function loadShareLink() {
  const seq = metadataSequence
  shareLink.value = ''
  if (info.value?.visibility !== 1) return
  void call<string>({ url: '/photo/getPath', params: params.value })
    .then((link) => seq === metadataSequence && (shareLink.value = link))
    .catch(() => {})
}

watch(
  () => currentPhoto.value,
  (photo, previousPhoto) => {
    if (previousPhoto) rememberRecommendations(previousPhoto.photoId)
    recommendations.value?.collapse()
    restoredRecommendations.value = undefined
    restoreScroll = null
    recommendationSession++
    metadataLoaded = false
    const seq = ++metadataSequence
    if (!photo) return
    resumeRecommendations(photo.photoId)
    void nextTick(() => {
      if (seq === metadataSequence)
        scrollRoot.value?.scrollTo({ top: restoreScroll ?? 0, behavior: 'instant' })
    })
    info.value = { ...photo }
    loaded.value = false
    editing.value = false
    version.value = 0
    exif.value = {}
    tags.value = []
    galleries.value = []
    loadShareLink()
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
    tagOpen.value = galleryOpen.value = false
    void Promise.allSettled([loadTags(), loadGalleries(), loadExif()]).then(() => {
      if (seq === metadataSequence) {
        metadataLoaded = true
        void restorePosition()
      }
    })
    // Anything that isn't a photo or video has nothing to wait for.
    if (mediaKind(photo.format) === 'other') void nextTick(mediaReady)
  },
  { immediate: true },
)

async function restorePosition() {
  if (restoreScroll === null || !metadataLoaded || !loaded.value) return
  const seq = metadataSequence
  await nextTick()
  if (seq !== metadataSequence || restoreScroll === null) return
  const top = restoreScroll
  restoreScroll = null
  scrollRoot.value?.scrollTo({ top, behavior: 'instant' })
}
function imageLoaded(e: Event) {
  const img = e.target as HTMLImageElement
  loaded.value = true
  // Decode first: decoding a big image mid-animation is what drops frames.
  void (img.decode?.() ?? Promise.resolve()).catch(() => {}).then(mediaReady)
  void restorePosition()
}

async function findSimilar() {
  showDetails.value = true
  recommendations.value?.start()
  await nextTick()
  ;(recommendations.value?.$el as HTMLElement | undefined)?.scrollIntoView?.({
    block: 'start',
    behavior: desktop.value ? 'smooth' : 'instant',
  })
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

const addTag = (tagId: number) =>
  run(
    call({ method: 'post', url: '/photo/addTags', params: { ...params.value, tagIds: tagId } }),
    'added to tag!',
    'add tag Response Error!',
    loadTags,
  )
const removeTag = (tagId: number) =>
  run(
    call({ method: 'post', url: '/photo/deleteTag', params: { ...params.value, tagId } }),
    'Tag detached!',
    'remove tag Response Error!',
    loadTags,
  )
const addGallery = (gaId: number) =>
  run(
    call({ method: 'post', url: '/photo/addToGallery', params: { ...params.value, gaId } }),
    'added to gallery!',
    'add gallery Response Error!',
    loadGalleries,
  )
const removeGallery = (gaId: number) =>
  run(
    call({ method: 'post', url: '/photo/deleteFromGallery', params: { ...params.value, gaId } }),
    'Gallery detached!',
    'remove gallery Response Error!',
    loadGalleries,
  )
/** Create a tag or gallery, then attach it (the insert response carries no id, so look it up by name). */
async function create(kind: 'tag' | 'gallery', name: string) {
  try {
    if (kind === 'tag') {
      await call({ method: 'post', url: '/tag/insert', data: { tagName: name } })
      await loadTagOptions()
      const created = tagOptions.value.find((t) => t.tagName === name)
      if (created) await addTag(created.tagId)
    } else {
      await call({
        method: 'post',
        url: '/gallery/insert',
        data: { gaName: name, coverId: 0, coverColor: '#000000' },
      })
      await loadGalleryOptions()
      const created = galleryOptions.value.find((g) => g.gaName === name)
      if (created) await addGallery(created.gaId)
    }
  } catch {
    toast.error(`create ${kind} Response Error!`)
  }
}
const pickerTags = computed(() =>
  tagOptions.value.map((t) => ({ id: t.tagId, name: t.tagName, smart: t.smart })),
)
const pickerGalleries = computed(() =>
  galleryOptions.value.map((g) => ({ id: g.gaId, name: g.gaName, smart: g.smart })),
)
const attachedTags = computed(() => tags.value.map((t) => t.tag_id))
const attachedGalleries = computed(() => galleries.value.map((g) => g.ga_id))

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
    () => {
      info.value!.visibility = visibility
      loadShareLink()
    },
  )
}

function renamed(name: string) {
  info.value!.photoName = name
  emit('changed', { type: 'metadata', photo: info.value! })
}

async function copyShareAddr() {
  try {
    const link =
      shareLink.value || (await call<string>({ url: '/photo/getPath', params: params.value }))
    await navigator.clipboard.writeText(link)
    toast.success('Successful copied to clipboard!')
  } catch {
    toast.error('get share addr Response Error!')
  }
}

const downloadPhoto = () =>
  download(`/photo/download/${auth.token}`, fileName.value, { params: params.value }).catch(
    quiet('Download failed'),
  )
const saveQr = () =>
  download('/photo/getQR2', `${info.value!.photoName}-qr.png`, {
    params: { token: auth.token, photoId: info.value!.photoId },
  }).catch(quiet('Download failed'))

function deletePhoto() {
  confirmingDelete.value = false
  emit('delete', info.value!)
}

// --- Motion ----------------------------------------------------------------------------------------
// GSAP-equivalent eases (powerN = easeIn/Out of degree N+1).
const EASE = {
  p1Out: [0.25, 0.46, 0.45, 0.94],
  p1InOut: [0.455, 0.03, 0.515, 0.955],
  p3Out: [0.165, 0.84, 0.44, 1],
  p3InOut: [0.77, 0, 0.175, 1],
} as const satisfies Record<string, [number, number, number, number]>
const reducedMotion = () => !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
const chrome = () => [...(content.value?.querySelectorAll<HTMLElement>('[data-chrome]') ?? [])]
const visible = (r: DOMRect | null | undefined): r is DOMRect =>
  !!r && r.width > 0 && r.bottom > 0 && r.top < innerHeight && r.right > 0 && r.left < innerWidth

/**
 * Morph the photo between its thumbnail and its place in the viewer. The thumbnail is a centred
 * square crop, so the photo is scaled to cover that square and clipped to it, then both relax.
 * `v` runs 0 (thumbnail) → 1 (in place).
 */
function morph(media: HTMLElement, from: DOMRect, to: 0 | 1, duration: number) {
  media.style.transform = media.style.clipPath = ''
  const box = media.getBoundingClientRect()
  if (!box.width || !box.height) return null
  const s = Math.max(from.width / box.width, from.height / box.height)
  const dx = from.left + from.width / 2 - (box.left + box.width / 2)
  const dy = from.top + from.height / 2 - (box.top + box.height / 2)
  const ix = Math.max(0, (box.width - from.width / s) / 2)
  const iy = Math.max(0, (box.height - from.height / s) / 2)
  const set = (v: number) => {
    const k = 1 - v
    media.style.transform = `translate(${dx * k}px, ${dy * k}px) scale(${s + (1 - s) * v})`
    media.style.clipPath = `inset(${iy * k}px ${ix * k}px round ${(4 / s) * k + 2 * v}px)`
  }
  set(1 - to)
  media.style.willChange = 'transform, clip-path'
  return animate(1 - to, to, {
    duration,
    ease: to ? EASE.p3Out : EASE.p3InOut,
    onUpdate: set,
  })
}
let openAnims: AnimationPlaybackControls[] | null = null
let openPlayed = false
/** Set while the open animation waits for the media size. */
let startOpen: (() => void) | null = null
const mediaReady = () => startOpen?.()
watch(
  root,
  (el) => {
    if (!el || openPlayed) return
    openPlayed = true
    const fast = reducedMotion()
    const media = () => mediaEl.value
    const items = chrome()
    if (backdrop.value) backdrop.value.style.opacity = '0'
    for (const item of items) item.style.opacity = '0'
    if (media()) media()!.style.opacity = '0'
    if (backdrop.value)
      animate(backdrop.value, { opacity: 1 }, { duration: fast ? 0 : 0.3, ease: EASE.p1Out })
    const run = () => {
      if (!el.isConnected) return
      const m = media()
      const anims: AnimationPlaybackControls[] = items.map((item, i) =>
        animate(
          item,
          { opacity: [0, 1], y: item.dataset.chrome === 'panel' ? 0 : [-4, 0] },
          { duration: fast ? 0 : 0.35, delay: fast ? 0 : 0.12 + i * 0.03, ease: EASE.p1Out },
        ),
      )
      if (m) {
        m.style.opacity = '1'
        const from = props.origin
        const grow = !fast && visible(from) ? morph(m, from, 1, 0.5) : null
        if (grow) anims.push(grow)
        else if (!fast)
          anims.push(
            animate(m, { opacity: [0, 1], scale: [0.96, 1] }, { duration: 0.3, ease: EASE.p1Out }),
          )
      }
      openAnims = anims
      void Promise.all(anims.map((a) => a.finished)).then(() => {
        if (openAnims !== anims) return // interrupted by a close
        openAnims = null
        if (m)
          for (const prop of ['transform', 'clip-path', 'will-change']) m.style.removeProperty(prop)
      })
    }
    // Wait for the photo/video size (layout depends on it); fall back if it never loads.
    const timer = setTimeout(() => startOpen?.(), 800)
    startOpen = () => {
      startOpen = null
      clearTimeout(timer)
      void nextTick(run)
    }
  },
  { flush: 'post' },
)
watch(acceptedPhoto, (photo) => photo || ((openPlayed = false), (startOpen = null)))

/** Shrink the photo back into its thumbnail while everything else fades. Resolves when done. */
function playClose() {
  if (reducedMotion() || !root.value?.getBoundingClientRect().width) return Promise.resolve()
  startOpen = null
  openAnims?.forEach((a) => a.stop())
  openAnims = null
  const m = mediaEl.value
  const photo = currentPhoto.value
  const target = photo && props.locate?.(photo)
  const anims: Promise<unknown>[] = chrome().map(
    (item) => animate(item, { opacity: 0 }, { duration: 0.18, ease: EASE.p1Out }).finished,
  )
  if (backdrop.value)
    anims.push(
      animate(backdrop.value, { opacity: 0 }, { duration: 0.4, ease: EASE.p1InOut }).finished,
    )
  if (m) {
    const shrink = !editing.value && visible(target) ? morph(m, target, 0, 0.4) : null
    anims.push(
      shrink?.finished ??
        animate(m, { opacity: 0, scale: 0.96 }, { duration: 0.25, ease: EASE.p1InOut }).finished,
    )
  }
  // Never let a stalled animation keep the viewer open.
  const cap = new Promise<void>((resolve) => setTimeout(resolve, 600))
  return Promise.race([Promise.all(anims).then(() => {}), cap])
}

const iconButton =
  'grid size-10 shrink-0 place-items-center rounded-lg text-foreground transition-colors outline-none hover:bg-white/10 focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-[18px]'
const section = 'flex flex-col gap-4 border-b px-5 py-5 md:px-6'
const heading = 'text-muted-foreground text-xs font-semibold tracking-[0.025em] uppercase'
const chip =
  'inline-flex h-7 max-w-full items-center gap-1 rounded-full border border-white/16 pr-1.5 pl-2.5 text-sm transition-colors hover:border-primary max-md:h-8 max-md:px-3'
const addChip =
  'text-muted-foreground inline-flex h-7 items-center gap-1 rounded-full border border-white/16 px-2.5 text-sm transition-colors outline-none hover:border-primary hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50 max-md:border-dashed max-md:border-primary/50 max-md:text-primary'
const removeButton =
  'text-muted-foreground hover:text-foreground grid size-5 shrink-0 place-items-center rounded-full transition-colors'
const overlay = 'dark text-foreground'
</script>

<template>
  <Dialog :open="!!acceptedPhoto" @update:open="closeViewer">
    <DialogContent
      v-if="currentPhoto && info"
      data-photo-viewer
      :show-close-button="false"
      class="dark text-foreground fixed inset-0 top-0 left-0 flex h-dvh w-screen max-w-none translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-none border-0 bg-transparent p-0 shadow-none outline-none data-[state=closed]:animate-none data-[state=open]:animate-none sm:max-w-none"
      @open-auto-focus="
        (e: Event) => {
          e.preventDefault()
          root?.focus({ preventScroll: true })
        }
      "
    >
      <DialogTitle class="sr-only">{{ fileName }}</DialogTitle>
      <DialogDescription class="sr-only"
        >Photo details and similar photos and videos</DialogDescription
      >
      <div ref="backdrop" class="bg-background absolute inset-0" aria-hidden="true" />

      <div
        ref="root"
        tabindex="-1"
        :data-testid="desktop ? undefined : 'photo-viewer-scroll'"
        :class="[
          'relative flex min-h-0 w-full flex-col overscroll-contain outline-none [scrollbar-width:none] md:flex-row md:overflow-hidden [&::-webkit-scrollbar]:hidden',
          sheetHidden ? 'overflow-hidden' : 'overflow-y-auto',
          !desktop && recommendations?.expanded && 'pb-20',
        ]"
      >
        <!-- The photo, its toolbar and the strip of neighbours -->
        <section
          aria-label="Photo"
          :class="[
            'relative flex shrink-0 flex-col md:min-w-0 md:flex-1 md:shrink',
            editing
              ? 'min-h-dvh md:min-h-0'
              : sheetHidden
                ? // The whole screen but the sheet's peek (72px, less the 16px it overlaps the photo)
                  'h-[calc(100dvh_-_3.5rem_-_env(safe-area-inset-bottom))]'
                : 'h-[52dvh] md:h-auto',
            !desktop &&
              !draggingSheet &&
              'transition-[height] duration-350 ease-(--ease-out-quart)',
          ]"
        >
          <div
            data-chrome
            class="flex h-14 shrink-0 items-center gap-1 pt-[env(safe-area-inset-top)] pr-2 pl-1 md:h-16 md:gap-2 md:pr-4 md:pl-3"
          >
            <button
              type="button"
              :class="[
                iconButton,
                'md:flex md:w-auto md:gap-1.5 md:pr-3 md:pl-2 md:text-sm md:font-medium',
              ]"
              @click="closeViewer"
            >
              <ChevronLeft class="md:hidden" aria-hidden="true" />
              <ArrowLeft class="max-md:hidden" aria-hidden="true" />
              <span v-if="context" class="sr-only">Back to </span
              ><span class="max-md:sr-only">{{ context ?? 'Back' }}</span>
            </button>
            <button
              v-if="history.length"
              type="button"
              :class="[iconButton, 'md:flex md:w-auto md:gap-1.5 md:px-3 md:text-sm']"
              @click="previous"
            >
              <Undo2 aria-hidden="true" /><span class="max-md:sr-only">Back to previous photo</span>
            </button>
            <span
              v-if="index >= 0"
              class="text-muted-foreground pl-1 text-sm whitespace-nowrap tabular-nums"
              aria-live="polite"
              >{{ index + 1 }} of {{ photos.length }}</span
            >

            <div v-if="!editing" class="ml-auto flex items-center gap-0.5 md:gap-1">
              <button type="button" aria-label="Share" :class="iconButton" @click="sharing = true">
                <Share2 aria-hidden="true" />
              </button>
              <button
                type="button"
                aria-label="Download"
                :class="iconButton"
                @click="downloadPhoto"
              >
                <Download aria-hidden="true" />
              </button>
              <button
                v-if="canEdit"
                type="button"
                aria-label="Edit image"
                :class="[iconButton, 'max-md:hidden']"
                @click="editImage"
              >
                <Pencil aria-hidden="true" />
              </button>
              <button
                v-if="canFullscreen"
                type="button"
                :aria-label="isFullscreen ? 'Exit full screen' : 'Full screen'"
                :class="[iconButton, 'max-md:hidden']"
                @click="toggleFullscreen"
              >
                <component :is="isFullscreen ? Minimize : Maximize" aria-hidden="true" />
              </button>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <button
                    type="button"
                    aria-label="More"
                    :class="[iconButton, 'data-[state=open]:bg-white/10']"
                  >
                    <Ellipsis aria-hidden="true" />
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" :class="[overlay, 'w-64']">
                  <DropdownMenuItem v-if="canEdit" class="md:hidden" @select="editImage"
                    ><Pencil /> Edit Image</DropdownMenuItem
                  >
                  <DropdownMenuItem v-if="canSimilar" @select="findSimilar"
                    ><Sparkles /> Find Similar</DropdownMenuItem
                  >
                  <DropdownMenuItem v-if="kind === 'photo'" as-child>
                    <a :href="imageUrl" target="_blank" rel="noopener"
                      ><ExternalLink /> Open Original</a
                    >
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem v-if="manage" @select="emit('move', info!)"
                    ><FolderInput /> Move to Folder…</DropdownMenuItem
                  >
                  <DropdownMenuItem @select="downloadPhoto"
                    ><Download /> Download Original
                    <DropdownMenuShortcut v-if="exif.File_Size">{{
                      exif.File_Size
                    }}</DropdownMenuShortcut></DropdownMenuItem
                  >
                  <template v-if="manage">
                    <DropdownMenuSeparator />
                    <DropdownMenuItem variant="destructive" @select="confirmingDelete = true"
                      ><Trash2 /> Delete Photo</DropdownMenuItem
                    >
                  </template>
                </DropdownMenuContent>
              </DropdownMenu>
              <span aria-hidden="true" class="mx-1 h-6 w-px bg-white/16 max-md:hidden" />
              <button
                type="button"
                :aria-label="showDetails ? 'Hide details' : 'Details'"
                :aria-pressed="showDetails"
                :class="[
                  iconButton,
                  'max-md:hidden',
                  showDetails && 'bg-primary/18 text-primary hover:bg-primary/25',
                ]"
                @click="showDetails = !showDetails"
              >
                <Info aria-hidden="true" />
              </button>
            </div>
          </div>

          <div v-if="editing" class="flex-1 overflow-y-auto px-4 pb-8 md:px-8">
            <div class="mx-auto max-w-5xl">
              <ImageEditor
                :photo="info"
                @cancel="editing = false"
                @saved="edited"
                @renamed="renamed"
              />
            </div>
          </div>

          <template v-else>
            <div ref="stage" class="relative min-h-0 flex-1">
              <Transition :name="`slide-${direction}`">
                <div
                  :key="`${info.photoId}:${version}`"
                  class="absolute inset-0 flex items-center justify-center md:px-[72px] md:pt-2 md:pb-6"
                >
                  <template v-if="kind === 'photo'">
                    <Skeleton
                      v-show="!loaded"
                      class="aspect-[4/3] w-[min(100%,40rem)] rounded-sm bg-white/5"
                    />
                    <img
                      v-show="loaded"
                      ref="mediaEl"
                      :src="imageUrl"
                      :alt="info.photoName"
                      draggable="false"
                      class="max-h-full max-w-full rounded-[2px] object-contain select-none"
                      @load="imageLoaded"
                    />
                  </template>
                  <video
                    v-else-if="kind === 'video'"
                    ref="mediaEl"
                    controls
                    class="max-h-full max-w-full"
                    @loadedmetadata="mediaReady"
                  >
                    <source
                      :src="photoUrl(info.photoId, 'renderV')"
                      :type="`video/${info.format}`"
                    />
                  </video>
                  <div v-else ref="mediaEl" class="flex flex-col items-center gap-3 text-center">
                    <FileIcon class="text-muted-foreground size-14" aria-hidden="true" />
                    <p class="text-sm">{{ fileName }}</p>
                    <Button size="sm" variant="secondary" @click="downloadPhoto"
                      ><Download aria-hidden="true" /> Download</Button
                    >
                  </div>
                </div>
              </Transition>
              <button
                v-if="prevPhoto"
                data-chrome
                type="button"
                aria-label="Previous photo"
                class="absolute top-1/2 left-4 z-10 -mt-[22px] hidden size-11 place-items-center rounded-full bg-white/6 transition-colors outline-none hover:bg-white/12 focus-visible:ring-3 focus-visible:ring-ring/50 md:grid"
                @click="step(-1)"
              >
                <ChevronLeft class="size-5" aria-hidden="true" />
              </button>
              <button
                v-if="nextPhoto"
                data-chrome
                type="button"
                aria-label="Next photo"
                class="absolute top-1/2 right-4 z-10 -mt-[22px] hidden size-11 place-items-center rounded-full bg-white/6 transition-colors outline-none hover:bg-white/12 focus-visible:ring-3 focus-visible:ring-ring/50 md:grid"
                @click="step(1)"
              >
                <ChevronRight class="size-5" aria-hidden="true" />
              </button>
            </div>

            <nav
              v-if="strip.length"
              data-chrome
              aria-label="Nearby photos"
              class="hidden shrink-0 justify-center gap-1 overflow-x-auto px-4 pt-1 pb-5 md:flex"
            >
              <button
                v-for="item in strip"
                :key="item.photo.photoId"
                type="button"
                :aria-label="`Photo ${item.n}`"
                :aria-current="item.n === index + 1 ? 'true' : undefined"
                class="group size-12 shrink-0 overflow-hidden rounded-[4px] outline-offset-2 transition-[outline-color] aria-[current=true]:outline-2 aria-[current=true]:outline-primary"
                @click="jump(item.photo)"
              >
                <img
                  v-if="mediaKind(item.photo.format) === 'photo'"
                  :src="photoUrl(item.photo.photoId, 'renderThumbnail')"
                  alt=""
                  loading="lazy"
                  draggable="false"
                  class="size-full object-cover opacity-55 transition-opacity duration-200 group-hover:opacity-100 group-aria-[current=true]:opacity-100"
                />
                <span
                  v-else
                  class="text-muted-foreground grid size-full place-items-center bg-white/8"
                >
                  <component
                    :is="mediaKind(item.photo.format) === 'video' ? Film : FileIcon"
                    class="size-4"
                    aria-hidden="true"
                  />
                </span>
              </button>
            </nav>
          </template>
        </section>

        <!-- Details: beside the photo on desktop, a sheet under it on phones -->
        <aside
          v-if="!editing"
          data-chrome="panel"
          aria-label="Photo details"
          :class="[
            'bg-card relative z-10 -mt-4 shrink-0 rounded-t-2xl md:mt-0 md:rounded-none md:transition-[width] md:duration-350 md:ease-(--ease-out-quart)',
            showDetails ? 'md:w-[360px] md:border-l' : 'md:w-0',
            'md:overflow-hidden',
            !desktop &&
              !draggingSheet &&
              'transition-transform duration-350 ease-(--ease-out-quart)',
          ]"
          :style="sheetDrag ? { transform: `translateY(${sheetDrag}px)` } : undefined"
          @touchstart.passive="sheetTouchStart"
          @touchmove="sheetTouchMove"
          @touchend="sheetTouchEnd"
          @touchcancel="sheetTouchEnd"
        >
          <div
            ref="panelScroll"
            tabindex="-1"
            :data-testid="desktop ? 'photo-viewer-scroll' : undefined"
            :class="[
              'outline-none md:absolute md:inset-y-0 md:left-0 md:w-[360px] md:overflow-y-auto md:overscroll-contain md:[scrollbar-width:thin]',
              desktop && recommendations?.expanded && 'pb-20',
            ]"
          >
            <button
              type="button"
              class="flex h-5 w-full items-center justify-center outline-none focus-visible:[&>span]:bg-white/60 md:hidden"
              :aria-label="sheetHidden ? 'Show details' : 'Hide details'"
              :aria-expanded="!sheetHidden"
              @click="showDetails = !showDetails"
            >
              <span class="h-1 w-9 rounded-full bg-white/20" />
            </button>
            <header
              :class="[
                'flex flex-col gap-1 px-5 pt-2 md:border-b md:px-6 md:pt-6 md:pb-5',
                sheetHidden && 'pb-[calc(1rem_+_env(safe-area-inset-bottom))]',
              ]"
              @click="sheetHidden && (showDetails = true)"
            >
              <h2
                :class="[
                  'text-lg leading-7 font-semibold',
                  sheetHidden ? 'truncate' : 'break-words',
                ]"
              >
                {{ info.photoName }}
              </h2>
              <p class="text-muted-foreground text-sm break-all max-md:hidden">{{ fileMeta }}</p>
              <p v-if="when && !sheetHidden" class="text-muted-foreground text-sm md:hidden">
                {{ when.short }}
              </p>
            </header>

            <!-- Out of reach while the phone sheet is pulled down -->
            <div :inert="sheetHidden || undefined">
              <div
                class="grid grid-cols-[20px_minmax(0,1fr)] gap-x-3 gap-y-4 px-5 py-4 text-sm md:border-b md:px-6 md:py-5 [&>svg]:mt-px [&>svg]:size-[18px] [&>svg]:text-muted-foreground"
              >
                <template v-if="when">
                  <CalendarDays class="max-md:hidden" aria-hidden="true" />
                  <div class="max-md:hidden">
                    <div>{{ when.day }}</div>
                    <div class="text-muted-foreground">{{ when.detail }}</div>
                  </div>
                </template>
                <template v-if="hasGps">
                  <MapPin aria-hidden="true" />
                  <div>
                    <div class="tabular-nums">
                      {{ exif.GPS_Latitude }}, {{ exif.GPS_Longitude }}
                    </div>
                    <div class="flex flex-wrap gap-x-3">
                      <router-link
                        :to="{ path: '/home/MapPart', query: { photo: info.photoId } }"
                        class="text-primary font-medium hover:underline"
                        >Open in Map</router-link
                      >
                      <a
                        :href="`https://maps.google.com/?q=${exif.GPS_Latitude},${exif.GPS_Longitude}`"
                        target="_blank"
                        rel="noopener"
                        class="text-muted-foreground hover:text-foreground"
                        >Google Maps</a
                      >
                    </div>
                  </div>
                </template>
                <template v-if="folder">
                  <Folder class="max-md:hidden" aria-hidden="true" />
                  <div class="break-words max-md:hidden">{{ folder }}</div>
                </template>
                <template v-if="camera || lens || exposure.length">
                  <Aperture class="max-md:hidden" aria-hidden="true" />
                  <!-- Phones: the exposure grid first, then "camera · lens" on one line. -->
                  <div
                    class="flex flex-col gap-2.5 max-md:order-first max-md:col-span-2 max-md:flex-col-reverse"
                  >
                    <div v-if="camera || lens" class="max-md:text-muted-foreground">
                      <span v-if="camera" class="md:block">{{ camera }}</span
                      ><span v-if="camera && lens" class="md:hidden"> · </span
                      ><span v-if="lens" class="md:text-muted-foreground md:block">{{ lens }}</span>
                    </div>
                    <dl
                      v-if="exposure.length"
                      class="m-0 grid gap-px overflow-hidden rounded-lg border bg-border"
                      :style="{ gridTemplateColumns: `repeat(${exposure.length}, minmax(0, 1fr))` }"
                    >
                      <div v-for="x in exposure" :key="x.label" class="bg-card px-2 py-1.5">
                        <dt class="text-muted-foreground text-[11px] leading-[14px]">
                          {{ x.label }}
                        </dt>
                        <dd class="m-0 truncate font-medium">{{ x.value }}</dd>
                      </div>
                    </dl>
                  </div>
                </template>
              </div>

              <!-- Phones run tags and galleries together as one row of chips. -->
              <div
                :class="[
                  section,
                  'max-md:flex-row max-md:flex-wrap max-md:gap-1.5 max-md:border-b-0 max-md:py-2',
                ]"
              >
                <h3 :class="[heading, 'w-full pb-0.5 md:hidden']">Tags and galleries</h3>
                <div class="flex flex-col gap-2 max-md:contents">
                  <h3 :class="[heading, 'max-md:hidden']">Tags</h3>
                  <div class="flex flex-wrap gap-1.5 max-md:contents">
                    <span
                      v-for="t in tags"
                      :key="t.tag_id"
                      :class="[
                        chip,
                        t.smart &&
                          'text-primary border-dashed border-primary/50 hover:border-primary',
                      ]"
                      :title="t.smart?.basis.method"
                    >
                      <Sparkles v-if="t.smart" class="size-3 shrink-0" aria-hidden="true" />
                      <span class="min-w-0 truncate">{{ t.tag_name }}</span>
                      <span v-if="t.smart" class="sr-only"><SmartBadge :smart="t.smart" /></span>
                      <button
                        type="button"
                        :class="[removeButton, 'max-md:hidden']"
                        :aria-label="`Remove ${t.tag_name}`"
                        @click="removeTag(t.tag_id)"
                      >
                        <X class="size-3" aria-hidden="true" />
                      </button>
                    </span>
                    <Popover v-model:open="tagOpen" @update:open="openPicker">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          aria-label="Add tag"
                          :class="[addChip, 'max-md:order-last']"
                        >
                          <Plus class="size-3" aria-hidden="true" />Add
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" :class="[overlay, 'w-80 p-0']">
                        <PhotoLabelPicker
                          mode="tag"
                          :tags="pickerTags"
                          :galleries="pickerGalleries"
                          :attached-tags="attachedTags"
                          :attached-galleries="attachedGalleries"
                          @toggle-tag="(id, on) => (on ? removeTag(id) : addTag(id))"
                          @toggle-gallery="(id, on) => (on ? removeGallery(id) : addGallery(id))"
                          @create="create"
                          @done="tagOpen = false"
                        />
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
                <div class="flex flex-col gap-2 max-md:contents">
                  <h3 :class="[heading, 'max-md:hidden']">Galleries</h3>
                  <div class="flex flex-wrap gap-1.5 max-md:contents">
                    <span
                      v-for="g in galleries"
                      :key="g.ga_id"
                      :class="chip"
                      :title="g.smart?.basis.method"
                    >
                      <span
                        class="bg-primary mr-0.5 size-2 shrink-0 rounded-full"
                        aria-hidden="true"
                      />
                      <span class="min-w-0 truncate">{{ g.ga_name }}</span>
                      <SmartBadge :smart="g.smart" />
                      <button
                        type="button"
                        :class="[removeButton, 'max-md:hidden']"
                        :aria-label="`Remove ${g.ga_name}`"
                        @click="removeGallery(g.ga_id)"
                      >
                        <X class="size-3" aria-hidden="true" />
                      </button>
                    </span>
                    <Popover v-model:open="galleryOpen" @update:open="openPicker">
                      <PopoverTrigger as-child>
                        <button
                          type="button"
                          aria-label="Add gallery"
                          :class="[addChip, 'max-md:hidden']"
                        >
                          <Plus class="size-3" aria-hidden="true" />Add
                        </button>
                      </PopoverTrigger>
                      <PopoverContent align="start" :class="[overlay, 'w-80 p-0']">
                        <PhotoLabelPicker
                          mode="gallery"
                          :tags="pickerTags"
                          :galleries="pickerGalleries"
                          :attached-tags="attachedTags"
                          :attached-galleries="attachedGalleries"
                          @toggle-tag="(id, on) => (on ? removeTag(id) : addTag(id))"
                          @toggle-gallery="(id, on) => (on ? removeGallery(id) : addGallery(id))"
                          @create="create"
                          @done="galleryOpen = false"
                        />
                      </PopoverContent>
                    </Popover>
                  </div>
                </div>
              </div>

              <div v-if="kind === 'photo'" class="border-b px-5 py-4 md:px-6">
                <PhotoText
                  :key="`${info.photoId}:${version}`"
                  ref="photoText"
                  :photo-id="info.photoId"
                />
              </div>

              <div v-if="canSimilar" :class="[section, 'gap-3']">
                <div class="flex items-center gap-2">
                  <h3 :class="heading">Similar</h3>
                  <Sparkles class="text-primary size-3.5" aria-label="Smart" role="img" />
                  <button
                    v-if="!recommendations?.expanded"
                    type="button"
                    class="text-primary ml-auto rounded-sm text-[13px] font-medium transition-colors outline-none hover:underline focus-visible:ring-3 focus-visible:ring-ring/50 disabled:text-muted-foreground disabled:no-underline"
                    :disabled="!recommendations?.allowed"
                    :title="recommendations?.allowed ? undefined : recommendations?.unavailable"
                    @click="findSimilar"
                  >
                    Find similar
                  </button>
                </div>
                <p v-if="!recommendations?.expanded" class="text-muted-foreground text-sm">
                  Photos and videos that look like this one.
                </p>
                <SimilarRecommendations
                  :key="`${info.photoId}:${version}:${recommendationSession}`"
                  ref="recommendations"
                  :photo="info"
                  :resume="restoredRecommendations"
                  @navigate="navigate"
                  @invalidated="invalidateHistory"
                />
              </div>

              <div class="flex flex-col gap-3 px-5 py-5 md:px-6">
                <label class="flex items-center gap-3 text-sm">
                  <span class="flex flex-1 flex-col">
                    <span class="font-medium">Share link</span>
                    <span class="text-muted-foreground">{{
                      info.visibility === 1
                        ? 'Anyone with the link can view'
                        : 'Only you can see this'
                    }}</span>
                  </span>
                  <Switch
                    aria-label="Visibility"
                    :model-value="info.visibility === 1"
                    @update:model-value="changeVisibility"
                  />
                </label>
                <div v-if="info.visibility === 1" class="flex gap-2">
                  <input
                    :value="shareLink"
                    readonly
                    aria-label="Share link"
                    class="h-9 min-w-0 flex-1 rounded-lg border border-white/16 bg-white/4 px-2.5 font-mono text-xs outline-none"
                    @focus="($event.target as HTMLInputElement).select()"
                  />
                  <Button variant="secondary" @click="copyShareAddr"
                    ><Copy aria-hidden="true" /> Copy</Button
                  >
                </div>
                <details v-collapse class="group text-sm">
                  <summary
                    class="text-muted-foreground hover:text-foreground flex cursor-pointer list-none items-center gap-1 text-xs transition-colors [&::-webkit-details-marker]:hidden"
                  >
                    Technical details
                    <ChevronRight
                      class="size-3 transition-transform group-open:rotate-90"
                      aria-hidden="true"
                    />
                  </summary>
                  <dl class="text-muted-foreground mt-2 space-y-1 text-xs">
                    <dt class="text-foreground font-medium">Token</dt>
                    <dd class="break-all">{{ info.token }}</dd>
                    <dt class="text-foreground font-medium">UUID</dt>
                    <dd class="break-all">{{ info.photoUuid }}</dd>
                  </dl>
                </details>
              </div>
            </div>
          </div>
        </aside>
      </div>

      <Transition name="reveal">
        <Button
          v-if="!editing && recommendations?.expanded"
          class="absolute right-4 bottom-4 z-20 shadow-lg"
          aria-label="Back to top and close similar photos and videos"
          @click="backToTop"
        >
          <ArrowUp aria-hidden="true" /> Back to top
        </Button>
      </Transition>

      <!-- Share -->
      <Dialog v-model:open="sharing">
        <DialogContent :class="[overlay, 'sm:max-w-[480px]']">
          <DialogHeader>
            <DialogTitle>Share Photo</DialogTitle>
            <DialogDescription class="sr-only">Share a link to {{ fileName }}</DialogDescription>
          </DialogHeader>
          <div class="flex items-center gap-3 rounded-lg border bg-white/4 p-2.5">
            <img
              v-if="kind === 'photo'"
              :src="photoUrl(info.photoId, 'renderThumbnail')"
              alt=""
              class="size-12 shrink-0 rounded-md object-cover"
            />
            <span v-else class="grid size-12 shrink-0 place-items-center rounded-md bg-white/8"
              ><FileIcon class="size-5" aria-hidden="true"
            /></span>
            <span class="flex min-w-0 flex-col text-sm">
              <span class="truncate font-medium">{{ info.photoName }}</span>
              <span class="text-muted-foreground truncate">{{
                [fileName, exif.File_Size].filter(Boolean).join(' · ')
              }}</span>
            </span>
          </div>
          <label class="flex items-center gap-3 text-sm">
            <span class="flex flex-1 flex-col">
              <span class="font-medium">Share link</span>
              <span class="text-muted-foreground">{{
                info.visibility === 1 ? 'Anyone with the link can view it' : 'Only you can see it'
              }}</span>
            </span>
            <Switch
              aria-label="Visibility"
              :model-value="info.visibility === 1"
              @update:model-value="changeVisibility"
            />
          </label>
          <template v-if="info.visibility === 1">
            <div class="flex flex-col gap-1.5">
              <span class="text-sm font-medium">Link</span>
              <div class="flex gap-2">
                <input
                  :value="shareLink"
                  readonly
                  aria-label="Link"
                  class="h-9 min-w-0 flex-1 rounded-lg border border-white/16 bg-white/4 px-2.5 font-mono text-xs outline-none"
                  @focus="($event.target as HTMLInputElement).select()"
                />
                <Button @click="copyShareAddr"><Copy aria-hidden="true" /> Copy Link</Button>
              </div>
            </div>
            <div class="flex items-center gap-4 rounded-lg border p-3">
              <img :src="qrUrl" alt="QR code" class="size-28 shrink-0 rounded-md bg-white p-1.5" />
              <div class="flex flex-col items-start gap-2 text-sm">
                <span>
                  <span class="block font-medium">Open on a phone</span>
                  <span class="text-muted-foreground block"
                    >Scan the code with the camera app.</span
                  >
                </span>
                <Button size="sm" variant="outline" @click="saveQr"
                  ><Download aria-hidden="true" /> Save QR Code</Button
                >
              </div>
            </div>
          </template>
          <p class="text-muted-foreground text-xs">
            Turning the link off stops it working for everyone who has it.
          </p>
          <DialogFooter>
            <Button @click="sharing = false">Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog v-model:open="confirmingDelete">
        <AlertDialogContent :class="overlay">
          <AlertDialogHeader>
            <AlertDialogTitle>Delete {{ fileName }}?</AlertDialogTitle>
            <AlertDialogDescription
              >It's deleted for good, along with its tags and gallery places. This can't be
              undone.</AlertDialogDescription
            >
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <Button variant="destructive" @click="deletePhoto">Delete</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
.slide-next-enter-active,
.slide-next-leave-active,
.slide-prev-enter-active,
.slide-prev-leave-active,
.slide-fade-enter-active,
.slide-fade-leave-active {
  transition:
    opacity 0.22s ease,
    transform 0.32s var(--ease-out-quart);
}
.slide-next-enter-from,
.slide-prev-leave-to {
  opacity: 0;
  transform: translateX(40px);
}
.slide-next-leave-to,
.slide-prev-enter-from {
  opacity: 0;
  transform: translateX(-40px);
}
.slide-fade-enter-from,
.slide-fade-leave-to {
  opacity: 0;
}
@media (prefers-reduced-motion: reduce) {
  [class*='slide-'] {
    transition: none !important;
  }
}
</style>

<style>
/* The viewer brings its own backdrop; the stock dialog overlay would flash under it. */
[data-slot='dialog-overlay']:has(+ [data-photo-viewer]) {
  display: none;
}
</style>
