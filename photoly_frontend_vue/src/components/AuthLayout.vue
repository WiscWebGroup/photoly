<script setup lang="ts">
import { defineAsyncComponent } from 'vue'

const ShapeWaves = defineAsyncComponent(() => import('@/components/ShapeWaves/ShapeWaves.vue'))

defineProps<{ title: string; warning?: string }>()
</script>

<template>
  <div class="relative flex min-h-dvh items-center justify-center overflow-hidden bg-[#031417] p-4">
    <div class="absolute inset-0">
      <ShapeWaves
        shapes="mixed"
        :cell-size="18"
        :dot-size="0.8"
        color="#39bab4"
        hover-color="#83e1db"
        background-color="#031417"
        :speed="0.8"
        :scale="1.2"
        :contrast="1.6"
        :brightness="1"
        :fade="0.35"
        :glow="0.8"
        :splash-radius="60"
        :introDuration="2.2"
        @error="(e: Error) => console.warn('ShapeWaves unavailable:', e.message)"
      />
    </div>
    <div class="bg-card/50 relative w-full max-w-sm space-y-4 rounded-xl border p-6 shadow-lg backdrop-blur">
      <h1 class="text-xl font-semibold">{{ title }}</h1>
      <p
        v-if="warning"
        role="alert"
        class="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm"
      >
        {{ warning }}
      </p>
      <slot />
    </div>
  </div>
</template>
