<script setup lang="ts">
import SmartBadge from '@/components/SmartBadge.vue'
import type { SmartSource } from '@/lib/smartOrganization'
import { onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Pencil, Trash2 } from '@lucide/vue'
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
  <div class="flex flex-col gap-6 md:flex-row md:items-start" @contextmenu.prevent.stop>
    <aside class="w-full shrink-0 space-y-3 md:w-56">
      <h2 class="text-2xl font-semibold">Tags</h2>

      <details class="rounded-lg border">
        <summary class="cursor-pointer px-3 py-2 text-sm font-medium">Add New Tag</summary>
        <div class="space-y-2 border-t p-3">
          <Input v-model="newTagName" placeholder="Tag Name" @keyup.enter="createTag" />
          <Button variant="secondary" class="w-full" @click="createTag">Add</Button>
        </div>
      </details>

      <details open class="rounded-lg border">
        <summary class="cursor-pointer px-3 py-2 text-sm font-medium">Tags</summary>
        <div class="flex flex-wrap gap-2 border-t p-3 md:flex-col">
          <ContextMenu v-for="tag in tags" :key="tag.tagId">
            <ContextMenuTrigger as-child>
              <button
                type="button"
                :aria-pressed="selectedTagIds.includes(tag.tagId)"
                :title="tag.tagName"
                :class="
                  cn(
                    'max-w-full truncate transition-colors rounded-full border px-3 py-1 text-left text-sm select-none',
                    selectedTagIds.includes(tag.tagId)
                      ? 'bg-primary text-primary-foreground border-transparent'
                      : 'hover:bg-accent',
                    tag.smart && 'border-violet-500',
                  )
                "
                @click="selectTag(tag)"
              >
                {{ tag.tagName }} <SmartBadge :smart="tag.smart" />
              </button>
            </ContextMenuTrigger>
            <ContextMenuContent>
              <ContextMenuItem @select="startEdit(tag)"><Pencil /> Edit</ContextMenuItem>
              <ContextMenuItem variant="destructive" @select="deleting = tag"
                ><Trash2 /> Delete</ContextMenuItem
              >
            </ContextMenuContent>
          </ContextMenu>
        </div>
      </details>
      <Button
        v-if="selectedTagIds.length"
        variant="outline"
        size="sm"
        class="w-full"
        @click="(selectedTagIds = []), queryPhotos()"
        >Deselect all ({{ selectedTagIds.length }})</Button
      >
    </aside>

    <MediaList
      ref="mediaList"
      class="min-w-0 flex-1"
      :ns-or-not="false"
      :detach-enabled="true"
      detach-str="Tag"
      @query-photos="queryPhotos"
      @do-detach="detach"
    />

    <AlertDialog :open="!!deleting" @update:open="deleting = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription>
            Do you really want to DELETE this tag? *All photos associated with this tag will be
            detached (photos will not been deleted).
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteTag">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <Dialog :open="!!editing" @update:open="editing = null">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Tag</DialogTitle>
          <DialogDescription>Rename this tag.</DialogDescription>
        </DialogHeader>
        <Input v-model="editName" placeholder="Tag Name" @keyup.enter="editTag" />
        <DialogFooter><Button @click="editTag">Submit</Button></DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
