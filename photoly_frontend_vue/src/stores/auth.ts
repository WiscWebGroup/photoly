import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { http } from '@/lib/http'

// Raw-string token under the key the backend/API client expects.
const KEY = 'HRD-Token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(KEY))
  const isLoggedIn = computed(() => token.value !== null)
  const avatarUrl = computed(() => `${import.meta.env.VITE_APP_BASE_URL}/user/getAvatar/${token.value}`)

  /** Re-reads storage, picking up a token changed elsewhere (e.g. another tab). */
  function refresh() {
    token.value = localStorage.getItem(KEY)
  }

  function setToken(value: string) {
    localStorage.setItem(KEY, value)
    token.value = value
  }

  function clear() {
    localStorage.removeItem(KEY)
    token.value = null
  }

  /** Ends the session on the server (best effort) and always forgets the token locally. */
  async function signOut() {
    try {
      await http.post('/user/signOut/')
    } finally {
      clear()
    }
  }

  return { token, isLoggedIn, avatarUrl, refresh, setToken, clear, signOut }
})
