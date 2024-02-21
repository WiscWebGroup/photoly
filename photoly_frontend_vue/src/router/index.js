import { createRouter, createWebHistory } from 'vue-router'
import Welcome from '../views/Welcome.vue'
import Login from '../views/Login.vue'
import Signup from '../views/Signup.vue'
import Home from '../views/Home.vue'
import MoveToShowFolderVue from '@/components/MoveToShowFolder.vue'
import AdminDashboard from '@/views/AdminDashboard.vue'
import AdminUserManagement from '@/views/AdminUserManagement.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: "/",
      name: "welcome",
      component: Welcome,
   },
   {
    path: "/login",
    name: "login",
    component: Login,
   },
   {
    path: "/signup",
    name: "signup",
    component: Signup,
   },
   {
    path: "/home/:part",
    name: "home",
    component: Home,
    props: true
   },
   {
    path: "/test",
    name: "test",
    component: MoveToShowFolderVue,
   },
   {
    path: "/thepersonincharge/dashboard",
    name: "thepersonincharge_dashboard",
    component: AdminDashboard,
   },
   {
    path: "/thepersonincharge/usermanage",
    name: "thepersonincharge_usermanage",
    component: AdminUserManagement,
   }
  ]
})

export default router
