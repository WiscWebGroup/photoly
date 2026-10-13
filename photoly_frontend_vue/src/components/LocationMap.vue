<script setup lang="ts">
import { onMounted, ref } from 'vue'
import L from 'leaflet'
import { useLeafletMap } from '@/composables/useLeafletMap'

const props = defineProps<{ lat: number | string; lon: number | string }>()

const el = ref<HTMLElement | null>(null)
const map = useLeafletMap(el, [Number(props.lat), Number(props.lon)], 13)

onMounted(() => L.marker([Number(props.lat), Number(props.lon)]).addTo(map.value!))
</script>

<template>
  <div ref="el" class="h-60 w-full overflow-hidden rounded-md" />
</template>
