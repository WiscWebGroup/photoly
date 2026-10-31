<script setup lang="ts">
import {
  Check,
  Download,
  Eye,
  File,
  Film,
  FolderInput,
  RefreshCw,
  Play,
  SquareCheck,
  Trash2,
} from '@lucide/vue'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import { ref } from 'vue'
import { useCardSizing } from '@/composables/useCardSizing'
import { viewMode } from '@/composables/useViewMode'
import type { Photo } from '@/composables/usePhotos'
import { mediaKind } from '@/lib/media'
import { cn } from '@/lib/utils'
import { photoUrl } from '@/lib/photoUrl'

export interface MediaGridActions {
  open: (photo: Photo, event?: MouseEvent) => void
  dragPhoto: (event: DragEvent, photo: Photo) => void
  selectPhoto: (photo: Photo) => void
  downloadPhoto: (photo: Photo) => void
  convertPhoto: (photo: Photo) => void
  askMove: (photos: Photo[]) => void
  deletePhoto: (photo: Photo) => void
}
defineProps<{
  items: Photo[]
  selectMode: boolean
  timeline?: boolean
  actions: MediaGridActions
}>()
const grid = ref<HTMLElement>()
const { compactStyle } = useCardSizing(grid, 'media')
function formatDate(iso?: string) {
  const [y, m, d] = (iso ?? '').slice(0, 10).split('-').map(Number)
  return y && m && d
    ? new Date(y, m - 1, d).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
      })
    : ''
}
</script>
<template>
  <ul
    ref="grid"
    :data-layout="viewMode"
    :style="viewMode === 'compact' ? compactStyle : undefined"
    :class="
      viewMode === 'list'
        ? 'flex flex-col gap-1'
        : '-mx-4 grid grid-cols-3 gap-0.5 sm:mx-0 sm:grid-cols-4 sm:gap-1 lg:grid-cols-5 xl:grid-cols-6 2xl:grid-cols-[repeat(auto-fill,minmax(200px,1fr))]'
    "
  >
    <li
      v-for="(photo, i) in items"
      :key="photo.photoId"
      class="animate-rise relative [content-visibility:auto]"
      :style="{ animationDelay: `${Math.min(i, 12) * 25}ms` }"
    >
      <ContextMenu>
        <ContextMenuTrigger as-child>
          <button
            type="button"
            :data-photo-id="photo.photoId"
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
            @dragstart="actions.dragPhoto($event, photo)"
            @click="actions.open(photo, $event)"
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
                :class="viewMode === 'grid' ? 'size-8' : 'size-4'"
                aria-hidden="true"
              />
            </span>
            <span
              v-if="viewMode !== 'list' && mediaKind(photo.format) === 'video'"
              class="pointer-events-none absolute right-1.5 bottom-1.5 flex items-center gap-1 rounded-full bg-black/60 px-1.5 py-0.5 text-[11px] leading-4 font-medium text-white"
              ><Play class="size-2.5 fill-current" aria-hidden="true" /><span
                v-if="viewMode === 'grid'"
                >Video</span
              ></span
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
              >{{
                formatDate((timeline ? photo.timelineDate : photo.uploadDate) ?? undefined)
              }}</span
            >
            <span
              v-if="selectMode"
              aria-hidden="true"
              :class="
                cn(
                  'animate-in fade-in zoom-in-75 grid size-[22px] shrink-0 place-items-center rounded-full transition-colors',
                  viewMode === 'list'
                    ? 'order-first ml-1'
                    : viewMode === 'compact'
                      ? 'absolute top-1 left-1'
                      : 'absolute top-2 left-2',
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
          <ContextMenuItem @select="actions.selectPhoto(photo)">
            <SquareCheck /> {{ photo.checked ? 'Deselect' : 'Select' }}
          </ContextMenuItem>
          <ContextMenuItem @select="actions.open(photo)"><Eye /> View</ContextMenuItem>
          <ContextMenuItem @select="actions.downloadPhoto(photo)"
            ><Download /> Download</ContextMenuItem
          >
          <ContextMenuItem
            v-if="
              [
                'jpg',
                'jpeg',
                'png',
                'gif',
                'webp',
                'heic',
                'heif',
                'bmp',
                'tif',
                'tiff',
                'ico',
                'jfif',
              ].includes((photo.format ?? '').toLowerCase())
            "
            @select="actions.convertPhoto(photo)"
            ><RefreshCw /> Format Convert</ContextMenuItem
          >
          <ContextMenuItem @select="actions.askMove([photo])"
            ><FolderInput /> Move To</ContextMenuItem
          >
          <ContextMenuItem variant="destructive" @select="actions.deletePhoto(photo)">
            <Trash2 /> Delete
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
    </li>
  </ul>
</template>
