<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { ExternalLink } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import hero from '@/assets/JH_20211030_5406.jpg'
import summary from '@/assets/JH_20220710_5856.jpg'
import about from '@/assets/JH_20220601_5686.jpg'

// Loaded lazily so the WebGPU runtime never blocks first paint of the landing page.
const ShapeWaves = defineAsyncComponent(() => import('@/components/ShapeWaves.vue'))

const team = [
  {
    name: 'WiscWebGroup',
    github: 'WiscWebGroup',
    avatar: 108884969,
    bio: 'We are WiscWebGroup, a team located in Madison, Wisconsin. We mainly develop interesting web applications.',
  },
  {
    name: 'Yixuan Ye (Harold-y)',
    github: 'Harold-y',
    avatar: 68500948,
    bio: 'BS from UW-Madison. I did the DB, backend, CI/CD, part of the frontend work, and also designed the logo.',
  },
  {
    name: 'Jeffrey Hui (Slijeff)',
    github: 'Slijeff',
    avatar: 31459252,
    bio: 'BS from UW-Madison. I did part of the frontend job in this project. I photographed the background images for the home page.',
  },
  {
    name: 'Zihan Zhao (NataRich)',
    github: 'NataRich',
    avatar: 46753484,
    bio: 'Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.',
  },
]

const section = 'flex min-h-dvh flex-col items-center gap-8 bg-cover bg-fixed bg-center px-4 py-24'
const panel = 'w-full max-w-3xl rounded-3xl bg-white/80 px-8 py-5 text-[#1d4044]/80 backdrop-blur'
const bg = (url: string) => ({ backgroundImage: `url(${url})` })
</script>

<template>
  <div>
    <header
      class="fixed inset-x-0 top-0 z-10 flex items-center justify-between bg-black/20 px-4 py-2 text-white backdrop-blur"
    >
      <div class="flex items-center gap-3">
        <img src="@/assets/logo.ico" alt="" class="size-10" />
        <span class="text-lg font-medium">PHOTOLY</span>
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

    <section
      class="relative flex min-h-dvh flex-col items-center justify-center gap-4 overflow-hidden bg-[#031417] px-4 text-center text-white"
    >
      <ShapeWaves
        class="absolute inset-0"
        shapes="mixed"
        :cell-size="14"
        :dot-size="0.7"
        color="#1f7f86"
        hover-color="#99f6e4"
        background-color="#031417"
        :speed="0.8"
        :scale="1.2"
        :contrast="1.2"
        :brightness="0.45"
        :fade="0.35"
        :glow="0.5"
        :splash-radius="60"
      />
      <!-- Keep the copy legible over the busiest part of the field, and melt into the next section. -->
      <div
        class="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,#031417b3_0%,#03141780_30%,transparent_70%)]"
      />
      <div
        class="pointer-events-none absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-black/60 to-transparent"
      />

      <div class="relative flex flex-col items-center gap-4">
        <span
          class="rounded-full border border-teal-300/30 bg-teal-300/10 px-4 py-1 text-sm tracking-wide text-teal-200 backdrop-blur"
        >
          Self-hosted photo management
        </span>
        <h1 class="max-w-3xl text-5xl leading-tight font-semibold tracking-tight md:text-7xl">
          See the Value of Photo Inside
        </h1>
        <p
          class="bg-gradient-to-r from-teal-200 via-white to-teal-200 bg-clip-text text-3xl text-transparent md:text-4xl"
        >
          The Beauty Shines Here
        </p>
        <p class="text-lg text-teal-100/70 md:text-xl">Photoly - Share Your Inspiration</p>
        <div class="mt-6 flex gap-4">
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

    <section :class="section" :style="bg(summary)">
      <h2 class="text-4xl font-semibold text-white">Project Summary</h2>
      <p :class="panel">
        There are demanding for self-host and reliable picture host application. Photoly provides a
        solution just right for that. Using Photoly, you can manage your pictures and photos with
        easy clicks, categorize them using tags, galleries, and namespaces. Moreover, Photoly
        provides API for you to access our functionalities for embedding in other applications.
      </p>
      <a
        :class="[panel, 'flex items-center justify-center gap-1']"
        href="https://github.com/WiscWebGroup"
      >
        Star our GitHub Repo! <ExternalLink class="size-4" />
      </a>
    </section>

    <section :class="section" :style="bg(about)">
      <h2 class="text-4xl font-semibold text-white">About Us</h2>
      <div
        v-for="p in team"
        :key="p.github"
        :class="[panel, 'flex flex-col items-center gap-6 sm:flex-row']"
      >
        <img
          :src="`https://avatars.githubusercontent.com/u/${p.avatar}`"
          alt=""
          class="size-32 rounded-full"
        />
        <div>
          <a
            :href="`https://github.com/${p.github}`"
            class="flex items-center gap-1 text-xl text-black"
          >
            {{ p.name }} <ExternalLink class="size-4" />
          </a>
          <p class="mt-2 text-black/70">{{ p.bio }}</p>
        </div>
      </div>
    </section>
  </div>
</template>
