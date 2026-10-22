<script setup lang="ts">
import { defineAsyncComponent, ref } from 'vue'

// Loaded lazily so the WebGPU runtime never blocks first paint.
const ShapeWaves = defineAsyncComponent(() => import('@/components/ShapeWaves/ShapeWaves.vue'))

/** `hero`: brightest at the top right, behind the landing hero. `ring`: a halo around a centered card. */
defineProps<{ variant: 'hero' | 'ring' }>()

// Without WebGPU the animated field can't run; a static dot grid stands in for it.
const failed = ref(false)
function onError(e: Error) {
  failed.value = true
  console.warn('ShapeWaves unavailable:', e.message)
}
</script>

<template>
  <div
    aria-hidden="true"
    class="night-backdrop pointer-events-none absolute inset-0"
    :class="variant"
  >
    <div v-if="failed" class="night-dots absolute inset-0" />
    <ShapeWaves
      v-else
      shapes="mixed"
      :cell-size="20"
      :dot-size="0.6"
      color="#39bab4"
      hover-color="#83e1db"
      background-color="#031417"
      :speed="0.8"
      :scale="1.2"
      :contrast="1.2"
      :brightness="0.3"
      :fade="0.35"
      :glow="0.3"
      :splash-radius="60"
      :intro-duration="2.2"
      @error="onError"
    />
  </div>
</template>

<style scoped>
.night-backdrop {
  /* Kept quiet so text on top stays easy to read. */
  opacity: 0.45;
  animation: var(--animate-fade);
  animation-duration: 1.2s;
}
.hero {
  /* Only the first screen, so the sections below sit on plain night. */
  bottom: auto;
  height: min(100dvh, 1000px);
  mask-image: radial-gradient(ellipse at 70% 30%, #000 10%, transparent 70%);
}
.ring {
  mask-image: radial-gradient(ellipse at 50% 45%, transparent 18%, #000 45%, transparent 80%);
}
.night-dots {
  background-image: radial-gradient(circle, #39bab4 1.2px, transparent 1.6px);
  background-size: 22px 22px;
  opacity: 0.6;
}
</style>
