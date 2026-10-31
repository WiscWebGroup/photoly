<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { onKeyStroke, useStorage } from '@vueuse/core'
import { toast } from 'vue-sonner'
import { CalendarDays, Camera, Images, LogOut, Map, Search, Tag, ChevronLeft } from '@lucide/vue'
import { Avatar, AvatarImage } from '@/components/ui/avatar'
import OnlineDownloadButton from '@/components/OnlineDownloadButton.vue'
import UploadButton from '@/components/UploadButton.vue'
import StorageActivity from '@/components/StorageActivity.vue'
import { resetStorageState } from '@/composables/useStorageRevision'
import { useUploadsStore } from '@/stores/uploads'
import ThemeToggle from '@/components/ThemeToggle.vue'
import { useProfile } from '@/composables/useProfile'
import { storageLabel, useStorageUsage } from '@/composables/useStorageUsage'
import { useAuthStore } from '@/stores/auth'
import { cn } from '@/lib/utils'

const sidebarHidden = useStorage('photoly-sidebar-hidden', false)
const auth = useAuthStore()
watch(() => auth.token, resetStorageState)
const router = useRouter()
const route = useRoute()
const { profile, load } = useProfile()
onMounted(() => load().catch(() => {}))

// Uploads and deletions happen on other pages, so re-read usage whenever the page changes.
const storage = useStorageUsage()
const uploads = useUploadsStore()
watch(() => uploads.revision, storage.refresh)
watch(() => route.params.part, storage.refresh, { immediate: true })

type NavItem = { part: string; label: string; icon: typeof Camera; key?: string }
const groups: { label: string; items: NavItem[] }[] = [
  {
    label: 'Library',
    items: [
      { part: 'HomePart', label: 'Photos', icon: Camera },
      { part: 'MapPart', label: 'Map', icon: Map },
    ],
  },
  {
    label: 'Organize',
    items: [
      { part: 'GalleryPart', label: 'Galleries', icon: Images },
      { part: 'TagPart', label: 'Tags', icon: Tag },
      { part: 'DatesPart', label: 'Dates', icon: CalendarDays },
    ],
  },
  { label: 'Find', items: [{ part: 'SearchPart', label: 'Search', icon: Search, key: '/' }] },
]

// Phones get five tabs; galleries and tags share "Albums", the account sits behind the avatar.
const tabs = [
  { part: 'HomePart', label: 'Photos', icon: Camera, also: [] as string[] },
  { part: 'GalleryPart', label: 'Albums', icon: Images, also: ['TagPart'] },
  { part: 'DatesPart', label: 'Dates', icon: CalendarDays, also: [] },
  { part: 'MapPart', label: 'Map', icon: Map, also: [] },
  { part: 'SearchPart', label: 'Search', icon: Search, also: [] },
]

const part = computed(() => route.params.part as string | undefined)
const onAccount = computed(() => part.value === 'ManagePart')

// "/" jumps to search from anywhere that isn't a text field.
onKeyStroke('/', (e) => {
  const el = e.target as HTMLElement | null
  if (e.metaKey || e.ctrlKey || e.altKey) return
  if (el?.closest('input, textarea, select, [contenteditable="true"], [role="dialog"]')) return
  e.preventDefault()
  router.push('/home/SearchPart')
})

async function signOut() {
  try {
    await auth.signOut()
  } catch {
    toast.error('Could not reach the server; you were signed out on this device only.')
  }
  router.push('/')
}

const link =
  'flex h-9 items-center gap-2.5 rounded-lg px-2.5 text-sm font-medium text-foreground transition-colors outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-3 focus-visible:ring-ring/50'
const iconButton =
  'text-muted-foreground hover:bg-accent hover:text-accent-foreground grid size-8 place-items-center rounded-lg transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50'
</script>

