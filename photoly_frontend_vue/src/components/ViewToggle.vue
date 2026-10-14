<script setup lang="ts">
import { LayoutGrid, List } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { viewMode } from '@/composables/useViewMode'

const options = [
  { value: 'grid', label: 'Grid view', icon: LayoutGrid },
  { value: 'list', label: 'List view', icon: List },
] as const
</script>

<template>
  <div class="flex gap-1" role="group" aria-label="Layout">
    <Button
      v-for="o in options"
      :key="o.value"
      :variant="viewMode === o.value ? 'secondary' : 'ghost'"
      size="icon-sm"
      :aria-label="o.label"
      :aria-pressed="viewMode === o.value"
      @click="viewMode = o.value"
    >
      <component :is="o.icon" />
    </Button>
  </div>
</template>
