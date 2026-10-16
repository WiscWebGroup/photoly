<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Copy, Pencil, Plus, Trash2 } from '@lucide/vue'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Avatar, AvatarImage } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import SmartOrganizationSettings from '@/components/SmartOrganizationSettings.vue'
import AvatarCropper from '@/components/AvatarCropper.vue'
import EmptyState from '@/components/EmptyState.vue'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

interface UserInfo {
  userId: number
  userName: string
  email: string
  createDate: string
  role: string
}
interface Credential {
  credId: number
  authorization: string
  token: string
}

const permissions = [
  { value: 'C', label: 'Upload' },
  { value: 'R', label: 'Read' },
  { value: 'D', label: 'Delete' },
]

const docs = [
  {
    title: 'C1 Upload Photo - POST:',
    addr: 'cred/uploadPhoto/YOUR_TOKEN',
    takein: 'file:MultipartFile | photo:JSONText containing nsId(num) and visibility(num)',
    ret: "a string that is the photo's uuid",
  },
  {
    title: 'C2 Upload Photos - POST:',
    addr: 'cred/uploadPhotos/YOUR_TOKEN',
    takein:
      'files:MultipartFile[] | photos:JSONText containing list of photos( nsId(num) and visibility(num) )',
    ret: 'a list of string of photo uuids',
  },
  {
    title: 'D Delete Photo - POST:',
    addr: 'cred/deletePhoto/YOUR_TOKEN',
    takein: "uuid: str, photo's uuid",
    ret: 'an int, 1 if successful',
  },
  {
    title: 'R1 Render Photo - POST:',
    addr: 'cred/render/YOUR_TOKEN',
    takein: "uuid: str, photo's uuid",
    ret: 'a byte[] that is the image (could be seen directly in browser url)',
  },
  {
    title: 'R2 Get RootNS - GET:',
    addr: 'cred/queryRootNamespace/YOUR_TOKEN',
    takein: 'None',
    ret: 'an JSON object that is the Root namespace',
  },
  {
    title: 'R3 Get NS - GET:',
    addr: 'cred/queryNamespaces/YOUR_TOKEN',
    takein: 'parentId: int, the parentId of ns',
    ret: 'an JSON object that is the list of namespaces under parentId',
  },
  {
    title: 'R4 Get Photo List - GET:',
    addr: 'cred/queryPhotoList/YOUR_TOKEN',
    takein: 'nsId: int, the namespace Id',
    ret: "an JSON object that is the list of photos' meta info",
  },
]

const router = useRouter()
const auth = useAuthStore()

const user = ref<UserInfo>()
const credentials = ref<Credential[]>([])
const avatarSrc = computed(() => auth.avatarUrl)

const loadUser = async () => (user.value = await call<UserInfo>({ url: '/user/getInfo/' }))
const loadCredentials = async () =>
  (credentials.value = await call<Credential[]>({ url: '/cred/query' }))
const createdOn = computed(() => user.value?.createDate.split('T')[0])

async function goToAdminPanel() {
  try {
    if (!(await call<boolean>({ url: '/admin/isAdmin' })))
      return toast.error('You are not an admin!')
    toast.success('Admin Identity Verified, Now Teleporting!')
    router.push('/thepersonincharge/dashboard')
  } catch {
    toast.error('You are not an admin!')
  }
}

// --- Edit profile -------------------------------------------------------------------------------
const editingInfo = ref(false)
const form = ref({ userName: '', email: '', oldPassword: '', password: '', confirmPassword: '' })

function startEditInfo() {
  form.value = {
    userName: user.value?.userName ?? '',
    email: user.value?.email ?? '',
    oldPassword: '',
    password: '',
    confirmPassword: '',
  }
  editingInfo.value = true
}

async function saveInfo() {
  const f = form.value
  if (!f.userName && !f.email && !f.password) return toast.warning('Nothing to update about!')
  if (f.password !== f.confirmPassword) return toast.warning('Passwords are not equal!')
  if (!/.+@.+\..+/.test(f.email)) return toast.warning('Wrong Email Format!')

  const updates: [string, string, object][] = []
  if (f.userName && f.userName !== user.value?.userName)
    updates.push(['Username', '/user/updateUsername', { username: f.userName }])
  if (f.email !== user.value?.email)
    updates.push(['Email', '/user/updateEmail', { email: f.email }])
  if (f.password)
    updates.push([
      'Password',
      '/user/updatePassword',
      { oldPass: f.oldPassword, newPass: f.password },
    ])

  editingInfo.value = false
  await Promise.all(
    updates.map(([name, url, params]) =>
      call({ method: 'post', url, params }).then(
        () => toast.success(`${name} updated!`),
        () => toast.error(`${name} update Response Error!`),
      ),
    ),
  )
  await loadUser().catch(() => toast.error('User Info Response Error!'))
}

