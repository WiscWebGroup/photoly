<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Sparkles, RefreshCw } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import SmartBadge from '@/components/SmartBadge.vue'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'
import { smartReason, type SmartSource, type SmartStatus } from '@/lib/smartOrganization'

const auth = useAuthStore()
const settings = ref<{ enabled: boolean; available: boolean; reason: string | null } | null>(null)
const status = ref<SmartStatus | null>(null)
const dismissed = ref<SmartSource[]>([])
const nextAfter = ref<number | null>(null)
const busy = ref(false)
const pendingToggle = ref<boolean | null>(null)
const error = ref('')
let controller = new AbortController()
let sequence = 0
let loadSequence = 0
let interval: ReturnType<typeof setInterval> | undefined

async function load(more = false) {
  const current = sequence
  const request = ++loadSequence
  const signal = controller.signal
  try {
    const [config, progress, deleted] = await Promise.all([
      call<NonNullable<typeof settings.value>>({ url: '/ai/smart/settings', signal }),
      call<SmartStatus>({ url: '/ai/smart/status', signal }),
      call<{ items: SmartSource[]; nextAfter: number | null }>({
        url: '/ai/smart/dismissed',
        params: { after: more ? nextAfter.value : 0 },
        signal,
      }),
    ])
    if (current !== sequence || signal.aborted || request !== loadSequence) return
    if (
      typeof config?.enabled !== 'boolean' ||
      typeof progress?.total !== 'number' ||
      !Array.isArray(deleted?.items)
    )
      throw new Error('Invalid Smart Tag response')
    settings.value = config
    status.value = progress
    dismissed.value = more ? [...dismissed.value, ...deleted.items] : deleted.items
    nextAfter.value = deleted.nextAfter
    error.value = ''
  } catch {
    if (current === sequence && !signal.aborted && request === loadSequence)
      error.value = 'Could not load Smart Tag settings. Please retry.'
  }
}
async function change(action: 'toggle' | 'refresh' | 'restore', item?: SmartSource) {
  if (busy.value || !settings.value) return
  busy.value = true
  if (action === 'toggle') pendingToggle.value = !settings.value.enabled
  loadSequence++
  const current = sequence
  const signal = controller.signal
  try {
    await call({
      method: action === 'toggle' ? 'patch' : 'post',
      url:
        action === 'toggle'
          ? '/ai/smart/settings'
          : action === 'refresh'
            ? '/ai/smart/refresh'
            : `/ai/smart/items/${item!.id}/restore`,
      data: action === 'toggle' ? { enabled: !settings.value.enabled } : undefined,
      signal,
    })
    if (current !== sequence || signal.aborted) return
    await load()
    if (current === sequence)
      toast.success(
        action === 'restore'
          ? 'Smart item restored.'
          : action === 'refresh'
            ? 'Refresh requested.'
            : 'Smart Tag settings saved.',
      )
  } catch {
    if (current === sequence && !signal.aborted)
      error.value = 'Could not save Smart Tag changes. Please retry.'
  } finally {
    if (current === sequence) {
      busy.value = false
      pendingToggle.value = null
    }
  }
}
watch(
  () => auth.token,
  () => {
    sequence++
    controller.abort()
    controller = new AbortController()
    settings.value = null
    status.value = null
    dismissed.value = []
    busy.value = false
    pendingToggle.value = null
    if (auth.token) void load()
  },
)
onMounted(() => {
  void load()
  interval = setInterval(() => {
    if (!document.hidden && !busy.value && auth.token) void load()
  }, 15000)
})
onBeforeUnmount(() => {
  sequence++
  controller.abort()
  clearInterval(interval)
})
</script>

<template>
  <Card aria-label="Smart Tag settings">
    <CardHeader
      ><CardTitle class="flex items-center gap-2"
        ><Sparkles class="size-5" />Smart Tag</CardTitle
      ></CardHeader
    >
    <CardContent class="space-y-4">
      <p class="text-sm text-muted-foreground">
        Automatically organize photos and videos into camera and lens tags, and galleries based on
        capture time and location.
      </p>
      <label class="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          aria-label="Enable my Smart Tag"
          :checked="pendingToggle ?? settings?.enabled ?? false"
          :disabled="busy || !settings || (!settings.available && !settings.enabled)"
          @change="change('toggle')"
        />Enable Smart Tag for my library
      </label>
      <p v-if="settings?.reason || status?.reason" role="status" class="text-sm">
        {{ smartReason(settings?.reason || status?.reason) }}
      </p>
      <div v-if="status" class="text-sm" aria-live="polite">
        <p>
          {{ status.processed }} / {{ status.total }} media inspected · {{ status.failed }} failed
        </p>
        <p v-if="status.pending">
          Organization pending<span v-if="status.phase">
            ·
            {{
              {
                metadata: 'Reading capture information',
                anchors: 'Grouping by time and location',
                missing: 'Matching photos and videos',
                items: 'Updating galleries',
                members: 'Updating members',
              }[status.phase] ?? 'Organizing'
            }}</span
          >
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <Button
          variant="outline"
          :disabled="busy || !settings?.enabled || !settings?.available"
          @click="change('refresh')"
          ><RefreshCw class="size-4" />Refresh and retry</Button
        >
        <Button v-if="error" variant="outline" :disabled="busy" @click="load()"
          >Retry loading</Button
        >
      </div>
      <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
      <details class="rounded-md border p-3">
        <summary class="cursor-pointer text-sm font-medium">Deleted smart items</summary>
        <p class="mt-2 text-xs text-muted-foreground">
          Deleted items stay hidden until restored. Restoring an item also restores its related
          merged or split items.
        </p>
        <p v-if="!dismissed.length" class="mt-2 text-sm">No deleted smart items.</p>
        <ul v-else class="mt-3 space-y-3">
          <li
            v-for="item in dismissed"
            :key="item.id"
            class="flex flex-wrap items-center justify-between gap-2 text-sm"
          >
            <span class="min-w-0 break-words">{{ item.name }} <SmartBadge :smart="item" /></span>
            <Button
              size="sm"
              variant="outline"
              :disabled="busy"
              :aria-label="`Restore ${item.name}`"
              @click="change('restore', item)"
              >Restore</Button
            >
          </li>
        </ul>
        <Button v-if="nextAfter" class="mt-2" variant="ghost" :disabled="busy" @click="load(true)"
          >Load more deleted items</Button
        >
      </details>
      <p class="text-xs text-muted-foreground">
        Approximate city names from
        <a href="https://www.geonames.org/" class="underline" target="_blank" rel="noreferrer"
          >GeoNames</a
        >
        (CC BY 4.0). Location data stays on your server.
      </p>
    </CardContent>
  </Card>
</template>
