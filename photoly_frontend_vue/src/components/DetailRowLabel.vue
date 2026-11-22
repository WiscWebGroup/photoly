<script lang="ts">
// One row grammar for the viewer's expandable details: icon, label, short value, chevron.
// Put rowTrigger on the <summary>/<button> and rowBody on the expanded content.
export const rowTrigger =
  'flex h-12 w-full cursor-pointer list-none items-center gap-3 px-5 text-left text-sm outline-none transition-colors hover:bg-white/5 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring/60 md:px-6 [&::-webkit-details-marker]:hidden'
export const rowBody = 'flex flex-col gap-3 pr-5 pb-4 pl-[50px] text-sm md:pr-6 md:pl-[54px]'
</script>

<script setup lang="ts">
import type { Component } from 'vue'
import { ChevronRight } from '@lucide/vue'
defineProps<{ icon: Component; label: string; value?: string | number; open?: boolean }>()
</script>

<template>
  <component :is="icon" class="text-muted-foreground size-[18px] shrink-0" aria-hidden="true" />
  <span class="flex-1 font-medium">{{ label }}</span>
  <span v-if="value !== undefined && value !== ''" class="text-muted-foreground">{{ value }}</span>
  <ChevronRight
    :class="[
      'text-muted-foreground size-4 shrink-0 transition-transform group-open:rotate-90 motion-reduce:transition-none',
      open && 'rotate-90',
    ]"
    aria-hidden="true"
  />
</template>
