<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useColorMode, useMediaQuery } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import {
  ChevronLeft,
  ChevronRight,
  Copy,
  Cpu,
  HardDrive,
  Key,
  KeyRound,
  LogOut,
  Pencil,
  Plus,
  Shield,
  Sparkles,
  SunMoon,
  Trash2,
} from '@lucide/vue'
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
import { Button } from '@/components/ui/button'
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
import SmartOrganizationSettings from '@/components/SmartOrganizationSettings.vue'
import StorageSourcesSetting from '@/components/StorageSourcesSetting.vue'
import AiDeviceSettings from '@/components/ai/AiDeviceSettings.vue'
import { useBrowserAiStore } from '@/stores/browserAi'
import AvatarCropper from '@/components/AvatarCropper.vue'
import EmptyState from '@/components/EmptyState.vue'
import PageHeader from '@/components/PageHeader.vue'
import { useProfile } from '@/composables/useProfile'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

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

const browserAi = useBrowserAiStore()
// AI placement settings only exist on servers that offer browser inference.
const hasAiSettings = computed(() => !!browserAi.capabilities?.placement)
const allSections = [
  { id: 'profile', label: 'Profile' },
  { id: 'password', label: 'Password' },
  { id: 'smart', label: 'Smart Tag' },
  { id: 'storage', label: 'Storage' },
  { id: 'ai', label: 'AI on this device' },
  { id: 'api', label: 'API tokens' },
]
const sections = computed(() => allSections.filter((s) => s.id !== 'ai' || hasAiSettings.value))
const activeSection = ref('profile')
function jump(id: string) {
  activeSection.value = id
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// Phones get a settings list instead; each row opens one section, kept in the URL hash so the
// system back gesture returns to the list.
const phone = useMediaQuery('(max-width: 767px)')
const route = useRoute()
const phoneSection = computed(() => {
  const id = route.hash.slice(1)
  return sections.value.some((s) => s.id === id) ? id : null
})
const showSection = (id: string) => !phone.value || phoneSection.value === id
const openSection = (id: string) => router.push({ hash: `#${id}` })
function backToList() {
  const back = (window.history.state as { back?: string } | null)?.back
  if (back === route.path) router.back()
  else router.replace({ hash: '' })
}
const themeLabels = { auto: 'System', light: 'Light', dark: 'Dark' }

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

const { profile: user, reload: loadUser } = useProfile()
const credentials = ref<Credential[]>([])
const avatarSrc = computed(() => auth.avatarUrl)
const mode = useColorMode()
const loadCredentials = async () =>
  (credentials.value = await call<Credential[]>({ url: '/cred/query' }))
const createdOn = computed(() => {
  const [y, m, d] = (user.value?.createDate ?? '').slice(0, 10).split('-').map(Number)
  return y && m && d
    ? new Date(y, m - 1, d).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'long',
        day: 'numeric',
      })
    : ''
})

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

// --- Profile and password -------------------------------------------------------------------------
const form = ref({ userName: '', email: '' })
const passwords = ref({ oldPassword: '', password: '', confirmPassword: '' })
const savingProfile = ref(false)
const savingPassword = ref(false)
watch(user, (u) => (form.value = { userName: u?.userName ?? '', email: u?.email ?? '' }), {
  immediate: true,
})
const profileChanged = computed(
  () => form.value.userName !== user.value?.userName || form.value.email !== user.value?.email,
)
const passwordMismatch = computed(
  () =>
    !!passwords.value.confirmPassword &&
    passwords.value.password !== passwords.value.confirmPassword,
)

/** Runs each update, reporting each outcome on its own. */
function runUpdates(updates: [string, string, object][]) {
  return Promise.all(
    updates.map(([name, url, params]) =>
      call({ method: 'post', url, params }).then(
        () => toast.success(`${name} updated!`),
        () => toast.error(`${name} update Response Error!`),
      ),
    ),
  )
}

