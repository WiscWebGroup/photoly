import { useStorage } from '@vueuse/core'

export const mapDayNight = useStorage<boolean>('photoly-map-day-night', true, undefined, {
  serializer: {
    read: (value) => value !== 'false',
    write: (value) => String(value),
  },
})
