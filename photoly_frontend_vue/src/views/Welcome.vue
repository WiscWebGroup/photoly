<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { ExternalLink, FolderTree, Tags, Code2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'

// Loaded lazily so the WebGPU runtime never blocks first paint of the landing page.
const ShapeWaves = defineAsyncComponent(() => import('@/components/ShapeWaves/ShapeWaves.vue'))

const features = [
  {
    icon: FolderTree,
    title: 'Effortless organizing',
    text: 'Upload, move and sort your photos in a few clicks, with folders, galleries and namespaces.',
  },
  {
    icon: Tags,
    title: 'Tag everything',
    text: 'Label photos your way and find any moment again in seconds.',
  },
  {
    icon: Code2,
    title: 'Open API',
    text: "Plug your library into other apps through Photoly's built-in API.",
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
</script>

<template>
  <div>
    <header
      class="fixed inset-x-0 top-0 z-10 flex items-center justify-between bg-black/20 px-6 py-4 text-white backdrop-blur"
    >
      <div class="flex items-center gap-3">
        <img src="@/assets/photoly-wordmark.svg" alt="Photoly" class="h-10" />
      </div>
      <nav class="flex gap-1">
        <Button variant="ghost" class="hover:text-white" @click="$router.push('/signup')"
          >Sign Up</Button
        >
        <Button variant="ghost" class="hover:text-white" @click="$router.push('/login')"
          >Sign In</Button
        >
      </nav>
    </header>

    <div class="relative overflow-hidden bg-[#031417]">
      <!-- ShapeWaves sets its own `relative`, so position it via a wrapper. It centers its text on the
           canvas, so this box is sized to put that center at the middle of the first screen. -->
      <div class="absolute inset-x-0 top-0 h-dvh">
        <ShapeWaves
          text="Photoly"
          :text-size="0.4"
          shapes="mixed"
          :cell-size="18"
          :dot-size="0.8"
          color="#39bab4"
          hover-color="#83e1db"
          background-color="#031417"
          :speed="0.8"
          :scale="1.2"
          :contrast="2"
          :brightness="1"
          :fade="0.35"
          :flow="1.8"
          :glow="0.8"
          :splash-radius="60"
          :introDuration="2.2"
          @error="(e: Error) => console.warn('ShapeWaves unavailable:', e.message)"
        />
      </div>
      <section
        class="relative flex min-h-dvh flex-col items-center justify-center gap-4 px-4 text-center text-white"
      >
        <!-- Keep the copy legible over the busiest part of the field, and melt into the next section. -->
        <div
          class="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,#031417cc_0%,#03141740_35%,transparent_65%)]"
        />

        <div class="relative flex flex-col items-center gap-4">
          <span
            style="animation-delay: 0ms"
            class="animate-rise rounded-full border border-teal-300/30 bg-teal-300/10 px-4 py-1 text-sm tracking-wide text-teal-200 backdrop-blur"
          >
            Self-hosted photo management
          </span>
          <h1
            style="animation-delay: 80ms"
            class="animate-rise max-w-3xl text-5xl leading-tight font-semibold tracking-tight md:text-7xl"
          >
            See the Value of Photo Inside
          </h1>
          <p
            style="animation-delay: 160ms"
            class="animate-rise bg-gradient-to-r from-teal-200 via-white to-teal-200 bg-clip-text text-3xl text-transparent md:text-4xl"
          >
            The Beauty Shines Here
          </p>
          <p
            style="animation-delay: 240ms"
            class="animate-rise text-lg text-teal-100/70 md:text-xl"
          >
            Photoly - Share Your Inspiration
          </p>
          <div style="animation-delay: 320ms" class="animate-rise mt-6 flex gap-4">
            <Button
              size="lg"
              class="bg-teal-400 text-teal-950 shadow-[0_0_32px_-4px] shadow-teal-400/60 hover:bg-teal-300"
              @click="$router.push('/signup')"
              >Get Started</Button
            >
            <Button
              variant="outline"
              size="lg"
              class="border-white/25 bg-white/5 text-white backdrop-blur hover:bg-white/15 hover:text-white"
              @click="$router.push('/login')"
              >Sign In</Button
            >
          </div>
        </div>
      </section>

      <section class="relative flex flex-col items-center gap-12 px-4 pt-8 pb-24 text-white">
        <div class="flex max-w-2xl flex-col items-center gap-4 text-center">
          <span
            class="rounded-full border border-teal-300/30 bg-teal-300/10 px-4 py-1 text-sm tracking-wide text-teal-200 backdrop-blur"
          >
            Why Photoly
          </span>
          <h2 class="text-4xl font-semibold tracking-tight md:text-5xl">
            Your photos, <span class="text-teal-300">on your terms</span>
          </h2>
          <p class="text-lg text-teal-100/70">
            Need a reliable home for your pictures that you fully control? Photoly is a self-hosted
            photo manager that keeps your library tidy, searchable and ready to share.
          </p>
        </div>

        <div class="grid w-full max-w-5xl gap-4 md:grid-cols-3">
          <div
            v-for="f in features"
            :key="f.title"
            class="rounded-2xl border border-white/10 bg-white/5 p-6 backdrop-blur transition duration-300 hover:-translate-y-1 hover:border-teal-300/40 hover:bg-white/10"
          >
            <div
              class="mb-4 flex size-11 items-center justify-center rounded-xl bg-teal-400/15 text-teal-300"
            >
              <component :is="f.icon" class="size-5" />
            </div>
            <h3 class="mb-1 text-lg font-medium">{{ f.title }}</h3>
            <p class="text-sm text-teal-100/70">{{ f.text }}</p>
          </div>
        </div>

        <Button
          as="a"
          href="https://github.com/WiscWebGroup"
          variant="outline"
          size="lg"
          class="border-white/25 bg-white/5 text-white backdrop-blur hover:bg-white/15 hover:text-white"
        >
          Star us on GitHub <ExternalLink class="size-4" />
        </Button>
      </section>

      <section class="relative flex flex-col items-center gap-12 px-4 pt-8 pb-32 text-white">
        <div class="flex max-w-2xl flex-col items-center gap-4 text-center">
          <span
            class="rounded-full border border-teal-300/30 bg-teal-300/10 px-4 py-1 text-sm tracking-wide text-teal-200 backdrop-blur"
          >
            The team
          </span>
          <h2 class="text-4xl font-semibold tracking-tight md:text-5xl">
            Built by <span class="text-teal-300">people who love photos</span>
          </h2>
        </div>

        <div class="grid w-full max-w-5xl gap-4 md:grid-cols-2">
          <a
            v-for="p in team"
            :key="p.github"
            :href="`https://github.com/${p.github}`"
            class="group flex items-start gap-5 rounded-2xl border border-white/10 bg-white/5 p-6 backdrop-blur transition duration-300 hover:-translate-y-1 hover:border-teal-300/40 hover:bg-white/10"
          >
            <img
              :src="`https://avatars.githubusercontent.com/u/${p.avatar}`"
              alt=""
              class="size-20 shrink-0 rounded-full ring-2 ring-teal-300/30"
            />
            <div>
              <span class="flex items-center gap-1 text-lg font-medium">
                {{ p.name }}
                <ExternalLink
                  class="size-4 text-teal-300 opacity-0 transition group-hover:opacity-100"
                />
              </span>
              <p class="mt-1 text-sm text-teal-100/70">{{ p.bio }}</p>
            </div>
          </a>
        </div>
      </section>
    </div>
  </div>
</template>
