<script setup lang="ts">
import type Artplayer from 'artplayer'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'

type WebKitVideo = HTMLVideoElement & {
  webkitDisplayingFullscreen?: boolean
  webkitExitFullscreen?: () => void
  webkitSupportsPresentationMode?: (mode: string) => boolean
  webkitPresentationMode?: string
  webkitSetPresentationMode?: (mode: string) => void
}

const props = defineProps<{ src: string; poster: string; mediaId: string | number }>()
const emit = defineEmits<{ ready: []; error: [] }>()
const host = ref<HTMLDivElement>()
const element = ref<HTMLDivElement>()
const size = ref({ width: 0, height: 0 })
const ratio = ref(16 / 9)
let player: Artplayer | undefined
let generation = 0
let failed = false

useResizeObserver(host, ([entry]) => {
  if (entry) size.value = entry.contentRect
})
const dimensions = computed(() => {
  const width = Math.min(size.value.width, size.value.height * ratio.value)
  // Keep controls usable on portrait clips; the video itself remains object-fit: contain.
  const controlWidth = Math.max(width, Math.min(320, size.value.width))
  return { width: `${controlWidth}px`, height: `${width / ratio.value}px` }
})

function pause() {
  player?.pause()
}

// Also called by the dialog's document-level Escape handler, which runs before bubbling.
function exitFullscreen() {
  if (!player || !element.value) return false
  if (player.fullscreenWeb) {
    player.fullscreenWeb = false
    return true
  }
  const video = player.video as WebKitVideo
  if (video.webkitDisplayingFullscreen) {
    video.webkitExitFullscreen?.()
    return true
  }
  if (document.fullscreenElement && element.value.contains(document.fullscreenElement)) {
    void document.exitFullscreen().catch(() => {})
    return true
  }
  if (player.fullscreen) {
    player.fullscreen = false
    return true
  }
  return false
}

function destroy() {
  const previous = player
  if (!previous) return
  pause()
  exitFullscreen()
  if (document.pictureInPictureElement === previous.video)
    void document.exitPictureInPicture().catch(() => {})
  const video = previous.video as WebKitVideo
  if (video.webkitPresentationMode === 'picture-in-picture')
    video.webkitSetPresentationMode?.('inline')
  player = undefined
  previous.destroy()
}

function focusPlayer(event: PointerEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('button, a, input, select')) element.value?.focus({ preventScroll: true })
}

function keydown(event: KeyboardEvent) {
  if (!player || failed || event.metaKey || event.ctrlKey || event.altKey) return
  const target = event.target as HTMLElement
  if (target.closest('input, textarea, select, [contenteditable="true"]')) return
  if (event.key === 'Escape') {
    if (exitFullscreen()) {
      event.preventDefault()
      event.stopPropagation()
    }
    return
  }
  // Let links and buttons keep their own keyboard activation.
  if (event.key === ' ' && target.closest('button, a, [role="button"]')) return
  if (![' ', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key)) return
  event.preventDefault()
  event.stopPropagation()
  if (event.key === ' ') {
    if (player.video.paused) void player.play().catch(() => {})
    else pause()
  } else if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
    if (Number.isFinite(player.duration))
      player.seek = Math.max(
        0,
        Math.min(player.duration, player.currentTime + (event.key === 'ArrowRight' ? 5 : -5)),
      )
  } else {
    player.volume = Math.max(0, Math.min(1, player.volume + (event.key === 'ArrowUp' ? 0.1 : -0.1)))
  }
}

watch(
  () => [element.value, props.src, props.poster, props.mediaId] as const,
  async ([container, src, poster]) => {
    const current = ++generation
    destroy()
    failed = false
    ratio.value = 16 / 9
    if (!container || !src) return
    try {
      const { default: Player } = await import('artplayer')
      if (current !== generation) return
      // Photoly owns retries; ArtPlayer's delayed reconnect can otherwise outlive a closed viewer.
      Player.RECONNECT_TIME_MAX = 0
      const art = new Player({
        container,
        url: src,
        poster,
        lang: 'en',
        theme: '#2a9d9d',
        cssVar: { '--art-bottom-gap': '0px', '--art-padding': '8px' },
        autoplay: false,
        playsInline: true,
        hotkey: false,
        gesture: false,
        setting: true,
        playbackRate: true,
        fullscreen: true,
        pip:
          !!document.pictureInPictureEnabled ||
          typeof (HTMLVideoElement.prototype as WebKitVideo).webkitSupportsPresentationMode ===
            'function',
        moreVideoAttr: { preload: 'metadata', poster },
        settings: [
          {
            name: 'licenses',
            html: '<a href="/player-licenses.txt" target="_blank" rel="noopener">Third-party licenses</a>',
          },
        ],
      })
      player = art
      art.on('video:loadedmetadata', () => {
        if (current !== generation || failed) return
        ratio.value = art.video.videoWidth / art.video.videoHeight || 16 / 9
        emit('ready')
      })
      art.on('video:error', () => {
        if (current !== generation || failed) return
        failed = true
        pause()
        emit('error')
      })
    } catch {
      if (current !== generation) return
      destroy()
      failed = true
      emit('error')
    }
  },
  { flush: 'post' },
)
onBeforeUnmount(() => {
  generation++
  destroy()
})
defineExpose({ element, pause, exitFullscreen })
</script>

<template>
  <div ref="host" class="flex size-full min-h-0 min-w-0 items-center justify-center">
    <div
      ref="element"
      data-video-player
      tabindex="0"
      role="region"
      aria-label="Video player"
      aria-keyshortcuts="Space ArrowLeft ArrowRight ArrowUp ArrowDown"
      class="shrink-0 bg-black focus-visible:outline-2 focus-visible:outline-primary"
      :style="dimensions"
      @pointerdown="focusPlayer"
      @keydown="keydown"
    />
  </div>
</template>

<style scoped>
/* Portrait clips can be narrower than the default control bar, especially beside details. */
[data-video-player] :deep(.art-control-time) {
  padding: 0 4px !important;
  font-size: 12px !important;
}
</style>
