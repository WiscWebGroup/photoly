<script setup lang="ts">
import { onMounted, ref } from 'vue'
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { assertAdmin, useAdminGuard } from '@/composables/useAdminGuard'
import { ApiError, call, http, type Envelope } from '@/lib/http'
import { aiCall, aiMessage } from '@/lib/ai'
import { Switch } from '@/components/ui/switch'

interface User {
  userId: number
  userName: string
  email: string
  createDate: string
  role: string
  uuid: string
}

const { guarded } = useAdminGuard()

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
const pageSize = ref(10)
const pageCount = ref(1)
/** `null` browses everything; a string filters by it. */
const activeSearch = ref<string | null>(null)
const searchText = ref('')

const passwordUser = ref<User | null>(null)
const newPassword = ref('')
const deleteUser = ref<User | null>(null)

async function load() {
  const sequence = ++loadSequence
  const search = activeSearch.value
  const { data } = await http.get<Envelope<User[]> & { pageNum: number }>(
    search === null ? '/admin/getUserPage' : '/admin/searchUserPage',
    {
      params: {
        rowsPerPage: pageSize.value,
        page: page.value,
        ...(search === null ? {} : { search }),
      },
    },
  )
  if (data.msgCode !== 200) throw new ApiError(data.msgCode)
  if (sequence !== loadSequence) return
  users.value = data.t
  pageCount.value = data.pageNum
  aiStates.value = {}
  aiError.value = ''
  try {
    if (data.t.length) {
      const states = await aiCall<{ userId: number; enabled: boolean }[]>({
        url: '/admin/ai/users',
        params: { ids: data.t.map((u) => u.userId).join(',') },
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
    await load()
  }),
)
</script>

<template>
  <AdminLayout>
    <Card>
      <CardHeader><CardTitle class="text-2xl">User Management</CardTitle></CardHeader>
      <CardContent class="space-y-4">
        <div class="flex flex-wrap gap-2">
          <Input
            v-model="searchText"
            class="max-w-xs"
            aria-label="Search users"
            @keyup.enter="search(searchText)"
          />
          <Button variant="outline" @click="search(searchText)">Search</Button>
          <Button variant="outline" @click="search(null)">Back to Indexing</Button>
        </div>

        <p class="text-sm text-muted-foreground">
          Enabling AI indexes existing photos in the background. Disabling stops search and new
          computation; stored vectors are retained. New-user defaults are in
          <router-link class="underline" to="/thepersonincharge/ai">AI Management</router-link>.
        </p>
        <p v-if="aiError" role="alert" class="text-sm text-destructive">
          {{ aiError }} <Button variant="outline" size="sm" @click="reload">Retry</Button>
        </p>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>User ID</TableHead>
              <TableHead>Username</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Create Date</TableHead>
              <TableHead>Role</TableHead>
              <TableHead>AI Search</TableHead>
              <TableHead>UUID</TableHead>
              <TableHead>Password</TableHead>
              <TableHead>Delete</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="u in users" :key="u.userId">
              <TableCell>{{ u.userId }}</TableCell>
              <TableCell
                ><Input v-model="u.userName" aria-label="Username" @change="rename(u)"
              /></TableCell>
              <TableCell
                ><Input v-model="u.email" aria-label="Email" @change="changeEmail(u)"
              /></TableCell>
              <TableCell>{{ u.createDate }}</TableCell>
              <TableCell>
                <Select :model-value="u.role" @update:model-value="changeRole(u, $event as string)">
                  <SelectTrigger class="w-28" aria-label="Role"><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="admin">admin</SelectItem>
                    <SelectItem value="user">user</SelectItem>
                  </SelectContent>
                </Select>
              </TableCell>
              <TableCell
                ><Switch
                  :aria-label="`AI search for user ${u.userId}`"
                  :model-value="aiStates[u.userId] ?? false"
                  :disabled="aiStates[u.userId] === undefined || aiSaving[u.userId]"
                  @update:model-value="(enabled) => changeAi(u.userId, enabled)"
                /><span v-if="aiStates[u.userId] === undefined" class="text-xs"
                  >Unknown</span
                ></TableCell
              >
              <TableCell>{{ u.uuid }}</TableCell>
              <TableCell
                ><Button variant="secondary" size="sm" @click="passwordUser = u"
                  >Change</Button
                ></TableCell
              >
              <TableCell
                ><Button variant="destructive" size="sm" @click="deleteUser = u"
                  >Delete User</Button
                ></TableCell
              >
            </TableRow>
          </TableBody>
        </Table>

        <div class="flex flex-wrap items-center justify-end gap-3">
          <Select :model-value="String(pageSize)" @update:model-value="goTo(1, Number($event))">
            <SelectTrigger class="w-28" aria-label="Rows per page"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="n in [10, 20, 30]" :key="n" :value="String(n)"
                >{{ n }} / page</SelectItem
              >
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" :disabled="page <= 1" @click="goTo(page - 1)"
            >Previous</Button
          >
          <span class="text-sm">Page {{ page }} of {{ pageCount }}</span>
          <Button variant="outline" size="sm" :disabled="page >= pageCount" @click="goTo(page + 1)"
            >Next</Button
          >
        </div>
      </CardContent>
    </Card>

    <Dialog :open="!!passwordUser" @update:open="passwordUser = null">
      <DialogContent>
        <DialogHeader><DialogTitle>Edit Password</DialogTitle></DialogHeader>
        <p class="text-sm">
          ID: {{ passwordUser?.userId }} | Username: {{ passwordUser?.userName }}
        </p>
        <Input v-model="newPassword" placeholder="New Password" @keyup.enter="changePassword" />
        <DialogFooter>
          <Button variant="outline" @click="passwordUser = null">Cancel</Button>
          <Button @click="changePassword">Submit</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="!!deleteUser" @update:open="deleteUser = null">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription>
            Do you really want to Delete this User with ID {{ deleteUser?.userId }} and Username
            {{ deleteUser?.userName }}?
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
