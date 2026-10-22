<script setup lang="ts">
import { Sparkles } from '@lucide/vue'
import type { SmartSource } from '@/lib/smartOrganization'
defineProps<{ smart?: SmartSource | null }>()
</script>

<template>
  <span
    v-if="smart"
    class="bg-accent text-accent-foreground inline-flex shrink-0 items-center gap-1 self-start rounded-full px-2 align-middle text-[11px] leading-5 font-medium"
    :title="smart.basis.method"
    ><Sparkles class="text-primary size-3" aria-hidden="true" />{{
      smart.source === 'trip'
        ? 'Smart trip'
        : `Smart · ${smart.source === 'camera' ? 'Camera' : 'Lens'}`
    }}</span
  >
</template>
