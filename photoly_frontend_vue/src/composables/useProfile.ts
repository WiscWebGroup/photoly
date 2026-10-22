import { ref, watch } from 'vue'
import { call } from '@/lib/http'
import { useAuthStore } from '@/stores/auth'

export interface Profile {
  userId: number
  userName: string
  email: string
  createDate: string
  role: string
}

// One copy for the whole app: the shell footer and the Account page read the same profile.
const profile = ref<Profile>()
let loading: Promise<Profile> | null = null
let watching = false

/** The signed-in user's profile, fetched once per session. `reload()` refetches after an edit. */
export function useProfile() {
  const auth = useAuthStore()
  if (!watching) {
    watching = true
    watch(
      () => auth.token,
      () => {
        profile.value = undefined
        loading = null
      },
    )
  }

  function reload() {
    loading = call<Profile>({ url: '/user/getInfo/' }).then((p) => (profile.value = p))
    loading.catch(() => (loading = null))
    return loading
  }

  function load() {
    return loading ?? reload()
  }

  return { profile, load, reload }
}
