<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { usePreferredReducedMotion } from '@vueuse/core'
import { MorphIcon } from 'morphicons/vue'
import { Search, Sparkles } from 'lucide'
import rocket from '@/assets/welcome/rocket.webp'
import astronaut from '@/assets/welcome/astronaut.webp'
import coffee from '@/assets/welcome/coffee.webp'
import chelsea from '@/assets/welcome/chelsea.webp'

type Photo = 'rocket' | 'astronaut' | 'coffee' | 'chelsea'
const src: Record<Photo, string> = { rocket, astronaut, coffee, chelsea }

const tiles: { photo: Photo; big?: boolean; position?: string }[] = [
  { photo: 'rocket', big: true },
  { photo: 'astronaut' },
  { photo: 'coffee' },
  { photo: 'chelsea' },
  { photo: 'coffee', position: '50% 90%' },
  { photo: 'astronaut', position: '80% 50%' },
]

// A looping demo of search by meaning: type a description, then light up the photo it finds.
const queries: { text: string; photo: Photo }[] = [
  { text: 'rocket on the launch pad', photo: 'rocket' },
  { text: 'a portrait of an astronaut', photo: 'astronaut' },
  { text: 'coffee on a wooden table', photo: 'coffee' },
  { text: 'a close up of a cat', photo: 'chelsea' },
]

const reducedMotion = usePreferredReducedMotion()
const typed = ref(queries[0]!.text)
const match = ref<Photo | null>(queries[0]!.photo)
const searching = computed(() => match.value === null)

let stopped = false
const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

async function loop() {
  let i = 0
  await wait(2600)
  while (!stopped) {
    // Erase the last query, type the next, then reveal its match.
    match.value = null
    while (typed.value && !stopped) {
      typed.value = typed.value.slice(0, -1)
      await wait(18)
    }
    i = (i + 1) % queries.length
    const next = queries[i]!
    for (const ch of next.text) {
      if (stopped) return
      typed.value += ch
      await wait(45 + Math.random() * 40)
    }
    await wait(350)
    match.value = next.photo
    await wait(2600)
  }
}

onMounted(() => {
  if (reducedMotion.value !== 'reduce') loop()
})
onBeforeUnmount(() => (stopped = true))
</script>

<template>
  <div aria-hidden="true" class="relative h-80 sm:h-[440px]">
    <div
      class="absolute inset-0 grid grid-cols-3 grid-rows-3 gap-1.5 rounded-[18px] border border-white/10 bg-white/5 p-3 sm:left-10"
    >
      <div
        v-for="(tile, i) in tiles"
        :key="i"
        :style="{ animationDelay: `${200 + i * 70}ms` }"
        class="animate-tile overflow-hidden rounded-lg"
        :class="tile.big && 'col-span-2 row-span-2'"
      >
        <img
          :src="src[tile.photo]"
          alt=""
          :style="tile.position ? { objectPosition: tile.position } : undefined"
          class="size-full object-cover transition-[filter,transform,opacity] duration-500 ease-(--ease-out-quart)"
          :class="
            match === null
              ? 'brightness-90'
              : match === tile.photo
                ? 'scale-[1.04]'
                : 'brightness-40 saturate-50'
          "
        />
      </div>
    </div>

    <div
      style="animation-delay: 700ms"
      class="animate-rise absolute bottom-3 left-3 right-3 sm:right-auto sm:bottom-9 sm:left-0 sm:w-[300px]"
    >
      <div
        class="animate-float flex items-center gap-2.5 rounded-xl border border-white/12 bg-[#031417]/85 px-3.5 py-3 shadow-[0_12px_32px_rgba(0,0,0,.4)] backdrop-blur-md"
      >
        <MorphIcon
          :icon="searching ? Search : Sparkles"
          :size="18"
          color="#46ecd5"
          spring="bouncy"
          class="shrink-0"
        />
        <span class="min-w-0 truncate text-white"
          >{{ typed
          }}<span
            class="ml-px inline-block h-4 w-px translate-y-0.5 bg-teal-300 transition-opacity"
            :class="searching ? 'animate-pulse opacity-100' : 'opacity-0'"
        /></span>
        <span
          class="ml-auto shrink-0 text-xs text-teal-200 transition-opacity duration-300"
          :class="searching ? 'opacity-0' : 'opacity-100'"
          >Top match</span
        >
      </div>
    </div>
  </div>
</template>
