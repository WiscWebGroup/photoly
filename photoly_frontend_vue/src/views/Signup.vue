<script setup lang="ts">
import { ref } from 'vue'
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

const username = ref('')
const email = ref('')
const password = ref('')
const loading = ref(false)

async function signup() {
  if (!/.+@.+\..+/.test(email.value) || !password.value || !username.value) {
    show('incorrect email, blank username or password format!')
    return
  }
  loading.value = true
  try {
    const { data } = await http.post('/user/signUp/', {
      userName: username.value,
      email: email.value,
      password: password.value,
    })
    if (data.msgCode === 200) {
      auth.setToken(data.token)
      await router.push('/home/HomePart')
    } else {
      show(
        data.t === 'Register is closed by Admin' ? 'Register Closed!' : 'Wrong Email or Password!',
      )
    }
  } catch {
    show('Could not reach the server.')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthLayout title="Signup" :warning="warning">
    <div class="space-y-4" @keydown.enter="signup">
      <Input v-model="username" placeholder="Username" autocomplete="username" />
      <Input v-model="email" type="email" placeholder="Email" autocomplete="email" />
      <Input
        v-model="password"
        type="password"
        placeholder="Password"
        autocomplete="new-password"
      />
      <Button class="w-full" :disabled="loading" @click="signup">Signup</Button>
      <Button type="button" variant="secondary" class="w-full" @click="router.push('/login')">
        Already have an account?
      </Button>
    </div>
  </AuthLayout>
</template>