async function saveProfile() {
  const f = form.value
  if (!f.userName && !f.email) return toast.warning('Nothing to update about!')
  if (!/.+@.+\..+/.test(f.email)) return toast.warning('Wrong Email Format!')
  const updates: [string, string, object][] = []
  if (f.userName && f.userName !== user.value?.userName)
    updates.push(['Username', '/user/updateUsername', { username: f.userName }])
  if (f.email !== user.value?.email)
    updates.push(['Email', '/user/updateEmail', { email: f.email }])
  if (!updates.length) return toast.warning('Nothing to update about!')
  savingProfile.value = true
  await runUpdates(updates)
  await loadUser().catch(() => toast.error('User Info Response Error!'))
  savingProfile.value = false
}

async function savePassword() {
  const f = passwords.value
  if (!f.password) return toast.warning('Nothing to update about!')
  if (f.password !== f.confirmPassword) return toast.warning('Passwords are not equal!')
  savingPassword.value = true
  await runUpdates([
    ['Password', '/user/updatePassword', { oldPass: f.oldPassword, newPass: f.password }],
  ])
  passwords.value = { oldPassword: '', password: '', confirmPassword: '' }
  savingPassword.value = false
}

async function signOut() {
  try {
    await auth.signOut()
  } catch {
    toast.error('Could not reach the server; you were signed out on this device only.')
  }
  router.push('/')
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
  void browserAi.refreshCapabilities()
  loadUser().catch(() => toast.error('User Info Response Error!'))
  loadCredentials().catch(() => toast.error('Get API Error!'))
})
</script>

