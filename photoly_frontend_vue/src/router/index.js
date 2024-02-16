import { createRouter, createWebHistory } from 'vue-router'
import Welcome from '../views/Welcome.vue'
import Login from '../views/Login.vue'
import Signup from '../views/Signup.vue'
import Home from '../views/Home.vue'

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
    path: "/home",
    name: "home",
    component: Home,
   }
  ]
})

export default router
