<script setup lang="ts">
import { useColorMode } from '@vueuse/core'
import { Toaster } from '@/components/ui/sonner'

const { state } = useColorMode()
</script>

<template>
  <router-view />
  <Toaster rich-colors position="top-center" :theme="state" />
</template>
