<script setup lang="ts">
import SmartBadge from '@/components/SmartBadge.vue'
import type { SmartSource } from '@/lib/smartOrganization'
import { gsap } from 'gsap'
import { useMediaQuery } from '@vueuse/core'
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import {
  ArrowLeft,
  ArrowUp,
  Copy,
  Download,
  ExternalLink,
  Info,
  Pencil,
  Share2,
  Sparkles,
  X,
} from '@lucide/vue'
import ImageEditor from '@/components/ImageEditor.vue'
import PhotoText from '@/components/PhotoText.vue'
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
  defineProps<{
    photo: Photo | null
    nsPath: string
    allowSimilar?: boolean
    origin?: DOMRect | null
  }>(),
  { allowSimilar: true },
)
const emit = defineEmits<{ close: []; changed: [change?: { type: string; photo: Photo }] }>()

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
const viewerBody = ref<HTMLElement>()
const scrollRoot = computed(() => viewerBody.value ?? null)
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
  history.value.push({ photo: { ...info.value }, path: currentPath.value })
  selected.value = { ...photo }
}
async function previous() {
  if (!(await leaveText())) return
  const item = history.value.pop()
  if (!item) return
  selected.value = { ...item.photo }
  currentPath.value = item.path
}

/** A copy, so renames and visibility changes show here without mutating the parent's list. */
const info = ref<Photo>()
const exif = ref<Record<string, string | undefined>>({})
const tags = ref<{ tag_id: number; tag_name: string; smart?: SmartSource | null }[]>([])
const galleries = ref<{ ga_id: number; ga_name: string; smart?: SmartSource | null }[]>([])
const tagOptions = ref<Tag[]>([])
const galleryOptions = ref<Gallery[]>([])
const tagChoice = ref('')
const galleryChoice = ref('')
const newName = ref('')
const loaded = ref(false)
const editing = ref(false)
/** Hidden by default and again for every photo; kept mounted (v-show) so a text draft survives toggling. */
const showDetails = ref(false)
const detailsLabel = ref<HTMLElement | null>(null)
/** Expand the "Details" label on hover/focus, collapse back to the round "i". */
function expandDetails(open: boolean) {
  if (!detailsLabel.value) return
  const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  gsap.to(detailsLabel.value, {
    width: open ? 'auto' : 0,
    opacity: open ? 1 : 0,
    duration: reduce ? 0 : 0.35,
    ease: 'power3.out',
    overwrite: true,
  })
}
// Stay expanded while the panel is open; collapse when it closes (click or photo change).
watch(showDetails, expandDetails)
/** Details pane enter/leave: fade + slide, plus a height reveal when it stacks under the media. */
/** Opening: the viewer zooms in while the backdrop fades to dark. Runs once per open, not per photo. */
let openPlayed = false
const reducedMotion = () => !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
// `animation: none` stays on while open (clearing it would replay the stock CSS open animation);
// hand it back right as the dialog closes so the stock close animation still plays.
watch(acceptedPhoto, (photo) => {
  if (photo) return
  for (const el of document.querySelectorAll<HTMLElement>(
    '[data-slot=dialog-content], [data-slot=dialog-overlay]',
  ))
    el.style.removeProperty('animation')
})
// The backdrop mounts as soon as the dialog opens but stays clear until the viewer is ready
// (`showBackdrop`, called when the card-to-modal animation starts), so it never darkens the screen
// while the photo is still decoding. Falls back after 2s in case the viewer never mounts.
let showBackdrop: (() => void) | null = null
watch(
  acceptedPhoto,
  (photo) => {
    if (!photo) return void ((openPlayed = false), (showBackdrop = null))
    const overlay = document.querySelector<HTMLElement>('[data-slot=dialog-overlay]')
    if (!overlay) return
    gsap.set(overlay, { animation: 'none', opacity: 0 })
    const timer = setTimeout(() => showBackdrop?.(), 2000)
    showBackdrop = () => {
      showBackdrop = null
      clearTimeout(timer)
      gsap.to(overlay, {
        opacity: 1,
        duration: reducedMotion() ? 0 : 0.6,
        ease: 'power1.out',
        clearProps: 'opacity',
      })
    }
  },
  { flush: 'post' },
)
/**
 * Card ↔ modal: where the clicked thumbnail sits relative to the open viewer. The viewer is placed
 * there (scaled down) and animated to its real position, and back again on dismiss. Not gsap
 * transforms on the element: GSAP would fold Tailwind's centering `translate` into px.
 */
