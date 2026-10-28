<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { toast } from 'vue-sonner'
import { RefreshCw } from '@lucide/vue'
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
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useAdminGuard } from '@/composables/useAdminGuard'
import { ApiError, call } from '@/lib/http'

/** One setting of a storage driver; the form is rendered from these, so new drivers need no UI. */
interface Field {
  name: string
  label: string
  kind: 'text' | 'bool'
  secret?: boolean
  required?: boolean
  default?: string
  help?: string
}
interface Driver {
  type: string
  label: string
  fields: Field[]
}
interface StorageState {
  active: string
  driver: string
  values: Record<string, string>
  hasSecret: boolean
  /** Last saved settings per driver, so switching the type back restores them. */
  saved?: Record<string, { values: Record<string, string>; hasSecret: boolean }>
  drivers: Driver[]
  canSave: boolean
}

const { guarded } = useAdminGuard()
const state = ref<StorageState | null>(null)
const driver = ref('local')
const values = ref<Record<string, string>>({})
const error = ref('')
const busy = ref<'' | 'test' | 'save'>('')
const confirming = ref(false)

const activeLabel = computed(
  () => state.value?.drivers.find((d) => d.type === state.value?.active)?.label ?? '',
)
const current = computed(() => state.value?.drivers.find((d) => d.type === driver.value))
const keepsSecret = computed(() => !!state.value?.saved?.[driver.value]?.hasSecret)
const needsKey = computed(
  () => !!current.value?.fields.some((f) => f.secret) && state.value?.canSave === false,
)

function defaults(d: Driver | undefined) {
  return Object.fromEntries((d?.fields ?? []).map((f) => [f.name, f.default ?? '']))
}

function pick(type: string) {
  driver.value = type
  const base = defaults(state.value?.drivers.find((d) => d.type === type))
  const remembered =
    state.value?.saved?.[type]?.values ?? (type === state.value?.driver ? state.value.values : {})
  values.value = { ...base, ...remembered }
  error.value = ''
}

async function load() {
  error.value = ''
  const s = await guarded(() => call<StorageState>({ url: '/admin/storage' }))
  if (!s) return
  state.value = s
  pick(s.driver)
}

const messages: Record<string, string> = {
  storage_secret_key_missing:
    'Set STORAGE_SECRET_KEY on the server to store credentials, then restart it.',
  unknown_driver: 'That storage type is not available.',
}

function describe(e: unknown) {
  if (e instanceof ApiError && typeof e.detail === 'string') return messages[e.detail] ?? e.detail
  return e instanceof Error ? e.message : 'Request failed'
}

function missingField() {
  const f = current.value?.fields.find(
    (f) =>
      f.required &&
      f.kind !== 'bool' &&
      !values.value[f.name]?.trim() &&
      !(f.secret && keepsSecret.value),
  )
  return f ? `${f.label} is required.` : ''
}

const payload = () => ({ driver: driver.value, values: values.value })

async function run(kind: 'test' | 'save', confirm = false) {
  confirming.value = false
  error.value = missingField()
  if (error.value) return
  busy.value = kind
  try {
    if (kind === 'test') {
      await call({ method: 'post', url: '/admin/storage/test', data: payload() })
      toast.success('Connection works')
    } else {
      await call({ method: 'put', url: '/admin/storage', data: { ...payload(), confirm } })
      toast.success('Storage settings saved')
      await load()
    }
  } catch (e) {
    // The server asks for an acknowledgement when the save changes which backend is shown.
    if (e instanceof ApiError && e.detail === 'storage_switch_confirm') confirming.value = true
    else error.value = describe(e)
  } finally {
    busy.value = ''
  }
}

onMounted(load)
</script>

<template>
  <AdminLayout>
    <PageHeader
      title="Storage"
      eyebrow="Admin"
      description="Where Photoly keeps photos and videos."
    >
      <Button variant="outline" @click="load"><RefreshCw /> Refresh</Button>
    </PageHeader>

    <p
      v-if="error"
      role="alert"
      class="border-destructive/30 bg-destructive/10 text-destructive rounded-xl border px-4 py-3 text-sm"
    >
      {{ error }}
    </p>
    <p v-if="!state" role="status" class="text-muted-foreground text-sm">Loading…</p>

    <section
      v-else
      aria-label="Storage backend"
      class="bg-card space-y-5 rounded-xl border p-5 shadow-xs"
    >
      <p class="text-muted-foreground text-sm">
        Currently using
        <strong class="text-foreground">{{
          state.drivers.find((d) => d.type === state?.active)?.label ?? state.active
        }}</strong
        >.
      </p>

      <div class="grid gap-1.5">
        <Label for="storage-driver">Storage type</Label>
        <select
          id="storage-driver"
          :value="driver"
          class="border-input bg-card focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full rounded-md border px-3 text-sm shadow-xs outline-none focus-visible:ring-3"
          @change="pick(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="d in state.drivers" :key="d.type" :value="d.type">{{ d.label }}</option>
        </select>
      </div>

      <p v-if="needsKey" role="note" class="text-sm text-amber-700 dark:text-amber-400">
        Credentials are stored encrypted. Set <code>STORAGE_SECRET_KEY</code> on the server and
        restart it before saving.
      </p>

      <div v-for="f in current?.fields" :key="f.name" class="grid gap-1.5">
        <template v-if="f.kind === 'bool'">
          <label class="flex items-center gap-3">
            <Switch
              :aria-label="f.label"
              :model-value="values[f.name] === 'true'"
              @update:model-value="values[f.name] = $event ? 'true' : 'false'"
            />
            <span class="text-sm font-medium">{{ f.label }}</span>
          </label>
        </template>
        <template v-else>
          <Label :for="`storage-${f.name}`">{{ f.label }}</Label>
          <Input
            :id="`storage-${f.name}`"
            v-model="values[f.name]"
            :type="f.secret ? 'password' : 'text'"
            :placeholder="f.secret && keepsSecret ? 'Saved — leave blank to keep' : ''"
            autocomplete="off"
          />
        </template>
        <p v-if="f.help" class="text-muted-foreground text-xs">{{ f.help }}</p>
      </div>

      <p class="text-muted-foreground text-xs">
        Each photo stays in the storage it was uploaded to. Photoly only shows media from the
        storage selected here; switch back at any time to see the rest. Nothing is moved or deleted.
      </p>

      <div class="flex flex-wrap gap-3">
        <Button
          v-if="current?.fields.length"
          variant="outline"
          :disabled="!!busy"
          @click="run('test')"
        >
          {{ busy === 'test' ? 'Testing…' : 'Test connection' }}
        </Button>
        <Button :disabled="!!busy || needsKey" @click="run('save')">
          {{ busy === 'save' ? 'Saving…' : 'Save' }}
        </Button>
      </div>
    </section>

    <AlertDialog :open="confirming" @update:open="confirming = $event">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Switch storage?</AlertDialogTitle>
          <AlertDialogDescription>
            New uploads will be saved to {{ current?.label }}. By default, users will only see media
            stored there, and media in {{ activeLabel }} will be hidden. Users can turn on "Show
            media from other storage" in their settings to see it. Nothing is deleted, and switching
            back shows it again.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <Button @click="run('save', true)">Switch storage</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </AdminLayout>
</template>
