<script setup lang="ts">
import { ArrowLeft, HardDrive, Server, Sparkles, Users } from '@lucide/vue'
import UploadButton from '@/components/UploadButton.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

withDefaults(defineProps<{ wide?: boolean }>(), { wide: false })

const nav = [
  { to: '/thepersonincharge/dashboard', label: 'Overview', icon: Server },
  { to: '/thepersonincharge/usermanage', label: 'Users', icon: Users },
  { to: '/thepersonincharge/ai', label: 'AI', icon: Sparkles },
  { to: '/thepersonincharge/storage', label: 'Storage', icon: HardDrive },
]

const link =
  'flex h-9 items-center gap-2.5 rounded-lg px-2.5 text-sm font-medium transition-colors outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-3 focus-visible:ring-ring/50'
const pill =
  'bg-foreground text-background rounded-full px-1.5 text-[11px] leading-5 font-semibold tracking-[0.06em] uppercase'
</script>

<template>
  <div class="bg-background flex h-dvh w-full min-w-0 flex-col overflow-hidden md:flex-row">
    <!-- Desktop sidebar -->
    <aside class="bg-card hidden w-58 shrink-0 flex-col gap-7 border-r px-3 pt-5 pb-4 md:flex">
      <router-link
        to="/thepersonincharge/dashboard"
        class="flex h-8 items-center gap-2 self-start rounded-md px-2.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-6 invert dark:invert-0" />
        <span :class="pill">Admin</span>
      </router-link>
      <nav aria-label="Admin" class="flex flex-col gap-0.5">
        <router-link
          v-for="item in nav"
          :key="item.to"
          :to="item.to"
          :class="link"
          active-class="!bg-accent !text-accent-foreground"
        >
          <component :is="item.icon" class="size-4" aria-hidden="true" />
          {{ item.label }}
        </router-link>
      </nav>
      <div class="mt-auto flex items-center gap-1">
        <router-link to="/home/HomePart" :class="[link, 'text-muted-foreground flex-1']">
          <ArrowLeft class="size-4" aria-hidden="true" />
          Back to Library
        </router-link>
        <ThemeToggle />
      </div>
    </aside>

    <!-- Mobile top bar -->
    <header
      class="bg-card flex min-h-14 shrink-0 items-center gap-2 border-b pt-[env(safe-area-inset-top)] pr-2 pl-4 md:hidden"
    >
      <img
        src="@/assets/photoly-wordmark.svg"
        alt="Photoly"
        class="h-[22px] invert dark:invert-0"
      />
      <span :class="pill">Admin</span>
      <UploadButton class="ml-auto" /><ThemeToggle />
    </header>

    <main class="min-h-0 min-w-0 flex-1 overflow-auto">
      <header
        class="bg-card sticky top-0 z-20 hidden h-14 items-center justify-end gap-3 border-b px-10 md:flex"
      >
        <router-link
          to="/home/SearchPart"
          class="text-muted-foreground rounded-lg border px-3 py-2 text-sm"
          >Search photos</router-link
        ><UploadButton />
      </header>
      <div
        :class="[
          'animate-rise flex w-full min-w-0 flex-col gap-6 px-4 pt-5 pb-10 md:px-10 md:pt-10 md:pb-16',
          wide ? 'max-w-[1120px]' : 'max-w-[1040px]',
        ]"
      >
        <slot />
      </div>
    </main>

    <!-- Mobile bottom tab bar -->
    <nav
      aria-label="Admin"
      class="bg-card grid shrink-0 grid-cols-5 border-t pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      <router-link
        v-for="item in nav"
        :key="item.to"
        :to="item.to"
        class="text-muted-foreground flex h-16 flex-col items-center justify-center gap-[3px] text-xs transition-colors [&>svg]:transition-transform"
        active-class="!text-primary font-medium [&>svg]:scale-110"
      >
        <component :is="item.icon" class="size-5" aria-hidden="true" />
        {{ item.label }}
      </router-link>
      <router-link
        to="/home/HomePart"
        class="text-muted-foreground flex h-16 flex-col items-center justify-center gap-[3px] text-xs"
      >
        <ArrowLeft class="size-5" aria-hidden="true" />
        Library
      </router-link>
    </nav>
  </div>
</template>
