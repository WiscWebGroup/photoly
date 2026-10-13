import type { AxiosRequestConfig } from 'axios'
import { http } from '@/lib/http'

/** Fetches `url` as a blob and saves it to the user's disk as `filename`. */
export async function download(url: string, filename: string, config: AxiosRequestConfig = {}) {
  const { data } = await http.get<Blob>(url, { ...config, responseType: 'blob' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(data)
  a.download = filename
  a.click()
  setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}
