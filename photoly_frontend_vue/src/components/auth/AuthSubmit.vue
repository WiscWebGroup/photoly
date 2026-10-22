<script setup lang="ts">
import { MorphIcon } from 'morphicons/vue'
import { ArrowRight, LoaderCircle } from 'lucide'

defineProps<{ loading?: boolean }>()
</script>

<template>
  <button
    type="submit"
    :disabled="loading"
    :aria-busy="loading"
    class="group flex h-13 w-full items-center justify-center gap-2 rounded-xl bg-teal-400 text-base font-semibold text-teal-950 shadow-[0_0_32px_-4px] shadow-teal-400/60 transition-[background-color,box-shadow,transform] duration-200 outline-none hover:bg-teal-300 hover:shadow-teal-300/70 focus-visible:ring-3 focus-visible:ring-teal-200/60 active:scale-[0.98] disabled:cursor-progress sm:h-11 sm:rounded-[10px]"
  >
    <slot />
    <MorphIcon
      :icon="loading ? LoaderCircle : ArrowRight"
      :size="18"
      :stroke-width="2.25"
      class="transition-transform duration-300 ease-(--ease-out-quart)"
      :class="loading ? 'animate-spin' : 'group-hover:translate-x-0.5'"
      reduced-motion="user"
    />
  </button>
</template>
