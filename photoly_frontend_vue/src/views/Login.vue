<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthField from '@/components/auth/AuthField.vue'
import AuthSubmit from '@/components/auth/AuthSubmit.vue'
import { useWarning } from '@/composables/useWarning'
import { http } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const { warning, show } = useWarning()

const email = ref('')
const password = ref('')
const loading = ref(false)

const goHome = () => router.push('/home/HomePart')

async function login() {
  if (loading.value) return
  if (!/.+@.+\..+/.test(email.value) || !password.value) {
    show('Please input correct email and password format!')
    return
  }
  loading.value = true
  // Also bound connection establishment; XHR's timeout alone can wait for it in WebKit.
  const controller = new AbortController()
  const deadline = setTimeout(() => controller.abort(), 15_000)
  try {
    const { data } = await http.post(
      '/user/signIn/',
      { email: email.value, password: password.value },
      { timeout: 15_000, signal: controller.signal },
    )
    if (data.msgCode === 200) {
      auth.setToken(data.token)
      await goHome()
    } else {
      show('Wrong Email or Password!')
    }
  } catch {
    show('Could not reach the server.')
  } finally {
    clearTimeout(deadline)
    loading.value = false
  }
}

// A still-valid stored token skips the form.
onMounted(async () => {
  if (!auth.isLoggedIn) return
  try {
    const { data } = await http.post('/user/pingToken/', {}, { timeout: 10_000 })
    if (data.msgCode === 200) await goHome()
  } catch {
    // stay on the form
  }
})
</script>

<template>
  <form class="flex flex-1 flex-col gap-4" novalidate @submit.prevent="login">
    <Transition name="reveal">
      <p
        v-if="warning"
        role="alert"
        class="rounded-[10px] border border-red-400/30 bg-red-400/10 px-3.5 py-2.5 text-red-200"
      >
        {{ warning }}
      </p>
    </Transition>
    <AuthField
      v-model="email"
      label="Email"
      type="email"
      autocomplete="email"
      placeholder="you@example.com"
    />
    <AuthField
      v-model="password"
      label="Password"
      type="password"
      autocomplete="current-password"
      hint="Forgot it? Your server admin can reset it."
    />
    <div class="mt-auto pt-4 sm:mt-2 sm:pt-0">
      <AuthSubmit :loading="loading">Sign In</AuthSubmit>
    </div>
  </form>
</template>
