<script setup lang="ts">
import type { Component } from 'vue'
import { Inbox } from '@lucide/vue'

// Shared empty / error placeholder; put an action button in the default slot.
withDefaults(defineProps<{ title: string; description?: string; icon?: Component }>(), {
  description: undefined,
  // Vue calls function defaults as factories, and icons are functional components.
  icon: () => Inbox,
})
</script>

<template>
  <div class="animate-fade flex flex-col items-center justify-center gap-3 px-6 py-16 text-center">
    <component :is="icon" class="text-muted-foreground size-10" aria-hidden="true" />
    <h2 class="text-lg font-semibold">{{ title }}</h2>
    <p v-if="description" class="text-muted-foreground max-w-sm text-sm">{{ description }}</p>
    <slot />
  </div>
</template>
