<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { isAxiosError } from 'axios'
import { DatabaseBackup, Download, LockKeyhole, RefreshCw, Wrench } from '@lucide/vue'
import AdminLayout from '@/components/AdminLayout.vue'
import PageHeader from '@/components/PageHeader.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
import { call, ApiError } from '@/lib/http'
import { useAdminGuard } from '@/composables/useAdminGuard'
import { useAuthStore } from '@/stores/auth'
import { useRouter } from 'vue-router'

type Job = {
  id: string
  kind: string
  state: string
  stage: string
  files: number
  bytes: number
  startedAt: string
  finishedAt?: string
  error?: string
}
type Backup = { id: string; createdAt: string; size: number; complete: boolean }
type Status = {
  state: { mode: 'off' | 'draining' | 'on'; error?: string; job?: Job }
  problems: string[] | null
  backupDirectory: string
  externalSources?: string[]
  rollbacks: { id: string; bytes: number; databaseBytes?: number; createdAt: string }[]
}
const { guarded } = useAdminGuard()
const router = useRouter()
const status = ref<Status>()
const backups = ref<Backup[]>([])
const password = ref('')
const repeatPassword = ref('')
const restorePassword = ref('')
const selected = ref<Backup>()
const restoreOpen = ref(false)
const enterOpen = ref(false)
const pending = ref(false)
const error = ref('')
const job = computed(() => status.value?.state.job)
const running = computed(() => job.value?.state === 'running')
const ready = computed(
  () =>
    status.value?.state.mode === 'on' &&
    !running.value &&
    !pending.value &&
    !status.value?.problems?.length,
)
const backupReady = computed(
  () => ready.value && password.value.length >= 12 && password.value === repeatPassword.value,
)
const modeLabel = computed(
  () =>
    ({ off: 'Service running', draining: 'Preparing maintenance', on: 'Maintenance active' })[
      status.value?.state.mode ?? 'off'
    ],
)
const formatBytes = (n: number) =>
  new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(n / 1024 ** 3) + ' GiB'
const date = (s: string) => new Date(s).toLocaleString()
const elapsed = computed(() =>
  job.value
    ? Math.max(
        0,
        Math.floor(
          (new Date(job.value.finishedAt ?? now.value).getTime() -
            new Date(job.value.startedAt).getTime()) /
            1000,
        ),
      ) + ' s'
    : '',
)
const now = ref(new Date().toISOString())
let timer: ReturnType<typeof setInterval> | undefined
let loading = false
async function load() {
  if (loading) return
  loading = true
  try {
    const result = await call<Status>({ url: '/admin/maintenance' })
    status.value = result
    backups.value = (await call<Backup[]>({ url: '/admin/backups' })).sort((a, b) =>
      b.createdAt.localeCompare(a.createdAt),
    )
    error.value = ''
  } catch (e) {
    if (
      (e instanceof ApiError && e.msgCode === 403) ||
      (isAxiosError(e) && e.response?.status === 403)
    ) {
      useAuthStore().clear()
      await router.replace('/login')
      return
    }
    error.value = message(e)
  } finally {
    loading = false
  }
}
function message(e: unknown) {
  if (e instanceof ApiError && typeof e.detail === 'string') return e.detail
  if (e && typeof e === 'object' && 'response' in e) {
    const response = (e as { response?: { data?: { t?: unknown } } }).response
    if (typeof response?.data?.t === 'string') return response.data.t
  }
  return 'The request could not be completed. Please retry.'
}
async function action(url: string, data?: object) {
  pending.value = true
  error.value = ''
  try {
    await call({ method: 'post', url, data })
    password.value = repeatPassword.value = restorePassword.value = ''
    restoreOpen.value = enterOpen.value = false
    await load()
  } catch (e) {
    error.value = message(e)
  } finally {
    pending.value = false
  }
}
function chooseRestore(backup: Backup) {
  selected.value = backup
  restorePassword.value = ''
  restoreOpen.value = true
}
onMounted(async () => {
  await guarded(load)
  timer = setInterval(() => {
    now.value = new Date().toISOString()
    void load()
  }, 2000)
})
onUnmounted(() => {
  clearInterval(timer)
  password.value = repeatPassword.value = restorePassword.value = ''
})
</script>

