<script setup lang="ts">
import { X } from '@lucide/vue'

// The dark toolbar that floats over the bottom of a view while items are selected.
defineProps<{ open: boolean; count: number }>()
const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <Transition name="bar">
      <div
        v-if="open"
        role="toolbar"
        aria-label="Selection"
        class="bg-foreground text-background fixed bottom-20 left-1/2 z-40 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 flex-wrap items-center gap-1 rounded-xl py-1.5 pr-1.5 pl-4 text-sm shadow-[0_10px_30px_rgba(0,0,0,.2)] md:bottom-6 md:left-[calc(50%+116px)] [&_button]:h-8 [&_button]:rounded-md [&_button]:px-2.5 [&_button]:transition-colors [&_button:disabled]:opacity-40 [&_button:not(:disabled)]:hover:bg-white/15 dark:[&_button:not(:disabled)]:hover:bg-black/10"
      >
        <span class="mr-3 font-medium tabular-nums" aria-live="polite">{{ count }} selected</span>
        <slot />
        <span aria-hidden="true" class="bg-background/25 mx-1 h-5 w-px" />
        <button
          type="button"
          aria-label="Done"
          class="grid !w-8 place-items-center !px-0"
          @click="emit('close')"
        >
          <X class="size-4" aria-hidden="true" />
        </button>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.bar-enter-active,
.bar-leave-active {
  transition:
    opacity 0.2s ease,
    transform 0.3s var(--ease-out-quart);
}
.bar-enter-from,
.bar-leave-to {
  opacity: 0;
  transform: translateY(0.75rem);
}
</style>
