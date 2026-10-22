<script setup lang="ts">
import { ref } from 'vue'
import { toast } from 'vue-sonner'
import { Plus, Pencil, Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { uploadCall } from '@/stores/uploads'
import type { StorageGroup } from '@/lib/storageGroups'
const props = defineProps<{ groups: StorageGroup[] }>()
const emit = defineEmits<{ changed: []; members: [id: number] }>()
const editing = ref(false)
const current = ref<StorageGroup | null>(null)
const name = ref('')
const unlimited = ref(false)
const limit = ref('50')
const removing = ref<StorageGroup | null>(null)
const reassign = ref('')
const saving = ref(false)
function edit(group: StorageGroup | null) {
  current.value = group
  name.value = group?.name ?? ''
  unlimited.value = group?.maxStorageBytes === null
  limit.value = String((group?.maxStorageBytes ?? 50e9) / 1e9)
  editing.value = true
}
async function save() {
  const bytes = Math.round(Number(limit.value) * 1e9)
  if (
    !name.value.trim() ||
    (!unlimited.value &&
      (!Number.isFinite(bytes) || bytes < 0 || String(limit.value).trim() === ''))
  )
    return toast.error('Enter a name and a valid storage limit.')
  saving.value = true
  try {
    await uploadCall(
      `/admin/user-groups${current.value ? `/${current.value.id}` : ''}`,
      current.value ? 'put' : 'post',
      { name: name.value.trim(), maxStorageBytes: unlimited.value ? null : bytes },
    )
    editing.value = false
    emit('changed')
    toast.success('Group saved')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Could not save group')
  } finally {
    saving.value = false
  }
}
async function remove() {
  if (!removing.value || !reassign.value) return
  saving.value = true
  try {
    await uploadCall(`/admin/user-groups/${removing.value.id}`, 'delete', {
      reassignTo: Number(reassign.value),
    })
    removing.value = null
    emit('changed')
    toast.success('Members reassigned and group deleted')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Could not delete group')
  } finally {
    saving.value = false
  }
}
</script>
<template>
  <section class="space-y-4">
    <div class="flex items-start justify-between gap-4">
      <p class="text-muted-foreground max-w-xl text-sm">
        Max Storage applies to each member. Personal overrides take precedence. New accounts join
        the default group.
      </p>
      <Button @click="edit(null)"><Plus />New group</Button>
    </div>
    <div class="relative overflow-x-auto rounded-xl border">
      <table class="w-full min-w-[560px] text-left text-sm">
        <thead class="bg-muted/40">
          <tr>
            <th class="p-4">Group</th>
            <th class="p-4">Max Storage per user</th>
            <th class="p-4">Members</th>
            <th class="p-4"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="group in props.groups" :key="group.id" class="border-t">
            <td class="p-4 font-medium">
              {{ group.name
              }}<span v-if="group.isDefault" class="bg-accent ml-2 rounded-full px-2 py-1 text-xs"
                >Default</span
              >
            </td>
            <td class="p-4">
              {{
                group.maxStorageBytes === null ? 'Unlimited' : `${group.maxStorageBytes / 1e9} GB`
              }}
            </td>
            <td class="p-4">
              <button class="text-primary underline" @click="emit('members', group.id)">
                {{ group.memberCount }} members
              </button>
            </td>
            <td class="p-4 text-right whitespace-nowrap">
              <Button
                variant="ghost"
                size="sm"
                :aria-label="`Edit group ${group.name}`"
                @click="edit(group)"
                ><Pencil />Edit</Button
              ><Button
                v-if="!group.isDefault"
                variant="ghost"
                size="sm"
                :aria-label="`Delete group ${group.name}`"
                @click="((removing = group), (reassign = ''))"
                ><Trash2 />Delete</Button
              >
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <Dialog v-model:open="editing"
      ><DialogContent
        ><DialogHeader
          ><DialogTitle>{{ current ? 'Edit group' : 'New group' }}</DialogTitle
          ><DialogDescription
            >Lowering the limit keeps existing data. Members above their limit cannot add
            storage.</DialogDescription
          ></DialogHeader
        ><Label for="group-name">Group name</Label
        ><Input id="group-name" v-model="name" maxlength="255" /><label class="flex gap-2 text-sm"
          ><input v-model="unlimited" type="checkbox" />Unlimited storage</label
        ><Label for="group-limit">Max Storage per user (GB)</Label
        ><Input
          id="group-limit"
          v-model="limit"
          type="number"
          min="0"
          step="0.1"
          :disabled="unlimited"
        /><DialogFooter
          ><Button variant="outline" @click="editing = false">Cancel</Button
          ><Button :disabled="saving" @click="save">Save group</Button></DialogFooter
        ></DialogContent
      ></Dialog
    >
    <Dialog :open="!!removing" @update:open="removing = null"
      ><DialogContent
        ><DialogHeader
          ><DialogTitle>Delete {{ removing?.name }}?</DialogTitle
          ><DialogDescription
            >Choose a group for its members. Personal storage overrides are
            retained.</DialogDescription
          ></DialogHeader
        ><Label for="reassign-group">Move members to</Label
        ><select id="reassign-group" v-model="reassign" class="bg-background rounded-md border p-2">
          <option disabled value="">Choose group</option>
          <option
            v-for="group in groups.filter((g) => g.id !== removing?.id)"
            :key="group.id"
            :value="String(group.id)"
          >
            {{ group.name }}
          </option></select
        ><DialogFooter
          ><Button variant="outline" @click="removing = null">Cancel</Button
          ><Button variant="destructive" :disabled="!reassign || saving" @click="remove"
            >Reassign and delete</Button
          ></DialogFooter
        ></DialogContent
      ></Dialog
    >
  </section>
</template>
