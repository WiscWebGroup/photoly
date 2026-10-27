<script setup lang="ts">
// "Search · Model loading": first search in this browser, download vs load-from-device progress.
import { computed } from 'vue'
import { Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { useBrowserAiStore } from '@/stores/browserAi'
import { loadingCopy } from '@/lib/browserAiCopy'

defineEmits<{ server: [] }>()
const store = useBrowserAiStore()
const state = store.state
const visible = computed(() => state.phase === 'loading' && state.kind === 'text')
const copy = computed(() => loadingCopy(state.source, state.loadedMB, state.totalMB, state.kind))
</script>

<template>
  <div
    v-if="visible"
    role="status"
    aria-live="polite"
    class="bg-card flex max-w-xl items-start gap-3.5 rounded-[14px] border p-4 shadow-xs"
  >
    <span
      class="bg-accent text-primary grid size-10 shrink-0 place-items-center rounded-[10px]"
      aria-hidden="true"
      ><Sparkles class="size-5"
    /></span>
    <div class="flex min-w-0 flex-1 flex-col gap-2">
      <div class="flex flex-wrap items-baseline gap-2">
        <span class="font-semibold">{{ copy.headline }}</span>
        <span class="text-muted-foreground ml-auto text-xs tabular-nums">{{ copy.amount }}</span>
      </div>
      <Progress :model-value="state.percent" aria-label="Search model loading" class="h-1.5" />
      <span class="text-muted-foreground">{{ copy.detail }}</span>
      <div class="flex flex-wrap gap-2">
        <Button
          v-if="store.choosable.text"
          type="button"
          variant="outline"
          size="sm"
          @click="$emit('server')"
          >Search on the Server This Time</Button
        >
        <router-link
          to="/home/ManagePart#ai"
          class="text-primary inline-flex h-8 items-center px-1 font-medium"
          >AI settings</router-link
        >
      </div>
    </div>
  </div>
</template>
