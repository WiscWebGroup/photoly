<script setup lang="ts">
import { ref } from 'vue'
import { onClickOutside, onKeyStroke, useIntersectionObserver } from '@vueuse/core'
import { ArrowRight, ExternalLink, FolderOpen, Share2, Sparkles } from '@lucide/vue'
import { MorphIcon } from 'morphicons/vue'
import { Menu, X } from 'lucide'
import NightBackdrop from '@/components/NightBackdrop.vue'
import HeroMosaic from '@/components/welcome/HeroMosaic.vue'

const GITHUB = 'https://github.com/WiscWebGroup/photoly'

const features = [
  {
    icon: FolderOpen,
    title: 'Organize your way',
    text: 'Folders for where things live, tags and galleries for what they mean. EXIF and GPS are read on upload.',
  },
  {
    icon: Sparkles,
    title: 'Search by meaning',
    text: 'Describe a scene in English or Chinese, drop in an image, or find text inside photos.',
  },
  {
    icon: Share2,
    title: 'Share and integrate',
    text: 'Share one photo by link or QR code. Scoped API tokens connect your other apps.',
  },
]

const team = [
  {
    name: 'WiscWebGroup',
    github: 'WiscWebGroup',
    avatar: 108884969,
    bio: 'A Madison, Wisconsin team that builds web applications we find interesting.',
  },
  {
    name: 'Yixuan Ye (Harold-y)',
    github: 'Harold-y',
    avatar: 68500948,
    bio: 'UW-Madison grad. Built the database, backend and CI/CD, part of the frontend, and designed the logo.',
  },
  {
    name: 'Jeffrey Hui (Slijeff)',
    github: 'Slijeff',
    avatar: 31459252,
    bio: 'UW-Madison grad. Built part of the frontend and shot the background photos for the home page.',
  },
  {
    name: 'Zihan Zhao (NataRich)',
    github: 'NataRich',
    avatar: 46753484,
    bio: 'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
  },
]

// Phone-width navigation collapses into a menu.
const menuOpen = ref(false)
const menu = ref<HTMLElement>()
onClickOutside(menu, () => (menuOpen.value = false))
onKeyStroke('Escape', () => (menuOpen.value = false))

// Sections below the fold rise in as they scroll into view.
const featuresEl = ref<HTMLElement>()
const teamEl = ref<HTMLElement>()
const seen = ref(new Set<string>())
for (const [key, el] of [
  ['features', featuresEl],
  ['team', teamEl],
] as const) {
  const { stop } = useIntersectionObserver(
    el,
    ([entry]) => {
      if (!entry?.isIntersecting) return
      seen.value = new Set(seen.value).add(key)
      stop()
    },
    { threshold: 0.15 },
  )
}
const revealClass = (key: string) => (seen.value.has(key) ? 'animate-rise' : 'opacity-0')

const ghost =
  'flex h-9 items-center rounded-lg px-3.5 font-medium text-white transition-colors outline-none hover:bg-white/8 focus-visible:ring-3 focus-visible:ring-teal-300/50'
const card =
  'rounded-2xl border border-white/10 bg-white/5 transition-[transform,border-color,background-color] duration-300 ease-(--ease-out-quart) hover:-translate-y-1 hover:border-teal-300/40'
</script>

