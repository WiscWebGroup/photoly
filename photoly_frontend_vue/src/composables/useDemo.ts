import { reactive } from 'vue'
import { http } from '@/lib/http'

/** Public demo-mode info from the backend (DEMO_EMAIL / DEMO_PASSWORD). */
const demo = reactive({ enabled: false, email: '', password: '' })
let loading: Promise<void> | undefined

/** Fetches the demo info once; any failure leaves demo mode off. */
export function loadDemo() {
  loading ??= (async () => {
    try {
      const { data } = await http.get('/demo', { timeout: 5_000 })
      if (data.msgCode === 200 && data.t?.enabled) Object.assign(demo, data.t)
    } catch {
      // not a demo instance, or the server is unreachable
    }
  })()
  return loading
}

/** Reactive demo info; `enabled` flips on once the fetch lands. */
export function useDemo() {
  void loadDemo()
  return demo
}