<template>
  <div class="bg-background flex h-dvh flex-col md:flex-row">
    <!-- Desktop sidebar -->
    <div
      class="sidebar-rail relative hidden shrink-0 md:block"
      :class="{ 'is-collapsed': sidebarHidden }"
    >
      <div class="h-full overflow-hidden">
        <aside
          id="main-sidebar"
          :inert="sidebarHidden"
          :aria-hidden="sidebarHidden"
          class="bg-card flex h-full w-58 flex-col gap-7 border-r px-3 pt-5 pb-4"
        >
          <router-link
            to="/home/HomePart"
            class="flex h-8 items-center self-start rounded-md px-2.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
          >
            <img
              src="@/assets/photoly-wordmark.svg"
              alt="Photoly"
              class="h-6 invert dark:invert-0"
            />
          </router-link>

          <nav aria-label="Main" class="flex flex-col gap-5">
            <div v-for="group in groups" :key="group.label" class="flex flex-col gap-0.5">
              <div
                class="text-muted-foreground px-2.5 pb-1.5 text-[11px] leading-4 font-semibold tracking-[0.06em] uppercase"
              >
                {{ group.label }}
              </div>
              <router-link
                v-for="item in group.items"
                :key="item.part"
                :to="`/home/${item.part}`"
                :class="link"
                active-class="!bg-accent !text-accent-foreground"
              >
                <component :is="item.icon" class="size-4" aria-hidden="true" />
                {{ item.label }}
                <kbd
                  v-if="item.key"
                  class="text-muted-foreground ml-auto rounded border px-1 font-mono text-[11px] leading-[18px] font-normal"
                  aria-hidden="true"
                  >{{ item.key }}</kbd
                >
              </router-link>
            </div>
          </nav>

          <div class="mt-auto flex flex-col gap-4">
            <div v-if="storage.usage.value" class="flex flex-col gap-2 px-2.5">
              <div class="flex justify-between text-xs">
                <span class="font-medium">Storage</span>
                <span class="text-muted-foreground">{{ storageLabel(storage.usage.value) }}</span>
              </div>
              <div
                role="progressbar"
                aria-label="Storage used"
                :aria-valuenow="storage.percent.value"
                aria-valuemin="0"
                aria-valuemax="100"
                :aria-valuetext="storageLabel(storage.usage.value)"
                class="bg-muted h-1 overflow-hidden rounded-full"
              >
                <div
                  class="bg-primary h-full rounded-full"
                  :style="{ width: `${storage.percent.value}%` }"
                />
              </div>
              <p v-if="storage.usage.value.backend" class="text-muted-foreground text-[11px]">
                Using {{ storage.usage.value.backend }}
              </p>
            </div>
            <div class="flex items-center gap-1 border-t pt-3">
              <router-link
                to="/home/ManagePart"
                aria-label="Account"
                :aria-current="onAccount ? 'page' : undefined"
                :class="
                  cn(
                    'flex min-w-0 flex-1 items-center gap-2.5 rounded-lg p-1 transition-colors outline-none hover:bg-accent focus-visible:ring-3 focus-visible:ring-ring/50',
                    onAccount && 'bg-accent text-accent-foreground',
                  )
                "
              >
                <Avatar class="size-8 shrink-0"
                  ><AvatarImage :src="auth.avatarUrl" alt=""
                /></Avatar>
                <span class="flex min-w-0 flex-col">
                  <span class="truncate text-sm font-medium">{{
                    profile?.userName ?? 'Account'
                  }}</span>
                  <span
                    v-if="profile"
                    :class="['text-xs capitalize', !onAccount && 'text-muted-foreground']"
                    >{{ profile.role }}</span
                  >
                </span>
              </router-link>
              <ThemeToggle />
              <button type="button" aria-label="Sign out" :class="iconButton" @click="signOut">
                <LogOut class="size-4" aria-hidden="true" />
              </button>
            </div>
          </div>
        </aside>
      </div>
      <button
        type="button"
        :aria-label="sidebarHidden ? 'Show sidebar' : 'Hide sidebar'"
        :title="sidebarHidden ? 'Show sidebar' : 'Hide sidebar'"
        :aria-expanded="!sidebarHidden"
        aria-controls="main-sidebar"
        class="sidebar-toggle bg-card text-muted-foreground hover:bg-accent hover:text-foreground absolute top-5 left-full z-30 grid size-7 place-items-center rounded-full border shadow-xs outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        @click="sidebarHidden = !sidebarHidden"
      >
        <ChevronLeft class="size-4" aria-hidden="true" />
      </button>
    </div>

    <!-- Mobile top bar -->
    <header
      class="bg-card flex min-h-14 shrink-0 items-center gap-1 border-b pt-[env(safe-area-inset-top)] pr-2 pl-4 md:hidden"
    >
      <router-link to="/home/HomePart" class="rounded-md">
        <img
          src="@/assets/photoly-wordmark.svg"
          alt="Photoly"
          class="h-[22px] invert dark:invert-0"
        />
      </router-link>
      <div class="ml-auto flex items-center gap-1">
        <OnlineDownloadButton /><UploadButton /><StorageActivity />
      </div>
      <router-link
        to="/home/ManagePart"
        aria-label="Account"
        :aria-current="onAccount ? 'page' : undefined"
        class="grid size-11 place-items-center rounded-full outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <Avatar :class="cn('ring-border size-8 ring-1', onAccount && 'ring-primary ring-2')"
          ><AvatarImage :src="auth.avatarUrl" alt=""
        /></Avatar>
      </router-link>
    </header>

    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <header
        class="bg-card hidden h-14 shrink-0 items-center justify-end gap-3 border-b px-10 md:flex"
      >
        <router-link
          to="/home/SearchPart"
          class="text-muted-foreground bg-background flex h-9 w-70 items-center gap-2 rounded-lg border px-3 text-sm"
          ><Search class="size-4" /><span class="flex-1">Search photos</span
          ><kbd class="rounded border px-1 text-xs">/</kbd></router-link
        >
        <OnlineDownloadButton />
        <UploadButton /><StorageActivity />
      </header>
      <main class="min-h-0 flex-1 overflow-auto"><slot /></main>
    </div>

    <!-- Mobile bottom tab bar -->
    <nav
      aria-label="Main"
      class="bg-card grid shrink-0 grid-cols-5 border-t pb-[env(safe-area-inset-bottom)] md:hidden"
    >
      <router-link
        v-for="tab in tabs"
        :key="tab.part"
        :to="`/home/${tab.part}`"
        :aria-current="part === tab.part || tab.also.includes(part ?? '') ? 'page' : undefined"
        class="text-muted-foreground aria-[current=page]:text-primary flex h-16 flex-col items-center justify-center gap-[3px] text-xs transition-colors aria-[current=page]:font-medium [&>svg]:transition-transform aria-[current=page]:[&>svg]:scale-110"
      >
        <component :is="tab.icon" class="size-5" aria-hidden="true" />
        {{ tab.label }}
      </router-link>
    </nav>
  </div>
