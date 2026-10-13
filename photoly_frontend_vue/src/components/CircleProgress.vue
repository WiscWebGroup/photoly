<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ value: number; label: string; ok?: boolean }>()

const R = 40
const C = 2 * Math.PI * R
const offset = computed(() => C * (1 - props.value / 100))
</script>

<template>
  <figure class="flex flex-col items-center gap-2">
    <svg
      viewBox="0 0 100 100"
      class="size-24"
      role="img"
      :aria-label="`${label}: ${Math.round(value)}%`"
    >
      <circle cx="50" cy="50" :r="R" fill="none" stroke-width="8" class="stroke-muted" />
      <circle
        cx="50"
        cy="50"
        :r="R"
        fill="none"
        stroke-width="8"
        stroke-linecap="round"
        transform="rotate(-90 50 50)"
        :stroke-dasharray="C"
        :stroke-dashoffset="offset"
        :class="ok === false ? 'stroke-destructive' : 'stroke-primary'"
      />
      <text x="50" y="55" text-anchor="middle" class="fill-foreground text-[18px] font-medium">
        {{ Math.round(value) }}%
      </text>
    </svg>
    <figcaption class="text-sm font-medium">{{ label }}</figcaption>
  </figure>
</template>