<template>
  <div class="relative min-h-dvh overflow-hidden bg-[#031417] text-sm text-white">
    <NightBackdrop variant="hero" />

    <div class="relative mx-auto flex max-w-[1200px] flex-col gap-14 px-4 pb-16 sm:px-10">
      <header class="animate-fade relative z-10 flex h-[88px] items-center gap-2">
        <RouterLink
          to="/"
          class="flex items-center rounded-md outline-none focus-visible:ring-3 focus-visible:ring-teal-300/50"
        >
          <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-8" />
        </RouterLink>

        <nav aria-label="Main" class="ml-auto hidden items-center gap-2 sm:flex">
          <a :href="GITHUB" :class="ghost">GitHub</a>
          <RouterLink to="/login" :class="ghost">Sign In</RouterLink>
          <RouterLink
            to="/signup"
            class="flex h-9 items-center rounded-lg border border-white/20 px-3.5 font-medium text-white transition-colors outline-none hover:border-teal-300/50 hover:bg-white/5 focus-visible:ring-3 focus-visible:ring-teal-300/50"
            >Sign Up</RouterLink
          >
        </nav>

        <div ref="menu" class="relative ml-auto sm:hidden">
          <button
            type="button"
            class="flex size-11 items-center justify-center rounded-lg text-white transition-colors outline-none hover:bg-white/8 focus-visible:ring-3 focus-visible:ring-teal-300/50"
            :aria-expanded="menuOpen"
            aria-controls="welcome-menu"
            aria-label="Menu"
            @click="menuOpen = !menuOpen"
          >
            <MorphIcon
              :icon="menuOpen ? X : Menu"
              :size="22"
              spring="bouncy"
              reduced-motion="user"
            />
          </button>
          <Transition name="reveal">
            <nav
              v-show="menuOpen"
              id="welcome-menu"
              aria-label="Main"
              class="absolute top-12 right-0 flex w-48 flex-col gap-1 rounded-xl border border-white/12 bg-[#031417]/95 p-1.5 shadow-[0_12px_32px_rgba(0,0,0,.4)] backdrop-blur-md"
            >
              <RouterLink to="/login" :class="[ghost, 'h-11']">Sign In</RouterLink>
              <RouterLink to="/signup" :class="[ghost, 'h-11']">Sign Up</RouterLink>
              <a :href="GITHUB" :class="[ghost, 'h-11']">GitHub</a>
            </nav>
          </Transition>
        </div>
      </header>

      <section
        class="grid items-center gap-14 min-[900px]:grid-cols-[minmax(0,1.05fr)_minmax(0,1fr)]"
      >
        <div class="flex flex-col items-start gap-6">
          <span
            class="animate-rise inline-flex h-7 items-center rounded-full border border-teal-300/30 bg-teal-300/10 px-3 tracking-wide text-teal-200"
          >
            Self-hosted photo and video library
          </span>
          <h1
            style="animation-delay: 80ms"
            class="animate-rise text-5xl leading-[52px] font-semibold tracking-tight min-[900px]:text-7xl min-[900px]:leading-[76px]"
          >
            Your photos, <span class="text-teal-300">on your terms</span>
          </h1>
          <p
            style="animation-delay: 160ms"
            class="animate-rise max-w-[480px] text-lg leading-7 text-teal-100/75"
          >
            Organize with folders, tags and galleries. Find anything by describing it. Share a link,
            or plug in through the API — all on a server you own.
          </p>
          <div style="animation-delay: 240ms" class="animate-rise flex flex-wrap gap-3 pt-2">
            <RouterLink
              to="/signup"
              class="group flex h-11 items-center gap-2 rounded-[10px] bg-teal-400 px-[22px] text-base font-semibold text-teal-950 shadow-[0_0_32px_-4px] shadow-teal-400/60 transition-[background-color,box-shadow,transform] duration-200 outline-none hover:bg-teal-300 hover:shadow-teal-300/70 focus-visible:ring-3 focus-visible:ring-teal-200/60 active:scale-[0.97]"
            >
              Get Started
              <ArrowRight
                class="size-4 transition-transform duration-300 ease-(--ease-out-quart) group-hover:translate-x-1"
              />
            </RouterLink>
            <a
              :href="GITHUB"
              class="flex h-11 items-center rounded-[10px] border border-white/16 bg-white/4 px-[18px] text-base font-medium text-white transition-colors outline-none hover:bg-white/8 focus-visible:ring-3 focus-visible:ring-teal-300/50"
              >Star us on GitHub</a
            >
          </div>
        </div>

        <HeroMosaic />
      </section>

      <section ref="featuresEl" aria-label="Features" class="grid gap-4 min-[900px]:grid-cols-3">
        <div
          v-for="(f, i) in features"
          :key="f.title"
          :style="{ animationDelay: `${i * 90}ms` }"
          :class="[card, revealClass('features')]"
          class="flex flex-col gap-2.5 p-6"
        >
          <component :is="f.icon" class="size-[22px] text-teal-300" />
          <h2 class="text-base font-semibold">{{ f.title }}</h2>
          <p class="text-teal-100/75">{{ f.text }}</p>
        </div>
      </section>

      <section ref="teamEl" class="flex flex-col gap-6 pt-8">
        <h2 :class="revealClass('team')" class="text-3xl font-semibold tracking-tight sm:text-4xl">
          Built by <span class="text-teal-300">people who love photos</span>
        </h2>
        <div class="grid gap-4 md:grid-cols-2">
          <a
            v-for="(p, i) in team"
            :key="p.github"
            :href="`https://github.com/${p.github}`"
            :style="{ animationDelay: `${80 + i * 80}ms` }"
            :class="[card, revealClass('team')]"
            class="group flex items-start gap-4 p-5 outline-none focus-visible:ring-3 focus-visible:ring-teal-300/50"
          >
            <img
              :src="`https://avatars.githubusercontent.com/u/${p.avatar}`"
              alt=""
              loading="lazy"
              class="size-14 shrink-0 rounded-full ring-2 ring-teal-300/30"
            />
            <div class="flex flex-col gap-1">
              <span class="flex items-center gap-1 text-base font-semibold">
                {{ p.name }}
                <ExternalLink
                  class="size-3.5 text-teal-300 opacity-0 transition-opacity group-hover:opacity-100"
                />
              </span>
              <p class="text-teal-100/75">{{ p.bio }}</p>
            </div>
          </a>
        </div>
      </section>
    </div>
  </div>
</template>
