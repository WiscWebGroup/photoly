<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { KeyRound, Search, Trash2, X } from '@lucide/vue'
import AdminStorageGroups from '@/components/AdminStorageGroups.vue'
import type { StorageGroup } from '@/lib/storageGroups'
import { uploadCall } from '@/stores/uploads'
import { storageLabel } from '@/composables/useStorageUsage'
import PageHeader from '@/components/PageHeader.vue'
import { toast } from 'vue-sonner'
import AdminLayout from '@/components/AdminLayout.vue'
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { assertAdmin, useAdminGuard } from '@/composables/useAdminGuard'
import { call } from '@/lib/http'
import { cn } from '@/lib/utils'
import { aiCall, aiMessage } from '@/lib/ai'
import { Switch } from '@/components/ui/switch'

interface User {
  userId: number
  userName: string
  email: string
  createDate: string
  role: string
  uuid: string
  groupId: number
  groupName: string
  usedBytes: number
  capacityBytes: number | null
  storageOverride: boolean
  maxStorageBytes: number | null
  overQuota: boolean
  mediaCount: number
}

const { guarded } = useAdminGuard()

const activeTab = ref('users')
const groups = ref<StorageGroup[]>([])
const groupFilter = ref('')
const storageUser = ref<User | null>(null)
const storageGroup = ref('')
const storageMode = ref('inherit')
const storageLimit = ref('50')
const storageSaving = ref(false)
async function loadGroups() {
  groups.value = await call<StorageGroup[]>({ url: '/admin/user-groups' })
}
async function groupsChanged() {
  await Promise.all([loadGroups(), reload()])
}
function showMembers(id: number) {
  activeTab.value = 'users'
  groupFilter.value = String(id)
  goTo(1)
}
function editStorage(user: User) {
  storageUser.value = user
  storageGroup.value = String(user.groupId)
  storageMode.value = user.storageOverride
    ? user.maxStorageBytes === null
      ? 'unlimited'
      : 'limit'
    : 'inherit'
  storageLimit.value = String((user.maxStorageBytes ?? user.capacityBytes ?? 50e9) / 1e9)
}
async function saveStorage() {
  if (!storageUser.value) return
  const bytes = Math.round(Number(storageLimit.value) * 1e9)
  if (
    storageMode.value === 'limit' &&
    (!Number.isFinite(bytes) || bytes < 0 || String(storageLimit.value).trim() === '')
  )
    return toast.error('Enter a valid storage limit.')
  storageSaving.value = true
  try {
    await uploadCall(`/admin/users/${storageUser.value.userId}/storage`, 'put', {
      groupId: Number(storageGroup.value),
      storageOverride: storageMode.value !== 'inherit',
      maxStorageBytes: storageMode.value === 'limit' ? bytes : null,
    })
    storageUser.value = null
    await groupsChanged()
    toast.success('Storage settings saved')
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Could not save storage settings')
  } finally {
    storageSaving.value = false
  }
}

const users = ref<User[]>([])
const aiStates = ref<Record<number, boolean>>({})
const aiSaving = ref<Record<number, boolean>>({})
const aiError = ref('')
let loadSequence = 0
async function changeAi(userId: number, enabled: boolean) {
  if (aiSaving.value[userId]) return
  aiSaving.value[userId] = true
  aiError.value = ''
  try {
    const result = await aiCall<{ userId: number; enabled: boolean }>({
      method: 'patch',
      url: `/admin/users/${userId}/ai`,
      data: { enabled },
    })
    aiStates.value[userId] = result.enabled
  } catch (e) {
    aiError.value = aiMessage(e)
  } finally {
    aiSaving.value[userId] = false
  }
}
const page = ref(1)
const pageSize = ref(8)
const total = ref(0)
const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const roles = [
  { value: null, label: 'All' },
  { value: 'admin', label: 'Admins' },
  { value: 'user', label: 'Users' },
] as const
const role = ref<'admin' | 'user' | null>(null)
/** "1–8 of 48", or "0 users" for an empty page. */
const range = computed(() => {
  if (!total.value) return 'No users'
  const first = (page.value - 1) * pageSize.value + 1
  return `${first}–${Math.min(total.value, first + users.value.length - 1)} of ${total.value}`
})
/** `null` browses everything; a string filters by it. */
const activeSearch = ref<string | null>(null)
const searchText = ref('')

// A steady color and initials per person, so rows are easy to tell apart.
const tints = [
  '#16827d',
  '#7c3aed',
  '#d97706',
  '#db2777',
  '#2563eb',
  '#0891b2',
  '#4d7c0f',
  '#9333ea',
]
const tint = (id: number) => tints[Math.abs(id) % tints.length]
const initials = (name = '') =>
  name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((w) => w[0])
    .join('')

