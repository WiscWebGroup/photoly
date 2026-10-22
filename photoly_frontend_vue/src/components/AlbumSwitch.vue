<script setup lang="ts">
import { Plus } from '@lucide/vue'

// Phones reach galleries and tags through one "Albums" tab: its heading, an add button for
// whichever kind is showing, and the switch between the two.
defineProps<{ addLabel?: string }>()
const emit = defineEmits<{ add: [] }>()
const items = [
  { part: 'GalleryPart', label: 'Galleries' },
  { part: 'TagPart', label: 'Tags' },
]
</script>

<template>
  <div class="flex flex-col gap-3">
    <div v-if="addLabel" class="flex items-center gap-2">
      <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Albums</h1>
      <button
        type="button"
        :aria-label="addLabel"
        class="text-primary hover:bg-accent ml-auto grid size-9 place-items-center rounded-lg transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        @click="emit('add')"
      >
        <Plus class="size-5" aria-hidden="true" />
      </button>
    </div>
    <nav aria-label="Album type" class="bg-muted grid grid-cols-2 rounded-[10px] p-[3px]">
      <router-link
        v-for="item in items"
        :key="item.part"
        :to="`/home/${item.part}`"
        class="text-muted-foreground flex h-9 items-center justify-center rounded-lg text-sm font-medium transition-[background-color,color,box-shadow]"
        active-class="!bg-card !text-foreground shadow-[0_1px_2px_rgba(0,0,0,.08)]"
        >{{ item.label }}</router-link
      >
    </nav>
  </div>
</template>
