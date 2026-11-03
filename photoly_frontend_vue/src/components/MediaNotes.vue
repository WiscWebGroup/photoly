<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { pinColor, type PinPoint } from '@/lib/notePins'
import { ApiError, call } from '@/lib/http'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
} from '@/components/ui/alert-dialog'
export interface MediaNote {
  id: number
  body: string
  x: number | null
  y: number | null
  contentVersion: number | null
  revision: number
  createdAt: string
  updatedAt: string
  needsReposition: boolean
}
const props = defineProps<{ photoId: number; canPin: boolean }>()
const emit = defineEmits<{ placing: []; positioned: [] }>()
const items = ref<MediaNote[]>([]),
  contentVersion = ref(0),
  open = ref(false),
  busy = ref(false),
  loaded = ref(false),
  error = ref('')
const editing = ref<MediaNote | null>(),
  draft = ref(''),
  x = ref<number | null>(null),
  y = ref<number | null>(null),
  anchorVersion = ref<number | null>(null)
const placing = ref(false),
  selectedId = ref(0),
  leaving = ref(false),
  deleting = ref<MediaNote | null>(null)
let resolver: ((value: boolean) => void) | undefined
let disposed = false
const dirty = computed(
  () =>
    editing.value !== undefined &&
    (draft.value !== (editing.value?.body ?? '') ||
      x.value !== (editing.value?.x ?? null) ||
      y.value !== (editing.value?.y ?? null) ||
      anchorVersion.value !== (editing.value?.contentVersion ?? null)),
)
const showPointNotes = ref(false)
const showAllBubbles = ref(false)
const generalNotes = computed(() => items.value.filter((n) => n.x === null))
const pointNotes = computed(() =>
  items.value
    .filter((n) => n.x !== null)
    .map((n, i) => ({ ...n, number: i + 1, color: pinColor(n.id) })),
)
const sections = computed(() => [
  { title: 'General Notes', notes: generalNotes.value, visible: true },
  { title: 'Pin-point Notes', notes: pointNotes.value, visible: showPointNotes.value },
])
const pins = computed<PinPoint[]>(() =>
  open.value
    ? pointNotes.value
        .filter((n) => n.y !== null && !n.needsReposition)
        .map((n) => ({ ...n, x: n.x!, y: n.y! }))
    : [],
)
watch(open, (value) => {
  if (!value) {
    showPointNotes.value = showAllBubbles.value = false
    selectedId.value = 0
  }
})
function togglePoints() {
  showAllBubbles.value = !showAllBubbles.value
  showPointNotes.value = showAllBubbles.value
  selectedId.value = 0
}
function dismissBubbles() {
  showAllBubbles.value = false
  selectedId.value = 0
}
async function load() {
  try {
    const result = await call<{ items: MediaNote[]; contentVersion: number }>({
      url: '/photo/notes',
      params: { photoId: props.photoId },
    })
    if (disposed) return
    items.value = result.items
    contentVersion.value = result.contentVersion
    loaded.value = true
  } catch {
    if (!disposed) error.value = 'Could not load notes. Please retry.'
  }
}
onMounted(load)
function message(e: unknown) {
  return e instanceof ApiError && e.msgCode === 409
    ? 'This note or photo has changed. Your draft is kept. Reload saved notes and reopen the note before saving again.'
    : 'Could not save notes. Check your connection and retry.'
}
function reset() {
  editing.value = undefined
  draft.value = ''
  x.value = y.value = null
  anchorVersion.value = null
  placing.value = false
}
function resolveLeave(value: boolean) {
  leaving.value = false
  if (value) reset()
  resolver?.(value)
  resolver = undefined
}
function requestLeave(): Promise<boolean> {
  if (busy.value) return Promise.resolve(false)
  if (!dirty.value) {
    reset()
    return Promise.resolve(true)
  }
  if (resolver) return Promise.resolve(false)
  leaving.value = true
  return new Promise((resolve) => {
    resolver = resolve
  })
}
async function edit(note: MediaNote | null, pin = false) {
  if (!(await requestLeave())) return
  open.value = true
  editing.value = note
  draft.value = note?.body ?? ''
  x.value = note?.x ?? null
  y.value = note?.y ?? null
  anchorVersion.value = note?.contentVersion ?? null
  selectedId.value = note?.id ?? 0
  if (pin) {
    placing.value = true
    emit('placing')
  }
}
function position(px: number, py: number) {
  if (!placing.value) return
  x.value = px
  y.value = py
  anchorVersion.value = contentVersion.value
  placing.value = false
  emit('positioned')
}
function move() {
  placing.value = true
  emit('placing')
}
function general() {
  x.value = y.value = null
  anchorVersion.value = null
  placing.value = false
}
async function select(id: number) {
  open.value = true
  showPointNotes.value = true
  selectedId.value = id
  await nextTick()
  document.getElementById(`media-note-${id}`)?.scrollIntoView({
    block: 'nearest',
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'instant' : 'smooth',
  })
}
async function save() {
  if (editing.value === undefined || !draft.value.trim() || placing.value) return
  busy.value = true
  error.value = ''
  try {
    const wasPoint = x.value !== null
    const result = await call<{ id: number }>({
      url: editing.value ? `/photo/notes/${editing.value.id}` : '/photo/notes',
      method: editing.value ? 'patch' : 'post',
      data: {
        photoId: props.photoId,
        body: draft.value,
        x: x.value,
        y: y.value,
        contentVersion: anchorVersion.value,
        revision: editing.value?.revision,
      },
    })
    reset()
    await load()
    if (wasPoint) await select(result.id)
  } catch (e) {
    error.value = message(e)
  } finally {
    busy.value = false
  }
}
async function remove() {
  const note = deleting.value
  if (!note) return
  busy.value = true
  error.value = ''
  try {
    await call({
      url: `/photo/notes/${note.id}`,
      method: 'delete',
      data: { photoId: props.photoId, revision: note.revision },
    })
    deleting.value = null
    if (selectedId.value === note.id) selectedId.value = 0
    await load()
  } catch (e) {
    error.value = message(e)
  } finally {
    busy.value = false
  }
}
function beforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeRouteLeave(requestLeave)
onBeforeRouteUpdate(requestLeave)
onBeforeUnmount(() => {
  disposed = true
  resolver?.(false)
  window.removeEventListener('beforeunload', beforeUnload)
})
defineExpose({
  requestLeave,
  open,
  pins,
  placing,
  position,
  select,
  selectedId,
  showAllBubbles,
  dismissBubbles,
  cancelPlacement: () => {
    placing.value = false
  },
  draftPoint: computed(() =>
    editing.value !== undefined && x.value !== null && anchorVersion.value === contentVersion.value
      ? { x: x.value, y: y.value! }
      : null,
  ),
})
</script>
<template>
  <section class="border-t px-5 py-4 md:px-6" aria-label="Notes">
    <button
      class="flex w-full items-center justify-between text-sm font-medium"
      :aria-expanded="open"
      @click="open = !open"
    >
      <span>Notes</span><span>{{ loaded ? items.length : '…' }}</span>
    </button>
    <div v-show="open" class="mt-4 flex flex-col gap-3">
      <p class="text-muted-foreground text-xs">
        Private to your account. Never included in public shares.
      </p>
      <div v-if="error" role="alert" class="text-sm">
        {{ error }} <button class="underline" @click="load">Reload saved notes</button>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button size="sm" variant="outline" :disabled="busy || !loaded" @click="edit(null)"
          >Add note</Button
        >
        <Button
          v-if="canPin"
          size="sm"
          variant="outline"
          :disabled="busy || !loaded"
          @click="edit(null, true)"
          >Pin on photo</Button
        >
      </div>
      <div v-if="editing !== undefined" class="flex flex-col gap-2 rounded-lg border p-3">
        <p v-if="placing" role="status" class="text-sm">Tap a point on the photo.</p>
        <label class="text-sm" for="media-note-draft">{{
          editing ? 'Edit note' : 'New note'
        }}</label>
        <textarea
          id="media-note-draft"
          v-model="draft"
          :disabled="busy"
          maxlength="100000"
          rows="7"
          class="bg-background min-h-32 w-full resize-y rounded-md border p-2 text-sm"
          placeholder="What makes this moment special?"
        />
        <div class="flex flex-wrap gap-2">
          <Button v-if="canPin" size="sm" variant="ghost" :disabled="busy" @click="move">{{
            x === null ? 'Add a point' : 'Reposition'
          }}</Button>
          <Button v-if="x !== null" size="sm" variant="ghost" :disabled="busy" @click="general"
            >Remove point</Button
          >
          <Button size="sm" :disabled="busy || placing || !draft.trim()" @click="save">Save</Button>
          <Button size="sm" variant="outline" :disabled="busy" @click="requestLeave">Cancel</Button>
        </div>
      </div>
      <p
        v-if="loaded && !items.length && editing === undefined"
        class="text-muted-foreground text-sm"
      >
        Record a memory or add a note to a point on this photo.
      </p>
      <Button size="sm" variant="outline" :aria-expanded="showPointNotes" @click="togglePoints">
        {{ showAllBubbles ? 'Hide Pin-point notes' : 'See all Pin-point notes' }} ({{
          pointNotes.length
        }})
      </Button>
      <template v-for="section in sections" :key="section.title">
        <section
          v-if="section.visible"
          :aria-label="section.title"
          class="flex flex-col gap-3 border-t pt-3"
        >
          <h3 class="flex justify-between text-sm font-medium">
            <span>{{ section.title }}</span
            ><span class="text-muted-foreground">{{ section.notes.length }}</span>
          </h3>
          <p v-if="!section.notes.length" class="text-muted-foreground text-sm">
            {{
              section.title === 'General Notes'
                ? 'No general notes yet.'
                : 'No pin-point notes yet.'
            }}
          </p>
          <article
            v-for="note in section.notes"
            :id="`media-note-${note.id}`"
            :key="note.id"
            class="rounded-lg border p-3"
            :class="selectedId === note.id ? 'border-primary bg-primary/5' : ''"
            @click="selectedId = note.x !== null ? note.id : 0"
          >
            <button
              v-if="note.x !== null"
              :style="{ color: pinColor(note.id) }"
              class="mb-2 text-xs underline"
              @click="select(note.id)"
            >
              {{
                note.needsReposition
                  ? 'Point needs repositioning'
                  : `Point ${pointNotes.find((p) => p.id === note.id)?.number}`
              }}
            </button>
            <p class="whitespace-pre-wrap break-words text-sm">{{ note.body }}</p>
            <p class="text-muted-foreground mt-2 text-xs">
              {{ new Date(note.updatedAt).toLocaleString() }}
            </p>
            <div class="mt-2 flex flex-wrap gap-2">
              <Button size="sm" variant="ghost" :disabled="busy" @click="edit(note)">Edit</Button>
              <Button
                v-if="canPin && note.needsReposition"
                size="sm"
                variant="ghost"
                :disabled="busy"
                @click="edit(note, true)"
                >Reposition</Button
              >
              <Button size="sm" variant="ghost" :disabled="busy" @click="deleting = note"
                >Delete</Button
              >
            </div>
          </article>
        </section>
      </template>
    </div>
    <AlertDialog :open="leaving" @update:open="!$event && resolveLeave(false)">
      <AlertDialogContent class="dark text-foreground"
        ><AlertDialogHeader
          ><AlertDialogTitle>Discard unsaved note?</AlertDialogTitle
          ><AlertDialogDescription
            >Your unsaved changes will be lost.</AlertDialogDescription
          ></AlertDialogHeader
        ><AlertDialogFooter
          ><Button variant="outline" @click="resolveLeave(false)">Keep editing</Button
          ><Button variant="destructive" @click="resolveLeave(true)"
            >Discard changes</Button
          ></AlertDialogFooter
        ></AlertDialogContent
      >
    </AlertDialog>
    <AlertDialog :open="!!deleting" @update:open="!$event && (deleting = null)">
      <AlertDialogContent class="dark text-foreground"
        ><AlertDialogHeader
          ><AlertDialogTitle>Delete this note?</AlertDialogTitle
          ><AlertDialogDescription
            >This permanently deletes the note and its point.</AlertDialogDescription
          ></AlertDialogHeader
        ><AlertDialogFooter
          ><Button variant="outline" :disabled="busy" @click="deleting = null">Cancel</Button
          ><Button variant="destructive" :disabled="busy" @click="remove"
            >Delete note</Button
          ></AlertDialogFooter
        ></AlertDialogContent
      >
    </AlertDialog>
  </section>
</template>
