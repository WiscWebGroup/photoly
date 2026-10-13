import { ref } from 'vue'

/** An inline form warning that hides itself after `ms`. */
export function useWarning(ms = 5000) {
  const warning = ref('')
  let timer: ReturnType<typeof setTimeout> | undefined

  function show(message: string) {
    warning.value = message
    clearTimeout(timer)
    timer = setTimeout(() => (warning.value = ''), ms)
  }

  return { warning, show }
}
