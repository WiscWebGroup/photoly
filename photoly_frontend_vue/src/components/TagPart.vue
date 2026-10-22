<script setup lang="ts">
import SmartBadge from '@/components/SmartBadge.vue'
import type { SmartSource } from '@/lib/smartOrganization'
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Check, Hash, Pencil, Plus, Search, Sparkles, Tag as TagIcon, Trash2, X } from '@lucide/vue'
import AlbumSwitch from '@/components/AlbumSwitch.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import MediaList from '@/components/MediaList.vue'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from '@/components/ui/context-menu'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import type { Photo } from '@/composables/usePhotos'
import { call } from '@/lib/http'
import { cn } from '@/lib/utils'

interface Tag {
  smart?: SmartSource | null
  tagId: number
  tagName: string
}

const mediaList = ref<InstanceType<typeof MediaList>>()
const tags = ref<Tag[]>([])
const selectedTagIds = ref<number[]>([])
const newTagName = ref('')
const creating = ref(false)
const filter = ref('')

const matching = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? tags.value.filter((t) => t.tagName.toLowerCase().includes(q)) : tags.value
})
const sections = computed(() => [
  { label: 'Your tags', smart: false, items: matching.value.filter((t) => !t.smart) },
  { label: 'Smart · Camera and lens', smart: true, items: matching.value.filter((t) => t.smart) },
])
const selectedTags = computed(() =>
  tags.value.filter((t) => selectedTagIds.value.includes(t.tagId)),
)
const onlyTag = computed(() => (selectedTags.value.length === 1 ? selectedTags.value[0] : null))

function clearTags() {
  selectedTagIds.value = []
  return queryPhotos()
}
const editing = ref<Tag | null>(null)
const editName = ref('')
const deleting = ref<Tag | null>(null)

const fail = (message: string) => () => toast.error(message)

const loadTags = async () =>
  (tags.value = await call<Tag[]>({ url: '/tag/getAll', params: { includeSmart: true } }))

async function queryPhotos() {
  if (!selectedTagIds.value.length) return mediaList.value?.updatePhotoList([])
  try {
    const photos = await call<Photo[]>({
      url: '/photo/getByTags',
      params: { tagIds: selectedTagIds.value.join(',') },
    })
    mediaList.value?.updatePhotoList(photos)
  } catch {
    toast.error('query photo Response Error!')
  }
}

function selectTag(tag: Tag) {
  const ids = selectedTagIds.value
  selectedTagIds.value = ids.includes(tag.tagId)
    ? ids.filter((id) => id !== tag.tagId)
    : [...ids, tag.tagId]
  return queryPhotos()
}

async function createTag() {
  if (!newTagName.value) return toast.warning('Please Input a Valid Tag Name!')
  try {
    await call({ method: 'post', url: '/tag/insert', data: { tagName: newTagName.value } })
    toast.success('Created a New Tag!')
    newTagName.value = ''
    creating.value = false
    await loadTags()
  } catch {
    toast.error('createTag Response Error!')
  }
}

function startEdit(tag: Tag) {
  editing.value = tag
  editName.value = tag.tagName
}

async function editTag() {
  if (!editing.value) return
  if (!editName.value) return toast.warning('Please Input a Valid Tag Name!')
  try {
    await call({
      method: 'post',
      url: '/tag/update',
      data: { tagId: editing.value.tagId, tagName: editName.value },
    })
    toast.success('Information Updated!')
    editing.value = null
    await loadTags()
  } catch {
    toast.error('Update Response Error!')
  }
}

async function deleteTag() {
  if (!deleting.value) return
  const { tagId } = deleting.value
  try {
    await call({ method: 'post', url: '/tag/delete', params: { tagId } })
    toast.success('Tag Deleted!')
    deleting.value = null
    selectedTagIds.value = selectedTagIds.value.filter((id) => id !== tagId)
    await Promise.all([loadTags(), queryPhotos()])
  } catch {
    toast.warning('Update Error!')
  }
}

async function detach(photos: (Photo & { checked?: boolean })[]) {
  if (selectedTagIds.value.length !== 1)
    return toast.warning(
      selectedTagIds.value.length
        ? 'Cannot operate when multiple tags are selected!'
        : 'Cannot operate when no tags is selected!',
    )
  const [tagId] = selectedTagIds.value
  try {
    await Promise.all(
      photos
        .filter((p) => p.checked)
        .map((p) =>
          call({ method: 'post', url: '/photo/deleteTag', params: { photoId: p.photoId, tagId } }),
        ),
    )
    toast.success('Tag detached!')
  } catch {
    toast.error('removeTag Response Error!')
  }
  await queryPhotos()
}

onMounted(() => loadTags().catch(fail('Tag Response Error!')))
</script>

