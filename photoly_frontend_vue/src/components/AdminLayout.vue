<script setup lang="ts">
import { ArrowLeft, LayoutDashboard, Users, Sparkles } from '@lucide/vue'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { cn } from '@/lib/utils'

const nav = [
  { to: '/thepersonincharge/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/thepersonincharge/ai', label: 'AI Management', icon: Sparkles },
  { to: '/thepersonincharge/usermanage', label: 'User Management', icon: Users },
]

const linkClass =
  'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors'
const idle = 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
</script>

<template>
  <div class="bg-background flex h-dvh flex-col md:flex-row">
    <!-- Desktop sidebar -->
    <aside class="bg-card hidden w-56 shrink-0 flex-col border-r p-3 md:flex">
      <router-link to="/thepersonincharge/dashboard" class="mb-4 flex items-center gap-2 px-3 py-2">
        <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-7 invert dark:invert-0" />
        <span class="text-muted-foreground text-xs font-semibold tracking-wide">ADMIN</span>
      </router-link>
      <nav aria-label="Admin" class="flex flex-1 flex-col gap-1">
        <router-link
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          :class="cn(linkClass, idle)"
          active-class="!bg-accent !text-accent-foreground"
        >
          <component :is="item.icon" class="size-4" />
          {{ item.label }}
        </router-link>
      </nav>
      <div class="flex items-center gap-1 border-t pt-3">
        <router-link to="/home/HomePart" :class="cn(linkClass, idle)">
          <ArrowLeft class="size-4" />
          Back to Home
        </router-link>
        <span class="ml-auto"><ThemeToggle /></span>
      </div>
    </aside>

    <!-- Mobile top bar -->
    <header
      class="bg-card flex min-h-14 shrink-0 items-center gap-2 border-b px-4 pt-[env(safe-area-inset-top)] md:hidden"
    >
      <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-7 invert dark:invert-0" />
      <span class="text-muted-foreground text-xs font-semibold tracking-wide">ADMIN</span>
      <span class="ml-auto"><ThemeToggle /></span>
    </header>

    <main class="min-h-0 flex-1 overflow-auto">
      <div class="mx-auto flex max-w-5xl flex-col gap-4 p-4 md:p-8">
        <slot />
      </div>
    </main>

    <!-- Mobile bottom tab bar -->
    <nav
      aria-label="Admin"
      class="bg-card grid shrink-0 grid-cols-4 border-t pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      <router-link
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="text-muted-foreground flex min-h-14 flex-col items-center justify-center gap-0.5 text-[11px]"
        active-class="!text-primary"
      >
        <component :is="item.icon" class="size-5" />
        {{ item.label }}
      </router-link>
      <router-link
        to="/home/HomePart"
        class="text-muted-foreground flex min-h-14 flex-col items-center justify-center gap-0.5 text-[11px]"
      >
        <ArrowLeft class="size-5" />
        Home
      </router-link>
    </nav>
  </div>
</template>
