<script setup lang="ts">
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { DOCS_URL } from '@/lib/docs'

// A small "!" that shows help on hover/focus. `doc` is a docs-site path for extra reading.
defineProps<{ label: string; doc?: string }>()
</script>

<template>
  <TooltipProvider :delay-duration="150">
    <Tooltip>
      <TooltipTrigger as-child>
        <button
          type="button"
          :aria-label="`About ${label}`"
          class="text-muted-foreground hover:text-foreground focus-visible:ring-ring inline-grid size-4 shrink-0 place-items-center rounded-full border border-current text-[10px] leading-none font-semibold focus-visible:ring-2 focus-visible:outline-none"
        >
          !
        </button>
      </TooltipTrigger>
      <TooltipContent>
        <slot />
        <a
          v-if="doc"
          :href="`${DOCS_URL}${doc}`"
          target="_blank"
          rel="noreferrer"
          class="mt-1 block underline"
          >Learn more</a
        >
      </TooltipContent>
    </Tooltip>
  </TooltipProvider>
</template>
