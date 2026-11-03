<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Users, ChevronLeft } from '@lucide/vue'
import { call } from '@/lib/http'
import { storeToRefs } from 'pinia'
import { usePeopleStore, type Person, type PersonFace, type PeoplePage } from '@/stores/people'
import type { Photo } from '@/composables/usePhotos'
import AlbumSwitch from './AlbumSwitch.vue'
import PeopleAvatar from './PeopleAvatar.vue'
import PeopleFaceEditor from './PeopleFaceEditor.vue'
import MediaList from './MediaList.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog'
const store = usePeopleStore(),
  route = useRoute(),
  router = useRouter(),
  id = computed(() => Number(route.query.person) || 0)
const { browseMode: mode, browseQuery: query } = storeToRefs(store)
const items = ref<Person[]>([]),
  person = ref<Person | null>(null),
  name = ref(''),
  after = ref<number | null>(null),
  loading = ref(false),
  error = ref(''),
  selected = ref<number[]>([]),
  merging = ref(false),
  target = ref(0),
  busy = ref(false),
  facesOpen = ref(false),
  faces = ref<PersonFace[]>([]),
  facesAfter = ref<number | null>(null),
  editing = ref<PersonFace | null>(null)
const media = ref<InstanceType<typeof MediaList>>(),
  photos = ref<Photo[]>([])
let sequence = 0,
  facesSequence = 0
