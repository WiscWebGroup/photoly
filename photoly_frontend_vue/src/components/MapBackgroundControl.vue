<script setup lang="ts">
import { Sparkles } from '@lucide/vue'
import { mapBackground } from '@/composables/useMapBackground'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
const options = [
  { value: 'stars', label: 'Starfield' },
  { value: 'gradient', label: 'Dark gradient' },
  { value: 'simple', label: 'Simple' },
]
</script>
<template>
  <DropdownMenu>
    <DropdownMenuTrigger as-child>
      <button
        type="button"
        aria-label="Globe background"
        title="Globe background"
        class="bg-card flex size-9 shrink-0 items-center justify-center rounded-lg border shadow-sm hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring"
      >
        <Sparkles class="size-4" aria-hidden="true" />
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="end" class="z-[1100]">
      <DropdownMenuRadioGroup v-model="mapBackground" aria-label="Globe background">
        <DropdownMenuRadioItem v-for="option in options" :key="option.value" :value="option.value">
          {{ option.label }}
        </DropdownMenuRadioItem>
      </DropdownMenuRadioGroup>
    </DropdownMenuContent>
  </DropdownMenu>
</template>
