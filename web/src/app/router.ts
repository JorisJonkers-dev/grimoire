import { createRouter, createWebHistory, type RouterHistory } from 'vue-router'
import HomePage from './HomePage.vue'

export function createAppRouter(history: RouterHistory = createWebHistory()) {
  return createRouter({ history, routes: [{ path: '/', name: 'home', component: HomePage }] })
}
