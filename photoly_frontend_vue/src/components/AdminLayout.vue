<script setup lang="ts">
import { Button } from '@/components/ui/button'
import ThemeToggle from '@/components/ThemeToggle.vue'
</script>

<template>
  <div class="bg-background min-h-dvh">
    <header class="bg-primary text-primary-foreground flex h-16 items-center gap-3 px-4">
      <router-link to="/thepersonincharge/dashboard" class="flex items-center gap-2">
        <img src="@/assets/logo.ico" alt="" class="size-10 rounded-full" />
        <span class="text-lg font-semibold">PHOTOLY ADMIN</span>
      </router-link>
      <nav class="ml-4 flex gap-1">
        <Button variant="ghost" as-child class="hover:text-primary-foreground">
          <router-link to="/thepersonincharge/dashboard">Dashboard</router-link>
        </Button>
        <Button variant="ghost" as-child class="hover:text-primary-foreground">
          <router-link to="/thepersonincharge/usermanage">User Mng</router-link>
        </Button>
      </nav>
      <span class="ml-auto"><ThemeToggle /></span>
    </header>
    <main class="mx-auto flex max-w-5xl flex-col gap-4 p-4 md:p-8">
      <slot />
    </main>
  </div>
</template>