<template>
  <div class="flex w-full max-w-[1040px] flex-col gap-6 px-4 pt-5 pb-24 md:px-10 md:pt-10">
    <!-- Phones: the settings list. -->
    <template v-if="phone && !phoneSection">
      <h1 class="text-2xl leading-8 font-semibold tracking-[-0.01em]">Account</h1>
      <button
        type="button"
        class="bg-card flex items-center gap-3.5 rounded-[14px] border p-4 text-left"
        @click="openSection('profile')"
      >
        <Avatar class="size-14"><AvatarImage :src="avatarSrc" alt="" /></Avatar>
        <span class="flex min-w-0 flex-1 flex-col">
          <span class="truncate text-base font-semibold">{{ user?.userName }}</span>
          <span class="text-muted-foreground truncate text-sm">{{ user?.email }}</span>
        </span>
        <span class="text-primary text-sm font-medium">Edit</span>
      </button>
      <section aria-labelledby="acc-settings-h" class="flex flex-col gap-2">
        <h2
          id="acc-settings-h"
          class="text-muted-foreground px-1 text-xs font-semibold tracking-[0.025em] uppercase"
        >
          Settings
        </h2>
        <ul class="bg-card divide-border/60 divide-y overflow-hidden rounded-[14px] border text-sm">
          <li
            v-for="row in [
              { id: 'password', label: 'Password', icon: KeyRound, meta: '' },
              { id: 'smart', label: 'Smart Tag', icon: Sparkles, meta: '' },
              { id: 'storage', label: 'Storage', icon: HardDrive, meta: '' },
              ...(hasAiSettings
                ? [{ id: 'ai', label: 'AI on this device', icon: Cpu, meta: '' }]
                : []),
              { id: 'api', label: 'API tokens', icon: Key, meta: String(credentials.length || '') },
            ]"
            :key="row.id"
          >
            <button
              type="button"
              class="active:bg-accent flex h-13 w-full items-center gap-3 px-3.5 text-left"
              @click="openSection(row.id)"
            >
              <component
                :is="row.icon"
                class="text-muted-foreground size-[18px]"
                aria-hidden="true"
              />
              <span class="flex-1">{{ row.label }}</span>
              <span class="text-muted-foreground">{{ row.meta }}</span>
              <ChevronRight class="text-muted-foreground size-4" aria-hidden="true" />
            </button>
          </li>
          <li class="flex h-13 items-center gap-3 px-3.5">
            <SunMoon class="text-muted-foreground size-[18px]" aria-hidden="true" />
            <label for="acc-theme" class="flex-1">Appearance</label>
            <select
              id="acc-theme"
              v-model="mode"
              class="text-muted-foreground h-8 bg-transparent text-right"
            >
              <option v-for="(label, value) in themeLabels" :key="value" :value="value">
                {{ label }}
              </option>
            </select>
          </li>
          <li v-if="user?.role === 'admin'">
            <button
              type="button"
              class="active:bg-accent flex h-13 w-full items-center gap-3 px-3.5 text-left"
              @click="goToAdminPanel"
            >
              <Shield class="text-muted-foreground size-[18px]" aria-hidden="true" />
              <span class="flex-1">Admin Panel</span>
              <span class="text-muted-foreground text-xs">Best on desktop</span>
              <ChevronRight class="text-muted-foreground size-4" aria-hidden="true" />
            </button>
          </li>
        </ul>
      </section>
      <button
        type="button"
        class="bg-card text-destructive flex h-12 items-center justify-center gap-2 rounded-[14px] border text-sm font-medium"
        @click="signOut"
      >
        <LogOut class="size-4" aria-hidden="true" /> Sign Out
      </button>
    </template>
    <button
      v-else-if="phone"
      type="button"
      class="text-muted-foreground -mb-3 -ml-1 flex h-8 items-center gap-0.5 self-start text-sm"
      @click="backToList"
    >
      <ChevronLeft class="size-4" aria-hidden="true" /> Account
    </button>

    <PageHeader v-if="!phone" title="Account" description="Your profile, password and API access.">
      <Button v-if="user?.role === 'admin'" variant="outline" @click="goToAdminPanel"
        ><Shield /> Admin Panel</Button
      >
    </PageHeader>

    <nav
      v-if="!phone"
      aria-label="Account sections"
      class="bg-background/95 sticky top-0 z-10 -mx-4 flex gap-6 overflow-x-auto border-b px-4 backdrop-blur [scrollbar-width:none] md:mx-0 md:px-0"
    >
      <a
        v-for="sec in sections"
        :key="sec.id"
        :href="`#${sec.id}`"
        :aria-current="activeSection === sec.id ? 'true' : undefined"
        class="text-muted-foreground hover:text-foreground aria-[current]:border-foreground aria-[current]:text-foreground -mb-px flex h-10 shrink-0 items-center border-b-2 border-transparent text-sm font-medium transition-colors"
        @click.prevent="jump(sec.id)"
        >{{ sec.label }}</a
      >
    </nav>

    <section
      v-show="showSection('profile')"
      id="profile"
      aria-labelledby="profile-h"
      class="grid scroll-mt-14 gap-x-12 gap-y-5 border-b pb-8 max-md:border-b-0 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
    >
      <div>
        <h2
          id="profile-h"
          class="mb-1 text-base font-semibold max-md:text-2xl max-md:leading-8 max-md:tracking-[-0.01em]"
        >
          Profile
        </h2>
        <p class="text-muted-foreground text-sm">
          <template v-if="createdOn">Member since {{ createdOn }}.</template>
        </p>
      </div>
      <form class="flex max-w-[480px] flex-col gap-5" @submit.prevent="saveProfile">
        <div class="flex flex-wrap items-center gap-4">
          <Avatar class="size-16"
            ><AvatarImage :src="avatarSrc" alt="Your profile picture"
          /></Avatar>
          <Button type="button" variant="outline" size="sm" @click="editingAvatar = true"
            >Change Picture</Button
          >
          <span class="text-muted-foreground text-xs">Cropped to a circle</span>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="acc-name">Username</Label>
          <Input id="acc-name" v-model="form.userName" autocomplete="username" />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="acc-email">Email</Label>
          <Input id="acc-email" v-model="form.email" type="email" autocomplete="email" />
        </div>
        <div>
          <Button type="submit" :disabled="!profileChanged || savingProfile">Save Changes</Button>
        </div>
      </form>
    </section>

    <section
      v-show="showSection('password')"
      id="password"
      aria-labelledby="password-h"
      class="grid scroll-mt-14 gap-x-12 gap-y-5 border-b pb-8 max-md:border-b-0 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
    >
      <div>
        <h2
          id="password-h"
          class="mb-1 text-base font-semibold max-md:text-2xl max-md:leading-8 max-md:tracking-[-0.01em]"
        >
          Password
        </h2>
        <p class="text-muted-foreground text-sm">
          You stay signed in on this device after changing it.
        </p>
      </div>
      <form class="grid max-w-[480px] gap-4 sm:grid-cols-2" @submit.prevent="savePassword">
        <div class="flex flex-col gap-1.5 sm:col-span-2">
          <Label for="pw-cur">Current Password</Label>
          <Input
            id="pw-cur"
            v-model="passwords.oldPassword"
            type="password"
            placeholder="Current Password"
            autocomplete="current-password"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="pw-new">New Password</Label>
          <Input
            id="pw-new"
            v-model="passwords.password"
            type="password"
            placeholder="New Password"
            autocomplete="new-password"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="pw-conf">Confirm Password</Label>
          <Input
            id="pw-conf"
            v-model="passwords.confirmPassword"
            type="password"
            placeholder="Confirm Password"
            autocomplete="new-password"
            :aria-invalid="passwordMismatch || undefined"
          />
        </div>
        <Transition name="reveal">
          <p v-if="passwordMismatch" role="alert" class="text-destructive text-xs sm:col-span-2">
            Passwords don't match.
          </p>
        </Transition>
        <div class="sm:col-span-2">
          <Button type="submit" variant="outline" :disabled="!passwords.password || savingPassword"
            >Update Password</Button
          >
        </div>
      </form>
    </section>

    <section
      v-show="showSection('smart')"
      id="smart"
      aria-labelledby="smart-h"
      class="grid scroll-mt-14 gap-x-12 gap-y-5 border-b pb-8 max-md:border-b-0 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
    >
      <div>
        <h2
          id="smart-h"
          class="mb-1 text-base font-semibold max-md:text-2xl max-md:leading-8 max-md:tracking-[-0.01em]"
        >
          Smart Tag
        </h2>
        <p class="text-muted-foreground text-sm">
          Sorts photos and videos into camera and lens tags, and trip galleries from time and place.
        </p>
      </div>
      <SmartOrganizationSettings class="max-w-[560px]" />
    </section>

    <section
      v-show="showSection('storage')"
      id="storage"
      aria-labelledby="storage-h"
      class="grid scroll-mt-14 gap-x-12 gap-y-5 border-b pb-8 max-md:border-b-0 md:grid-cols-[minmax(0,280px)_minmax(0,1fr)]"
    >
      <div>
        <h2
          id="storage-h"
          class="mb-1 text-base font-semibold max-md:text-2xl max-md:leading-8 max-md:tracking-[-0.01em]"
        >
          Storage
        </h2>
        <p class="text-muted-foreground text-sm">
          Manage your local library and personal S3 connection.
        </p>
      </div>
      <StorageSourcesSetting />
    </section>

    <div v-if="hasAiSettings" v-show="showSection('ai')" id="ai" class="scroll-mt-14">
      <AiDeviceSettings />
    </div>

    <section
      v-show="showSection('api')"
      id="api"
      aria-labelledby="api-h"
      class="flex scroll-mt-14 flex-col gap-4"
    >
      <div class="flex flex-wrap items-end gap-4">
        <div>
          <h2
            id="api-h"
            class="mb-1 text-base font-semibold max-md:text-2xl max-md:leading-8 max-md:tracking-[-0.01em]"
          >
            API tokens
          </h2>
        </div>
        <Button variant="outline" class="ml-auto" aria-label="Add API" @click="openApiDialog(-1)"
          ><Plus /> New Token</Button
        >
      </div>
      <EmptyState
        v-if="!credentials.length"
        title="No API tokens yet"
        class="bg-card rounded-xl border py-10"
      />
      <div v-else class="bg-card overflow-x-auto rounded-xl border shadow-xs">
        <table class="w-full min-w-[600px] border-collapse text-sm">
          <thead>
            <tr class="text-muted-foreground text-left text-xs">
              <th scope="col" class="border-b px-4 py-3 font-medium">Name</th>
              <th scope="col" class="border-b px-4 py-3 font-medium">Token</th>
              <th scope="col" class="border-b px-4 py-3 font-medium">Permissions</th>
              <th scope="col" class="border-b px-4 py-3"><span class="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(api, i) in credentials"
              :key="api.credId"
              class="animate-rise hover:bg-muted/50 transition-colors [&:not(:last-child)>td]:border-b"
              :style="{ animationDelay: `${i * 40}ms` }"
            >
              <td class="px-4 py-3 font-medium whitespace-nowrap">Token #{{ api.credId }}</td>
              <td class="px-4 py-2">
                <span class="inline-flex items-center gap-1.5 font-mono text-xs break-all">
                  {{ api.token }}
                  <button
                    type="button"
                    aria-label="Copy token"
                    class="text-muted-foreground hover:bg-accent hover:text-foreground grid size-7 shrink-0 place-items-center rounded-md transition-colors"
                    @click="copy(api.token)"
                  >
                    <Copy class="size-3.5" aria-hidden="true" />
                  </button>
                </span>
              </td>
              <td class="px-4 py-3">
                <span class="flex gap-1">
                  <span
                    v-for="p in permissions"
                    :key="p.value"
                    :title="`${p.label} ${api.authorization.includes(p.value) ? 'allowed' : 'not allowed'}`"
                    :class="
                      api.authorization.includes(p.value)
                        ? 'bg-accent text-accent-foreground'
                        : 'bg-muted text-muted-foreground line-through'
                    "
                    class="rounded-full px-2 text-xs leading-[22px] font-medium"
                    >{{ p.label }}</span
                  >
                </span>
              </td>
              <td class="px-4 py-2 text-right whitespace-nowrap">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label="Edit API"
                  @click="openApiDialog(api.credId, api.authorization)"
                  ><Pencil
                /></Button>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-destructive hover:text-destructive"
                  aria-label="Delete API"
                  @click="deletingCredId = api.credId"
                  ><Trash2
                /></Button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <details class="group bg-card rounded-xl border">
        <summary
          class="flex cursor-pointer list-none items-center gap-2 px-4 py-3 text-sm font-medium [&::-webkit-details-marker]:hidden"
        >
          <ChevronRight
            class="text-muted-foreground size-4 transition-transform group-open:rotate-90"
            aria-hidden="true"
          />
          API documents
        </summary>
        <div class="space-y-4 border-t px-4 py-4 text-sm">
          <ul class="text-muted-foreground space-y-1">
            <li><span class="text-foreground font-medium">Upload</span> (C): upload photos.</li>
            <li>
              <span class="text-foreground font-medium">Read</span> (R): list folders and photos,
              and render a photo.
            </li>
            <li><span class="text-foreground font-medium">Delete</span> (D): delete a photo.</li>
          </ul>
          <section v-for="d in docs" :key="d.addr" class="space-y-1">
            <h3 class="font-medium">{{ d.title }}</h3>
            <dl class="text-muted-foreground border-l-2 pl-3 text-xs leading-5">
              <div>
                <dt class="inline">Address:</dt>
                <dd class="text-foreground inline font-mono">server_path/{{ d.addr }}</dd>
              </div>
              <div>
                <dt class="inline">Takes:</dt>
                <dd class="inline">{{ d.takein }}</dd>
              </div>
              <div>
                <dt class="inline">Returns:</dt>
                <dd class="inline">{{ d.ret }}</dd>
              </div>
            </dl>
          </section>
        </div>
      </details>
    </section>

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
          <DialogTitle>Change Picture</DialogTitle>
          <DialogDescription class="sr-only">Choose a new profile picture.</DialogDescription>
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
      <DialogContent v-if="apiDialog" class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ apiDialog.credId === -1 ? 'Add API' : 'Edit API' }}</DialogTitle>
          <DialogDescription>
            {{
              apiDialog.credId === -1
                ? 'Choose what the new token may do.'
                : `Token #${apiDialog.credId}`
            }}
          </DialogDescription>
        </DialogHeader>
        <div class="flex flex-col divide-y rounded-xl border">
          <Label
            v-for="p in permissions"
            :key="p.value"
            class="hover:bg-muted/50 flex cursor-pointer items-center gap-3 px-4 py-3 font-normal transition-colors"
          >
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
          <AlertDialogTitle>Delete Token #{{ deletingCredId }}?</AlertDialogTitle>
          <AlertDialogDescription
            >Integrations using it stop working right away. This can't be
            undone.</AlertDialogDescription
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
