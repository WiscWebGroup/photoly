<script setup lang="ts">
import { ref, watch } from 'vue'
import { call } from '@/lib/http'
import { usePeopleStore, type PersonFace, type Person, type PeoplePage } from '@/stores/people'
import PeopleAvatar from './PeopleAvatar.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
const props = defineProps<{ face: PersonFace | null }>(),
  emit = defineEmits<{ close: []; changed: [] }>()
const store = usePeopleStore(),
  name = ref(''),
  query = ref(''),
  people = ref<Person[]>([]),
  after = ref<number | null>(null),
  busy = ref(false),
  error = ref('')
let sequence = 0
async function search(more = false) {
  const seq = ++sequence
  try {
    const data = await call<PeoplePage<Person>>({
      url: '/ai/people/persons',
      params: { mode: 'all', q: query.value, after: more ? after.value : 0 },
    })
    if (seq === sequence) {
      people.value = more ? [...people.value, ...data.items] : data.items
      after.value = data.nextAfter
    }
  } catch {
    if (seq === sequence) error.value = 'Could not load people.'
  }
}
watch(
  () => props.face,
  () => {
    name.value = ''
    query.value = ''
    error.value = ''
    if (props.face) void search()
  },
)
async function save(data: Record<string, unknown>) {
  if (!props.face) return
  busy.value = true
  error.value = ''
  try {
    await call({ method: 'patch', url: `/ai/people/faces/${props.face.id}`, data })
    store.revision++
    emit('changed')
    emit('close')
  } catch {
    error.value = 'Could not update this face.'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <Dialog :open="!!face" @update:open="!$event && emit('close')"
    ><DialogContent class="max-h-[85dvh] overflow-auto"
      ><DialogHeader
        ><DialogTitle>Who is this?</DialogTitle
        ><DialogDescription
          >Move this face to a person, create a new person, or remove an incorrect
          detection.</DialogDescription
        ></DialogHeader
      >
      <PeopleAvatar :face-id="face?.id ?? null" class="size-24 self-center" />
      <form class="flex gap-2" @submit.prevent="save({ personId: 0, name })">
        <Input
          v-model="name"
          maxlength="120"
          aria-label="New person name"
          placeholder="New person's name"
        /><Button :disabled="busy" type="submit">Create person</Button>
      </form>
      <form class="flex gap-2" @submit.prevent="search()">
        <Input
          v-model="query"
          aria-label="Find existing person"
          placeholder="Find an existing person"
        /><Button variant="outline" type="submit">Search</Button>
      </form>
      <div class="grid max-h-64 grid-cols-3 gap-3 overflow-auto">
        <button
          v-for="person in people"
          :key="person.id"
          :disabled="busy"
          class="flex flex-col items-center gap-1 rounded-lg p-2 hover:bg-accent"
          @click="save({ personId: person.id })"
        >
          <PeopleAvatar :face-id="person.coverFaceId" class="size-14" /><span
            class="w-full truncate text-sm"
            >{{ person.name || 'Unnamed person' }}</span
          >
        </button>
      </div>
      <Button v-if="after" variant="outline" @click="search(true)">Load more people</Button>
      <Button variant="outline" :disabled="busy" @click="save({ dismissed: true })"
        >Not a face</Button
      >
      <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    </DialogContent></Dialog
  >
</template>
