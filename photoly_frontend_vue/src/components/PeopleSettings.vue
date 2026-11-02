<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { usePeopleStore, peopleLabel } from '@/stores/people'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
} from '@/components/ui/alert-dialog'
const store = usePeopleStore(),
  confirm = ref(false),
  busy = ref(false),
  error = ref('')
onMounted(() => store.refresh())
async function run(fn: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await fn()
    confirm.value = false
  } catch {
    error.value = 'Could not save People settings. Please retry.'
  } finally {
    busy.value = false
  }
}
</script>
<template>
  <div class="flex flex-col gap-4 text-sm">
    <label class="flex items-center gap-2 font-medium"
      ><input
        type="checkbox"
        aria-label="People for my library"
        class="accent-primary size-4"
        :checked="store.preference?.enabled"
        :disabled="busy || (!store.preference?.available && !store.preference?.enabled)"
        @change="run(() => store.change({ enabled: ($event.target as HTMLInputElement).checked }))"
      />People for my library</label
    >
    <p class="text-muted-foreground">
      Find faces in photos, group people, and keep the names and corrections you choose. Only you
      can see your people.
    </p>
    <p v-if="store.preference?.reason" role="status">{{ peopleLabel(store.preference.reason) }}</p>
    <label v-if="store.preference?.choosable" class="flex items-center gap-3"
      >Where People runs<select
        aria-label="Where People runs"
        class="bg-background rounded-md border p-2"
        :value="store.preference.placement"
        :disabled="busy"
        @change="
          run(() =>
            store.change({
              placement: ($event.target as HTMLSelectElement).value as 'browser' | 'server',
            }),
          )
        "
      >
        <option value="browser">Browser</option>
        <option value="server">Server</option>
      </select></label
    >
    <p v-if="store.preference?.placement === 'browser'" class="text-muted-foreground">
      Keep Photoly open to process photos on this device. Processing continues next time you open
      it. Devices without WebGPU use the browser CPU.
    </p>
    <div v-if="store.preference?.enabled && store.status" class="flex flex-col gap-2">
      <progress
        class="h-2 w-full accent-primary"
        :max="Math.max(1, store.status.total)"
        :value="store.status.ready"
        aria-label="People indexing progress"
      />
      <p>
        {{ store.status.ready }} / {{ store.status.total }} photos processed ·
        {{ store.status.failed }} failed
      </p>
      <p v-if="store.device" class="text-muted-foreground">
        This device: {{ store.device === 'webgpu' ? 'WebGPU' : 'Browser CPU' }}
      </p>
      <p v-if="store.status.waitReason && !store.preference?.reason" role="status">
        {{ peopleLabel(store.status.waitReason) }}
      </p>
      <p v-if="store.message" role="status">{{ store.message }}</p>
      <div class="flex flex-wrap gap-2">
        <Button
          variant="outline"
          :disabled="busy"
          @click="run(() => store.change({ paused: !store.preference?.paused }))"
          >{{ store.preference?.paused ? 'Resume processing' : 'Pause processing' }}</Button
        ><Button variant="outline" :disabled="busy" @click="run(store.retry)"
          >Retry failed photos</Button
        >
      </div>
    </div>
    <p v-if="error || store.error" role="alert" class="text-destructive">
      {{ error || store.error }}
    </p>
    <Button variant="outline" class="self-start" @click="confirm = true"
      >Delete all face data</Button
    >
    <AlertDialog :open="confirm" @update:open="confirm = $event"
      ><AlertDialogContent
        ><AlertDialogHeader
          ><AlertDialogTitle>Delete all face data?</AlertDialogTitle
          ><AlertDialogDescription
            >This deletes people, names, face embeddings and manual corrections, and turns People
            off. Your photos stay in your library.</AlertDialogDescription
          ></AlertDialogHeader
        ><AlertDialogFooter
          ><AlertDialogCancel :disabled="busy">Cancel</AlertDialogCancel
          ><Button variant="destructive" :disabled="busy" @click="run(store.clear)"
            >Delete all face data</Button
          ></AlertDialogFooter
        ></AlertDialogContent
      ></AlertDialog
    >
  </div>
</template>
