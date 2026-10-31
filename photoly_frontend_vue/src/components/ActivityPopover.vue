<script setup lang="ts">
import type { Component } from 'vue'
import { X } from '@lucide/vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

defineProps<{ title: string; description?: string; icon?: Component; panel?: string }>()
const open = defineModel<boolean>('open', { default: false })
</script>
<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child><slot name="trigger" /></PopoverTrigger>
    <PopoverContent
      align="end"
      :side-offset="10"
      :collision-padding="12"
      :data-activity-panel="panel || title"
      :data-storage-panel="panel === 'storage' ? '' : undefined"
      class="max-h-[min(75dvh,var(--reka-popover-content-available-height))] w-[min(440px,calc(100vw-24px))] overflow-y-auto rounded-xl p-0 shadow-lg"
    >
      <header class="bg-popover sticky top-0 z-10 flex items-center gap-3 border-b px-4 py-3">
        <span
          v-if="icon"
          class="bg-accent text-primary grid size-8 shrink-0 place-items-center rounded-lg"
        >
          <component :is="icon" class="size-4" aria-hidden="true" />
        </span>
        <div class="min-w-0 flex-1">
          <h2 class="text-sm font-semibold">{{ title }}</h2>
          <p v-if="description" class="text-muted-foreground mt-0.5 text-xs">{{ description }}</p>
        </div>
        <slot name="actions" />
        <button
          type="button"
          aria-label="Close"
          title="Close"
          class="text-muted-foreground hover:bg-accent hover:text-foreground grid size-8 shrink-0 place-items-center rounded-lg outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @click="open = false"
        >
          <X class="size-4" aria-hidden="true" />
        </button>
      </header>
      <div class="space-y-4 p-4"><slot /></div>
    </PopoverContent>
  </Popover>
</template>
