<script setup lang="ts">
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { Camera, Images, LogOut, Map, Search, Tag, User } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Avatar, AvatarImage } from '@/components/ui/avatar'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { useAuthStore } from '@/stores/auth'
import { cn } from '@/lib/utils'

const auth = useAuthStore()
const router = useRouter()

const nav = [
  { part: 'HomePart', label: 'Home', icon: Camera },
  { part: 'MapPart', label: 'Map', icon: Map },
  { part: 'GalleryPart', label: 'Gallery', icon: Images },
  { part: 'TagPart', label: 'Tag', icon: Tag },
  { part: 'SearchPart', label: 'Search', icon: Search },
  { part: 'ManagePart', label: 'User', icon: User },
]

async function logOut() {
  try {
    await auth.signOut()
  } catch {
    toast.error('Could not reach the server; you were signed out on this device only.')
  }
  router.push('/')
}

const linkClass = 'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors'
</script>

<template>
  <div class="bg-background flex h-dvh flex-col md:flex-row">
    <!-- Desktop sidebar -->
    <aside class="bg-card hidden w-56 shrink-0 flex-col border-r p-3 md:flex">
      <router-link to="/home/HomePart" class="mb-4 flex items-center gap-2 px-3 py-2">
        <img src="@/assets/logo.ico" alt="" class="size-8 rounded-full" />
        <span class="text-lg font-bold tracking-wide">PHOTOLY</span>
      </router-link>
      <nav aria-label="Main" class="flex flex-1 flex-col gap-1">
        <router-link
          v-for="item in nav"
          :key="item.part"
          :to="`/home/${item.part}`"
          :class="cn(linkClass, 'text-muted-foreground hover:bg-accent hover:text-accent-foreground')"
          active-class="!bg-accent !text-accent-foreground"
        >
          <component :is="item.icon" class="size-4" />
          {{ item.label }}
        </router-link>
      </nav>
      <div class="flex items-center gap-1 border-t pt-3">
        <router-link to="/home/ManagePart" aria-label="User">
          <Avatar class="size-8"><AvatarImage :src="auth.avatarUrl" alt="" /></Avatar>
        </router-link>
        <span class="ml-auto"><ThemeToggle /></span>
        <Button variant="ghost" size="icon" aria-label="Logout" @click="logOut">
          <LogOut class="size-4" />
        </Button>
      </div>
    </aside>

    <!-- Mobile top bar -->
    <header class="bg-card flex min-h-14 shrink-0 items-center gap-2 border-b px-4 pt-[env(safe-area-inset-top)] md:hidden">
      <img src="@/assets/logo.ico" alt="" class="size-8 rounded-full" />
      <span class="font-bold tracking-wide">PHOTOLY</span>
      <span class="ml-auto"><ThemeToggle /></span>
      <Button variant="ghost" size="icon" aria-label="Logout" @click="logOut">
        <LogOut class="size-4" />
      </Button>
    </header>

    <main class="min-h-0 flex-1 overflow-auto">
      <slot />
    </main>

    <!-- Mobile bottom tab bar -->
    <nav
      aria-label="Main"
      class="bg-card grid shrink-0 grid-cols-6 border-t pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      <router-link
        v-for="item in nav"
        :key="item.part"
        :to="`/home/${item.part}`"
        class="text-muted-foreground flex min-h-14 flex-col items-center justify-center gap-0.5 text-[11px]"
        active-class="!text-primary"
      >
        <component :is="item.icon" class="size-5" />
        {{ item.label }}
      </router-link>
    </nav>
  </div>
</template>
