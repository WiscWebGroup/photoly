<script setup lang="ts">
import bg from '@/assets/JH_20211030_5406.jpg'

defineProps<{ title: string; warning?: string }>()
</script>

<template>
  <div
    class="flex min-h-dvh items-center justify-center bg-cover bg-center p-4"
    :style="{ backgroundImage: `url(${bg})` }"
  >
    <div class="bg-card/92 w-full max-w-sm space-y-4 rounded-xl border p-6 shadow-lg backdrop-blur">
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
