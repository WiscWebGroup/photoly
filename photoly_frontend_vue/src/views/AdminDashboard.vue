<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { isAxiosError } from 'axios'
import { toast } from 'vue-sonner'
import { Database, FolderCog, HardDrive, Power, RefreshCw, Server } from '@lucide/vue'
import AdminLayout from '@/components/AdminLayout.vue'
import PageHeader from '@/components/PageHeader.vue'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { assertAdmin, useAdminGuard } from '@/composables/useAdminGuard'
import { call } from '@/lib/http'

interface Addresses {
  redis: string
  upload: string
  config: string
}
interface Settings {
  OnlineDownload: string
  SignUp: string
  SSafeUUID: string
  TokenDuration: number | string
}

const { guarded } = useAdminGuard()

const databaseStatus = ref(0)
const redisStatus = ref(0)
const backendStatus = ref(0)
const loaded = ref(false)
const checkedAt = ref<Date>()
const services = computed(() => [
  {
    name: 'Backend',
    icon: Server,
    up: backendStatus.value === 100,
    detail: backendAddr,
    extra: '',
  },
  {
    name: 'PostgreSQL',
    icon: Database,
    up: databaseStatus.value === 100,
    detail: database.value?.address ?? '',
    extra: database.value
      ? `Version ${database.value.version} · pgvector ${database.value.pgvectorVersion ?? 'unavailable'} · schema ${database.value.schemaRevision}`
      : '',
  },
  {
    name: 'Redis',
    icon: HardDrive,
    up: redisStatus.value === 100,
    detail: addresses.value.redis ?? '',
    extra: '',
  },
])
const down = computed(() => services.value.filter((s) => !s.up).map((s) => s.name))
const checkedLabel = computed(() =>
  checkedAt.value
    ? checkedAt.value.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
    : '',
)

const addresses = ref<Partial<Addresses>>({})
const database = ref<{
  type: string
  version: string
  address: string
  pgvectorVersion: string | null
  schemaRevision: string
} | null>(null)
const loadError = ref('')
const backendAddr = import.meta.env.VITE_APP_BASE_URL

const onlineDownload = ref(false)
const onlineSaving = ref(false)
async function setOnlineDownload(on: boolean) {
  onlineSaving.value = true
  try {
    await call({
      method: 'post',
      url: '/admin/setGeneralSetting',
      params: { name: 'OnlineDownload', val: on ? 1 : 0 },
    })
    onlineDownload.value = on
    window.dispatchEvent(new Event('focus'))
    toast.success('Setting Changed!')
  } catch {
    toast.error('Could not update Online Download')
  } finally {
    onlineSaving.value = false
  }
}
const userRegistry = ref(false)
const ssafeUuid = ref(false)
const tokenDuration = ref('')
const tokenDurations = [1, 3, 5, 7, 14, 30, 60, 90, 120]

const shuttingDown = ref(false)
const shutdownOpen = ref(false)

const ping = (url: string) =>
  call({ url }).then(
    (value) => (value === 1 ? 100 : 0),
    () => 0,
  )

async function change(url: string, params: Record<string, number>) {
  try {
    await call({ method: 'post', url: `/admin/${url}`, params })
    toast.success('Setting Changed!')
  } catch {
    toast.error('Error!')
  }
}

function setRegistry(on: boolean) {
  userRegistry.value = on
  change('setSignUpPermission', { permission: on ? 1 : 0 })
}

function setSsafeUuid(on: boolean) {
  ssafeUuid.value = on
  change('setSSafeUUIDMode', { mode: on ? 1 : 0 })
}

function setTokenDuration(days: string) {
  tokenDuration.value = days
  change('setTokenDuration', { days: Number(days) })
}

async function powerDown() {
  shuttingDown.value = true
  try {
    await call({ method: 'post', url: '/admin/stop' })
  } catch (e) {
    // The backend dies mid-request, so a network error is the expected success signal.
    if (isAxiosError(e) && e.code === 'ERR_NETWORK') {
      backendStatus.value = 0
      toast.success('Backend stopped!')
    } else {
      toast.error('Error!')
    }
  } finally {
    shuttingDown.value = false
    shutdownOpen.value = false
  }
}