// --- Avatar ---------------------------------------------------------------------------------------
const editingAvatar = ref(false)
const savingAvatar = ref(false)
const avatarError = ref('')
watch(editingAvatar, () => {
  avatarError.value = ''
})

async function saveAvatar(file: File) {
  if (savingAvatar.value) return
  savingAvatar.value = true
  avatarError.value = ''
  const data = new FormData()
  data.append('file', file)
  try {
    await call({ method: 'post', url: '/user/updateAvatar', data })
    toast.success('Avatar updated!')
    auth.refreshAvatar()
    editingAvatar.value = false
  } catch {
    avatarError.value = 'Could not save the avatar. Please try again.'
  } finally {
    savingAvatar.value = false
  }
}

// --- API credentials ----------------------------------------------------------------------------
/** `null`: closed; `credId` -1: creating a new credential. */
const apiDialog = ref<{ credId: number; perms: string[] } | null>(null)
const deletingCredId = ref<number>()

const openApiDialog = (credId: number, authorization = '') =>
  (apiDialog.value = { credId, perms: authorization.split('') })
const togglePerm = (perm: string, on: boolean | 'indeterminate') => {
  const d = apiDialog.value!
  d.perms = on === true ? [...d.perms, perm] : d.perms.filter((p) => p !== perm)
}

async function saveApi() {
  const d = apiDialog.value
  if (!d) return
  // Canonical order, so the stored string doesn't depend on click order.
  const authorization = permissions
    .filter((p) => d.perms.includes(p.value))
    .map((p) => p.value)
    .join('')
  const creating = d.credId === -1
  try {
    await call(
      creating
        ? { method: 'post', url: '/cred/create', data: { authorization } }
        : { method: 'post', url: '/cred/update', data: { authorization, credId: d.credId } },
    )
    toast.success(creating ? 'API Created!' : 'Update Success!')
    apiDialog.value = null
    await loadCredentials()
  } catch {
    toast.error(creating ? 'Create API Error!' : 'Update API Error!')
  }
}

async function deleteApi() {
  try {
    await call({ method: 'post', url: '/cred/delete', params: { credId: deletingCredId.value } })
    toast.success('Delete Success!')
    deletingCredId.value = undefined
    await loadCredentials()
  } catch {
    toast.error('Update API Error!')
  }
}

async function copy(token: string) {
  await navigator.clipboard.writeText(token)
  toast.success('Copied to the Clipboard!')
}

onMounted(() => {
  loadUser().catch(() => toast.error('User Info Response Error!'))
  loadCredentials().catch(() => toast.error('Get API Error!'))
})
</script>

