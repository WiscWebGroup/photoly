<script setup lang="ts">
import { useColorMode } from '@vueuse/core'
import UploadCenter from '@/components/UploadCenter.vue'
import { Toaster } from '@/components/ui/sonner'

const { state } = useColorMode()
</script>

<template>
  <router-view />
  <UploadCenter />
  <Toaster rich-colors position="top-center" :theme="state" />
</template>
