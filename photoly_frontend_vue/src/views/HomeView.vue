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
// These two lay themselves out edge to edge.
const fullBleed = computed(() => ['ManagePart', 'MapPart'].includes(props.part))
</script>

<template>
  <AppShell>
    <component :is="current" v-if="current" :class="fullBleed ? '' : 'p-4 md:p-6'" />
    <EmptyState v-else :icon="SearchX" title="Page not found" description="That section doesn't exist." />
  </AppShell>
</template>
