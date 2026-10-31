<script setup lang="ts">
import { LayoutGrid, Grid3X3, List } from '@lucide/vue'
import { viewMode } from '@/composables/useViewMode'

const options = [
  { value: 'grid', label: 'Large cards', icon: LayoutGrid },
  { value: 'compact', label: 'Small cards', icon: Grid3X3 },
  { value: 'list', label: 'List view', icon: List },
] as const
</script>

<template>
  <div class="bg-card flex rounded-lg border p-0.5 shadow-xs" role="group" aria-label="Layout">
    <button
      v-for="o in options"
      :key="o.value"
      type="button"
      :aria-label="o.label"
      :title="o.label"
      :aria-pressed="viewMode === o.value"
      class="text-muted-foreground hover:text-foreground aria-pressed:bg-muted aria-pressed:text-foreground grid h-[30px] w-8 place-items-center rounded-md transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      @click="viewMode = o.value"
    >
      <component :is="o.icon" class="size-4" aria-hidden="true" />
    </button>
  </div>
</template>
