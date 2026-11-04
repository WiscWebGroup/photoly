<script setup lang="ts">
import { Globe, Map, Eclipse } from '@lucide/vue'
import { mapProjection, type MapProjection } from '@/composables/useMapProjection'
import { mapDayNight } from '@/composables/useMapDayNight'
import MapBackgroundControl from './MapBackgroundControl.vue'
defineProps<{ fallback: boolean; dayNightUnavailable?: boolean }>()
const options: { value: MapProjection; label: string; icon: typeof Globe }[] = [
  { value: 'mercator', label: 'Flat map', icon: Map },
  { value: 'globe', label: '3D globe', icon: Globe },
]
</script>
<template>
  <div class="pointer-events-auto flex flex-col gap-1">
    <div class="flex items-center gap-1">
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
      <MapBackgroundControl v-if="!fallback && mapProjection === 'globe'" />
      <button
        v-if="!fallback && mapProjection === 'globe'"
        type="button"
        aria-label="Day/night"
        :aria-pressed="mapDayNight && !dayNightUnavailable"
        :disabled="dayNightUnavailable"
        :title="
          dayNightUnavailable
            ? 'Day/night unavailable on this device'
            : 'Real-time day/night · fades out as you zoom in'
        "
        class="bg-card flex size-9 shrink-0 items-center justify-center rounded-lg border shadow-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-40"
        :class="mapDayNight && !dayNightUnavailable ? 'text-primary' : 'text-muted-foreground'"
        @click="mapDayNight = !mapDayNight"
      >
        <Eclipse class="size-4" aria-hidden="true" />
      </button>
    </div>
    <span v-if="fallback" role="status" class="bg-card rounded px-1 text-[10px]"
      >Compatibility mode · WebGL unavailable</span
    >
    <span
      v-else-if="dayNightUnavailable && mapProjection === 'globe'"
      role="status"
      class="bg-card rounded px-1 text-[10px]"
      >Day/night unavailable · map remains interactive</span
    >
  </div>
</template>