const page = ref<HTMLElement>()
const viewKey = computed(() =>
  id.value ? `person:${id.value}` : `list:${mode.value}:${query.value.trim()}`,
)
let activeKey = ''
function savePosition() {
  const cached = store.view(activeKey)
  if (cached)
    store.saveView(activeKey, { ...cached, scroll: page.value?.closest('main')?.scrollTop ?? 0 })
}
async function applyCached(value: NonNullable<ReturnType<typeof store.view>>) {
  items.value = value.items
  person.value = value.person
  name.value = value.person?.name ?? ''
  photos.value = value.photos
  after.value = value.after
  await nextTick()
  media.value?.updatePhotoList(photos.value)
  page.value?.closest('main')?.scrollTo({ top: value.scroll })
}
async function load(more = false, force = false) {
  if (!store.visible) return
  const seq = ++sequence
  const key = viewKey.value
  if (activeKey !== key) savePosition()
  activeKey = key
  const cached = !more && !force ? store.view(key) : undefined
  if (cached) {
    await applyCached(cached)
    if (seq !== sequence) return
    if (Date.now() - cached.savedAt < 30_000) {
      loading.value = false
      error.value = ''
      return
    }
  } else if (!more) {
    items.value = []
    person.value = null
    photos.value = []
    after.value = null
  }
  loading.value = true
  error.value = ''
  try {
    if (id.value) {
      const personId = id.value
      const detail = await store.request(`detail:${personId}`, () =>
        call<Person>({ url: `/ai/people/persons/${personId}` }),
      )
      if (seq !== sequence) return
      person.value = detail
      name.value = detail.name
      if (detail.id !== id.value) {
        router.replace({ query: { person: String(detail.id) } })
        return
      }
      const cursor = more ? after.value : 0
      const d = await store.request(`photos:${personId}:${cursor}`, () =>
        call<PeoplePage<Photo>>({
          url: `/ai/people/persons/${id.value}/photos`,
          params: { after: cursor, limit: 100 },
        }),
      )
      if (seq !== sequence) return
      photos.value = more ? [...photos.value, ...d.items] : d.items
      after.value = d.nextAfter
      await nextTick()
      media.value?.updatePhotoList(photos.value)
    } else {
      person.value = null
      const cursor = more ? after.value : 0
      const d = await store.request(`${key}:${cursor}`, () =>
        call<PeoplePage<Person>>({
          url: '/ai/people/persons',
          params: { mode: mode.value, q: query.value.trim(), after: cursor },
        }),
      )
      if (seq !== sequence) return
      items.value = more ? [...items.value, ...d.items] : d.items
      after.value = d.nextAfter
    }
    store.saveView(key, {
      items: [...items.value],
      person: person.value,
      photos: [...photos.value],
      after: after.value,
      savedAt: Date.now(),
      scroll: cached?.scroll ?? 0,
    })
  } catch {
    if (seq === sequence) error.value = 'Could not load people. Please retry.'
  } finally {
    if (seq === sequence) loading.value = false
  }
}
async function action(fn: () => Promise<unknown>) {
  busy.value = true
  error.value = ''
  try {
    await fn()
    store.revision++
    await load(false, true)
  } catch {
    error.value = 'Could not save your changes. Please retry.'
  } finally {
    busy.value = false
  }
}
function edit(data: Record<string, unknown>) {
  return action(() => call({ method: 'patch', url: `/ai/people/persons/${id.value}`, data }))
}
function visit(personId: number) {
  router.push({ path: '/home/PeoplePart', query: { person: String(personId) } })
}
const selectedRecords = ref<Record<number, Person>>({})
const mergeOptions = computed(() =>
  selected.value.map((id) => selectedRecords.value[id]).filter((p): p is Person => !!p),
)
function startMerge() {
  target.value = selected.value[0] ?? 0
  merging.value = true
}
function refreshMedia() {
  store.revision++
  void load(false, true)
}
function facesChanged() {
  void loadFaces()
  store.revision++
  void load(false, true)
}
async function merge() {
  await action(async () => {
    await call({
      method: 'post',
      url: `/ai/people/persons/${target.value}/merge`,
      data: { ids: selected.value.filter((id) => id !== target.value) },
    })
    merging.value = false
    selected.value = []
    visit(target.value)
  })
}
async function loadFaces(more = false) {
  const seq = ++facesSequence
  try {
    const d = await call<PeoplePage<PersonFace>>({
      url: `/ai/people/persons/${id.value}/faces`,
      params: { after: more ? facesAfter.value : 0 },
    })
    if (seq !== facesSequence) return
    faces.value = more ? [...faces.value, ...d.items] : d.items
    facesAfter.value = d.nextAfter
  } catch {
    if (seq === facesSequence) error.value = 'Could not load faces.'
  }
}
watch(facesOpen, (on) => {
  if (on) void loadFaces()
})
watch(id, () => {
  sequence++
  facesSequence++
  faces.value = []
  editing.value = null
  selected.value = []
  after.value = null
  facesOpen.value = false
  photos.value = []
  void load()
})
watch(mode, () => {
  after.value = null
  void load()
})
watch(
  () => store.visible,
  (on) => {
    if (on) void load()
    else {
      sequence++
      facesSequence++
      faces.value = []
      items.value = []
      person.value = null
      photos.value = []
      editing.value = null
      facesOpen.value = false
      merging.value = false
    }
  },
)
watch(
  () => store.revision,
  () => {
    if (
      store.visible &&
      !id.value &&
      !selected.value.length &&
      items.value.length <= 50 &&
      !busy.value
    )
      void load(false, true)
  },
)
onMounted(() => {
  if (store.visible) void load()
  void store.refresh()
})
onBeforeUnmount(() => {
  savePosition()
  sequence++
  facesSequence++
})
</script>
<template>
  <div ref="page" class="flex flex-col gap-6 p-4 md:px-10 md:py-8">
    <AlbumSwitch class="md:hidden" />
    <div v-if="!store.visible" class="flex flex-col items-start gap-3 py-10">
      <Users class="size-10 text-muted-foreground" />
      <h1 class="text-2xl font-semibold">People</h1>
      <p class="text-muted-foreground">
        Enable People in your account settings when AI access is available.
      </p>
      <router-link class="text-primary hover:underline" to="/home/ManagePart#people"
        >People settings</router-link
      >
    </div>
    <template v-else>
      <template v-if="!id"
        ><header class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-2xl font-semibold">People</h1>
            <p class="text-muted-foreground text-sm">
              Familiar faces, together across your library.
            </p>
          </div>
          <router-link class="text-primary text-sm hover:underline" to="/home/ManagePart#people"
            >Settings</router-link
          >
        </header>
        <nav aria-label="People categories" class="flex gap-2">
          <Button
            v-for="tab in [
              { id: 'people', label: 'People' },
              { id: 'pending', label: 'To organize' },
              { id: 'hidden', label: 'Hidden' },
            ]"
            :key="tab.id"
            :variant="mode === tab.id ? 'secondary' : 'ghost'"
            @click="mode = tab.id"
            >{{ tab.label }}</Button
          >
        </nav>
        <form class="flex max-w-lg gap-2" @submit.prevent="load(false, true)">
          <Input
            v-model="query"
            placeholder="Search people by name"
            aria-label="Search people by name"
          /><Button type="submit" variant="outline">Search</Button>
        </form>
        <div v-if="selected.length" class="bg-muted flex items-center gap-3 rounded-lg p-3 text-sm">
          <span>{{ selected.length }} selected</span
          ><Button :disabled="selected.length < 2" @click="startMerge">Merge people</Button
          ><Button variant="ghost" @click="selected = []">Cancel</Button>
        </div>
        <div class="grid grid-cols-3 gap-x-4 gap-y-6 sm:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8">
          <div v-for="p in items" :key="p.id" class="relative flex min-w-0 flex-col items-center">
            <input
              v-model="selected"
              type="checkbox"
              :value="p.id"
              @change="selectedRecords[p.id] = p"
              :aria-label="`Select ${p.name || 'unnamed person ' + p.id}`"
              class="absolute top-0 left-0 z-10 size-4 accent-primary"
            /><button
              class="flex w-full flex-col items-center gap-2 rounded-xl p-2 transition-colors hover:bg-accent"
              @click="visit(p.id)"
            >
              <PeopleAvatar :face-id="p.coverFaceId" class="w-full max-w-28" /><span
                class="w-full truncate text-sm font-medium"
                >{{ p.name || 'Unnamed person' }}</span
              ><span class="text-muted-foreground text-xs"
                >{{ p.count }} {{ p.count === 1 ? 'photo' : 'photos' }}</span
              >
            </button>
          </div>
        </div>
        <p
          v-if="!loading && !items.length && !error"
          class="text-muted-foreground py-10 text-center"
        >
          {{
            mode === 'pending'
              ? 'Single appearances will show here for you to name or merge.'
              : mode === 'hidden'
                ? 'No hidden people.'
                : 'People will appear here as your photos are processed. You can name single appearances in To organize.'
          }}
        </p>
      </template>
      <template v-else-if="person"
        ><button
          class="text-muted-foreground flex items-center gap-1 self-start text-sm hover:text-foreground"
          @click="router.push('/home/PeoplePart')"
        >
          <ChevronLeft class="size-4" />People
        </button>
        <header class="flex flex-wrap items-center gap-5">
          <PeopleAvatar :face-id="person.coverFaceId" class="size-24" />
          <div class="flex flex-1 flex-col gap-2">
            <h1 class="text-2xl font-semibold">{{ person.name || 'Unnamed person' }}</h1>
            <p class="text-muted-foreground text-sm">
              {{ person.count }} photos across your library
            </p>
            <form class="flex max-w-md gap-2" @submit.prevent="edit({ name })">
              <Input
                v-model="name"
                maxlength="120"
                aria-label="Person name"
                placeholder="Give this person a name"
              /><Button :disabled="busy" type="submit">Save name</Button>
            </form>
          </div>
        </header>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" @click="facesOpen = true">Manage faces</Button
          ><Button variant="outline" :disabled="busy" @click="edit({ hidden: !person.hidden })">{{
            person.hidden ? 'Restore person' : 'Hide person'
          }}</Button>
        </div>
        <MediaList
          ref="media"
          :ns-or-not="false"
          :context="person.name || 'Person'"
          :total-count="person.count"
          @query-photos="refreshMedia"
        />
      </template>
      <p v-if="loading" role="status" class="text-muted-foreground text-sm">Loading people…</p>
      <p v-if="error" role="alert" class="text-destructive text-sm">
        {{ error }} <button class="underline" @click="load(false, true)">Retry</button>
      </p>
      <Button
        v-if="after"
        variant="outline"
        class="self-center"
        :disabled="loading"
        @click="load(true)"
        >Load more</Button
      >
    </template>
    <Dialog :open="merging" @update:open="merging = $event"
      ><DialogContent
        ><DialogHeader
          ><DialogTitle>Merge people</DialogTitle
          ><DialogDescription
            >Choose the person whose name and identity to keep. You can move individual faces later
            if needed.</DialogDescription
          ></DialogHeader
        ><select
          v-model="target"
          aria-label="Person to keep"
          class="bg-background rounded-md border p-2"
        >
          <option v-for="p in mergeOptions" :key="p.id" :value="p.id">
            {{ p.name || 'Unnamed person ' + p.id }}
          </option></select
        ><Button :disabled="busy" @click="merge">Merge selected people</Button></DialogContent
      ></Dialog
    >
    <Dialog :open="facesOpen" @update:open="facesOpen = $event"
      ><DialogContent class="max-h-[85dvh] overflow-auto"
        ><DialogHeader
          ><DialogTitle>Manage faces</DialogTitle
          ><DialogDescription
            >Choose a cover, correct a person, or remove an incorrect face
            detection.</DialogDescription
          ></DialogHeader
        >
        <div class="grid grid-cols-3 gap-4">
          <div v-for="f in faces" :key="f.id" class="flex flex-col items-center gap-2">
            <PeopleAvatar :face-id="f.id" class="size-20" /><button
              class="text-primary text-xs hover:underline"
              @click="editing = f"
            >
              This isn't this person</button
            ><button
              class="text-primary text-xs hover:underline"
              @click="edit({ coverFaceId: f.id })"
            >
              Use as cover
            </button>
          </div>
        </div>
        <Button v-if="facesAfter" variant="outline" @click="loadFaces(true)"
          >Load more faces</Button
        ></DialogContent
      ></Dialog
    >
    <PeopleFaceEditor :face="editing" @close="editing = null" @changed="facesChanged" />
  </div>
</template>