let openTl: gsap.core.Timeline | null = null
function cardOffset(content: HTMLElement) {
  const box = content.getBoundingClientRect()
  const o = selected.value ? null : props.origin // navigated to another photo: no card to return to
  return o && box.width
    ? {
        s: Math.min(1, o.width / box.width),
        dx: o.left + o.width / 2 - (box.left + box.width / 2),
        dy: o.top + o.height / 2 - (box.top + box.height / 2),
      }
    : { s: 0.8, dx: 0, dy: 0 }
}
function placeCard(content: HTMLElement, from: { s: number; dx: number; dy: number }, v: number) {
  const k = 1 - v
  content.style.transform = `translate(${from.dx * k}px, ${from.dy * k}px) scale(${from.s + (1 - from.s) * v})`
}
/** Shrink the viewer back into its card while the backdrop fades out. Resolves when done. */
function playClose() {
  const content = viewerBody.value?.closest<HTMLElement>('[data-slot=dialog-content]')
  const overlay = document.querySelector<HTMLElement>('[data-slot=dialog-overlay]')
  if (!content || reducedMotion()) return Promise.resolve()
  startOpen = null
  if (openTl) {
    openTl.kill()
    openTl = null
    for (const prop of ['transform', 'opacity']) content.style.removeProperty(prop)
  }
  if (!content.getBoundingClientRect().width) return Promise.resolve() // not laid out (e.g. jsdom)
  content.style.animation = 'none'
  content.style.transition = 'none'
  content.style.willChange = 'transform, opacity'
  const from = cardOffset(content)
  const p = { v: 1 }
  const fade = { v: 1 }
  return new Promise<void>((resolve) => {
    gsap
      .timeline({ onComplete: resolve })
      .to(
        p,
        {
          v: 0,
          duration: 0.4,
          ease: 'power3.inOut',
          onUpdate: () => placeCard(content, from, p.v),
        },
        0,
      )
      .to(
        fade,
        {
          v: 0,
          duration: 0.2,
          ease: 'power1.in',
          onUpdate: () => (content.style.opacity = String(fade.v)),
        },
        0.2,
      )
      .to(overlay, { opacity: 0, duration: 0.4, ease: 'power1.inOut' }, 0)
  })
}
/** Set while the open animation waits for the media size, so the viewer doesn't resize mid-grow. */
let startOpen: (() => void) | null = null
const mediaReady = () => startOpen?.()
watch(
  viewerBody,
  (body) => {
    const content = body?.closest<HTMLElement>('[data-slot=dialog-content]')
    if (!content || openPlayed) return
    openPlayed = true
    // Silence the stock CSS open animation, then hand `animation` back so the close one still plays.
    // Plain style writes on `content`: a GSAP tween on it would overwrite our inline transform.
    content.style.animation = 'none'
    content.style.transition = 'none' // the dialog's stock CSS transition would lag behind our frames
    content.style.opacity = '0'
    content.style.willChange = 'transform, opacity' // own compositor layer: no re-raster per frame
    const run = () => {
      if (!content.isConnected) return
      const duration = reducedMotion() ? 0 : 0.55
      // Not gsap transforms: GSAP would fold Tailwind's centering `translate` into px and break the position.
      // With a thumbnail rect the viewer grows out of it; otherwise it just zooms from the centre.
      showBackdrop?.()
      const from = cardOffset(content)
      const p = { v: 0 }
      const place = () => placeCard(content, from, p.v)
      place()
      const fade = { v: 0 }
      openTl = gsap
        .timeline({
          onComplete: () => {
            content.style.removeProperty('transform')
            content.style.removeProperty('transition')
            content.style.removeProperty('opacity')
            content.style.removeProperty('will-change')
          },
        })
        .to(
          fade,
          {
            v: 1,
            duration: duration / 3,
            ease: 'power1.out',
            onUpdate: () => (content.style.opacity = String(fade.v)),
          },
          0,
        )
        .to(p, { v: 1, duration, ease: 'power3.out', onUpdate: place }, 0)
    }
    // Wait for the photo/video size (layout depends on it); fall back if it never loads.
    const timer = setTimeout(() => startOpen?.(), 800)
    startOpen = () => {
      startOpen = null
      clearTimeout(timer)
      void nextTick(run)
    }
  },
  { flush: 'post' }, // after the DOM patch but before paint, so no full-size frame shows first
)

