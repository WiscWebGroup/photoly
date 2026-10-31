<script setup lang="ts">
import { computed } from 'vue'
import { Progress } from '@/components/ui/progress'
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
const progress = computed(() =>
  phase.value === 'enumerating' || !props.status.scanTotal
    ? null
    : Math.min(
        100,
        Math.max(0, ((props.status.scanCompleted || 0) / props.status.scanTotal) * 100),
      ),
)
const metrics = computed(() => [
  { label: 'Discovered', value: props.status.scanDiscovered || 0, color: 'bg-blue-500' },
  {
    label: 'Processed',
    value: `${props.status.scanCompleted || 0}/${props.status.scanTotal || 0}`,
    color: 'bg-primary',
  },
  {
    label: 'Retrying',
    value:
      props.status.scanAutomaticRetries ??
      Math.max(0, (props.status.scanRetries || 0) - (props.status.scanBlocked || 0)),
    color: 'bg-amber-400',
  },
  { label: 'Warning', value: props.status.scanBlocked || 0, color: 'bg-amber-400' },
])
</script>
<template>
  <div class="space-y-3 text-sm" role="status" aria-live="polite">
    <p class="text-muted-foreground text-xs">
      {{ labels[phase] || phase }}<span v-if="status.scanQueued"> · Scan queued</span>
    </p>
    <Progress v-if="active" :model-value="progress" aria-label="Media indexing progress" />
    <dl class="space-y-2">
      <div v-for="metric in metrics" :key="metric.label" class="flex items-center gap-2.5">
        <span :class="['size-2 shrink-0 rounded-full', metric.color]" aria-hidden="true" />
        <dt class="text-muted-foreground flex-1">{{ metric.label }}</dt>
        <dd class="font-medium tabular-nums">{{ metric.value }}</dd>
      </div>
    </dl>
    <p v-if="status.scannedAt" class="text-muted-foreground text-xs">
      Last success: {{ new Date(status.scannedAt).toLocaleString() }}
    </p>
    <p v-if="status.scanError" class="text-destructive text-xs">{{ status.scanError }}</p>
  </div>
</template>
