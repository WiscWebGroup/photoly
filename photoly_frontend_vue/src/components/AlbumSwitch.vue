<script setup lang="ts">
// Phones reach galleries and tags through one "Albums" tab; this switches between the two.
const items = [
  { part: 'GalleryPart', label: 'Galleries' },
  { part: 'TagPart', label: 'Tags' },
]
</script>

<template>
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
</template>
