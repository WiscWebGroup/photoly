<script setup lang="ts">
import { ref, watch } from 'vue'
import { call } from '@/lib/http'
import { mediaCategories, type CategoryOverride } from '@/lib/mediaCategories'
const props = defineProps<{ photoId: number }>()
const emit = defineEmits<{ changed: [] }>()
const items = ref<CategoryOverride[]>([]),
  error = ref(''),
  busy = ref(false)
let sequence = 0
async function load() {
  const seq = ++sequence
  try {
    const data = await call<CategoryOverride[]>({
      url: '/photo/category',
      params: { photoId: props.photoId },
    })
    if (seq === sequence) items.value = data
  } catch {
    if (seq === sequence) error.value = 'Could not load classifications.'
  }
}
watch(
  () => props.photoId,
  () => {
    items.value = []
    error.value = ''
    void load()
  },
  { immediate: true },
)
async function change(item: CategoryOverride, event: Event) {
  busy.value = true
  error.value = ''
  const value = (event.target as HTMLSelectElement).value
  try {
    await call({
      url: '/photo/category',
      method: 'put',
      data: {
        photoId: props.photoId,
        category: item.category,
        included: value === 'auto' ? null : value === 'yes',
      },
    })
    await load()
    emit('changed')
  } catch {
    error.value = 'Could not save classification. Please retry.'
    await load()
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <details class="border-t px-5 py-4 md:px-6">
    <summary class="cursor-pointer text-sm font-medium">Category</summary>
    <p v-if="error" role="alert" class="mt-3 text-sm">{{ error }}</p>
    <label
      v-for="item in items"
      :key="item.category"
      class="mt-3 flex items-center justify-between gap-3 text-sm"
    >
      {{ mediaCategories.find((c) => c.id === item.category)?.name }}
      <select
        :value="item.override === null ? 'auto' : item.override ? 'yes' : 'no'"
        :disabled="busy"
        class="bg-background rounded border p-1.5"
        @change="change(item, $event)"
      >
        <option value="auto">Auto ({{ item.automatic ? 'included' : 'not included' }})</option>
        <option value="yes">Include</option>
        <option value="no">Exclude</option>
      </select>
    </label>
  </details>
</template>
