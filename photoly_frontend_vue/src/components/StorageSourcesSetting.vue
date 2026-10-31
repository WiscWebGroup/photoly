<script setup lang="ts">
import { onMounted, ref } from 'vue'
import StorageScanProgress from './StorageScanProgress.vue'
import { useStorageRevision } from '@/composables/useStorageRevision'
import ExternalStorageSettings from './ExternalStorageSettings.vue'
import { Cloud, RefreshCw } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ApiError, call } from '@/lib/http'

interface Field {
  name: string
  label: string
  kind: string
  secret?: boolean
  required?: boolean
  default?: string
  help?: string
}
interface Settings {
  configured: boolean
  enabled: boolean
  values: Record<string, string>
  hasSecret: boolean
  fields: Field[]
  canSave: boolean
}
const settings = ref<Settings>()
const values = ref<Record<string, string | boolean>>({})
const enabled = ref(false)
const busy = ref(false)
const { sources, poll } = useStorageRevision(
  () => {},
  () => false,
)
const failure = ref('')
async function load() {
  try {
    const [state] = await Promise.all([call<Settings>({ url: '/user/storage/s3' }), poll()])
    settings.value = state
    enabled.value = state.enabled
    values.value = Object.fromEntries(
      state.fields.map((f) => [
        f.name,
        f.kind === 'bool'
          ? (state.values[f.name] ?? f.default) === 'true'
          : (state.values[f.name] ?? ''),
      ]),
    )
    failure.value = ''
  } catch {
    failure.value = 'Could not load storage settings.'
  }
}
async function submit(test: boolean) {
  busy.value = true
  try {
    await call({
      method: test ? 'post' : 'put',
      url: `/user/storage/s3${test ? '/test' : ''}`,
      data: { values: values.value, enabled: enabled.value },
    })
    toast.success(test ? 'S3 connection verified' : 'Storage settings saved')
    if (!test) await load()
  } catch (error) {
    toast.error(
      error instanceof ApiError && typeof error.detail === 'string'
        ? error.detail
        : 'Could not save or connect to S3.',
    )
  } finally {
    busy.value = false
  }
}
async function scanLocal(id: number) {
  try {
    await call({ url: `/user/storage/sources/${id}/scan`, method: 'post', data: {} })
    await poll()
  } catch {
    toast.error('Could not request scan')
  }
}
async function toggleS3(value: boolean) {
  const previous = enabled.value
  enabled.value = value
  if (!settings.value?.configured) return
  busy.value = true
  try {
    await call({ method: 'put', url: '/user/storage/s3', data: { enabled: value } })
    settings.value.enabled = value
    await poll()
  } catch {
    enabled.value = previous
    toast.error('Could not change S3 status. Your saved configuration was kept.')
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>

<template>
  <section class="flex max-w-[560px] flex-col gap-5" aria-label="Personal storage">
    <div>
      <h3 class="font-medium">Local library</h3>
      <p class="text-muted-foreground text-sm">
        Your library folders are stored on disk. External changes are scanned automatically,
        normally every minute.
      </p>
      <template
        v-for="source in sources.filter((s) => s.sourceType === 'local')"
        :key="source.sourceId"
      >
        <p class="mt-2 font-medium">{{ source.name || 'Photoly' }}</p>
        <StorageScanProgress :status="source" />
        <Button
          v-if="source.enabled"
          size="sm"
          variant="outline"
          @click="scanLocal(source.sourceId)"
          >Scan source</Button
        >
      </template>
      <Button class="mt-2" size="sm" variant="outline" @click="load"
        ><RefreshCw class="size-4" /> Refresh status</Button
      >
    </div>
    <p v-if="failure" role="alert" class="text-destructive text-sm">{{ failure }}</p>
    <form
      v-if="settings"
      class="flex flex-col gap-4 rounded-xl border p-4"
      @submit.prevent="submit(false)"
    >
      <h3 class="flex items-center gap-2 font-medium">
        <Cloud class="size-5 text-sky-600" /> Your S3 storage
      </h3>
      <p class="text-muted-foreground text-sm">
        Connect one S3-compatible bucket. A cloud folder appears in your library. Only files
        uploaded through Photoly are shown. Remote storage does not count toward your local quota.
      </p>
      <label class="flex items-center gap-3"
        ><Switch
          :model-value="enabled"
          aria-label="Enable S3"
          :disabled="busy"
          @update:model-value="toggleS3"
        />
        Enable S3</label
      >
      <p v-if="!enabled" class="text-muted-foreground text-sm">
        Disabling hides the S3 folder and keeps its files and organization intact.
      </p>
      <template v-if="enabled">
        <div v-for="field in settings.fields" :key="field.name" class="flex flex-col gap-1.5">
          <Label :for="`s3-${field.name}`">{{ field.label }}</Label>
          <Switch
            v-if="field.kind === 'bool'"
            :id="`s3-${field.name}`"
            :model-value="!!values[field.name]"
            :aria-label="field.label"
            :disabled="busy"
            @update:model-value="values[field.name] = $event"
          />
          <Input
            v-else
            :id="`s3-${field.name}`"
            :model-value="String(values[field.name] ?? '')"
            :type="field.secret || field.name === 'access_key' ? 'password' : 'text'"
            :autocomplete="field.secret || field.name === 'access_key' ? 'new-password' : 'off'"
            :placeholder="
              settings.hasSecret && (field.secret || field.name === 'access_key')
                ? 'Saved — leave blank to keep'
                : ''
            "
            :disabled="busy"
            @update:model-value="values[field.name] = String($event)"
          />
          <p v-if="field.help" class="text-muted-foreground text-xs">{{ field.help }}</p>
        </div>
        <p class="text-muted-foreground text-xs">
          Once files exist, the endpoint, bucket and prefix cannot be changed. Moving files between
          local storage and S3 is not supported.
        </p>
        <p v-if="!settings.canSave" role="alert" class="text-destructive text-sm">
          Ask the server administrator to configure storage credential encryption before connecting
          S3.
        </p>
        <div class="flex gap-2">
          <Button
            type="button"
            variant="outline"
            :disabled="busy || !settings.canSave"
            @click="submit(true)"
            >Test connection</Button
          ><Button type="submit" :disabled="busy || !settings.canSave">Save</Button>
        </div>
      </template>
    </form>
    <ExternalStorageSettings />
  </section>
</template>
