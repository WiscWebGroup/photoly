import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
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
      meta: { requiresAuth: true },
    },
    {
      path: '/thepersonincharge/usermanage',
      name: 'thepersonincharge_usermanage',
      component: () => import('@/views/AdminUserManagement.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  auth.refresh()
  if (to.meta.requiresAuth && !auth.isLoggedIn) return { name: 'login' }
})

export default router
