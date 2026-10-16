<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { toast } from 'vue-sonner'
import { http, type Envelope } from '@/lib/http'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

const props = defineProps<{ photoId: number }>()
interface OCRResult {
  photoId: number
  contentVersion: number
  revision: number
  text: string | null
  languages: string
  engineVersion: string | null
  updatedAt: string | null
}
const result = ref<OCRResult>()
const draft = ref('')
const busy = ref(false)
const error = ref('')
const conflict = ref(false)
const confirmRecognize = ref(false)
const leaving = ref(false)
const dirty = computed(() => !!result.value && draft.value !== (result.value.text ?? ''))
let sequence = 0
let controller: AbortController | undefined
let leaveResolver: ((allowed: boolean) => void) | undefined
const errors: Record<string, string> = {
  ocr_busy: 'Text recognition is busy. Please try again shortly.',
  ocr_timeout: 'Text recognition timed out. Please try a smaller image.',
  ocr_conflict:
    'The image or saved text has changed. Refresh the saved version before trying again. Your draft is kept.',
  photo_not_found: 'This image is no longer available.',
  image_missing: 'The original image file is missing.',
  unsupported_format: 'Text recognition is not supported for this image format.',
  image_too_large: 'Text recognition supports images up to 20 MiB and 40 million pixels.',
  invalid_image: 'The image could not be decoded.',
  engine_unavailable: 'Tesseract is not installed on the server.',
  languages_unavailable: 'The required OCR language files are missing on the server.',
  recognition_failed: 'Text recognition failed. Please try again.',
  text_too_long: 'The recognized text exceeds the 100,000 character limit.',
  invalid_text: 'Text must contain at most 100,000 characters and no null characters.',
}

async function request(method: 'get' | 'post' | 'put', preserveDraft = false) {
  if (busy.value) return false
  const seq = ++sequence
  controller?.abort()
  controller = new AbortController()
  busy.value = true
  error.value = ''
  conflict.value = false
  try {
    const { data } = await http.request<Envelope<OCRResult | { error: string }>>({
      method,
      url: method === 'post' ? '/photo/ocr/recognize' : '/photo/ocr',
      params: method === 'get' ? { photoId: props.photoId } : undefined,
      data:
        method === 'get'
          ? undefined
          : {
              photoId: props.photoId,
              contentVersion: result.value?.contentVersion,
              revision: result.value?.revision,
              ...(method === 'put' ? { text: draft.value } : {}),
            },
      signal: controller.signal,
      timeout: 55_000,
    })
    if (seq !== sequence) return false
    if (data.msgCode !== 200) {
      const reason = data.t && 'error' in data.t ? data.t.error : ''
      conflict.value = data.msgCode === 409
      error.value = errors[reason] ?? 'Could not complete this request. Please try again.'
      return false
    }
    result.value = data.t as OCRResult
    if (!preserveDraft) draft.value = result.value.text ?? ''
    if (method === 'put') toast.success('Text saved')
    return true
  } catch {
    if (seq === sequence) error.value = 'Could not reach the server. Please try again.'
    return false
  } finally {
    if (seq === sequence) busy.value = false
  }
}
function opened(event: Event) {
  if ((event.target as HTMLDetailsElement).open && !result.value && !busy.value) void request('get')
}
function recognize() {
  if (result.value?.text !== null || dirty.value) confirmRecognize.value = true
  else void request('post')
}
function replaceText() {
  confirmRecognize.value = false
  void request('post')
}
async function copy() {
  try {
    await navigator.clipboard.writeText(draft.value)
    toast.success('Text copied')
  } catch {
    error.value = 'Copy failed. Select the text and copy it manually.'
  }
}
function finishLeave(allowed: boolean) {
  if (allowed) draft.value = result.value?.text ?? ''
  leaving.value = false
  leaveResolver?.(allowed)
  leaveResolver = undefined
}
async function saveAndLeave() {
  if (await request('put')) finishLeave(true)
}
function requestLeave(): Promise<boolean> {
  if (!dirty.value) return Promise.resolve(true)
  if (leaveResolver) return Promise.resolve(false)
  leaving.value = true
  return new Promise((resolve) => {
    leaveResolver = resolve
  })
}
function reset() {
  sequence++
  controller?.abort()
  result.value = undefined
  error.value = ''
  conflict.value = false
  confirmRecognize.value = false
  draft.value = ''
  busy.value = false
  finishLeave(false)
}
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault()
    event.returnValue = ''
  }
}
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => {
  reset()
  window.removeEventListener('beforeunload', beforeUnload)
})
defineExpose({ requestLeave, reset })
</script>

<template>
  <details class="rounded-md border" @toggle="opened">
    <summary class="cursor-pointer px-3 py-2 font-medium">Extract Text</summary>
    <div class="space-y-3 border-t p-3 text-sm" :aria-busy="busy">
      <p class="text-muted-foreground">
        Recognize Simplified Chinese, Traditional Chinese, English and Japanese. Animated and
        multi-page images use the first frame or page.
      </p>
      <p v-if="busy" role="status">Processing…</p>
      <p v-if="error" role="alert" class="text-destructive">{{ error }}</p>
      <Button v-if="!result" size="sm" :disabled="busy" @click="request('get')"
        >Retry loading text</Button
      >
      <template v-else>
        <label :for="`ocr-text-${photoId}`" class="block font-medium">Image text</label>
        <textarea
          :id="`ocr-text-${photoId}`"
          v-model="draft"
          rows="10"
          maxlength="100000"
          :disabled="busy"
          class="w-full resize-y rounded-md border bg-background p-2"
          placeholder="Recognized text will appear here."
        />
        <p v-if="result.text === '' && !dirty" role="status">
          No text was recognized or saved. You can enter text manually.
        </p>
        <p v-if="dirty" class="text-muted-foreground">Unsaved changes</p>
        <div class="flex flex-wrap gap-2">
          <Button size="sm" :disabled="busy || conflict" @click="recognize">{{
            result.text === null ? 'Recognize Text' : 'Recognize Again'
          }}</Button>
          <Button
            size="sm"
            variant="secondary"
            :disabled="busy || !dirty || conflict"
            @click="request('put')"
            >Save</Button
          >
          <Button size="sm" variant="outline" :disabled="busy || !draft" @click="copy">Copy</Button>
          <Button
            v-if="conflict"
            size="sm"
            variant="outline"
            :disabled="busy"
            @click="request('get', true)"
            >Refresh saved version</Button
          >
        </div>
      </template>
    </div>
    <AlertDialog
      :open="confirmRecognize"
      @update:open="
        (open) => {
          if (!open) confirmRecognize = false
        }
      "
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Recognize this image again?</AlertDialogTitle>
          <AlertDialogDescription
            >Successful recognition will replace the saved text and any unsaved
            changes.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" @click="confirmRecognize = false">Cancel</Button>
          <Button @click="replaceText">Recognize and replace</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
    <AlertDialog
      :open="leaving"
      @update:open="
        (open) => {
          if (!open) finishLeave(false)
        }
      "
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Save text changes?</AlertDialogTitle>
          <AlertDialogDescription
            >Your image text has unsaved changes. Save them before leaving or discard the
            changes.</AlertDialogDescription
          >
        </AlertDialogHeader>
        <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
        <AlertDialogFooter>
          <Button variant="outline" @click="finishLeave(false)">Cancel</Button>
          <Button variant="secondary" @click="finishLeave(true)">Discard</Button>
          <Button :disabled="busy || conflict" @click="saveAndLeave">Save and continue</Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </details>
</template>
