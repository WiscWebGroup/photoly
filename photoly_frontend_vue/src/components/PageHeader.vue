<script setup lang="ts">
// The heading block every app view opens with: optional eyebrow, title, one line of context,
// and the view's actions on the right.
defineProps<{ title: string; eyebrow?: string; description?: string }>()
</script>

<template>
  <header class="flex flex-wrap items-end gap-x-4 gap-y-3">
    <div class="flex min-w-0 flex-col gap-1">
      <div
        v-if="eyebrow"
        class="text-muted-foreground text-xs font-semibold tracking-[0.025em] uppercase"
      >
        {{ eyebrow }}
      </div>
      <h1 class="truncate text-2xl leading-8 font-semibold tracking-[-0.01em]">
        <slot name="title">{{ title }}</slot>
      </h1>
      <div v-if="description || $slots.description" class="text-muted-foreground text-sm">
        <slot name="description">{{ description }}</slot>
      </div>
    </div>
    <div v-if="$slots.default" class="ml-auto flex flex-wrap items-center gap-2">
      <slot />
    </div>
  </header>
</template>
