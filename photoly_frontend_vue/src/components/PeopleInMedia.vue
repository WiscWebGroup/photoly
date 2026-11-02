<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Users } from '@lucide/vue'
import { call } from '@/lib/http'
import { usePeopleStore, type Person, type PersonFace, type PeoplePage } from '@/stores/people'
import PeopleAvatar from './PeopleAvatar.vue'
import PeopleFaceEditor from './PeopleFaceEditor.vue'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
const props = defineProps<{ photoId?: number; galleryId?: number }>(),
  emit = defineEmits<{ navigate: [] }>()
const store = usePeopleStore(),
  router = useRouter(),
  open = ref(false),
  people = ref<Person[]>([]),
  faces = ref<PersonFace[]>([]),
  state = ref('pending'),
  error = ref(''),
  loading = ref(false),
  after = ref<number | null>(null),
  editing = ref<PersonFace | null>(null)
let sequence = 0
async function load(more = false) {
  const seq = ++sequence
  loading.value = true
  error.value = ''
  try {
    if (props.photoId) {
      const d = await call<PeoplePage<PersonFace> & { status: string }>({
        url: `/ai/people/photos/${props.photoId}`,
        params: { after: more ? after.value : 0, limit: 100 },
      })
      if (seq !== sequence) return
      faces.value = more ? [...faces.value, ...d.items] : d.items
      state.value = d.status
      after.value = d.nextAfter
    } else {
      const d = await call<PeoplePage<Person>>({
        url: '/ai/people/persons',
        params: { mode: 'all', galleryId: props.galleryId, after: more ? after.value : 0 },
      })
      if (seq !== sequence) return
      people.value = more ? [...people.value, ...d.items] : d.items
      after.value = d.nextAfter
    }
  } catch {
    if (seq === sequence) error.value = 'Could not load people. Please retry.'
  } finally {
    if (seq === sequence) loading.value = false
  }
}
watch(open, (on) => {
  if (on) void load()
})
watch(
  () => [props.photoId, props.galleryId],
  () => {
    sequence++
    open.value = false
    editing.value = null
    faces.value = []
    people.value = []
  },
)
watch(
  () => store.visible,
  (on) => {
    if (!on) {
      sequence++
      open.value = false
      editing.value = null
    }
  },
)
watch(
  () => store.revision,
  () => {
    if (open.value) void load()
  },
)
function visit(id: number) {
  open.value = false
  emit('navigate')
  router.push({ path: '/home/PeoplePart', query: { person: String(id) } })
}
</script>
<template>
  <div v-if="store.visible">
    <Button variant="outline" @click="open = true"
      ><Users class="size-4" />{{ photoId ? 'People in this photo' : 'People' }}</Button
    >
    <Dialog :open="open" @update:open="open = $event"
      ><DialogContent class="max-h-[85dvh] overflow-auto"
        ><DialogHeader
          ><DialogTitle>{{
            photoId ? 'People in this photo' : 'People in this gallery'
          }}</DialogTitle
          ><DialogDescription
            >Open a person to see their photos across your library.</DialogDescription
          ></DialogHeader
        >
        <p v-if="loading" role="status" class="text-sm text-muted-foreground">Loading people…</p>
        <div v-if="photoId" class="grid grid-cols-3 gap-4">
          <div v-for="face in faces" :key="face.id" class="flex flex-col items-center gap-2">
            <button
              class="flex w-full flex-col items-center gap-2 rounded-lg p-2 hover:bg-accent"
              @click="visit(face.personId)"
            >
              <PeopleAvatar :face-id="face.id" class="size-20" /><span
                class="w-full truncate text-sm"
                >{{ face.name || 'Unnamed person' }}</span
              ></button
            ><button class="text-primary text-xs hover:underline" @click="editing = face">
              Name or correct
            </button>
          </div>
        </div>
        <div v-else class="grid grid-cols-3 gap-4">
          <button
            v-for="person in people"
            :key="person.id"
            class="flex flex-col items-center gap-2 rounded-lg p-2 hover:bg-accent"
            @click="visit(person.id)"
          >
            <PeopleAvatar :face-id="person.coverFaceId" class="size-20" /><span
              class="w-full truncate text-sm"
              >{{ person.name || 'Unnamed person' }}</span
            ><span class="text-muted-foreground text-xs">{{ person.count }} photos in library</span>
          </button>
        </div>
        <p
          v-if="!loading && !error && !faces.length && !people.length"
          class="text-sm text-muted-foreground"
        >
          {{
            photoId
              ? state === 'ready'
                ? 'No visible people detected in this photo.'
                : state === 'failed'
                  ? 'Face processing failed. Retry from People settings.'
                  : state === 'processing'
                    ? 'Finding faces in this photo…'
                    : 'This photo is waiting for face processing.'
              : 'No visible people detected in this gallery yet.'
          }}
        </p>
        <Button v-if="after" variant="outline" @click="load(true)">Load more</Button>
        <p v-if="error" role="alert" class="text-sm text-destructive">
          {{ error }} <button @click="load()">Retry</button>
        </p>
      </DialogContent></Dialog
    ><PeopleFaceEditor :face="editing" @close="editing = null" @changed="load()" />
  </div>
</template>
