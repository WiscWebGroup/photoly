import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresAdmin?: boolean
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', name: 'welcome', component: () => import('@/views/Welcome.vue') },
    { path: '/login', name: 'login', component: () => import('@/views/Login.vue') },
    { path: '/signup', name: 'signup', component: () => import('@/views/Signup.vue') },
    {
      path: '/home/:part',
      name: 'home',
      component: () => import('@/views/HomeView.vue'),
      props: true,
      meta: { requiresAuth: true },
    },
    {
      path: '/thepersonincharge/dashboard',
      name: 'thepersonincharge_dashboard',
      component: () => import('@/views/AdminDashboard.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
    {
      path: '/thepersonincharge/usermanage',
      name: 'thepersonincharge_usermanage',
      component: () => import('@/views/AdminUserManagement.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
    {
      path: '/thepersonincharge/ai',
      name: 'admin_ai',
      component: () => import('@/views/AdminAiManagement.vue'),
      meta: { requiresAuth: true, requiresAdmin: true },
    },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  auth.refresh()
  if (to.meta.requiresAuth && !auth.isLoggedIn) return { name: 'login' }
  if (to.meta.requiresAdmin) {
    const { assertAdmin } = await import('@/composables/useAdminGuard')
    const { ApiError } = await import('@/lib/http')
    try {
      await assertAdmin()
    } catch (error) {
      if (error instanceof ApiError && error.msgCode === 403) {
        const { toast } = await import('vue-sonner')
        toast.error('You are not an admin!')
        return '/home/HomePart'
      }
      // The page renders a retryable load error for network failures.
    }
  }
})

export default router
