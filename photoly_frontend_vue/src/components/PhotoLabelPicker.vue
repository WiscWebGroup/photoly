<script setup lang="ts">
import { computed, ref } from 'vue'
import { Check, Hash, Plus, Search } from '@lucide/vue'
import SmartBadge from '@/components/SmartBadge.vue'
import { Button } from '@/components/ui/button'
import type { SmartSource } from '@/lib/smartOrganization'

export interface PickerOption {
  id: number
  name: string
  smart?: SmartSource | null
}

// One searchable list for both tags and galleries: clicking a row attaches or detaches it,
// Enter creates whichever kind the picker was opened for.
const props = defineProps<{
  mode: 'tag' | 'gallery'
  tags: PickerOption[]
  galleries: PickerOption[]
  attachedTags: number[]
  attachedGalleries: number[]
}>()
const emit = defineEmits<{
  toggleTag: [id: number, attached: boolean]
  toggleGallery: [id: number, attached: boolean]
  create: [kind: 'tag' | 'gallery', name: string]
  done: []
}>()

const query = ref('')
const q = computed(() => query.value.trim())
const match = (o: PickerOption) => o.name.toLowerCase().includes(q.value.toLowerCase())
const tagRows = computed(() => props.tags.filter(match))
const galleryRows = computed(() => props.galleries.filter(match))
const exists = (list: PickerOption[]) =>
  list.some((o) => o.name.toLowerCase() === q.value.toLowerCase())
const canCreateTag = computed(() => !!q.value && !exists(props.tags))
const canCreateGallery = computed(() => !!q.value && !exists(props.galleries))

function submit() {
  if (!q.value) return
  const list = props.mode === 'tag' ? props.tags : props.galleries
  const hit = list.find((o) => o.name.toLowerCase() === q.value.toLowerCase())
  if (hit) {
    if (props.mode === 'tag') emit('toggleTag', hit.id, props.attachedTags.includes(hit.id))
    else emit('toggleGallery', hit.id, props.attachedGalleries.includes(hit.id))
  } else emit('create', props.mode, q.value)
  query.value = ''
}

const row =
  'flex h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm transition-colors outline-none hover:bg-accent focus-visible:bg-accent'
const group =
  'text-muted-foreground px-2 pt-2 pb-1 text-xs font-semibold tracking-[0.025em] uppercase'
</script>

<template>
  <div class="flex max-h-[min(28rem,70dvh)] flex-col">
    <div class="flex flex-col gap-2 p-3 pb-2">
      <p class="text-sm font-semibold">Tags and Galleries</p>
      <form class="relative" @submit.prevent="submit">
        <Search
          class="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2"
          aria-hidden="true"
        />
        <input
          v-model="query"
          autofocus
          :aria-label="mode === 'tag' ? 'New tag' : 'New gallery'"
          :placeholder="mode === 'tag' ? 'Find or create a tag' : 'Find or create a gallery'"
          class="border-input focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full rounded-lg border bg-white/5 pr-2 pl-8 text-sm outline-none focus-visible:ring-3"
        />
      </form>
    </div>
    <div class="min-h-0 flex-1 overflow-y-auto px-1 pb-1">
      <p :class="group">Tags</p>
      <button
        v-if="canCreateTag"
        type="button"
        :class="[row, mode === 'tag' && 'bg-accent/60']"
        @click="(emit('create', 'tag', q), (query = ''))"
      >
        <Plus class="text-primary size-4" aria-hidden="true" />Create tag “{{ q }}”
      </button>
      <button
        v-for="o in tagRows"
        :key="o.id"
        type="button"
        :class="row"
        :aria-pressed="attachedTags.includes(o.id)"
        @click="emit('toggleTag', o.id, attachedTags.includes(o.id))"
      >
        <Hash class="text-muted-foreground size-3.5 shrink-0" aria-hidden="true" />
        <span class="min-w-0 truncate">{{ o.name }}</span>
        <SmartBadge :smart="o.smart" />
        <Check
          v-if="attachedTags.includes(o.id)"
          class="text-primary ml-auto size-4 shrink-0"
          aria-hidden="true"
        />
      </button>
      <p v-if="!tagRows.length && !canCreateTag" class="text-muted-foreground px-2 py-1.5 text-sm">
        No tags yet
      </p>

      <div class="my-1 border-t" />
      <p :class="group">Galleries</p>
      <button
        v-if="canCreateGallery"
        type="button"
        :class="[row, mode === 'gallery' && 'bg-accent/60']"
        @click="(emit('create', 'gallery', q), (query = ''))"
      >
        <Plus class="text-primary size-4" aria-hidden="true" />Create gallery “{{ q }}”
      </button>
      <button
        v-for="o in galleryRows"
        :key="o.id"
        type="button"
        :class="row"
        :aria-pressed="attachedGalleries.includes(o.id)"
        @click="emit('toggleGallery', o.id, attachedGalleries.includes(o.id))"
      >
        <span class="bg-primary size-2 shrink-0 rounded-full" aria-hidden="true" />
        <span class="min-w-0 truncate">{{ o.name }}</span>
        <SmartBadge :smart="o.smart" />
        <Check
          v-if="attachedGalleries.includes(o.id)"
          class="text-primary ml-auto size-4 shrink-0"
          aria-hidden="true"
        />
      </button>
      <p
        v-if="!galleryRows.length && !canCreateGallery"
        class="text-muted-foreground px-2 py-1.5 text-sm"
      >
        No galleries yet
      </p>
    </div>
    <div class="flex items-center gap-2 border-t px-3 py-2.5">
      <span class="text-muted-foreground text-xs">Enter to create · Esc to close</span>
      <Button size="sm" class="ml-auto" @click="emit('done')">Done</Button>
    </div>
  </div>
</template>
