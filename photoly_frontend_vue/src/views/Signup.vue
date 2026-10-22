<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { MorphIcon } from 'morphicons/vue'
import { Check, X } from 'lucide'
import AuthField from '@/components/auth/AuthField.vue'
import AuthSubmit from '@/components/auth/AuthSubmit.vue'
import { useWarning } from '@/composables/useWarning'
import { http } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const { warning, show } = useWarning()

const username = ref('')
const email = ref('')
const password = ref('')
const confirm = ref('')
const loading = ref(false)
const submitted = ref(false)

const STRENGTH_LABELS = ['Too short', 'Weak', 'Fair', 'Good', 'Strong']
/** 0–4: length first, then variety of character kinds. */
const strength = computed(() => {
  const pw = password.value
  if (pw.length < 6) return 0
  const kinds = [/[a-z]/, /[A-Z]/, /\d/, /[^A-Za-z0-9]/].filter((r) => r.test(pw)).length
  let score = 1
  if (pw.length >= 8) score++
  if (pw.length >= 12 || kinds >= 3) score++
  if (pw.length >= 10 && kinds >= 3) score++
  return Math.min(score, 4)
})

const matches = computed(() => confirm.value === password.value)
// Flag a mismatch as soon as the typed confirmation diverges, or on submit.
const mismatch = computed(
  () =>
    !!confirm.value &&
    !matches.value &&
    (submitted.value || !password.value.startsWith(confirm.value)),
)

async function signup() {
  if (loading.value) return
  submitted.value = true
  if (!/.+@.+\..+/.test(email.value) || !password.value || !username.value) {
    show('incorrect email, blank username or password format!')
    return
  }
  if (!matches.value) return
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
  <form class="flex flex-1 flex-col gap-4" novalidate @submit.prevent="signup">
    <Transition name="reveal">
      <p
        v-if="warning"
        role="alert"
        class="rounded-[10px] border border-red-400/30 bg-red-400/10 px-3.5 py-2.5 text-red-200"
      >
        {{ warning }}
      </p>
    </Transition>
    <div class="grid gap-4 sm:grid-cols-2 sm:gap-3">
      <AuthField v-model="username" label="Username" autocomplete="username" />
      <AuthField
        v-model="email"
        label="Email"
        type="email"
        autocomplete="email"
        placeholder="you@example.com"
      />
    </div>
    <AuthField v-model="password" label="Password" type="password" autocomplete="new-password">
      <template #below>
        <Transition name="reveal">
          <div
            v-if="password"
            role="meter"
            aria-label="Password strength"
            :aria-valuenow="strength"
            aria-valuemin="0"
            aria-valuemax="4"
            :aria-valuetext="STRENGTH_LABELS[strength]"
            class="flex items-center gap-2 pt-0.5"
          >
            <span class="grid flex-1 grid-cols-4 gap-1">
              <span
                v-for="i in 4"
                :key="i"
                class="h-1 origin-left rounded-full transition-[background-color,transform] duration-300 ease-(--ease-out-quart)"
                :class="
                  i <= strength
                    ? strength <= 1
                      ? 'bg-red-400'
                      : strength === 2
                        ? 'bg-amber-300'
                        : 'bg-teal-300'
                    : 'scale-x-95 bg-white/12'
                "
                :style="{ transitionDelay: `${(i - 1) * 40}ms` }"
              />
            </span>
            <span class="w-16 text-right text-xs text-teal-100/75">{{
              STRENGTH_LABELS[strength]
            }}</span>
          </div>
        </Transition>
      </template>
    </AuthField>
    <AuthField
      v-model="confirm"
      label="Confirm Password"
      type="password"
      autocomplete="new-password"
      :error="mismatch ? `Passwords don't match.` : undefined"
    >
      <template #aside>
        <span
          class="flex items-center gap-1 text-xs transition-[opacity,color] duration-300"
          :class="[
            confirm && (matches || mismatch) ? 'opacity-100' : 'opacity-0',
            matches ? 'text-teal-300' : 'text-red-300',
          ]"
          aria-live="polite"
        >
          <MorphIcon
            :icon="matches ? Check : X"
            :size="14"
            :stroke-width="2.5"
            spring="bouncy"
            reduced-motion="user"
          />
          <span>{{ confirm && matches ? 'Matches' : '' }}</span>
        </span>
      </template>
    </AuthField>
    <div class="mt-auto flex flex-col gap-4 pt-4 sm:mt-2 sm:pt-0">
      <AuthSubmit :loading="loading">Create Account</AuthSubmit>
      <p class="hidden text-center text-xs text-teal-100/70 sm:block">
        Already have an account?
        <RouterLink
          to="/login"
          class="font-medium text-teal-300 no-underline outline-none hover:text-teal-200 focus-visible:underline"
          >Sign in</RouterLink
        >
      </p>
    </div>
  </form>
</template>
