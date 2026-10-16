<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { isAxiosError } from 'axios'
import { toast } from 'vue-sonner'
import { Settings, Upload, Database, Server } from '@lucide/vue'
import AdminLayout from '@/components/AdminLayout.vue'
import CircleProgress from '@/components/CircleProgress.vue'
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
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import redisLogo from '@/assets/Redis.png'
import { assertAdmin, useAdminGuard } from '@/composables/useAdminGuard'
import { call } from '@/lib/http'

interface Addresses {
  redis: string
  upload: string
  config: string
}
interface Settings {
  SignUp: string
  SSafeUUID: string
  TokenDuration: number | string
}

const { guarded } = useAdminGuard()

const databaseStatus = ref(0)
const redisStatus = ref(0)
const backendStatus = ref(0)
const totalStatus = computed(
  () => (databaseStatus.value + redisStatus.value + backendStatus.value) / 3,
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
      userRegistry.value = settings.SignUp === '1'
      ssafeUuid.value = settings.SSafeUUID === '1'
      tokenDuration.value = String(settings.TokenDuration)
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
    <p v-if="loadError" role="alert">
      {{ loadError }} <Button variant="outline" @click="loadDashboard">Retry</Button>
    </p>
    <router-link
      class="rounded-lg border bg-card p-4 font-medium hover:bg-accent"
      to="/thepersonincharge/ai"
      >AI Management → Models, search availability and scheduling limits</router-link
    >
    <Card>
      <CardHeader><CardTitle class="text-2xl">Server Status</CardTitle></CardHeader>
      <CardContent class="flex flex-wrap justify-around gap-6">
        <CircleProgress label="Total Status" :value="totalStatus" />
        <CircleProgress
          label="PostgreSQL Status"
          :value="databaseStatus"
          :ok="databaseStatus === 100"
        />
        <CircleProgress label="Redis Status" :value="redisStatus" :ok="redisStatus === 100" />
        <CircleProgress label="Backend Status" :value="backendStatus" :ok="backendStatus === 100" />
      </CardContent>
    </Card>

    <Card>
      <CardHeader><CardTitle class="text-2xl">Locations</CardTitle></CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Icon</TableHead>
              <TableHead>Name</TableHead>
              <TableHead>Address</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell><Database class="size-10" aria-hidden="true" /></TableCell>
              <TableCell
                >PostgreSQL {{ database?.version }}
                <p class="text-xs">
                  pgvector {{ database?.pgvectorVersion ?? 'Unavailable' }} · Schema
                  {{ database?.schemaRevision }}
                </p></TableCell
              >
              <TableCell>{{ database?.address }}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell
                ><img :src="redisLogo" alt="" class="h-12 w-16 rounded-md object-contain"
              /></TableCell>
              <TableCell>Redis</TableCell>
              <TableCell>{{ addresses.redis }}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell><Server class="size-10" aria-hidden="true" /></TableCell>
              <TableCell>Backend</TableCell>
              <TableCell>{{ backendAddr }}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell><Upload class="size-10" aria-hidden="true" /></TableCell>
              <TableCell>Upload</TableCell>
              <TableCell>{{ addresses.upload }}</TableCell>
            </TableRow>
            <TableRow>
              <TableCell><Settings class="size-10" aria-hidden="true" /></TableCell>
              <TableCell>Configuration</TableCell>
              <TableCell>{{ addresses.config }}</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <Card>
      <CardHeader><CardTitle class="text-2xl">Settings</CardTitle></CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Item</TableHead>
              <TableHead>Description</TableHead>
              <TableHead>Action</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell>Registry</TableCell>
              <TableCell>Allow user to register</TableCell>
              <TableCell
                ><Switch
                  aria-label="Registry"
                  :model-value="userRegistry"
                  @update:model-value="setRegistry"
              /></TableCell>
            </TableRow>
            <TableRow>
              <TableCell>SSafe UUID</TableCell>
              <TableCell>Enable the S-Safe UUID Mode</TableCell>
              <TableCell
                ><Switch
                  aria-label="SSafe UUID"
                  :model-value="ssafeUuid"
                  @update:model-value="setSsafeUuid"
              /></TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Token Duration</TableCell>
              <TableCell>How long will the login token expire</TableCell>
              <TableCell>
                <Select
                  :model-value="tokenDuration"
                  @update:model-value="setTokenDuration($event as string)"
                >
                  <SelectTrigger class="w-32" aria-label="Token Duration"
                    ><SelectValue
                  /></SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="d in tokenDurations" :key="d" :value="String(d)">
                      {{ d }} {{ d === 1 ? 'Day' : 'Days' }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Power</TableCell>
              <TableCell>Power down the backend server</TableCell>
              <TableCell>
                <AlertDialog v-model:open="shutdownOpen">
                  <AlertDialogTrigger as-child>
                    <Button variant="destructive" :disabled="shuttingDown">Shut Down</Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Confirm to Shut Down</AlertDialogTitle>
                      <AlertDialogDescription
                        >Do you really want to Shutdown the Backend?</AlertDialogDescription
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
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  </AdminLayout>
</template>
