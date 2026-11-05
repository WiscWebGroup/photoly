<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const revision = import.meta.env.VITE_PHOTOLY_REVISION || 'development'
function sourceLink(value: string | undefined): string | null {
  if (!value) return null
  try {
    const u = new URL(value)
    return ['https:', 'http:'].includes(u.protocol) ? u.href : null
  } catch {
    return null
  }
}
const frontendSource = sourceLink(import.meta.env.VITE_PHOTOLY_SOURCE_URL)
const apiBase = (import.meta.env.VITE_APP_BASE_URL || '/api').replace(/\/$/, '')
const backend = ref<{ revision: string; sourceURL: string; license: string } | null>(null)
const unavailable = ref(false)
onMounted(async () => {
  try {
    const response = await fetch(`${apiBase}/legal`)
    if (!response.ok) throw new Error('unavailable')
    const data = await response.json()
    if (typeof data.revision !== 'string' || typeof data.sourceURL !== 'string')
      throw new Error('invalid')
    backend.value = data
  } catch {
    unavailable.value = true
  }
})
const models = [
  {
    name: 'SigLIP 2',
    license: 'Apache-2.0 (base model)',
    url: '/models/ba1f3b0843f2/siglip2-base-patch16-224/LICENSE',
    note: '/models/ba1f3b0843f2/siglip2-base-patch16-224/NOTICE',
  },
  { name: 'YuNet', license: 'MIT', url: '/models/people/yunet2023-sface2021-rgb-v1/LICENSE.YuNet' },
  {
    name: 'SFace',
    license: 'Apache-2.0',
    url: '/models/people/yunet2023-sface2021-rgb-v1/LICENSE.SFace',
  },
]
</script>

<template>
  <main class="bg-background text-foreground min-h-dvh px-5 py-8 sm:px-8 sm:py-12">
    <div class="mx-auto flex max-w-3xl flex-col gap-8">
      <RouterLink
        :to="auth.isLoggedIn ? '/home/HomePart' : '/login'"
        class="text-muted-foreground w-fit text-sm underline underline-offset-4"
        >Back to Photoly</RouterLink
      >
      <header class="space-y-3">
        <h1 class="text-3xl font-semibold tracking-tight">Licenses &amp; source</h1>
        <p class="text-muted-foreground leading-relaxed">
          Photoly is free software under AGPL-3.0-or-later. Third-party software, models and data
          retain their own licenses.
        </p>
        <p class="text-muted-foreground text-sm">
          Provided without warranty, including warranties of merchantability or fitness for a
          particular purpose. See the full license for your rights and its terms.
        </p>
        <a href="/legal/AGPL-3.0.txt" class="underline underline-offset-4"
          >Read the Photoly license</a
        >
      </header>
      <section class="space-y-3" aria-labelledby="source-heading">
        <h2 id="source-heading" class="text-xl font-semibold">Source for this version</h2>
        <div class="bg-card space-y-3 rounded-xl border p-5 text-sm break-words">
          <p>
            Frontend: <code>{{ revision }}</code>
          </p>
          <a
            v-if="frontendSource"
            :href="frontendSource"
            class="inline-block underline underline-offset-4"
            >Download frontend source</a
          >
          <p v-else class="text-muted-foreground">
            This frontend build has no source URL configured. Ask the operator for its corresponding
            source.
          </p>
          <template v-if="backend">
            <p>
              Backend: <code>{{ backend.revision }}</code>
            </p>
            <a
              v-if="sourceLink(backend.sourceURL)"
              :href="sourceLink(backend.sourceURL)!"
              class="inline-block underline underline-offset-4"
              >Download backend source</a
            >
            <p v-else class="text-muted-foreground">
              This backend build has no source URL configured. Ask the operator for its
              corresponding source.
            </p>
          </template>
          <p v-else class="text-muted-foreground">
            {{
              unavailable ? 'Backend version is currently unavailable.' : 'Loading backend version…'
            }}
          </p>
        </div>
        <p class="text-muted-foreground text-sm">
          You may modify and redistribute Photoly under its license. Operators of modified network
          deployments must offer their users the corresponding source of the running version.
        </p>
      </section>
      <section class="space-y-3" aria-labelledby="notices-heading">
        <h2 id="notices-heading" class="text-xl font-semibold">Third-party software</h2>
        <ul
          class="list-inside list-disc space-y-3 text-sm [&_a]:underline [&_a]:underline-offset-4"
        >
          <li>
            <a href="/legal/third-party-notices.txt">Frontend and build-tool notices</a> ·
            <a href="/legal/dependencies.json">Dependency inventory</a>
          </li>
          <li>
            <a :href="`${apiBase}/legal/third-party-notices.txt`">Go dependency notices</a> ·
            <a :href="`${apiBase}/legal/dependencies.json`">Go inventory</a>
          </li>
          <li>
            <a :href="`${apiBase}/legal/GPL-3.0.txt`">GPLv3</a> accompanies the LGPLv3 library
            cloudsoda/sddl used by the backend. Source and build instructions permit rebuilding with
            a modified library.
          </li>
          <li><a href="/map-licenses.txt">Map component notices</a></li>
          <li><a href="/player-licenses.txt">Video player notices</a></li>
        </ul>
        <p class="text-muted-foreground text-sm">
          Container notices and exact system-package source records accompany distributed images.
          The dependency inventory distinguishes collected notices from entries still requiring
          review; it is not a blanket legal certification.
        </p>
      </section>
      <section class="space-y-3" aria-labelledby="models-heading">
        <h2 id="models-heading" class="text-xl font-semibold">AI models</h2>
        <ul class="space-y-3 text-sm">
          <li v-for="model in models" :key="model.name">
            <a :href="model.url" class="underline underline-offset-4"
              >{{ model.name }} — {{ model.license }}</a
            >
            <span v-if="model.note">
              ·
              <a :href="model.note" class="underline underline-offset-4"
                >ONNX conversion provenance</a
              ></span
            >
          </li>
        </ul>
        <p class="text-muted-foreground text-sm">
          Model notices are available without downloading weights. The SigLIP 2 conversion
          identifies the Google base model but does not declare a separate conversion license.
        </p>
      </section>
      <section class="space-y-3 text-sm" aria-labelledby="data-heading">
        <h2 id="data-heading" class="text-xl font-semibold">Geographic data</h2>
        <p>
          Approximate city names use
          <a href="https://www.geonames.org/" class="underline">GeoNames</a>, licensed under
          <a href="https://creativecommons.org/licenses/by/4.0/" class="underline">CC BY 4.0</a>.
          Photoly converts the cities15000 snapshot of October 16, 2026 into an indexed city
          database, retaining selected fields and omitting alternate names.
        </p>
        <p>
          Maps credit
          <a href="https://www.openstreetmap.org/copyright" class="underline"
            >© OpenStreetMap contributors</a
          >. OpenStreetMap data is available under the ODbL. Attribution also remains visible on
          each map.
        </p>
      </section>
    </div>
  </main>
</template>