<template>
  <AdminLayout wide maintenance>
    <PageHeader
      title="Maintenance and Backup"
      description="Pause your instance, protect your local library, and restore a backup."
    >
      <Button variant="outline" :disabled="pending" @click="load"><RefreshCw /> Refresh</Button>
    </PageHeader>
    <p
      v-if="error"
      role="alert"
      class="border-destructive/30 bg-destructive/10 text-destructive rounded-xl border p-4 text-sm"
    >
      {{ error }}
    </p>
    <section
      aria-labelledby="maintenance-heading"
      class="bg-card flex flex-col gap-4 rounded-xl border p-5 shadow-xs"
    >
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 id="maintenance-heading" class="flex items-center gap-2 font-semibold">
          <Wrench class="size-4" /> Maintenance mode
        </h2>
        <span
          role="status"
          class="bg-accent text-accent-foreground rounded-full px-3 py-1 text-xs font-medium"
          >{{ status ? modeLabel : 'Checking…' }}</span
        >
      </div>
      <p class="text-muted-foreground text-sm">
        Maintenance pauses library access, shared links, uploads, and background processing. Only
        administrators can sign in to this page.
      </p>
      <p v-if="status?.state.mode === 'draining'" class="text-muted-foreground text-sm">
        Waiting for active requests and background tasks to stop. Backups become available when this
        completes.
      </p>
      <p v-if="status?.state.error" role="alert" class="text-destructive text-sm">
        {{ status.state.error }}
      </p>
      <div>
        <Button v-if="status?.state.mode === 'off'" :disabled="pending" @click="enterOpen = true"
          >Enter Maintenance</Button
        >
        <Button
          v-else
          variant="outline"
          :disabled="!status || running || pending"
          @click="action('/admin/maintenance/exit')"
          >{{
            status?.state.mode === 'draining' ? 'Return to Service When Ready' : 'Exit Maintenance'
          }}</Button
        >
      </div>
    </section>

    <section
      aria-labelledby="backup-heading"
      class="bg-card flex flex-col gap-4 rounded-xl border p-5 shadow-xs"
    >
      <h2 id="backup-heading" class="flex items-center gap-2 font-semibold">
        <DatabaseBackup class="size-4" /> Create a backup
      </h2>
      <p class="text-muted-foreground text-sm">
        Includes the database, Photoly-managed local originals, thumbnails, previews, and encrypted
        recovery settings. Originals on external mounts, S3, WebDAV, and SMB are not copied; their
        connections are retained for reconfiguration.
      </p>
      <ul
        v-if="status?.externalSources?.length"
        class="text-muted-foreground list-inside list-disc text-sm"
        aria-label="External originals excluded"
      >
        <li v-for="source in status.externalSources" :key="source">{{ source }}</li>
      </ul>
      <p class="text-muted-foreground text-sm">
        Backups are saved to <code>{{ status?.backupDirectory ?? '/backups' }}</code
        >. Keep the password separately: it cannot be recovered.
      </p>
      <ul v-if="status?.problems?.length" class="text-destructive list-inside list-disc text-sm">
        <li v-for="problem in status.problems" :key="problem">{{ problem }}</li>
      </ul>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="flex flex-col gap-2 text-sm font-medium"
          >Backup password<Input
            v-model="password"
            type="password"
            autocomplete="new-password"
            :disabled="!ready"
            placeholder="At least 12 characters"
        /></label>
        <label class="flex flex-col gap-2 text-sm font-medium"
          >Confirm password<Input
            v-model="repeatPassword"
            type="password"
            autocomplete="new-password"
            :disabled="!ready"
        /></label>
      </div>
      <div>
        <Button :disabled="!backupReady" @click="action('/admin/backups', { password })"
          ><LockKeyhole /> Create Backup</Button
        >
      </div>
      <p v-if="status?.state.mode === 'off'" class="text-muted-foreground text-xs">
        Enter maintenance mode to create or restore a backup.
      </p>
    </section>

    <section
      v-if="job"
      aria-labelledby="task-heading"
      class="bg-card flex flex-col gap-3 rounded-xl border p-5 shadow-xs"
      aria-live="polite"
    >
      <h2 id="task-heading" class="font-semibold">{{ running ? 'Current task' : 'Last task' }}</h2>
      <div class="flex flex-wrap items-center gap-3 text-sm">
        <RefreshCw v-if="running" class="size-4 animate-spin" /><span class="font-medium"
          >{{ job.kind === 'restore' ? 'Restore' : 'Backup' }} · {{ job.stage }}</span
        ><span class="text-muted-foreground">{{ elapsed }}</span>
      </div>
      <p class="text-muted-foreground text-sm">
        {{ job.files.toLocaleString() }} files processed · {{ formatBytes(job.bytes) }}
      </p>
      <p v-if="job.error" role="alert" class="text-destructive text-sm">{{ job.error }}</p>
      <p class="text-muted-foreground text-sm">
        {{
          running
            ? 'Another backup or restore cannot start while this task is running.'
            : 'Maintenance stays enabled until an administrator exits it.'
        }}
      </p>
      <p v-if="running && job.kind === 'restore'" class="text-muted-foreground text-sm">
        After restoration, sign in again with an administrator account from the backup.
      </p>
    </section>

    <section aria-labelledby="backups-heading" class="flex flex-col gap-3">
      <h2 id="backups-heading" class="font-semibold">Available backups</h2>
      <p class="text-muted-foreground text-sm">
        To migrate, copy the entire encrypted backup directory into this server’s backups volume,
        then refresh. A restore replaces all current users, database records, and local media.
      </p>
      <div class="bg-card divide-y rounded-xl border shadow-xs">
        <p v-if="!backups.length" class="text-muted-foreground p-5 text-sm">
          No completed backups found.
        </p>
        <div
          v-for="backup in backups"
          :key="backup.id"
          class="flex flex-wrap items-center gap-4 p-5"
        >
          <div class="min-w-0 flex-1">
            <p class="font-medium">{{ date(backup.createdAt) }}</p>
            <p class="text-muted-foreground mt-1 text-xs break-all">
              {{ backup.id }} · {{ formatBytes(backup.size) }}
            </p>
          </div>
          <Button variant="outline" :disabled="!ready" @click="chooseRestore(backup)"
            ><Download /> Restore</Button
          >
        </div>
      </div>
    </section>
    <section
      v-if="status?.rollbacks.length"
      class="bg-card flex flex-col gap-2 rounded-xl border p-5 text-sm shadow-xs"
    >
      <h2 class="font-semibold">Retained recovery data</h2>
      <p class="text-muted-foreground">
        Previous databases and media are retained on the server for rollback. They are not removed
        automatically.
      </p>
      <p v-for="item in status.rollbacks" :key="item.id" class="text-muted-foreground">
        {{ date(item.createdAt) }} · {{ formatBytes(item.bytes) }} previous media ·
        {{ formatBytes(item.databaseBytes ?? 0) }} previous database
      </p>
    </section>

    <AlertDialog v-model:open="enterOpen"
      ><AlertDialogContent
        ><AlertDialogHeader
          ><AlertDialogTitle>Enter maintenance mode?</AlertDialogTitle
          ><AlertDialogDescription
            >Library access and shared links will pause for everyone. Photoly will wait for active
            operations to finish before allowing backups.</AlertDialogDescription
          ></AlertDialogHeader
        ><AlertDialogFooter
          ><AlertDialogCancel>Cancel</AlertDialogCancel
          ><Button :disabled="pending" @click="action('/admin/maintenance/enter')"
            >Enter Maintenance</Button
          ></AlertDialogFooter
        ></AlertDialogContent
      ></AlertDialog
    >
    <AlertDialog v-model:open="restoreOpen"
      ><AlertDialogContent
        ><AlertDialogHeader
          ><AlertDialogTitle>Replace this instance from backup?</AlertDialogTitle
          ><AlertDialogDescription v-if="selected"
            >Restore {{ date(selected.createdAt) }}. This replaces all users, database records, and
            Photoly-managed local media. External originals are not included. Current data is
            retained for rollback. You will need to sign in using an administrator account from the
            backup.</AlertDialogDescription
          ></AlertDialogHeader
        ><label class="flex flex-col gap-2 text-sm font-medium"
          >Backup password<Input
            v-model="restorePassword"
            type="password"
            autocomplete="off" /></label
        ><AlertDialogFooter
          ><AlertDialogCancel :disabled="pending">Cancel</AlertDialogCancel
          ><Button
            variant="destructive"
            :disabled="!restorePassword || !ready"
            @click="
              action(`/admin/backups/${selected?.id}/restore`, {
                password: restorePassword,
                confirm: true,
              })
            "
            >Replace and Restore</Button
          ></AlertDialogFooter
        ></AlertDialogContent
      ></AlertDialog
    >
  </AdminLayout>
</template>
