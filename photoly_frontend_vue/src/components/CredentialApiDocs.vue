<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { toast } from 'vue-sonner'
import { Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { endpoints, apiAddress, curlCommand, type Endpoint } from '@/lib/credentialApi'
const props = defineProps<{
  credentials: { credId: number; token: string; authorization: string }[]
}>()
const selected = ref('')
const credential = computed(() =>
  props.credentials.find((c) => String(c.credId) === selected.value),
)
const fallback = ref('')
watch(
  () => props.credentials,
  () => {
    if (selected.value && !credential.value) selected.value = ''
    fallback.value = ''
  },
  { deep: true },
)
watch(selected, () => {
  fallback.value = ''
})
const address = (e: Endpoint) =>
  apiAddress(e, import.meta.env.VITE_APP_BASE_URL, window.location.origin)
const command = (e: Endpoint) =>
  curlCommand(e, import.meta.env.VITE_APP_BASE_URL, window.location.origin, credential.value?.token)
const allowed = (e: Endpoint) =>
  !credential.value || credential.value.authorization.includes(e.scope)
async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    fallback.value = ''
    toast.success('Copied')
  } catch {
    fallback.value = text
    toast.error('Clipboard unavailable. Select and copy the text below.')
  }
}
</script>
<template>
  <details class="bg-card rounded-xl border">
    <summary class="cursor-pointer px-4 py-3 text-sm font-medium">API documents</summary>
    <div class="flex flex-col gap-5 border-t p-4">
      <div class="flex flex-col gap-2 text-sm">
        <label for="api-example-token" class="font-medium">Token for examples</label>
        <select
          id="api-example-token"
          v-model="selected"
          class="bg-background w-full rounded-md border p-2 sm:max-w-xs"
        >
          <option value="">YOUR_TOKEN placeholder</option>
          <option v-for="c in credentials" :key="c.credId" :value="String(c.credId)">
            Token #{{ c.credId }} · {{ c.authorization }}
          </option>
        </select>
        <p class="text-muted-foreground text-xs">
          C: upload · R: read · D: delete. Replace YOUR_NAMESPACE_ID, YOUR_MEDIA_UUID and file paths
          before running commands. New read endpoints return {msgCode, t}; legacy endpoints retain
          their original response format.
        </p>
        <p v-if="credential" class="text-muted-foreground text-xs">
          Copied cURL commands include Token #{{ credential.credId }} in the Authorization header.
        </p>
      </div>
      <div v-if="fallback" class="bg-muted rounded-lg p-3">
        <label for="api-copy-fallback" class="text-sm font-medium">Select and copy</label
        ><textarea
          id="api-copy-fallback"
          :value="fallback"
          readonly
          rows="6"
          class="bg-background mt-2 w-full rounded-md border p-2 font-mono text-xs"
          @focus="($event.target as HTMLTextAreaElement).select()"
        />
      </div>
      <article
        v-for="e in endpoints"
        :key="e.path"
        class="flex flex-col gap-2 border-t pt-4"
        :aria-label="e.title"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span class="bg-muted rounded px-2 py-1 font-mono text-xs">{{ e.method }}</span>
          <h3 class="text-sm font-medium">{{ e.title }}</h3>
          <span class="text-muted-foreground text-xs">Requires {{ e.scope }}</span>
        </div>
        <code class="text-muted-foreground text-xs break-all">{{ address(e) }}</code>
        <p class="text-sm">{{ e.description }}</p>
        <dl class="text-muted-foreground text-xs leading-5">
          <div>
            <dt class="inline font-medium">Parameters:</dt>
            <dd class="inline">{{ e.parameters }}</dd>
          </div>
          <div>
            <dt class="inline font-medium">Returns:</dt>
            <dd class="inline">{{ e.returns }}</dd>
          </div>
        </dl>
        <p v-if="!allowed(e)" class="text-destructive text-xs">
          This token does not have {{ e.scope }} permission.
        </p>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" @click="copy(address(e))"
            ><Copy class="size-3.5" />Copy address</Button
          ><Button variant="outline" size="sm" :disabled="!allowed(e)" @click="copy(command(e))"
            ><Copy class="size-3.5" />Copy cURL</Button
          >
        </div>
        <details class="text-xs">
          <summary class="text-muted-foreground cursor-pointer">cURL example</summary>
          <pre class="bg-muted mt-2 overflow-auto rounded-md p-3">{{
            curlCommand(e, '', 'https://photoly.example')
          }}</pre>
        </details>
      </article>
    </div>
  </details>
</template>