const paneOpen = ref(false) // keeps the side-by-side grid until the leave animation ends
/**
 * Textarea-style resize (desktop only): drag the corner grip to scale the viewer, locked to the
 * media's aspect ratio. Landscape sizes the dialog (the media fills it); tall media sizes the picture.
 */
const RESIZE_MIN = 280
const PANE_WIDTH = 336 // 21rem details column beside tall media
const canResize = useMediaQuery('(min-width: 768px)')
const mediaWidth = ref<number | null>(null)
const ratio = ref(1) // media width / height
const resizeActive = computed(() => canResize.value && mediaWidth.value !== null)
const resizeMax = () => {
  const pane = !landscape.value && paneOpen.value ? PANE_WIDTH : 0
  return Math.max(RESIZE_MIN, Math.min(innerWidth - 32 - pane, innerHeight * 0.9 * ratio.value))
}
// Opening the details column beside tall media must not push a big picture off-screen.
const effectiveWidth = () => Math.min(mediaWidth.value ?? 0, resizeMax())
/** Default size scales with the screen: ~75% of its width, ~70% (wide) or 90% (tall) of its height. */
function setDefaultWidth() {
  mediaWidth.value = canResize.value
    ? Math.round(
        Math.max(
          RESIZE_MIN,
          Math.min(
            innerWidth * 0.75,
            innerHeight * (landscape.value ? 0.7 : 0.9) * ratio.value,
            resizeMax(),
          ),
        ),
      )
    : null
}
// A fixed, centred dialog can only shrink-wrap to the space right of its left edge (half the screen),
// so tall media needs an explicit width too: picture + the details column as it animates open.
const panePx = ref(0)
const paneTweenState = { v: 0 }
watch(paneOpen, (open) => open || (panePx.value = paneTweenState.v = 0))
const dialogStyle = computed(() =>
  resizeActive.value && !editing.value
    ? {
        width: `${effectiveWidth() + (landscape.value ? 0 : panePx.value + 2)}px`,
        maxWidth: 'calc(100vw - 2rem)',
        transition: 'none',
      }
    : undefined,
)
const mediaStyle = computed(() =>
  !resizeActive.value
    ? undefined
    : landscape.value
      ? { width: '100%', height: 'auto', maxHeight: 'none' }
      : { width: `${effectiveWidth()}px`, height: 'auto', maxWidth: 'none', maxHeight: 'none' },
)
function startResize(e: PointerEvent) {
  const grip = e.currentTarget as HTMLElement
  const media = grip.parentElement?.querySelector<HTMLElement>('img:not([hidden]), video')
  const dialog = grip.closest<HTMLElement>('[data-slot=dialog-content]')
  const start = landscape.value ? dialog?.offsetWidth : media?.offsetWidth
  if (!start) return
  const x0 = e.clientX
  grip.setPointerCapture(e.pointerId)
  // The dialog is centred, so it grows on both sides: 2× keeps the grip under the pointer.
  const move = (m: PointerEvent) =>
    (mediaWidth.value = Math.round(
      Math.min(resizeMax(), Math.max(RESIZE_MIN, start + 2 * (m.clientX - x0))),
    ))
  const stop = () => {
    grip.removeEventListener('pointermove', move)
    grip.removeEventListener('pointerup', stop)
    grip.removeEventListener('pointercancel', stop)
  }
  grip.addEventListener('pointermove', move)
  grip.addEventListener('pointerup', stop)
  grip.addEventListener('pointercancel', stop)
  e.preventDefault()
}
// A new photo has its own aspect ratio, so start from the default size again.
watch(currentPhoto, () => (mediaWidth.value = null))

