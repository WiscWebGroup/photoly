<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import AuthLayout from '@/components/AuthLayout.vue'
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
  if (!/.+@.+\..+/.test(email.value) || !password.value) {
    show('Please input correct email and password format!')
    return
  }
  loading.value = true
  try {
    const { data } = await http.post('/user/signIn/', {
      email: email.value,
      password: password.value,
    })
    if (data.msgCode === 200) {
      auth.setToken(data.token)
      await goHome()
    } else {
      show('Wrong Email or Password!')
    }
  } catch {
    show('Could not reach the server.')
  } finally {
    loading.value = false
  }
}

// A still-valid stored token skips the form.
onMounted(async () => {
  if (!auth.isLoggedIn) return
  try {
    const { data } = await http.post('/user/pingToken/', {})
    if (data.msgCode === 200) await goHome()
  } catch {
    // stay on the form
  }
})
</script>

<template>
  <AuthLayout title="Login" :warning="warning">
    <div class="space-y-4" @keydown.enter="login">
      <Input v-model="email" type="email" placeholder="Email" autocomplete="email" />
      <Input
        v-model="password"
        type="password"
        placeholder="Password"
        autocomplete="current-password"
      />
      <Button class="w-full" :disabled="loading" @click="login">Login</Button>
      <Button type="button" variant="secondary" class="w-full" @click="router.push('/signup')"
        >Signup</Button
      >
    </div>
  </AuthLayout>
</template>
