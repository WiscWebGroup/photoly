<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Cloud, Plus } from '@lucide/vue'
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
  default?: string
  help?: string
}
interface Connection {
  sourceId: number
  name: string
  enabled: boolean
  values: Record<string, string>
  hasSecret: boolean
  canRemove: boolean
}
interface Settings {
  connections: Connection[]
  fields: Field[]
  canSave: boolean
}
const emit = defineEmits<{ changed: [] }>()
const settings = ref<Settings>()
const editing = ref<Connection | 'new' | null>(null)
const removing = ref<Connection | null>(null)
const alias = ref('S3')
const values = ref<Record<string, string | boolean>>({})
let initial = ''
const busy = ref(false)
const failure = ref('')
function message(e: unknown) {
  return e instanceof ApiError && typeof e.detail === 'string'
    ? e.detail
    : 'Storage request failed. Please try again.'
}
async function load() {
  try {
    settings.value = await call<Settings>({ url: '/user/storage/s3' })
    failure.value = ''
  } catch (e) {
    failure.value = message(e)
  }
}
function edit(connection: Connection | 'new') {
  editing.value = connection
  alias.value = connection === 'new' ? 'S3' : connection.name
  const saved = connection === 'new' ? {} : connection.values
  values.value = Object.fromEntries(
    (settings.value?.fields ?? []).map((f) => [
      f.name,
      f.kind === 'bool' ? (saved[f.name] ?? f.default) === 'true' : (saved[f.name] ?? ''),
    ]),
  )
  initial = JSON.stringify(values.value)
  failure.value = ''
}
function cancel() {
  editing.value = null
  values.value = {}
  failure.value = ''
}
async function submit(test: boolean) {
  if (!editing.value) return
  busy.value = true
  failure.value = ''
  const connection = editing.value
  const sourceId = connection === 'new' ? undefined : connection.sourceId
  try {
    const changed = initial !== JSON.stringify(values.value)
    await call({
      method: test || sourceId === undefined ? 'post' : 'patch',
      url: `/user/storage/s3${test ? '/test' : sourceId === undefined ? '' : `/${sourceId}`}`,
      data: test
        ? { sourceId, values: values.value }
        : {
            name: alias.value,
            ...(changed || sourceId === undefined ? { values: values.value } : {}),
          },
    })
    toast.success(test ? 'S3 connection verified' : 'S3 connection saved')
    if (!test) {
      cancel()
      await load()
      emit('changed')
    }
  } catch (e) {
    failure.value = message(e)
  } finally {
    busy.value = false
  }
}
async function toggle(connection: Connection, enabled: boolean) {
  busy.value = true
  failure.value = ''
  try {
    await call({
      method: 'patch',
      url: `/user/storage/s3/${connection.sourceId}`,
      data: { enabled },
    })
    connection.enabled = enabled
    emit('changed')
  } catch (e) {
    failure.value = message(e)
  } finally {
    busy.value = false
  }
}
async function remove() {
  if (!removing.value) return
  busy.value = true
  failure.value = ''
  try {
    await call({ method: 'delete', url: `/user/storage/s3/${removing.value.sourceId}` })
    removing.value = null
    await load()
    emit('changed')
  } catch (e) {
    failure.value = message(e)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
<template>
  <section class="flex flex-col gap-4 rounded-xl border p-4" aria-label="S3 connections">
    <h3 class="flex items-center gap-2 font-medium">
      <Cloud class="size-5 text-sky-600" /> Your S3 storage
    </h3>
    <p class="text-muted-foreground text-sm">
      Connect S3-compatible buckets. Each connection appears as a cloud folder in your library. Only
      Photoly uploads are shown; buckets are not scanned.
    </p>
    <article
      v-for="connection in settings?.connections"
      :key="connection.sourceId"
      class="flex flex-col gap-2 rounded-lg border p-3"
    >
      <div class="flex min-w-0 items-center gap-2">
        <Cloud class="size-4 shrink-0 text-sky-600" /><strong
          class="truncate"
          :title="connection.name"
          >{{ connection.name }}</strong
        ><span class="text-muted-foreground text-xs">S3</span>
      </div>
      <p class="text-muted-foreground truncate text-xs">
        {{ connection.values.endpoint }} / {{ connection.values.bucket }}
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <Switch
          :model-value="connection.enabled"
          :disabled="busy"
          :aria-label="`Enable ${connection.name}`"
          @update:model-value="toggle(connection, $event)"
        />
        <span class="mr-auto text-sm">{{ connection.enabled ? 'Enabled' : 'Disabled' }}</span>
        <Button size="sm" variant="outline" :disabled="busy" @click="edit(connection)">Edit</Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="busy || !connection.canRemove"
          @click="removing = connection"
          >Remove</Button
        >
      </div>
      <p v-if="!connection.canRemove" class="text-muted-foreground text-xs">
        Connections with media or unfinished uploads can be disabled, but cannot be removed.
      </p>
    </article>
    <div
      v-if="removing"
      role="alertdialog"
      aria-label="Remove S3 connection"
      class="space-y-3 rounded-lg border p-3"
    >
      <p>Remove {{ removing.name }}? Remote objects will be kept.</p>
      <div class="flex flex-wrap gap-2">
        <Button variant="destructive" :disabled="busy" @click="remove">Confirm removal</Button
        ><Button variant="outline" :disabled="busy" @click="removing = null">Cancel</Button>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <Button variant="outline" :disabled="busy || !settings" @click="edit('new')"
        ><Plus class="size-4 text-sky-600" /> Add S3</Button
      >
    </div>
    <form
      v-if="editing && settings"
      class="flex flex-col gap-4 rounded-lg border p-3"
      @submit.prevent="submit(false)"
    >
      <h4 class="font-medium">
        {{ editing === 'new' ? 'Add S3 connection' : 'Edit S3 connection' }}
      </h4>
      <div class="space-y-1.5">
        <Label for="s3-alias">Alias</Label
        ><Input id="s3-alias" v-model="alias" :disabled="busy" required />
      </div>
      <div v-for="field in settings.fields" :key="field.name" class="flex flex-col gap-1.5">
        <Label :for="`s3-${field.name}`">{{ field.label }}</Label>
        <Switch
          v-if="field.kind === 'bool'"
          :id="`s3-${field.name}`"
          :aria-label="field.label"
          :model-value="!!values[field.name]"
          :disabled="busy"
          @update:model-value="values[field.name] = $event"
        />
        <Input
          v-else
          :id="`s3-${field.name}`"
          :model-value="String(values[field.name] ?? '')"
          :type="field.secret || field.name === 'access_key' ? 'password' : 'text'"
          autocomplete="off"
          :placeholder="
            editing !== 'new' && (field.secret || field.name === 'access_key')
              ? 'Saved — leave blank to keep'
              : ''
          "
          :disabled="busy"
          @update:model-value="values[field.name] = String($event)"
        />
        <p v-if="field.help" class="text-muted-foreground text-xs">{{ field.help }}</p>
      </div>
      <p class="text-muted-foreground text-xs">
        Once media or unfinished uploads exist, the endpoint, bucket and prefix cannot change.
      </p>
      <p v-if="!settings.canSave" role="alert">
        Ask the administrator to configure storage credential encryption.
      </p>
      <div class="flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          :disabled="busy || !settings.canSave"
          @click="submit(true)"
          >Test connection</Button
        ><Button type="submit" :disabled="busy || !settings.canSave">Save</Button
        ><Button type="button" variant="outline" :disabled="busy" @click="cancel">Cancel</Button>
      </div>
    </form>
    <p v-if="failure" role="alert" class="text-destructive text-sm">{{ failure }}</p>
  </section>
</template>