</template>

<style scoped>
.sidebar-rail {
  width: 14.5rem;
  transition: width 280ms cubic-bezier(0.22, 1, 0.36, 1);
}
.sidebar-rail.is-collapsed {
  width: 0;
}
#main-sidebar {
  transition:
    opacity 180ms ease,
    transform 280ms ease,
    visibility 0s;
}
.is-collapsed #main-sidebar {
  opacity: 0;
  transform: translateX(-20px);
  visibility: hidden;
  transition:
    opacity 180ms ease,
    transform 280ms ease,
    visibility 0s 280ms;
}
.sidebar-toggle {
  transform: translateX(-50%);
  transition:
    transform 280ms cubic-bezier(0.22, 1, 0.36, 1),
    background-color 150ms ease,
    color 150ms ease;
}
.is-collapsed .sidebar-toggle {
  transform: translateX(4px);
}
.sidebar-toggle svg {
  transition: transform 280ms ease;
}
.is-collapsed .sidebar-toggle svg {
  transform: rotate(180deg);
}
@media (prefers-reduced-motion: reduce) {
  .sidebar-rail,
  #main-sidebar,
  .is-collapsed #main-sidebar,
  .sidebar-toggle,
  .sidebar-toggle svg {
    transition: none;
  }
}
</style>
