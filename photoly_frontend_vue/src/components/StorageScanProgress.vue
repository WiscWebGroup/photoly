<script setup lang="ts">
import { computed } from 'vue'
import type { StorageStatus } from '@/composables/useStorageRevision'
const props = defineProps<{ status: StorageStatus }>()
const labels: Record<string, string> = {
  ready: 'Scan complete',
  idle: 'Waiting for first scan',
  enumerating: 'Discovering files',
  processing: 'Reading media information and generating previews',
  committing: 'Updating library',
  waiting: 'Waiting for files to finish writing',
  complete: 'Scan complete',
  error: 'Scan failed',
}
const active = computed(() => props.status.scanStatus === 'scanning')
const phase = computed(() => props.status.scanPhase || props.status.scanStatus)
</script>
<template>
  <div class="space-y-1 text-sm" role="status" aria-live="polite">
    <p>{{ labels[phase] || phase }}<span v-if="status.scanQueued"> · Scan queued</span></p>
    <progress
      v-if="active"
      class="h-2 w-full"
      aria-label="Media indexing progress"
      :max="Math.max(1, status.scanTotal || 0)"
      :value="phase === 'enumerating' ? undefined : status.scanCompleted || 0"
    />
    <p class="text-muted-foreground">
      {{ status.scanDiscovered || 0 }} discovered · {{ status.scanCompleted || 0 }}/{{
        status.scanTotal || 0
      }}
      media processed · {{ status.scanUnchanged || 0 }} unchanged ·
      {{ status.scanWaiting || 0 }} waiting for stability · {{ status.scanRetries || 0 }} awaiting
      retry
    </p>
    <p v-if="status.scannedAt" class="text-muted-foreground">
      Last success: {{ new Date(status.scannedAt).toLocaleString() }}
    </p>
    <p v-if="status.scanError" class="text-destructive">{{ status.scanError }}</p>
  </div>
</template>
