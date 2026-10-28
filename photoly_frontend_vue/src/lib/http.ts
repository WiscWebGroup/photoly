import axios, { type AxiosRequestConfig } from 'axios'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

export const http = axios.create({ baseURL: import.meta.env.VITE_APP_BASE_URL })

http.interceptors.request.use((config) => {
  const auth = useAuthStore()
  auth.refresh()
  if (auth.token) config.headers.set('HRD-Token', auth.token)
  return config
})

// The backend reports bad tokens as HTTP 200 + `msgCode: 403`, but uses the same code for
// "not your photo" / "not an admin". Only HTTP 401 proves the session has expired.
http.interceptors.response.use(undefined, (error) => {
  if (error.response?.status === 401) {
    useAuthStore().clear()
    router.push('/login')
  }
  return Promise.reject(error)
})

/** Backend envelope: `{ msgCode, t }`, where `msgCode` 200 means success. */
export interface Envelope<T> {
  msgCode: number
  t: T
}

/** A well-formed backend reply whose `msgCode` was not 200. */
export class ApiError extends Error {
  constructor(
    public msgCode: number,
    /** The envelope's `t`, which carries the reason for some failures. */
    public detail?: unknown,
  ) {
    super(`Request failed with msgCode ${msgCode}`)
  }
}

/** Sends the request and returns the envelope's `t`; throws `ApiError` unless `msgCode` is 200. */
export async function call<T = null>(config: AxiosRequestConfig): Promise<T> {
  const { data } = await http.request<Envelope<T>>(config)
  if (data.msgCode !== 200) throw new ApiError(data.msgCode, data.t)
  return data.t
}
