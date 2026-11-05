<script setup lang="ts">
import { useColorMode } from '@vueuse/core'
import { useRoute } from 'vue-router'
const route = useRoute()
import UploadCenter from '@/components/UploadCenter.vue'
import { Toaster } from '@/components/ui/sonner'

const { state } = useColorMode()
</script>

<template>
  <router-view />
  <UploadCenter
    v-if="
      route.matched.length > 0 &&
      !['/maintenance', '/thepersonincharge/maintenance', '/login', '/legal'].includes(route.path)
    "
  />
  <Toaster rich-colors position="top-center" :theme="state" />
</template>
