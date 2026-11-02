<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { File, Film, ImageOff } from '@lucide/vue'
import { photoUrl } from '@/lib/photoUrl'
import { mediaKind } from '@/lib/media'
const props = defineProps<{ photoId: number; format?: string; contentVersion?: number }>()
const failed = ref(false),
  attempt = ref(0)
let retry: ReturnType<typeof setTimeout> | undefined
const kind = computed(() => mediaKind(props.format))
const src = computed(
  () =>
    `${photoUrl(props.photoId, 'renderThumbnail')}&contentVersion=${props.contentVersion ?? 0}&retry=${attempt.value}`,
)
function failure() {
  failed.value = true
  if (kind.value === 'video' && attempt.value < 6) {
    clearTimeout(retry)
    retry = setTimeout(() => {
      attempt.value++
      failed.value = false
    }, 10000)
  }
}
watch(
  () => [props.photoId, props.contentVersion],
  () => {
    clearTimeout(retry)
    failed.value = false
    attempt.value = 0
  },
)
onUnmounted(() => clearTimeout(retry))
</script>
<template>
  <span class="relative block overflow-hidden bg-muted">
    <img
      v-if="!failed && kind !== 'other'"
      :src="src"
      alt=""
      loading="lazy"
      decoding="async"
      draggable="false"
      class="size-full object-cover"
      @error="failure"
    />
    <span
      v-else
      class="text-muted-foreground flex size-full items-center justify-center"
      :aria-label="kind === 'video' ? 'Video preview unavailable' : 'Preview unavailable'"
      role="img"
    >
      <component
        :is="kind === 'video' ? Film : kind === 'photo' ? ImageOff : File"
        class="size-7"
      />
    </span>
  </span>
</template>