<template>
  <div class="mx-auto max-w-4xl space-y-6 p-4 md:p-8">
    <Card v-if="user" class="from-primary/15 to-primary/5 bg-gradient-to-r">
      <CardContent class="flex flex-col items-center gap-6 sm:flex-row">
        <div class="relative">
          <Avatar class="size-32"><AvatarImage :src="avatarSrc" alt="" /></Avatar>
          <Badge class="absolute -top-1 -right-1">{{ user.role }}</Badge>
        </div>
        <div class="space-y-1 text-center sm:text-left">
          <h2 class="text-2xl font-semibold">{{ user.userName }}</h2>
          <p>email: {{ user.email }}</p>
          <p>created: {{ createdOn }}</p>
          <div class="flex flex-wrap justify-center gap-2 pt-3 sm:justify-start">
            <Button variant="secondary" @click="startEditInfo">Edit Info</Button>
            <Button variant="secondary" @click="editingAvatar = true">Edit Avatar</Button>
            <Button v-if="user.role === 'admin'" @click="goToAdminPanel">Admin Panel</Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <SmartOrganizationSettings />

    <Card>
      <CardHeader class="flex-row items-center justify-between">
        <CardTitle class="text-2xl">API Table</CardTitle>
        <Button size="icon" aria-label="Add API" @click="openApiDialog(-1)"><Plus /></Button>
      </CardHeader>
      <CardContent class="space-y-6">
        <EmptyState v-if="!credentials.length" title="No API tokens yet" class="py-6" />
        <Table v-else>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>Authorization</TableHead>
              <TableHead>Token</TableHead>
              <TableHead>Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow v-for="api in credentials" :key="api.credId">
              <TableCell>{{ api.credId }}</TableCell>
              <TableCell>{{ api.authorization }}</TableCell>
              <TableCell class="font-mono text-xs break-all">{{ api.token }}</TableCell>
              <TableCell class="space-x-1 whitespace-nowrap">
                <Button variant="ghost" size="icon" aria-label="Copy token" @click="copy(api.token)"
                  ><Copy
                /></Button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Edit API"
                  @click="openApiDialog(api.credId, api.authorization)"
                  ><Pencil
                /></Button>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Delete API"
                  @click="deletingCredId = api.credId"
                  ><Trash2
                /></Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>

        <div>
          <h3 class="mb-2 text-xl font-semibold">API Documents</h3>
          <details class="rounded-lg border">
            <summary class="cursor-pointer px-3 py-2 font-medium">API functionalities</summary>
            <ul class="space-y-1 border-t p-3 text-sm">
              <li>C: Able to upload photo (Create)</li>
              <li>R: Able to query namespace, photos in namespaces and render the photo (Read)</li>
              <li>D: Able to delete photo (Delete)</li>
            </ul>
          </details>
          <details class="mt-2 rounded-lg border">
            <summary class="cursor-pointer px-3 py-2 font-medium">
              How to use API in external build
            </summary>
            <div class="space-y-4 border-t p-3 text-sm">
              <section v-for="d in docs" :key="d.addr">
                <h4 class="font-semibold">{{ d.title }}</h4>
                <ul class="border-l-4 pl-3">
                  <li>Addr: server_path/{{ d.addr }}</li>
                  <li>Takein: {{ d.takein }}</li>
                  <li>Return: {{ d.ret }}</li>
                </ul>
              </section>
            </div>
          </details>
        </div>
      </CardContent>
    </Card>

    <Dialog v-model:open="editingInfo">
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Information</DialogTitle>
          <DialogDescription>Leave a field unchanged to keep it as is.</DialogDescription>
        </DialogHeader>
        <form class="space-y-3" @submit.prevent="saveInfo">
          <Label for="edit-name">Username</Label>
          <Input id="edit-name" v-model="form.userName" placeholder="User Name" />
          <Label for="edit-email">Email</Label>
          <Input id="edit-email" v-model="form.email" placeholder="Email" />
          <Label for="edit-old">Current Password</Label>
          <Input
            id="edit-old"
            v-model="form.oldPassword"
            type="password"
            placeholder="Current Password"
          />
          <Label for="edit-new">New Password</Label>
          <Input id="edit-new" v-model="form.password" type="password" placeholder="New Password" />
          <Label for="edit-confirm">Confirm Password</Label>
          <Input
            id="edit-confirm"
            v-model="form.confirmPassword"
            type="password"
            placeholder="Confirm Password"
          />
          <DialogFooter>
            <Button type="button" variant="outline" @click="editingInfo = false">Cancel</Button>
            <Button type="submit">Submit</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <Dialog
      :open="editingAvatar"
      @update:open="
        (open) => {
          if (!savingAvatar) editingAvatar = open
        }
      "
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Edit Avatar</DialogTitle>
          <DialogDescription>Choose a new profile picture.</DialogDescription>
        </DialogHeader>
        <AvatarCropper
          v-if="editingAvatar"
          :src="avatarSrc"
          :saving="savingAvatar"
          @submit="saveAvatar"
          @cancel="editingAvatar = false"
        />
        <p v-if="avatarError" role="alert" class="text-sm text-destructive">{{ avatarError }}</p>
      </DialogContent>
    </Dialog>

    <Dialog :open="!!apiDialog" @update:open="apiDialog = null">
      <DialogContent v-if="apiDialog">
        <DialogHeader>
          <DialogTitle>{{ apiDialog.credId === -1 ? 'Add API' : 'Edit API' }}</DialogTitle>
          <DialogDescription>
            {{
              apiDialog.credId === -1
                ? 'Choose what the new token may do.'
                : `ID: ${apiDialog.credId}`
            }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-wrap gap-4">
          <Label v-for="p in permissions" :key="p.value">
            <Checkbox
              :model-value="apiDialog.perms.includes(p.value)"
              @update:model-value="togglePerm(p.value, $event)"
            />
            {{ p.label }}
          </Label>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="apiDialog = null">Cancel</Button>
          <Button @click="saveApi">{{ apiDialog.credId === -1 ? 'Create' : 'Update' }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog :open="deletingCredId !== undefined" @update:open="deletingCredId = undefined">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Confirm to Delete</AlertDialogTitle>
          <AlertDialogDescription
            >Do you really want to DELETE this API? *The process cannot be
            reversed</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button variant="destructive" @click="deleteApi">Delete</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