<template>
  <div class="flex min-h-full flex-col md:flex-row" @contextmenu.prevent.stop>
    <!-- The tag list. -->
    <aside
      class="bg-card flex shrink-0 flex-col gap-4 border-b px-4 pt-5 pb-4 md:sticky md:top-0 md:h-[100dvh] md:w-80 md:overflow-y-auto md:border-r md:border-b-0 md:px-6 md:pt-8"
    >
      <AlbumSwitch class="md:hidden" />
      <div class="flex items-center gap-2">
        <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Tags</h1>
        <Transition name="reveal">
          <button
            v-if="selectedTagIds.length"
            type="button"
            class="text-primary hover:bg-accent ml-auto h-8 rounded-md px-2 text-sm font-medium transition-colors"
            @click="clearTags"
          >
            Deselect all ({{ selectedTagIds.length }})
          </button>
        </Transition>
        <Button
          variant="outline"
          size="icon-sm"
          aria-label="Add New Tag"
          :class="!selectedTagIds.length && 'ml-auto'"
          @click="creating = true"
          ><Plus
        /></Button>
      </div>
      <label class="relative">
        <Search
          class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
          aria-hidden="true"
        />
        <Input v-model="filter" class="pl-9" placeholder="Filter tags" aria-label="Filter tags" />
      </label>

      <template v-for="section in sections" :key="section.label">
        <section v-if="section.items.length" class="flex flex-col gap-0.5">
          <h2
            class="text-muted-foreground flex items-center gap-1.5 pb-1.5 text-xs font-semibold tracking-[0.025em] uppercase"
          >
            <Sparkles v-if="section.smart" class="text-primary size-3" aria-hidden="true" />
            {{ section.label }}
          </h2>
          <div class="flex flex-wrap gap-1.5 md:flex-col md:gap-0.5">
            <ContextMenu v-for="tag in section.items" :key="tag.tagId">
              <ContextMenuTrigger as-child>
                <button
                  type="button"
                  :aria-pressed="selectedTagIds.includes(tag.tagId)"
                  :title="tag.tagName"
                  :class="
                    cn(
                      'hover:bg-accent hover:text-accent-foreground flex h-9 min-w-0 items-center gap-1.5 rounded-lg px-2.5 text-left text-sm transition-colors select-none max-md:rounded-full max-md:border max-md:h-8',
                      selectedTagIds.includes(tag.tagId) &&
                        'bg-accent text-accent-foreground font-medium max-md:border-primary',
                    )
                  "
                  @click="selectTag(tag)"
                >
                  <Hash
                    v-if="!section.smart"
                    class="text-muted-foreground size-3.5 shrink-0"
                    aria-hidden="true"
                  />
                  <span class="truncate">{{ tag.tagName }}</span>
                  <!-- The section heading shows this; keep it in the name so twins stay distinct. -->
                  <span v-if="tag.smart" class="sr-only">{{
                    ` Smart · ${tag.smart.source === 'lens' ? 'Lens' : 'Camera'}`
                  }}</span>
                  <Check
                    v-if="selectedTagIds.includes(tag.tagId)"
                    class="text-primary animate-in zoom-in-50 ml-auto size-4 shrink-0"
                    aria-hidden="true"
                  />
                </button>
              </ContextMenuTrigger>
              <ContextMenuContent>
                <ContextMenuItem @select="startEdit(tag)"><Pencil /> Rename</ContextMenuItem>
                <ContextMenuItem variant="destructive" @select="deleting = tag"
                  ><Trash2 /> Delete</ContextMenuItem
                >
              </ContextMenuContent>
            </ContextMenu>
          </div>
        </section>
      </template>
      <p v-if="!matching.length" class="text-muted-foreground animate-fade text-sm">
        {{ tags.length ? 'No tags match.' : 'No tags yet.' }}
      </p>
    </aside>

    <!-- The selected tags' photos. -->
    <div class="flex min-w-0 flex-1 flex-col gap-6 px-4 pt-6 pb-32 md:px-10 md:pt-8">
      <EmptyState
        v-if="!selectedTagIds.length"
        :icon="TagIcon"
        title="Pick a tag"
        description="Choose one or more tags to see the photos and videos that carry them."
      />
      <template v-else>
        <PageHeader eyebrow="Tag" :title="selectedTags.map((t) => `#${t.tagName}`).join(' + ')">
          <template #title>
            <span v-for="(t, i) in selectedTags" :key="t.tagId"
              ><span v-if="i" class="text-muted-foreground font-normal"> + </span
              ><span class="text-muted-foreground font-normal">#</span>{{ t.tagName }}</span
            >
          </template>
          <template v-if="onlyTag" #description><SmartBadge :smart="onlyTag.smart" /></template>
          <template v-if="onlyTag">
            <Button variant="outline" @click="startEdit(onlyTag)"><Pencil /> Rename</Button>
            <Button
              variant="outline"
              size="icon"
              class="text-destructive hover:text-destructive"
              aria-label="Delete tag"
              @click="deleting = onlyTag"
              ><Trash2
            /></Button>
          </template>
          <Button v-else variant="outline" @click="clearTags"><X /> Clear</Button>
        </PageHeader>
      </template>
      <MediaList
        v-show="selectedTagIds.length"
        ref="mediaList"
        class="min-w-0"
        :ns-or-not="false"
        :detach-enabled="true"
        detach-str="Tag"
        @query-photos="queryPhotos"
        @do-detach="detach"
      />
    </div>

    <Dialog v-model:open="creating">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New Tag</DialogTitle>
          <DialogDescription>Tags describe what's in a photo, wherever it lives.</DialogDescription>
        </DialogHeader>
        <form class="flex flex-col gap-2" @submit.prevent="createTag">
          <Label for="new-tag">Name</Label>
          <Input id="new-tag" v-model="newTagName" placeholder="Tag Name" />
        </form>
        <DialogFooter>
          <Button variant="outline" @click="creating = false">Cancel</Button>
          <Button @click="createTag">Add</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete #{{ deleting?.tagName }}?</AlertDialogTitle>
          <AlertDialogDescription>
            The tag comes off every photo. The photos themselves stay.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteTag">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <Dialog :open="!!editing" @update:open="editing = null">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Rename Tag</DialogTitle>
          <DialogDescription
            >Every photo with #{{ editing?.tagName }} keeps it under the new
            name.</DialogDescription
          >
        </DialogHeader>
        <form class="flex flex-col gap-2" @submit.prevent="editTag">
          <Label for="rename-tag">Name</Label>
          <Input id="rename-tag" v-model="editName" placeholder="Tag Name" />
        </form>
        <DialogFooter>
          <Button variant="outline" @click="editing = null">Cancel</Button>
          <Button @click="editTag">Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
