<script setup lang="ts">
import StorageScanProgress from '@/components/StorageScanProgress.vue'
import type { StorageStatus } from '@/composables/useStorageRevision'
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { HardDrive, Network, Server, Folder, RefreshCw } from '@lucide/vue'
import { toast } from 'vue-sonner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ApiError, call } from '@/lib/http'

const props = defineProps<{ adminUserId?: number }>()
interface Binding {
  sourceId: number
  name: string
  sourceType?: 'smb' | 'webdav'
  server?: string
  share?: string
  subdirectory?: string
  url?: string
  rootId?: string
  relativePath?: string
  enabled: boolean
  scanInterval: number
  scanStatus?: string
  scanError?: string
  overlaps?: { userId: number; name: string }[]
}
const admin = computed(() => props.adminUserId !== undefined)
const kind = ref<'smb' | 'webdav'>('webdav')
const base = computed(() =>
  admin.value ? `/admin/users/${props.adminUserId}/storage/local` : `/user/storage/${kind.value}`,
)
const bindings = ref<Binding[]>([])
const statuses = ref<StorageStatus[]>([])
const roots = ref<{ id: string; name: string }[]>([])
const directories = ref<{ name: string; relativePath: string }[]>([])
const browsing = ref('')
const editing = ref<number | null>(null)
const form = ref({
  name: '',
  url: '',
  server: '',
  share: '',
  subdirectory: '',
  domain: '',
  username: '',
  password: '',
  rootId: '',
  relativePath: '',
  enabled: true,
  scanInterval: 60,
})
const busy = ref(false)
const failure = ref('')
const detaching = ref<Binding | null>(null)
function parseUNC() {
  const text = form.value.server.trim()
  if (!text.startsWith('\\\\') && !text.startsWith('//') && !text.startsWith('smb://')) return
  const parts = text
    .replace(/^smb:\/\//, '')
    .replace(/\\/g, '/')
    .split('/')
    .filter(Boolean)
  form.value.server = parts[0] || ''
  form.value.share = parts[1] || ''
  form.value.subdirectory = parts.slice(2).join('/')
}
function errorMessage(e: unknown) {
  return e instanceof ApiError && typeof e.detail === 'string'
    ? e.detail
    : 'Storage request failed. Please try again.'
}
async function load() {
  try {
    if (admin.value) bindings.value = await call<Binding[]>({ url: base.value })
    else
      bindings.value = (
        await Promise.all(
          ['webdav', 'smb'].map(async (type) =>
            (await call<Binding[]>({ url: `/user/storage/${type}` })).map((b) => ({
              ...b,
              sourceType: type as 'webdav' | 'smb',
            })),
          ),
        )
      ).flat()
    if (admin.value) roots.value = await call({ url: '/admin/storage/local-roots' })
    else statuses.value = await call({ url: '/user/storage/sources' })
    failure.value = ''
  } catch (e) {
    failure.value = errorMessage(e)
  }
}
function edit(binding?: Binding, type: 'smb' | 'webdav' = 'webdav') {
  kind.value = binding?.sourceType ?? type
  editing.value = binding?.sourceId ?? 0
  form.value = {
    name: binding?.name ?? (admin.value ? 'Local mount' : kind.value === 'smb' ? 'SMB' : 'WebDAV'),
    url: binding?.url ?? '',
    server: binding?.server ?? '',
    share: binding?.share ?? '',
    subdirectory: binding?.subdirectory ?? '',
    domain: '',
    username: '',
    password: '',
    rootId: binding?.rootId ?? roots.value[0]?.id ?? '',
    relativePath: binding?.relativePath ?? '',
    enabled: binding?.enabled ?? true,
    scanInterval: binding?.scanInterval ?? 60,
  }
  browsing.value = form.value.relativePath
  if (admin.value && !binding) void browse('')
}
async function browse(relativePath: string) {
  try {
    directories.value = await call({
      url: '/admin/storage/local-directories',
      params: { rootId: form.value.rootId, relativePath },
    })
    browsing.value = relativePath
    form.value.relativePath = relativePath
  } catch (e) {
    toast.error(errorMessage(e))
  }
}
async function save(test = false) {
  if (busy.value) return
  if (kind.value === 'smb') parseUNC()
  busy.value = true
  try {
    await call({
      url: test ? `${base.value}/test` : `${base.value}${editing.value ? `/${editing.value}` : ''}`,
      method: test || !editing.value ? 'post' : 'patch',
      params: test && editing.value ? { sourceId: editing.value } : undefined,
      data: { ...form.value, scanInterval: Number(form.value.scanInterval) },
    })
    toast.success(test ? 'Read and write access verified' : 'Storage saved')
    if (!test) {
      editing.value = null
      form.value.username = ''
      form.value.password = ''
      await load()
    }
  } catch (e) {
    toast.error(errorMessage(e))
  } finally {
    busy.value = false
  }
}
async function detach() {
  if (!detaching.value || busy.value) return
  busy.value = true
  try {
    await call({
      url: `${admin.value ? base.value : `/user/storage/${detaching.value.sourceType || 'webdav'}`}/${detaching.value.sourceId}`,
      method: 'delete',
    })
    detaching.value = null
    await load()
  } catch (e) {
    toast.error(errorMessage(e))
  } finally {
    busy.value = false
  }
}
async function scan(id: number, force: boolean) {
  try {
    await call({ url: `/user/storage/sources/${id}/scan`, method: 'post', data: { force } })
    toast.success('Scan requested')
    await load()
  } catch (e) {
    toast.error(errorMessage(e))
  }
}
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void load()
  let last = Date.now()
  timer = setInterval(async () => {
    if (document.hidden) return
    if (admin.value) {
      if (Date.now() - last >= 15000 && !busy.value && editing.value === null) {
        last = Date.now()
        void load()
      }
      return
    }
    const active = statuses.value.some((s) => s.scanStatus === 'scanning' || s.scanQueued)
    if (Date.now() - last < (active ? 2000 : 15000)) return
    last = Date.now()
    try {
      statuses.value = await call({ url: '/user/storage/sources' })
    } catch {
      /* retry */
    }
  }, 2000)
})
onUnmounted(() => clearInterval(timer))
</script>
<template>
  <section
    class="flex flex-col gap-3 rounded-xl border p-4"
    :aria-label="admin ? 'Local mounts' : 'External storage connections'"
  >
    <h3 class="flex items-center gap-2 font-medium">
      <HardDrive v-if="admin" class="size-5 text-amber-600" /><Network
        v-else
        class="size-5 text-purple-600"
      />{{ admin ? 'Local mounts' : 'External storage connections' }}
    </h3>
    <p class="text-muted-foreground text-sm">
      Each connection appears as a separate library folder. External changes sync automatically.
      Originals do not count toward your quota; thumbnails and previews do.
    </p>
    <p v-if="admin" class="text-muted-foreground text-sm">
      Shared or overlapping folders have independent indexes. Editing or deleting files affects
      everyone using the same actual folder.
    </p>
    <p v-if="failure" role="alert" class="text-destructive text-sm">{{ failure }}</p>
    <article
      v-for="binding in bindings"
      :key="binding.sourceId"
      class="rounded-lg border p-3 text-sm"
    >
      <div class="flex items-center justify-between gap-2">
        <strong class="flex items-center gap-2"
          ><Server v-if="binding.sourceType === 'smb'" class="size-4 text-teal-600" />{{
            binding.name
          }}
          <span v-if="!admin">{{ binding.sourceType === 'smb' ? 'SMB' : 'WebDAV' }}</span></strong
        ><span>{{ binding.enabled ? 'Enabled' : 'Disabled' }}</span>
      </div>
      <p class="text-muted-foreground break-all">
        {{
          admin
            ? `${binding.rootId}/${binding.relativePath || ''}`
            : binding.sourceType === 'smb'
              ? `${binding.server}/${binding.share}/${binding.subdirectory || ''}`
              : binding.url
        }}
      </p>
      <p v-if="binding.overlaps?.length" class="text-amber-700">
        Shared / overlapping:
        {{ binding.overlaps.map((o) => `${o.name} (user ${o.userId})`).join(', ') }}
      </p>
      <template
        v-for="status in statuses.filter((s) => s.sourceId === binding.sourceId)"
        :key="status.sourceId"
      >
        <StorageScanProgress :status="status" />
      </template>
      <p v-if="admin" role="status">Scan: {{ binding.scanStatus }} {{ binding.scanError }}</p>
      <div class="mt-2 flex flex-wrap gap-2">
        <Button size="sm" variant="outline" :disabled="busy" @click="edit(binding)">Edit</Button>
        <Button size="sm" variant="outline" :disabled="busy" @click="detaching = binding"
          >Detach</Button
        >
        <template v-if="!admin && binding.enabled"
          ><Button size="sm" variant="outline" @click="scan(binding.sourceId, false)"
            ><RefreshCw class="size-3" /> Scan</Button
          ><Button size="sm" variant="ghost" @click="scan(binding.sourceId, true)"
            >Verify all content</Button
          ></template
        >
      </div>
    </article>
    <div
      v-if="detaching"
      role="alertdialog"
      aria-label="Detach storage"
      class="rounded border p-3 text-sm"
    >
      <p>
        Detach {{ detaching.name }}? This removes its Photoly index, tags, album links and shares.
        Actual files will be kept.
      </p>
      <div class="mt-2 flex gap-2">
        <Button variant="destructive" :disabled="busy" @click="detach">Detach and keep files</Button
        ><Button variant="outline" @click="detaching = null">Cancel</Button>
      </div>
    </div>
    <div class="flex flex-wrap gap-2">
      <Button
        v-if="!admin && editing === null"
        variant="outline"
        :disabled="busy"
        @click="edit(undefined, 'smb')"
        ><Server class="size-4 text-teal-600" /> Add SMB connection</Button
      >
      <Button
        v-if="editing === null"
        variant="outline"
        :disabled="busy || (admin && roots.length === 0)"
        @click="edit()"
        >{{ admin ? 'Bind local folder' : 'Add WebDAV connection' }}</Button
      >
    </div>
    <p v-if="admin && roots.length === 0" class="text-muted-foreground text-sm">
      No local roots are configured for this deployment.
    </p>
    <form
      v-if="editing !== null"
      class="flex flex-col gap-3 border-t pt-3"
      @submit.prevent="save()"
    >
      <Label :for="`mount-name-${adminUserId || 'self'}`">Entry name</Label
      ><Input
        :id="`mount-name-${adminUserId || 'self'}`"
        v-model="form.name"
        required
        :disabled="busy"
      />
      <template v-if="admin">
        <Label for="mount-root">Allowed root</Label
        ><select
          id="mount-root"
          v-model="form.rootId"
          class="bg-background rounded border p-2"
          :disabled="!!editing || busy"
          @change="browse('')"
        >
          <option v-for="root in roots" :key="root.id" :value="root.id">{{ root.name }}</option>
        </select>
        <div v-if="!editing" class="flex max-h-48 flex-col gap-1 overflow-auto rounded border p-2">
          <p class="text-sm">Selected: /{{ browsing }}</p>
          <Button
            v-if="browsing"
            type="button"
            variant="ghost"
            size="sm"
            @click="browse(browsing.split('/').slice(0, -1).join('/'))"
            >Parent directory</Button
          >
          <Button
            v-for="dir in directories"
            :key="dir.relativePath"
            type="button"
            variant="ghost"
            size="sm"
            class="justify-start"
            @click="browse(dir.relativePath)"
            ><Folder class="size-4" />{{ dir.name }}</Button
          >
        </div>
      </template>
      <template v-else>
        <template v-if="kind === 'webdav'"
          ><Label for="dav-url">Full directory URL</Label
          ><Input
            id="dav-url"
            v-model="form.url"
            type="url"
            required
            :disabled="!!editing || busy"
            placeholder="https://cloud.example/remote.php/dav/files/user/Photos/"
          />
        </template>
        <template v-else>
          <Label for="smb-server">Server or UNC path</Label
          ><Input
            id="smb-server"
            v-model="form.server"
            :disabled="!!editing || busy"
            required
            @change="parseUNC"
            placeholder="NASFSTBFD"
          />
          <Label for="smb-share">Share</Label
          ><Input id="smb-share" v-model="form.share" :disabled="!!editing || busy" required />
          <Label for="smb-directory">Subdirectory (optional)</Label
          ><Input id="smb-directory" v-model="form.subdirectory" :disabled="!!editing || busy" />
          <Label for="smb-domain">Domain (optional)</Label
          ><Input
            id="smb-domain"
            v-model="form.domain"
            type="password"
            autocomplete="new-password"
            :placeholder="editing ? 'Leave blank to keep saved domain' : ''"
          />
        </template>
        <Label for="dav-user">Username</Label
        ><Input
          id="dav-user"
          v-model="form.username"
          type="password"
          autocomplete="new-password"
          :placeholder="editing ? 'Leave blank to keep saved username' : ''"
          :disabled="busy"
        />
        <Label for="dav-password">Password or app password</Label
        ><Input
          id="dav-password"
          v-model="form.password"
          type="password"
          autocomplete="new-password"
          :placeholder="editing ? 'Leave blank to keep saved password' : ''"
          :disabled="busy"
        />
      </template>
      <Label for="mount-interval">Scan interval (seconds)</Label
      ><Input
        id="mount-interval"
        v-model="form.scanInterval"
        type="number"
        min="60"
        step="1"
        required
        :disabled="busy"
      />
      <label class="flex items-center gap-2 text-sm"
        ><Switch v-model="form.enabled" :disabled="busy" aria-label="Enable connection" /> Enable
        connection</label
      >
      <p class="text-muted-foreground text-xs">
        Disabling hides the entry and preserves its index and files. To change the location, detach
        and bind it again. Moving files across connections is not supported.
      </p>
      <div class="flex flex-wrap gap-2">
        <Button v-if="!admin" type="button" variant="outline" :disabled="busy" @click="save(true)"
          >Test connection</Button
        ><Button type="submit" :disabled="busy">Save connection</Button
        ><Button type="button" variant="ghost" :disabled="busy" @click="editing = null"
          >Cancel</Button
        >
      </div>
    </form>
  </section>
</template>
