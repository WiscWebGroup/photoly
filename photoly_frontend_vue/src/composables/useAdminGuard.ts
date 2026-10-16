import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { ApiError, call } from '@/lib/http'

/** Fails with `ApiError` unless the signed-in user is an admin. */
export async function assertAdmin() {
  if (!(await call<boolean>({ url: '/admin/isAdmin' }))) throw new ApiError(403)
}

/** Admin pages: permission failures return home; network failures remain retryable. */
export function useAdminGuard() {
  const router = useRouter()

  async function guarded<T>(task: () => Promise<T>): Promise<T | undefined> {
    try {
      return await task()
    } catch (e) {
      toast.error(e instanceof ApiError ? 'You are not an admin!' : String(e))
      if (e instanceof ApiError && e.msgCode === 403) router.push('/home/HomePart')
    }
  }

  return { guarded }
}