watch(showDetails, (v) => v && (paneOpen.value = true))
function paneTween(el: Element, show: boolean, done: () => void) {
  const reduce = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  gsap.killTweensOf(el) // a quick re-toggle must not fight the previous tween
  const vars = {
    duration: reduce ? 0 : 0.4,
    overflow: 'hidden',
    onComplete: () => (
      show ? gsap.set(el, { clearProps: 'all' }) : (paneOpen.value = false),
      done()
    ),
  }
  // Tall media on md+ puts the pane beside the picture, so reveal it by width; otherwise by height.
  const side = !landscape.value && !!window.matchMedia?.('(min-width: 768px)').matches
  const prop = side ? 'width' : 'height'
  if (side)
    gsap.to(paneTweenState, {
      v: show ? PANE_WIDTH : 0,
      duration: vars.duration,
      ease: show ? 'power3.out' : 'power2.in',
      overwrite: true,
      onUpdate: () => (panePx.value = paneTweenState.v),
    })
  // `to` on leave: the size must start from the measured value, not "auto".
  if (show)
    gsap.fromTo(
      el,
      { opacity: 0, [prop]: 0, ...(!side && { y: -12 }), ...(side && { x: -12 }) },
      { ...vars, opacity: 1, x: 0, y: 0, [prop]: side ? '21rem' : 'auto', ease: 'power3.out' },
    )
  else
    gsap.to(el, {
      ...vars,
      opacity: 0,
      [prop]: 0,
      ...(side ? { x: -12 } : { y: -12 }),
      ease: 'power2.in',
    })
}
onBeforeUnmount(() => detailsLabel.value && gsap.killTweensOf(detailsLabel.value))
/** Bumped after an overwrite so the browser refetches the image under the same URL. */
const version = ref(0)

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

const loadExif = () =>
  currentCall<typeof exif.value>('/photo/getPhotoExif', (value) => {
    exif.value = value
  }).catch(quiet('Exif Info Response Error!'))

