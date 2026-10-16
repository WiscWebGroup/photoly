<script setup lang="ts">
import { Sparkles } from '@lucide/vue'
import type { SmartSource } from '@/lib/smartOrganization'
defineProps<{ smart?: SmartSource | null }>()
</script>

<template>
  <span
    v-if="smart"
    :class="[
      'inline-flex self-start shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px] font-medium align-middle',
      smart.source === 'trip'
        ? 'border-teal-500 text-teal-700 dark:text-teal-300'
        : 'border-violet-500 text-violet-700 dark:text-violet-300',
    ]"
    :title="smart.basis.method"
    ><Sparkles class="size-3" aria-hidden="true" />Smart<span v-if="smart.source !== 'trip'"
      >· {{ smart.source === 'camera' ? 'Camera' : 'Lens' }}</span
    ></span
  >
</template>
