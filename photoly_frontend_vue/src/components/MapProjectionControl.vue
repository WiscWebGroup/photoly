<script setup lang="ts">
import { Globe, Map } from '@lucide/vue'
import { mapProjection, type MapProjection } from '@/composables/useMapProjection'
defineProps<{ fallback: boolean }>()
const options: { value: MapProjection; label: string; icon: typeof Globe }[] = [
  { value: 'mercator', label: 'Flat map', icon: Map },
  { value: 'globe', label: '3D globe', icon: Globe },
]
</script>
<template>
  <div class="pointer-events-auto flex flex-col gap-1">
    <div
      role="group"
      aria-label="Map projection"
      class="bg-card flex h-9 shrink-0 gap-0.5 rounded-lg border p-0.5 shadow-sm"
    >
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        :aria-label="option.label"
        :title="
          fallback && option.value === 'globe'
            ? '3D globe needs WebGL. Using compatible flat map.'
            : option.label
        "
        :aria-pressed="(fallback ? 'mercator' : mapProjection) === option.value"
        :disabled="fallback && option.value === 'globe'"
        class="flex items-center gap-1 rounded-md px-2 text-xs font-medium hover:bg-accent disabled:cursor-not-allowed disabled:opacity-40"
        :class="
          (fallback ? 'mercator' : mapProjection) === option.value
            ? 'bg-muted text-foreground'
            : 'text-muted-foreground'
        "
        @click="mapProjection = option.value"
      >
        <component :is="option.icon" class="size-4" aria-hidden="true" />
        <span>{{ option.value === 'globe' ? '3D' : '2D' }}</span>
      </button>
    </div>
    <span v-if="fallback" role="status" class="bg-card rounded px-1 text-[10px]"
      >Compatibility mode · WebGL unavailable</span
    >
  </div>
</template>
