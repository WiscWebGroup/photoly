<script setup lang="ts">
import { computed } from 'vue'
import { Button } from '@/components/ui/button'
const props = defineProps<{
  total: number
  offset: number
  limit: number
  disabled: boolean
  position: 'top' | 'bottom'
}>()
const emit = defineEmits<{ page: [offset: number, limit?: number] }>()
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.limit)))
</script>
<template>
  <nav
    :aria-label="`Duplicate pagination ${position}`"
    class="flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm"
  >
    <label class="flex items-center gap-2"
      >Groups per page<select
        class="bg-background rounded-md border px-2 py-1.5"
        :value="limit"
        :disabled="disabled"
        @change="emit('page', 0, Number(($event.target as HTMLSelectElement).value))"
      >
        <option :value="10">10</option>
        <option :value="20">20</option>
        <option :value="50">50</option>
      </select></label
    >
    <div class="flex flex-wrap items-center gap-2">
      <Button
        variant="outline"
        size="sm"
        :disabled="disabled || offset === 0"
        @click="emit('page', Math.max(0, offset - limit))"
        >Previous groups</Button
      >
      <span aria-live="polite">Page {{ Math.floor(offset / limit) + 1 }} of {{ pages }}</span>
      <Button
        variant="outline"
        size="sm"
        :disabled="disabled || offset + limit >= total"
        @click="emit('page', offset + limit)"
        >Next groups</Button
      >
    </div>
  </nav>
</template>
