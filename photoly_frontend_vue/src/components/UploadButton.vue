<script setup lang="ts">
import { Upload, CircleAlert } from '@lucide/vue'
import { useUploadsStore } from '@/stores/uploads'
const uploads = useUploadsStore()
</script>
<template>
  <button
    type="button"
    aria-label="Upload activity"
    title="Uploads"
    class="hover:bg-accent focus-visible:ring-ring relative grid size-9 shrink-0 place-items-center rounded-lg outline-none focus-visible:ring-2"
    @click="((uploads.open = true), uploads.refresh())"
  >
    <svg
      v-if="uploads.active"
      class="text-primary absolute inset-0 size-9 -rotate-90"
      viewBox="0 0 36 36"
      aria-hidden="true"
    >
      <circle
        cx="18"
        cy="18"
        r="16"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        :stroke-dasharray="`${Math.max(8, uploads.progress)} 100`"
        pathLength="100"
      />
    </svg>
    <Upload class="size-4" aria-hidden="true" />
    <span
      v-if="uploads.active"
      class="bg-primary text-primary-foreground absolute -top-1 -right-1 min-w-4 rounded-full px-1 text-[10px]"
      >{{ uploads.active > 99 ? '99+' : uploads.active }}</span
    >
    <CircleAlert
      v-else-if="uploads.failed"
      class="text-destructive bg-background absolute -top-1 -right-1 size-3.5 rounded-full"
      aria-label="Some uploads failed"
    />
  </button>
</template>