watch(
  () => currentPhoto.value,
  (photo, previousPhoto) => {
    if (previousPhoto) rememberRecommendations(previousPhoto.photoId)
    showDetails.value = false
    paneOpen.value = false // the pane may unmount mid-leave, so its onComplete cannot be relied on
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
  const seq = metadataSequence
  await nextTick()
  if (seq !== metadataSequence || restoreScroll === null) return
  const top = restoreScroll
  restoreScroll = null
  scrollRoot.value?.scrollTo({ top, behavior: 'instant' })
}
/** Wide media gets a stacked layout so the picture can use the full width; tall or square sits beside the details. */
const landscape = ref(true)
function imageLoaded(e: Event) {
  const img = e.target as HTMLImageElement
  if (img.naturalWidth) {
    landscape.value = img.naturalWidth > img.naturalHeight * 1.05
    ratio.value = img.naturalWidth / img.naturalHeight
    setDefaultWidth()
  }
  loaded.value = true
  // Decode first: decoding a big image mid-animation is what drops frames.
  void (img.decode?.() ?? Promise.resolve()).catch(() => {}).then(mediaReady)
  void restorePosition()
}

async function findSimilar() {
  recommendations.value?.start()
  await nextTick()
  ;(recommendations.value?.$el as HTMLElement | undefined)?.scrollIntoView?.({
    block: 'start',
    behavior: 'smooth',
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

const section = 'group rounded-lg border bg-card'
const summary =
  'flex cursor-pointer list-none items-center justify-between px-3 py-2.5 text-sm font-medium [&::-webkit-details-marker]:hidden after:text-muted-foreground after:transition-transform after:content-["›"] group-open:after:rotate-90'
const body = 'space-y-3 border-t p-3 text-sm'
const label = 'text-xs font-medium text-muted-foreground'
</script>

<template>
  <Dialog :open="!!acceptedPhoto" @update:open="closeViewer">
    <DialogContent
      v-if="currentPhoto && info"
      :style="dialogStyle"
      :class="[
        'max-h-[95dvh] w-full overflow-hidden p-0',
        landscape || editing ? 'sm:max-w-4xl' : 'sm:w-fit sm:max-w-[calc(100%-2rem)]',
      ]"
    >
      <DialogTitle class="sr-only">{{ info.photoName }}.{{ info.format }}</DialogTitle>
      <DialogDescription class="sr-only"
        >Photo details and similar photos and videos</DialogDescription
      >
      <div
        ref="viewerBody"
        data-testid="photo-viewer-scroll"
        tabindex="-1"
        :class="[
          'max-h-[95dvh] min-h-0 overflow-y-auto overscroll-contain outline-none [scrollbar-width:none] [&::-webkit-scrollbar]:hidden',
          recommendations?.expanded && 'pb-20',
        ]"
      >
        <div v-if="history.length" class="px-4 pt-4">
          <Button variant="ghost" size="sm" @click="previous"
            ><ArrowLeft aria-hidden="true" /> Back to previous photo</Button
          >
        </div>
        <div v-if="editing" class="p-6">
          <ImageEditor :photo="info" @cancel="editing = false" @saved="edited" />
        </div>
        <div v-else :class="!landscape && paneOpen ? 'grid md:grid-cols-[auto_auto]' : ''">
          <!-- Portrait media stays in view while the details scroll. -->
          <div
            :class="[
              'relative flex items-center justify-center',
              landscape ? 'min-h-48 bg-muted' : '',
            ]"
          >
            <Button
              variant="secondary"
              size="sm"
              class="absolute top-3 left-3 z-10 gap-0 rounded-full bg-secondary/70 px-2 shadow backdrop-blur-sm hover:bg-secondary/90"
              :aria-pressed="showDetails"
              :aria-label="showDetails ? 'Hide details' : 'Details'"
              @click="showDetails = !showDetails"
              @mouseenter="expandDetails(true)"
              @mouseleave="expandDetails(showDetails)"
              @focus="expandDetails(($event.target as HTMLElement).matches(':focus-visible'))"
              @blur="expandDetails(showDetails)"
              ><Info aria-hidden="true" /><span
                ref="detailsLabel"
                class="overflow-hidden whitespace-nowrap"
                style="width: 0; opacity: 0"
                aria-hidden="true"
                ><span class="pr-1 pl-1.5">{{
                  showDetails ? 'Hide details' : 'Details'
                }}</span></span
              ></Button
            >
            <template v-if="kind === 'photo'">
              <Skeleton
                v-show="!loaded"
                :class="
                  landscape
                    ? 'h-[50vh] w-full rounded-none'
                    : 'h-[60vh] w-full rounded-none md:w-80'
                "
              />
              <img
                v-show="loaded"
                :src="imageUrl"
                :alt="info.photoName"
                :class="[
                  'max-w-full object-contain',
                  landscape ? 'max-h-[62dvh]' : 'max-h-[60vh] md:max-h-[90dvh] md:max-w-[34rem]',
                ]"
                :style="mediaStyle"
                @load="imageLoaded"
              />
            </template>
            <video
              v-else-if="kind === 'video'"
              :key="info.photoId"
              controls
              :style="mediaStyle"
              :class="[
                'w-full',
                landscape ? 'max-h-[62dvh]' : 'max-h-[60vh] md:max-h-[90dvh] md:max-w-[34rem]',
              ]"
              @loadedmetadata="
                (e) => {
                  const v = e.target as HTMLVideoElement
                  if (v.videoWidth) {
                    landscape = v.videoWidth > v.videoHeight * 1.05
                    ratio = v.videoWidth / v.videoHeight
                    setDefaultWidth()
                  }
                  mediaReady()
                }
              "
            >
              <source :src="photoUrl(info.photoId, 'renderV')" :type="`video/${info.format}`" />
            </video>
            <!-- Textarea-style grip; desktop only. Raised on video to clear the player controls. -->
            <div
              v-if="canResize && (kind === 'photo' ? loaded : kind === 'video')"
              data-testid="viewer-resize"
              aria-hidden="true"
              :class="[
                'absolute right-0 z-10 hidden size-5 cursor-nwse-resize touch-none items-end justify-end p-1 text-white/80 mix-blend-difference md:flex',
                kind === 'video' ? 'bottom-12' : 'bottom-0',
              ]"
              @pointerdown="startResize"
            >
              <svg
                viewBox="0 0 10 10"
                class="size-3"
                fill="none"
                stroke="currentColor"
                stroke-width="1.2"
              >
                <path d="M9 1 1 9M9 5 5 9" />
              </svg>
            </div>
          </div>

          <Transition
            :css="false"
            @enter="(el, done) => paneTween(el, true, done)"
            @leave="(el, done) => paneTween(el, false, done)"
          >
            <div v-show="showDetails" :class="landscape ? '' : 'md:relative md:w-[21rem]'">
              <div
                :class="[
                  'min-w-0 p-4 md:p-5',
                  !landscape &&
                    'md:absolute md:top-0 md:bottom-0 md:left-0 md:w-[21rem] md:overflow-y-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden',
                ]"
              >
                <div
                  :class="[
                    'mb-4 flex flex-col gap-3',
                    landscape && 'md:flex-row md:items-start md:justify-between',
                  ]"
                >
                  <header class="min-w-0 space-y-1 pr-6">
                    <h2 class="break-all text-lg font-semibold leading-snug">
                      {{ info.photoName }}.{{ info.format }}
                    </h2>
                    <p class="text-sm text-muted-foreground">
                      Uploaded {{ info.uploadDate
                      }}<template v-if="exif.File_Size">, {{ exif.File_Size }}</template>
                    </p>
                    <p class="break-all text-xs text-muted-foreground">{{ currentPath }}</p>
                  </header>

                  <div class="flex flex-wrap gap-2 md:shrink-0">
                    <Button size="sm" @click="downloadPhoto"
                      ><Download aria-hidden="true" /> Download</Button
                    >
                    <Button v-if="canEdit" variant="secondary" size="sm" @click="editImage"
                      ><Pencil aria-hidden="true" /> Edit Image</Button
                    >
                    <Button v-if="kind === 'photo'" variant="secondary" size="sm" as-child>
                      <a :href="imageUrl" target="_blank" rel="noopener"
                        ><ExternalLink aria-hidden="true" /> Full size</a
                      >
                    </Button>
                    <Popover>
                      <PopoverTrigger as-child
                        ><Button variant="secondary" size="sm"
                          ><Share2 aria-hidden="true" /> Share</Button
                        ></PopoverTrigger
                      >
                      <PopoverContent class="w-56 space-y-3">
                        <Button variant="outline" size="sm" class="w-full" @click="copyShareAddr"
                          ><Copy aria-hidden="true" /> Copy link</Button
                        >
                        <img :src="qrUrl" alt="QR code" class="mx-auto w-40" />
                      </PopoverContent>
                    </Popover>
                    <Button
                      v-if="canSimilar && !recommendations?.expanded"
                      variant="secondary"
                      size="sm"
                      :disabled="!recommendations?.allowed"
                      :title="recommendations?.allowed ? undefined : recommendations?.unavailable"
                      @click="findSimilar"
                      ><Sparkles aria-hidden="true" /> Find similar</Button
                    >
                  </div>
                </div>

                <div
                  :class="
                    landscape
                      ? 'space-y-4 md:columns-2 md:gap-5 md:space-y-0 md:[&>*]:mb-4 md:[&>*]:break-inside-avoid'
                      : 'space-y-4'
                  "
                >
                  <label
                    class="flex items-center justify-between gap-3 rounded-lg border bg-card px-3 py-2.5 text-sm"
                  >
                    <span>
                      <span class="block font-medium">Shared</span>
                      <span class="block text-xs text-muted-foreground">{{
                        info.visibility === 0
                          ? 'Only you can see this'
                          : 'Anyone with the link can see this'
                      }}</span>
                    </span>
                    <Switch
                      aria-label="Visibility"
                      :model-value="info.visibility === 1"
                      @update:model-value="changeVisibility"
                    />
                  </label>

                  <PhotoText
                    v-if="kind === 'photo'"
                    :key="`${info.photoId}:${version}`"
                    ref="photoText"
                    :photo-id="info.photoId"
                  />

                  <details open :class="section">
                    <summary :class="summary">Tags and galleries</summary>
                    <div :class="body">
                      <div class="space-y-2">
                        <p :class="label">Tags</p>
                        <div class="flex flex-wrap gap-1">
                          <Badge
                            v-for="t in tags"
                            :key="t.tag_id"
                            variant="secondary"
                            class="gap-1"
                          >
                            {{ t.tag_name }} <SmartBadge :smart="t.smart" />
                            <button
                              type="button"
                              :aria-label="`Remove ${t.tag_name}`"
                              @click="removeTag(t.tag_id)"
                            >
                              <X class="size-3" />
                            </button>
                          </Badge>
                          <span v-if="!tags.length" class="text-muted-foreground">No tags yet</span>
                        </div>
                        <div class="flex gap-2">
                          <!-- Options load on first open so the viewer doesn't fetch every tag up front. -->
                          <Select v-model="tagChoice" @update:open="(o) => o && loadTagOptions()">
                            <SelectTrigger class="flex-1" aria-label="Tag"
                              ><SelectValue placeholder="Add a tag"
                            /></SelectTrigger>
                            <SelectContent>
                              <SelectItem
                                v-for="t in tagOptions"
                                :key="t.tagId"
                                :value="String(t.tagId)"
                                >{{ t.tagName }} <SmartBadge :smart="t.smart"
                              /></SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="sm" :disabled="!tagChoice" @click="addTag">Add</Button>
                        </div>
                      </div>
                      <div class="space-y-2">
                        <p :class="label">Galleries</p>
                        <div class="flex flex-wrap gap-1">
                          <Badge
                            v-for="g in galleries"
                            :key="g.ga_id"
                            variant="secondary"
                            class="gap-1"
                          >
                            {{ g.ga_name }} <SmartBadge :smart="g.smart" />
                            <button
                              type="button"
                              :aria-label="`Remove ${g.ga_name}`"
                              @click="removeGallery(g.ga_id)"
                            >
                              <X class="size-3" />
                            </button>
                          </Badge>
                          <span v-if="!galleries.length" class="text-muted-foreground"
                            >Not in a gallery</span
                          >
                        </div>
                        <div class="flex gap-2">
                          <Select
                            v-model="galleryChoice"
                            @update:open="(o) => o && loadGalleryOptions()"
                          >
                            <SelectTrigger class="flex-1" aria-label="Gallery"
                              ><SelectValue placeholder="Add to a gallery"
                            /></SelectTrigger>
                            <SelectContent>
                              <SelectItem
                                v-for="g in galleryOptions"
                                :key="g.gaId"
                                :value="String(g.gaId)"
                                >{{ g.gaName }} <SmartBadge :smart="g.smart"
                              /></SelectItem>
                            </SelectContent>
                          </Select>
                          <Button size="sm" :disabled="!galleryChoice" @click="addGallery"
                            >Add</Button
                          >
                        </div>
                      </div>
                    </div>
                  </details>

                  <details v-if="hasGps" :class="section">
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
                          >Open in Maps</a
                        >
                      </Button>
                    </div>
                  </details>

                  <details :class="section">
                    <summary :class="summary">Camera and EXIF</summary>
                    <dl :class="[body, 'grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5']">
                      <template v-if="exif.Time"
                        ><dt :class="label">Taken</dt>
                        <dd>{{ exif.Time }}</dd></template
                      >
                      <template v-if="exif.Image_Height && exif.Image_Width"
                        ><dt :class="label">Dimensions</dt>
                        <dd>
                          {{ exif.Image_Width }} × {{ exif.Image_Height.split(' ')[0] }}
                        </dd></template
                      >
                      <template v-if="exif.Model"
                        ><dt :class="label">Camera</dt>
                        <dd>{{ exif.Make }} {{ exif.Model }}</dd></template
                      >
                      <template v-if="exif.Lens_Make || exif.Lens_Model"
                        ><dt :class="label">Lens</dt>
                        <dd>{{ exif.Lens_Make }} {{ exif.Lens_Model }}</dd></template
                      >
                      <template v-if="exif.Focal_Length"
                        ><dt :class="label">Focal length</dt>
                        <dd>{{ exif.Focal_Length }}</dd></template
                      >
                      <template v-if="exif.Aperture_Value"
                        ><dt :class="label">Aperture</dt>
                        <dd>{{ exif.Aperture_Value }}</dd></template
                      >
                      <template v-if="exif.Shutter_Speed_Value"
                        ><dt :class="label">Shutter</dt>
                        <dd>{{ exif.Shutter_Speed_Value }}</dd></template
                      >
                      <template v-if="exif.ISO_Speed_Ratings"
                        ><dt :class="label">ISO</dt>
                        <dd>{{ exif.ISO_Speed_Ratings }}</dd></template
                      >
                      <template v-if="exif.GPS_Latitude"
                        ><dt :class="label">GPS</dt>
                        <dd>{{ exif.GPS_Latitude }}, {{ exif.GPS_Longitude }}</dd></template
                      >
                      <p
                        v-if="!Object.values(exif).some(Boolean)"
                        class="col-span-2 text-muted-foreground"
                      >
                        No EXIF data for this file.
                      </p>
                    </dl>
                  </details>

                  <details :class="section">
                    <summary :class="summary">Rename</summary>
                    <div :class="[body, 'flex gap-2']">
                      <Input v-model="newName" placeholder="New Name" @keyup.enter="changeName" />
                      <Button size="sm" @click="changeName">Update</Button>
                    </div>
                  </details>

                  <details :class="section">
                    <summary :class="summary">Technical IDs</summary>
                    <dl :class="[body, 'space-y-1']">
                      <dt :class="label">Token</dt>
                      <dd class="break-all">{{ info.token }}</dd>
                      <dt :class="label">UUID</dt>
                      <dd class="break-all">{{ info.photoUuid }}</dd>
                    </dl>
                  </details>
                </div>
              </div>
            </div>
          </Transition>
        </div>
        <div :class="recommendations?.expanded && 'px-4 md:px-6'">
          <SimilarRecommendations
            v-if="canSimilar"
            :key="`${info.photoId}:${version}:${recommendationSession}`"
            ref="recommendations"
            :photo="info"
            :resume="restoredRecommendations"
            @navigate="navigate"
            @invalidated="invalidateHistory"
          />
        </div>
      </div>
      <Button
        v-if="!editing && recommendations?.expanded"
        class="absolute bottom-4 right-4 z-10 shadow-lg"
        aria-label="Back to top and close similar photos and videos"
        @click="backToTop"
      >
        <ArrowUp aria-hidden="true" /> Back to top
      </Button>
    </DialogContent>
  </Dialog>
</template>