async function loadDashboard() {
  loadError.value = ''
  await guarded(async () => {
    try {
      const [, addrs, settings, info] = await Promise.all([
        assertAdmin(),
        call<Addresses>({ url: '/admin/getAddress' }),
        call<Settings>({ url: '/admin/getSettings' }),
        call<typeof database.value>({ url: '/admin/databaseInfo' }),
        ping('/admin/pingDatabase').then((v) => (databaseStatus.value = v)),
        ping('/admin/pingRedis').then((v) => (redisStatus.value = v)),
      ])
      backendStatus.value = 100
      addresses.value = addrs
      database.value = info
      onlineDownload.value = settings.OnlineDownload === '1'
      userRegistry.value = settings.SignUp === '1'
      ssafeUuid.value = settings.SSafeUUID === '1'
      tokenDuration.value = String(settings.TokenDuration)
      checkedAt.value = new Date()
      loaded.value = true
    } catch (e) {
      loadError.value = 'Could not load dashboard. Please retry.'
      throw e
    }
  })
}
onMounted(loadDashboard)
</script>

<template>
  <AdminLayout>
    <PageHeader title="Server Status">
      <template #description>
        <span v-if="loaded" class="flex items-center gap-2">
          <span
            :class="['size-2 rounded-full', down.length ? 'bg-destructive' : 'bg-green-600']"
            aria-hidden="true"
          />
          {{ down.length ? `${down.join(' and ')} not responding` : 'All services running' }}
          · Checked at {{ checkedLabel }}
        </span>
        <span v-else>Checking services…</span>
      </template>
      <Button variant="outline" @click="loadDashboard"><RefreshCw /> Refresh</Button>
    </PageHeader>

    <p
      v-if="loadError"
      role="alert"
      class="border-destructive/30 bg-destructive/10 text-destructive flex items-center gap-3 rounded-xl border px-4 py-3 text-sm"
    >
      {{ loadError }}
      <Button variant="outline" size="sm" class="ml-auto" @click="loadDashboard">Retry</Button>
    </p>

    <ul class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" aria-label="Services">
      <li
        v-for="(s, i) in services"
        :key="s.name"
        class="bg-card animate-rise flex flex-col gap-3 rounded-xl border p-5 shadow-xs"
        :style="{ animationDelay: `${i * 60}ms` }"
      >
        <div class="flex items-center gap-2">
          <component :is="s.icon" class="text-muted-foreground size-4" aria-hidden="true" />
          <span class="font-medium">{{ s.name }}</span>
          <span
            role="status"
            :aria-label="`${s.name} Status: ${loaded ? (s.up ? 'Running' : 'Down') : 'Checking'}`"
            :class="[
              'ml-auto inline-flex items-center gap-1.5 rounded-full px-2 text-xs leading-[22px] font-medium',
              !loaded
                ? 'bg-muted text-muted-foreground'
                : s.up
                  ? 'bg-green-600/10 text-green-700 dark:text-green-400'
                  : 'bg-destructive/10 text-destructive',
            ]"
          >
            <span
              :class="[
                'size-1.5 rounded-full bg-current',
                loaded && s.up && 'animate-pulse [animation-duration:2.5s]',
              ]"
              aria-hidden="true"
            />
            {{ !loaded ? 'Checking' : s.up ? 'Running' : 'Down' }}
          </span>
        </div>
        <div class="font-mono text-sm break-all">{{ s.detail || '—' }}</div>
        <div v-if="s.extra" class="text-muted-foreground text-xs">{{ s.extra }}</div>
      </li>
    </ul>

    <section aria-labelledby="storage-h" class="flex flex-col gap-3">
      <h2 id="storage-h" class="text-base font-semibold">Storage</h2>
      <dl class="bg-card divide-y rounded-xl border shadow-xs">
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-4">
          <dt class="flex w-40 items-center gap-2 font-medium">
            <HardDrive class="text-muted-foreground size-4" aria-hidden="true" />Uploads
          </dt>
          <dd class="text-muted-foreground font-mono text-sm break-all">{{ addresses.upload }}</dd>
        </div>
        <div class="flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-4">
          <dt class="flex w-40 items-center gap-2 font-medium">
            <FolderCog class="text-muted-foreground size-4" aria-hidden="true" />Configuration
          </dt>
          <dd class="text-muted-foreground font-mono text-sm break-all">{{ addresses.config }}</dd>
        </div>
      </dl>
    </section>

    <section aria-labelledby="settings-h" class="flex flex-col gap-3">
      <h2 id="settings-h" class="text-base font-semibold">Settings</h2>
      <div class="bg-card divide-y rounded-xl border shadow-xs">
        <label class="flex items-center gap-4 px-5 py-4"
          ><span class="flex flex-1 flex-col"
            ><span class="font-medium">Online Download</span
            ><span class="text-muted-foreground text-sm"
              >Allow users to import public image links.</span
            ></span
          ><Switch
            aria-label="Online Download"
            :model-value="onlineDownload"
            :disabled="onlineSaving"
            @update:model-value="setOnlineDownload"
        /></label>
        <label class="flex items-center gap-4 px-5 py-4">
          <span class="flex flex-1 flex-col">
            <span class="font-medium">Allow sign-ups</span>
            <span class="text-muted-foreground text-sm"
              >Anyone who reaches this server can create an account.</span
            >
          </span>
          <Switch
            aria-label="Registry"
            :model-value="userRegistry"
            @update:model-value="setRegistry"
          />
        </label>
        <label class="flex items-center gap-4 px-5 py-4">
          <span class="flex flex-1 flex-col">
            <span class="font-medium">S-Safe UUID mode</span>
            <span class="text-muted-foreground text-sm"
              >Share links use unguessable IDs instead of sequential ones.</span
            >
          </span>
          <Switch
            aria-label="SSafe UUID"
            :model-value="ssafeUuid"
            @update:model-value="setSsafeUuid"
          />
        </label>
        <div class="flex flex-wrap items-center gap-4 px-5 py-4">
          <span class="flex flex-1 flex-col">
            <span class="font-medium">Sign-in duration</span>
            <span class="text-muted-foreground text-sm"
              >How long a sign-in lasts before asking again.</span
            >
          </span>
          <Select
            :model-value="tokenDuration"
            @update:model-value="setTokenDuration($event as string)"
          >
            <SelectTrigger class="w-32" aria-label="Token Duration"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem v-for="d in tokenDurations" :key="d" :value="String(d)">
                {{ d }} {{ d === 1 ? 'day' : 'days' }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </section>

    <section aria-labelledby="power-h" class="flex flex-col gap-3">
      <h2 id="power-h" class="text-base font-semibold">Power</h2>
      <div
        class="bg-card border-destructive/40 flex flex-wrap items-center gap-4 rounded-xl border px-5 py-4"
      >
        <span class="flex flex-1 flex-col">
          <span class="font-medium">Shut down the backend</span>
          <span class="text-muted-foreground text-sm"
            >Everyone is signed out. Restart it from the server.</span
          >
        </span>
        <AlertDialog v-model:open="shutdownOpen">
          <AlertDialogTrigger as-child>
            <Button
              variant="outline"
              class="border-destructive/50 text-destructive hover:bg-destructive/10 hover:text-destructive"
              :disabled="shuttingDown"
              ><Power /> Shut Down</Button
            >
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Shut down the backend?</AlertDialogTitle>
              <AlertDialogDescription
                >Photoly stops for everyone until someone restarts it on the
                server.</AlertDialogDescription
              >
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <Button variant="destructive" :disabled="shuttingDown" @click="powerDown"
                >Shutdown</Button
              >
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </section>
  </AdminLayout>
</template>
