<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { SearchX } from '@lucide/vue'
import AppShell from '@/components/AppShell.vue'
import EmptyState from '@/components/EmptyState.vue'

const props = defineProps<{ part: string }>()

const parts = {
  HomePart: defineAsyncComponent(() => import('@/components/HomePart.vue')),
  GalleryPart: defineAsyncComponent(() => import('@/components/GalleryPart.vue')),
  TagPart: defineAsyncComponent(() => import('@/components/TagPart.vue')),
  SearchPart: defineAsyncComponent(() => import('@/components/SearchPart.vue')),
  ManagePart: defineAsyncComponent(() => import('@/components/ManagePart.vue')),
  MapPart: defineAsyncComponent(() => import('@/components/MapPart.vue')),
}
const current = computed(() => parts[props.part as keyof typeof parts])
</script>

<template>
  <AppShell>
    <!-- Each part mounts fresh, so the entry animation replays on every switch. -->
    <!-- Each part lays out its own page: padding, header and any side panes. -->
    <component :is="current" v-if="current" class="animate-rise" />
    <EmptyState
      v-else
      :icon="SearchX"
      title="Page not found"
      description="That section doesn't exist."
    />
  </AppShell>
</template>
