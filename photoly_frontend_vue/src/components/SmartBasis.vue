<script setup lang="ts">
import type { SmartSource } from '@/lib/smartOrganization'
defineProps<{ smart?: SmartSource | null }>()
</script>

<template>
  <div
    v-if="smart"
    class="space-y-1 text-xs text-muted-foreground"
    aria-label="Smart organization details"
  >
    <p v-if="smart.basis.startDate">{{ smart.basis.startDate }} – {{ smart.basis.endDate }}</p>
    <p v-if="smart.basis.places">{{ Object.keys(smart.basis.places).join(' · ') }}</p>
    <p>{{ smart.basis.method }}</p>
    <p v-if="smart.basis.timezoneSources?.some((source) => source !== 'explicit')">
      Some capture time zones are inferred from the city or application time zone.
    </p>
  </div>
</template>
