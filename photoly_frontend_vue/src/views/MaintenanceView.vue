<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { Wrench } from '@lucide/vue'
import { call } from '@/lib/http'
import { Button } from '@/components/ui/button'
const router = useRouter()
let timer: ReturnType<typeof setInterval> | undefined
async function check() {
  try {
    const state = await call<{ mode: string }>({ url: '/maintenance/status' })
    if (state.mode === 'off') await router.replace('/login')
  } catch {
    /* keep maintenance visible while reconnecting */
  }
}
onMounted(() => {
  void check()
  timer = setInterval(check, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>
<template>
  <main class="bg-background text-foreground flex min-h-dvh items-center justify-center p-6">
    <div
      class="bg-card flex w-full max-w-md flex-col items-center gap-5 rounded-2xl border p-8 text-center shadow-xs"
    >
      <Wrench class="text-primary size-8" />
      <h1 class="text-2xl font-semibold">Photoly is under maintenance</h1>
      <p class="text-muted-foreground text-sm">
        Your library is temporarily unavailable while an administrator maintains this instance. This
        page will check for updates automatically.
      </p>
      <Button variant="outline" @click="router.push('/login')">Administrator Sign In</Button>
    </div>
  </main>
</template>