const passwordUser = ref<User | null>(null)
const newPassword = ref('')
const deleteUser = ref<User | null>(null)

async function load() {
  const sequence = ++loadSequence
  const search = activeSearch.value
  const data = await call<{ users: User[]; total: number }>({
    url: '/admin/getUsers',
    params: {
      page: page.value,
      ...(groupFilter.value ? { groupId: Number(groupFilter.value) } : {}),
      rowsPerPage: pageSize.value,
      ...(search === null ? {} : { search }),
      ...(role.value === null ? {} : { role: role.value }),
    },
  })
  if (sequence !== loadSequence) return
  users.value = data.users
  total.value = data.total
  aiStates.value = {}
  aiError.value = ''
  try {
    if (data.users.length) {
      const states = await aiCall<{ userId: number; enabled: boolean }[]>({
        url: '/admin/ai/users',
        params: { ids: data.users.map((u) => u.userId).join(',') },
      })
      if (sequence === loadSequence)
        aiStates.value = Object.fromEntries(states.map((u) => [u.userId, u.enabled]))
    }
  } catch (e) {
    if (sequence === loadSequence) aiError.value = aiMessage(e)
  }
}

const reload = () => load().catch(() => toast.error('Get User Information Error!'))

function goTo(p: number, size = pageSize.value) {
  page.value = p
  pageSize.value = size
  return reload()
}

function search(query: string | null) {
  activeSearch.value = query
  return goTo(1)
}

function filterRole(value: 'admin' | 'user' | null) {
  role.value = value
  return goTo(1)
}

/** POSTs an admin mutation, then refreshes the current page. Returns whether it succeeded. */
async function save(
  url: string,
  body: { params?: object; data?: object },
  message = 'update successful!',
) {
  try {
    await call({ method: 'post', url: `/admin/${url}`, ...body })
    toast.success(message)
    await reload()
    return true
  } catch {
    toast.error('Update Error!')
    return false
  }
}

const rename = (u: User) =>
  save('resetUsername', { data: { userId: u.userId, userName: u.userName } })
const changeEmail = (u: User) =>
  save('resetUserEmail', { data: { userId: u.userId, email: u.email } })
const changeRole = (u: User, role: string) =>
  save('resetUserRole', { data: { userId: u.userId, role } })

async function changePassword() {
  const user = passwordUser.value
  if (
    user &&
    (await save(
      'resetUserPassword',
      { data: { userId: user.userId, password: newPassword.value } },
      'Update successful!',
    ))
  )
    passwordUser.value = null
}

async function removeUser() {
  const user = deleteUser.value
  if (user && (await save('deleteUser', { params: { userId: user.userId } }, 'Delete successful!')))
    deleteUser.value = null
}

onMounted(() =>
  guarded(async () => {
    await assertAdmin()
    await Promise.all([load(), loadGroups()])
  }),
)
</script>

