<script setup lang="ts">
import { computed } from 'vue'
import { useColorMode } from '@vueuse/core'
import { darkTheme, NConfigProvider, NMessageProvider } from 'naive-ui'
import { Toaster } from '@/components/ui/sonner'

// Naive UI (until Phase 5) follows the same light/dark state as the Tailwind tokens.
const { state } = useColorMode()
const theme = computed(() => (state.value === 'dark' ? darkTheme : null))
</script>

<template>
  <n-config-provider :theme="theme">
    <n-message-provider>
      <router-view />
    </n-message-provider>
  </n-config-provider>
  <Toaster rich-colors :theme="state" />
</template>
