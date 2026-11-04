<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { Copy, ShieldCheck, FolderOpen } from '@lucide/vue'
import { call } from '@/lib/http'
import { photoUrl } from '@/lib/photoUrl'
import {
  duplicateBytes,
  duplicateError,
  type DuplicateGroup,
  type DuplicateMember,
  type DuplicateMembers,
} from '@/lib/duplicates'
import { Button } from '@/components/ui/button'
const props = defineProps<{ group: DuplicateGroup; disabled: boolean }>()
const emit = defineEmits<{
  changed: []
  select: [items: DuplicateMember[]]
  preview: [photo: DuplicateMember]
}>()
const members = ref<DuplicateMember[]>([]),
  offset = ref(0),
  loading = ref(false),
  error = ref('')
const selected = ref(new Map<number, DuplicateMember>())
const limit = 24
let sequence = 0
async function load() {
  const seq = ++sequence
  loading.value = true
  error.value = ''
  try {
    const data = await call<DuplicateMembers>({
      url: '/photo/duplicates/members',
      params: { key: props.group.key, version: props.group.version, offset: offset.value, limit },
    })
    if (seq === sequence) members.value = data.items
  } catch (e) {
    if (seq === sequence) error.value = duplicateError(e)
  } finally {
    if (seq === sequence) loading.value = false
  }
}
function choose(member: DuplicateMember) {
  if (member.kept) return
  if (selected.value.has(member.photoId)) selected.value.delete(member.photoId)
  else {
    if (selected.value.size >= 1000) {
      error.value = 'Select up to 1,000 copies per group at a time.'
      return
    }
    selected.value.set(member.photoId, member)
  }
  emit('select', [...selected.value.values()])
}
async function selectOthers() {
  loading.value = true
  error.value = ''
  try {
    const next = new Map(selected.value)
    if (props.group.count > 1000) {
      for (const member of members.value) if (!member.kept) next.set(member.photoId, member)
    } else {
      // Explicit selection pages through this group; ordinary browsing stays paged.
      for (let start = 0; start < props.group.count; start += 100) {
        const data = await call<DuplicateMembers>({
          url: '/photo/duplicates/members',
          params: { key: props.group.key, version: props.group.version, offset: start, limit: 100 },
        })
        for (const member of data.items) if (!member.kept) next.set(member.photoId, member)
      }
    }
    if (next.size > 1000) {
      error.value = 'Select up to 1,000 copies per group at a time.'
      return
    }
    selected.value = next
    emit('select', [...next.values()])
  } catch (e) {
    error.value = duplicateError(e)
  } finally {
    loading.value = false
  }
}
async function change(action: 'keep' | 'ignore', value: boolean, photoId?: number) {
  loading.value = true
  error.value = ''
  try {
    await call({
      method: 'post',
      url: `/photo/duplicates/${action}`,
      data: { key: props.group.key, version: props.group.version, value, photoId },
    })
    emit('changed')
  } catch (e) {
    error.value = duplicateError(e)
  } finally {
    loading.value = false
  }
}
watch(offset, load)
onMounted(load)
</script>
<template>
  <section class="bg-card overflow-hidden rounded-xl border" aria-label="Duplicate group">
    <div class="flex flex-wrap items-center gap-3 border-b p-4">
      <div class="bg-muted grid size-10 place-items-center rounded-lg"><Copy class="size-5" /></div>
      <div class="mr-auto">
        <h2 class="font-semibold">{{ group.count }} identical photos</h2>
        <p class="text-muted-foreground text-xs">{{ group.keptCount }} marked Keep</p>
      </div>
      <Button
        variant="outline"
        size="sm"
        :disabled="disabled || loading || group.keptCount === 0"
        @click="selectOthers"
        >{{
          group.count > 1000 ? 'Select other copies on this page' : 'Select other copies'
        }}</Button
      >
      <Button
        variant="ghost"
        size="sm"
        :disabled="disabled || loading"
        @click="change('ignore', !group.ignored)"
        >{{ group.ignored ? 'Restore group' : 'Ignore group' }}</Button
      >
    </div>
    <div
      v-if="error"
      role="alert"
      class="flex flex-wrap items-center gap-3 p-4 text-sm text-destructive"
    >
      {{ error
      }}<Button variant="outline" size="sm" :disabled="disabled || loading" @click="load"
        >Retry</Button
      >
    </div>
    <p v-if="loading" role="status" class="text-muted-foreground px-4 py-2 text-sm">Loading…</p>
    <div class="grid grid-cols-1 gap-4 p-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
      <article
        v-for="member in members"
        :key="member.photoId"
        class="min-w-0 overflow-hidden rounded-lg border"
        :class="member.kept ? 'border-primary/50' : ''"
      >
        <button
          class="bg-muted block aspect-[4/3] w-full overflow-hidden focus-visible:ring-2 focus-visible:ring-ring"
          :aria-label="`Preview ${member.fileName}`"
          @click="emit('preview', member)"
        >
          <img
            :src="photoUrl(member.photoId, 'renderThumbnail')"
            :alt="member.fileName"
            class="size-full object-contain"
            loading="lazy"
          />
        </button>
        <div class="space-y-2 p-3">
          <p class="truncate text-sm font-medium" :title="member.fileName">{{ member.fileName }}</p>
          <p class="text-muted-foreground truncate text-xs">
            {{ member.sourceName || member.sourceType }} · {{ duplicateBytes(member.bytes) }}
          </p>
          <router-link
            :to="{ path: '/home/HomePart', query: { folder: String(member.nsId) } }"
            class="text-muted-foreground flex min-w-0 items-center gap-1 text-xs hover:underline"
            :title="member.folder"
            ><FolderOpen class="size-3 shrink-0" /><span class="truncate">{{
              member.folder
            }}</span></router-link
          >
          <div class="flex flex-wrap items-center justify-between gap-2 pt-1">
            <label class="flex items-center gap-2 text-sm"
              ><input
                type="checkbox"
                :aria-label="`Select ${member.fileName}`"
                :checked="selected.has(member.photoId)"
                :disabled="disabled || loading || member.kept"
                @change="choose(member)"
              />Select</label
            >
            <Button
              :variant="member.kept ? 'secondary' : 'outline'"
              size="sm"
              :disabled="disabled || loading"
              :aria-label="`${member.kept ? 'Unmark' : 'Keep'} ${member.fileName}`"
              @click="change('keep', !member.kept, member.photoId)"
              ><ShieldCheck class="size-3.5" />{{ member.kept ? 'Kept' : 'Keep' }}</Button
            >
          </div>
        </div>
      </article>
    </div>
    <div
      v-if="group.count > limit"
      class="flex items-center justify-end gap-3 border-t p-3 text-sm"
    >
      <Button
        variant="outline"
        size="sm"
        :disabled="disabled || loading || offset === 0"
        @click="offset -= limit"
        >Previous photos</Button
      >
      <span>{{ offset + 1 }}–{{ Math.min(offset + limit, group.count) }} / {{ group.count }}</span>
      <Button
        variant="outline"
        size="sm"
        :disabled="disabled || loading || offset + limit >= group.count"
        @click="offset += limit"
        >Next photos</Button
      >
    </div>
  </section>
</template>