<template>
  <AdminLayout wide>
    <PageHeader title="User Management" description="Everyone with an account on this server." />

    <div role="tablist" aria-label="User management" class="flex gap-2">
      <Button
        role="tab"
        :aria-selected="activeTab === 'users'"
        :variant="activeTab === 'users' ? 'default' : 'outline'"
        @click="activeTab = 'users'"
        >Users</Button
      ><Button
        role="tab"
        :aria-selected="activeTab === 'groups'"
        :variant="activeTab === 'groups' ? 'default' : 'outline'"
        @click="activeTab = 'groups'"
        >Groups</Button
      >
    </div>
    <AdminStorageGroups
      v-if="activeTab === 'groups'"
      :groups="groups"
      @changed="groupsChanged"
      @members="showMembers"
    />
    <template v-if="activeTab === 'users'">
      <form
        class="flex flex-wrap items-center gap-2"
        role="search"
        @submit.prevent="search(searchText)"
      >
        <label class="relative w-full max-w-80">
          <Search
            class="text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2"
            aria-hidden="true"
          />
          <Input
            v-model="searchText"
            class="pl-9"
            placeholder="Search name or email"
            aria-label="Search users"
          />
        </label>
        <Button type="submit" variant="outline">Search</Button>
        <select
          v-model="groupFilter"
          aria-label="Filter by group"
          class="bg-background h-9 rounded-md border px-2 text-sm"
          @change="goTo(1)"
        >
          <option value="">All groups</option>
          <option v-for="group in groups" :key="group.id" :value="String(group.id)">
            {{ group.name }}
          </option>
        </select>
        <Button
          v-if="activeSearch !== null"
          type="button"
          variant="ghost"
          @click="((searchText = ''), search(null))"
          ><X /> Clear</Button
        >
        <div
          role="radiogroup"
          aria-label="Role"
          class="bg-muted flex gap-0.5 rounded-lg p-0.5 sm:ml-auto"
        >
          <button
            v-for="r in roles"
            :key="r.label"
            type="button"
            role="radio"
            :aria-checked="role === r.value"
            :class="
              cn(
                'text-muted-foreground hover:text-foreground h-8 rounded-md px-3 text-sm font-medium transition-[background-color,color,box-shadow]',
                role === r.value && 'bg-card text-foreground shadow-xs',
              )
            "
            @click="filterRole(r.value)"
          >
            {{ r.label }}
          </button>
        </div>
      </form>

      <p class="text-muted-foreground -mt-2 text-sm">
        Turning on AI search indexes that person's photos in the background; turning it off stops
        new work and keeps what's stored. Defaults for new accounts live in
        <router-link class="text-primary font-medium hover:underline" to="/thepersonincharge/ai"
          >AI</router-link
        >.
      </p>
      <p
        v-if="aiError"
        role="alert"
        class="border-destructive/30 bg-destructive/10 text-destructive flex items-center gap-3 rounded-xl border px-4 py-3 text-sm"
      >
        {{ aiError }}
        <Button variant="outline" size="sm" class="ml-auto" @click="reload">Retry</Button>
      </p>

      <div class="bg-card min-w-0 overflow-hidden rounded-xl border shadow-xs">
        <!-- Contain absolutely positioned accessible labels within the horizontal scroller. -->
        <div class="relative w-full min-w-0 overflow-x-auto">
          <table class="w-full min-w-[880px] border-collapse text-sm">
            <thead>
              <tr class="text-muted-foreground text-left text-xs">
                <th scope="col" class="border-b px-4 py-3 font-medium">User</th>
                <th scope="col" class="border-b px-4 py-3 font-medium">Role</th>
                <th scope="col" class="border-b px-4 py-3 font-medium">Storage / Group</th>
                <th scope="col" class="border-b px-4 py-3 font-medium">AI search</th>
                <th scope="col" class="border-b px-4 py-3 text-right font-medium">Media</th>
                <th scope="col" class="border-b px-4 py-3 font-medium">Joined</th>
                <th scope="col" class="border-b px-4 py-3"><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="(u, i) in users"
                :key="u.userId"
                class="animate-rise hover:bg-muted/40 transition-colors [&>td]:border-b"
                :style="{ animationDelay: `${i * 30}ms` }"
              >
                <td class="px-4 py-2.5">
                  <div class="flex items-center gap-3">
                    <span
                      class="grid size-8 shrink-0 place-items-center rounded-full text-xs font-semibold text-white uppercase"
                      :style="{ background: tint(u.userId) }"
                      aria-hidden="true"
                      >{{ initials(u.userName) }}</span
                    >
                    <div class="flex min-w-0 flex-1 flex-col">
                      <input
                        v-model="u.userName"
                        aria-label="Username"
                        class="hover:border-input focus:border-ring focus:ring-ring/30 -mx-1.5 rounded-md border border-transparent bg-transparent px-1.5 font-medium outline-none focus:ring-3"
                        @change="rename(u)"
                      />
                      <input
                        v-model="u.email"
                        aria-label="Email"
                        class="text-muted-foreground hover:border-input focus:border-ring focus:ring-ring/30 -mx-1.5 rounded-md border border-transparent bg-transparent px-1.5 text-xs outline-none focus:ring-3"
                        @change="changeEmail(u)"
                      />
                    </div>
                  </div>
                </td>
                <td class="px-4 py-2.5">
                  <Select
                    :model-value="u.role"
                    @update:model-value="changeRole(u, $event as string)"
                  >
                    <SelectTrigger size="sm" class="w-24 capitalize" aria-label="Role"
                      ><SelectValue
                    /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="admin">admin</SelectItem>
                      <SelectItem value="user">user</SelectItem>
                    </SelectContent>
                  </Select>
                </td>
                <td class="px-4 py-2.5">
                  <button
                    class="text-left text-xs hover:underline"
                    :aria-label="`Storage for ${u.userName}`"
                    @click="editStorage(u)"
                  >
                    <span :class="u.overQuota ? 'text-destructive' : 'text-foreground'">{{
                      storageLabel(u)
                    }}</span
                    ><span class="text-muted-foreground mt-1 block"
                      >{{ u.groupName }} ·
                      {{ u.storageOverride ? 'Personal override' : 'Inherited' }}</span
                    >
                  </button>
                </td>
                <td class="px-4 py-2.5">
                  <span class="flex items-center gap-2">
                    <Switch
                      :aria-label="`AI search for user ${u.userId}`"
                      :model-value="aiStates[u.userId] ?? false"
                      :disabled="aiStates[u.userId] === undefined || aiSaving[u.userId]"
                      @update:model-value="(enabled) => changeAi(u.userId, enabled)"
                    /><span
                      v-if="aiStates[u.userId] === undefined"
                      class="text-muted-foreground text-xs"
                      >Unknown</span
                    >
                  </span>
                </td>
                <td class="px-4 py-2.5 text-right tabular-nums">
                  {{ u.mediaCount.toLocaleString() }}
                </td>
                <td class="text-muted-foreground px-4 py-2.5 whitespace-nowrap">
                  {{ u.createDate?.slice(0, 10) }}
                </td>
                <td class="px-4 py-2.5 text-right whitespace-nowrap">
                  <Button variant="ghost" size="sm" @click="passwordUser = u"
                    ><KeyRound /> Reset Password</Button
                  >
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    class="text-destructive hover:text-destructive"
                    aria-label="Delete User"
                    @click="deleteUser = u"
                    ><Trash2
                  /></Button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
          <span class="text-muted-foreground tabular-nums" aria-live="polite">{{ range }}</span>
          <span class="text-muted-foreground ml-auto">Rows</span>
          <Select :model-value="String(pageSize)" @update:model-value="goTo(1, Number($event))">
            <SelectTrigger size="sm" class="w-20" aria-label="Rows per page"
              ><SelectValue
            /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="n in [8, 20, 50]" :key="n" :value="String(n)">{{ n }}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" :disabled="page <= 1" @click="goTo(page - 1)"
            >Previous</Button
          >
          <Button variant="outline" size="sm" :disabled="page >= pageCount" @click="goTo(page + 1)"
            >Next</Button
          >
        </div>
      </div>
    </template>
    <Dialog :open="!!storageUser" @update:open="storageUser = null"
      ><DialogContent
        ><DialogHeader
          ><DialogTitle>Storage for {{ storageUser?.userName }}</DialogTitle
          ><DialogDescription
            >Personal limits override the group. Existing files are kept when a limit is
            lowered.</DialogDescription
          ></DialogHeader
        ><Label for="user-group">User group</Label
        ><select id="user-group" v-model="storageGroup" class="bg-background rounded-md border p-2">
          <option v-for="group in groups" :key="group.id" :value="String(group.id)">
            {{ group.name }}
          </option></select
        ><Label for="quota-mode">Storage limit</Label
        ><select id="quota-mode" v-model="storageMode" class="bg-background rounded-md border p-2">
          <option value="inherit">Inherit group limit</option>
          <option value="limit">Personal limit</option>
          <option value="unlimited">Unlimited</option></select
        ><template v-if="storageMode === 'limit'"
          ><Label for="user-limit">Max Storage (GB)</Label
          ><Input
            id="user-limit"
            v-model="storageLimit"
            type="number"
            min="0"
            step="0.1" /></template
        ><DialogFooter
          ><Button variant="outline" @click="storageUser = null">Cancel</Button
          ><Button :disabled="storageSaving" @click="saveStorage"
            >Save storage</Button
          ></DialogFooter
        ></DialogContent
      ></Dialog
    >
    <Dialog :open="!!passwordUser" @update:open="passwordUser = null">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Reset Password</DialogTitle>
          <DialogDescription
            >Set a new password for {{ passwordUser?.userName }} (ID
            {{ passwordUser?.userId }}).</DialogDescription
          >
        </DialogHeader>
        <form class="flex flex-col gap-2" @submit.prevent="changePassword">
          <Label for="reset-pw">New Password</Label>
          <Input id="reset-pw" v-model="newPassword" placeholder="New Password" />
        </form>
        <DialogFooter>
          <Button variant="outline" @click="passwordUser = null">Cancel</Button>
          <Button @click="changePassword">Submit</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="!!deleteUser" @update:open="deleteUser = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete {{ deleteUser?.userName }}?</AlertDialogTitle>
          <AlertDialogDescription>
            The account with ID {{ deleteUser?.userId }} and its media folder
            <span class="font-mono text-xs">{{ deleteUser?.uuid }}</span> are deleted. This can't be
            undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="removeUser">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </AdminLayout>
</template>
