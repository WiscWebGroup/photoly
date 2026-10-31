<script setup lang="ts">
import { computed, onMounted, ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useResizeObserver } from '@vueuse/core'
import NightBackdrop from '@/components/NightBackdrop.vue'
import { useDemo } from '@/composables/useDemo'

const route = useRoute()
const router = useRouter()
const isSignup = computed(() => route.name === 'signup')

// A demo instance only offers the shared demo login: no sign-up tab, and /signup bounces.
const demo = useDemo()
const tabs = computed(() =>
  demo.enabled
    ? ([{ name: 'login', label: 'Sign In' }] as const)
    : ([
        { name: 'login', label: 'Sign In' },
        { name: 'signup', label: 'Sign Up' },
      ] as const),
)
watchEffect(() => {
  if (demo.enabled && route.name === 'signup') void router.replace({ name: 'login' })
})

// Fetch both forms up front so switching never waits on the network.
onMounted(() => {
  void import('@/views/Login.vue')
  void import('@/views/Signup.vue')
})

// On wider screens the card animates its height between the two forms instead of jumping.
const content = ref<HTMLElement>()
const height = ref<number>()
// Clip only while the height animates, so the button glow and focus rings show the rest of the time.
const clipping = ref(false)
function onHeightTransition(e: TransitionEvent, running: boolean) {
  if (e.target === e.currentTarget && e.propertyName === 'height') clipping.value = running
}
useResizeObserver(content, ([entry]) => {
  if (entry) height.value = entry.target.getBoundingClientRect().height
})
</script>

<template>
  <div
    class="relative flex min-h-dvh flex-col overflow-hidden bg-[#031417] px-6 pt-14 pb-8 text-sm text-white sm:items-center sm:justify-center sm:gap-8 sm:px-4 sm:py-12"
  >
    <NightBackdrop variant="ring" />

    <RouterLink
      to="/"
      class="animate-rise relative flex self-start rounded-md outline-none focus-visible:ring-3 focus-visible:ring-teal-300/50 sm:self-center"
    >
      <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-8 sm:h-9" />
    </RouterLink>

    <main
      style="animation-delay: 80ms"
      class="animate-rise relative mt-8 flex w-full flex-1 flex-col gap-8 transition-[max-width] duration-500 ease-(--ease-out-quart) sm:mt-0 sm:flex-none sm:gap-6 sm:rounded-[18px] sm:border sm:border-white/10 sm:bg-white/5 sm:p-8 sm:shadow-[0_24px_64px_rgba(0,0,0,.35)] sm:backdrop-blur-sm"
      :class="isSignup ? 'sm:max-w-[440px]' : 'sm:max-w-[400px]'"
    >
      <nav
        v-if="tabs.length > 1"
        aria-label="Account"
        class="relative order-2 grid grid-cols-2 rounded-xl bg-white/6 p-[3px] sm:order-1 sm:rounded-[10px]"
      >
        <span
          aria-hidden="true"
          class="absolute inset-y-[3px] left-[3px] w-[calc(50%-3px)] rounded-[9px] bg-white/12 shadow-sm transition-transform duration-500 ease-[cubic-bezier(0.34,1.36,0.64,1)] sm:rounded-lg"
          :class="isSignup ? 'translate-x-full' : 'translate-x-0'"
        />
        <RouterLink
          v-for="tab in tabs"
          :key="tab.name"
          :to="{ name: tab.name }"
          class="relative flex h-10 items-center justify-center rounded-[9px] font-medium text-teal-100/75 transition-colors duration-300 outline-none hover:text-white focus-visible:ring-3 focus-visible:ring-teal-300/50 sm:h-[34px] sm:rounded-lg [&.router-link-exact-active]:text-white"
          >{{ tab.label }}</RouterLink
        >
      </nav>

      <Transition name="auth-swap" mode="out-in">
        <div
          :key="String(route.name)"
          class="order-1 flex flex-col gap-1.5 sm:order-2 sm:text-center"
        >
          <h1 class="text-3xl font-semibold tracking-tight sm:text-2xl">
            {{ route.meta.authTitle }}
          </h1>
          <p class="text-teal-100/75">{{ route.meta.authSubtitle }}</p>
        </div>
      </Transition>

      <div
        class="order-3 flex flex-1 flex-col transition-[height] duration-400 ease-(--ease-out-quart) sm:h-(--card-h) sm:flex-none"
        :class="clipping && 'overflow-clip [overflow-clip-margin:24px]'"
        :style="height ? { '--card-h': `${height}px` } : undefined"
        @transitionrun="onHeightTransition($event, true)"
        @transitionend="onHeightTransition($event, false)"
        @transitioncancel="onHeightTransition($event, false)"
      >
        <div ref="content" class="flex flex-1 flex-col sm:flex-none">
          <RouterView v-slot="{ Component }">
            <Transition name="auth-swap" mode="out-in">
              <component :is="Component" />
            </Transition>
          </RouterView>
        </div>
      </div>
    </main>

    <p
      style="animation-delay: 160ms"
      class="animate-rise relative hidden text-xs text-teal-100/60 sm:block"
    >
      Self-hosted · your photos stay on your server
    </p>
  </div>
</template>

<style scoped>
.auth-swap-enter-active,
.auth-swap-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.25s var(--ease-out-quart),
    filter 0.18s ease;
}
.auth-swap-enter-from {
  opacity: 0;
  transform: translateY(6px);
  filter: blur(2px);
}
.auth-swap-leave-to {
  opacity: 0;
  transform: translateY(-4px);
  filter: blur(2px);
}
</style>
